package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"systemburo/internal/database"
	"systemburo/internal/models"
)

const applicationChangesAction = "assignments_changed"

func (s *notificationService) GetApplicationChanges(ctx context.Context, userID, applicationID int) ([]models.Notification, error) {
	rows := make([]models.Notification, 0)
	err := s.db.WithContext(ctx).Model(&models.Notification{}).Select(notificationMetadataColumns).
		Where("user_id = ? AND projection_ref->>'application_id' = ?", userID, fmt.Sprint(applicationID)).
		Where(database.PendingApplicationChangeNotice).Order("id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return s.projectNotifications(ctx, userID, rows)
}

// Historical facts contain technical references only. Names are resolved on read.
type applicationFieldChange struct {
	Kind     string `json:"kind"`
	EntityID int    `json:"entity_id"`
	Field    string `json:"field"`
	Before   []int  `json:"before"`
	After    []int  `json:"after"`
}

type applicationChangeEvent struct {
	Key     string                   `json:"event_key"`
	Changes []applicationFieldChange `json:"changes"`
}

func (s *applicationService) recordApplicationChanges(ctx context.Context, tx *gorm.DB, appID, actorID int, changes []applicationFieldChange) ([]NotificationDispatch, error) {
	if len(changes) == 0 {
		return nil, nil
	}
	var key [16]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	event := applicationChangeEvent{Key: hex.EncodeToString(key[:]), Changes: changes}
	if err := s.recorder.Record(ctx, tx, models.AuditEntityApplication, &appID, applicationChangesAction, &actorID, event); err != nil {
		return nil, err
	}
	if s.notificationService == nil {
		return nil, nil
	}
	notifications, ok := s.notificationService.(*notificationService)
	if !ok {
		return nil, fmt.Errorf("transactional notifications unavailable")
	}
	var sender struct{ SenderUserID *int }
	if err := tx.WithContext(ctx).Table("applications").Select("sender_user_id").Where("id = ?", appID).Take(&sender).Error; err != nil {
		return nil, err
	}
	if sender.SenderUserID == nil || *sender.SenderUserID == actorID {
		return nil, nil
	}
	ref := &notificationReference{Version: 1, Type: NotificationTypeApplicationElementsChanged, ApplicationID: appID, ActorID: actorID, EventKey: event.Key}
	return notifications.PersistNotificationBatchTx(ctx, tx, []NotificationDraft{{UserID: *sender.SenderUserID, Type: ref.Type, Title: "Изменена заявка", reference: ref}})
}

func (s *applicationService) notifyApplicationChanges(ctx context.Context, effects []NotificationDispatch) {
	if notifications, ok := s.notificationService.(*notificationService); ok && notifications != nil {
		notifications.DispatchCommitted(ctx, effects)
	}
}

func (s *notificationService) applicationChangeEvent(ctx context.Context, ref notificationReference) (applicationChangeEvent, error) {
	var row models.AuditLog
	err := s.db.WithContext(ctx).Where("entity_type = ? AND entity_id = ? AND action = ? AND details->>'event_key' = ?", models.AuditEntityApplication, ref.ApplicationID, applicationChangesAction, ref.EventKey).Take(&row).Error
	if err != nil {
		return applicationChangeEvent{}, err
	}
	var event applicationChangeEvent
	if err := json.Unmarshal(row.Details, &event); err != nil {
		return event, err
	}
	if event.Key != ref.EventKey || row.ActorUserID == nil || *row.ActorUserID != ref.ActorID {
		return event, fmt.Errorf("invalid change event")
	}
	return event, nil
}

func (s *notificationService) projectApplicationChanges(ctx context.Context, p *notificationProjectionPolicy, ref notificationReference, data map[string]any) (string, error) {
	// A forwarded notification must never reveal another sender's change history.
	var sender struct{ SenderUserID *int }
	if err := s.db.WithContext(ctx).Table("applications").Select("sender_user_id").Where("id = ?", ref.ApplicationID).Scan(&sender).Error; err != nil {
		return "", err
	}
	if sender.SenderUserID == nil || *sender.SenderUserID != p.viewerID {
		return "", nil
	}
	event, err := s.applicationChangeEvent(ctx, ref)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	entities, catalogs, err := s.loadChangeProjection(ctx, ref.ApplicationID, event.Changes)
	if err != nil {
		return "", err
	}
	changes := make([]map[string]any, 0, len(event.Changes))
	lines := []string{fmt.Sprintf("Принимающий изменил вашу заявку %s.", p.batch.applications[ref.ApplicationID].Number)}
	for index, change := range event.Changes {
		if change.Kind != "cars" && change.Kind != "employees" {
			continue
		}
		entity, exists := entities[change.Kind][change.EntityID]
		if !exists {
			continue
		}
		before, after, label := "В заявке", "Убран из заявки", "Состав заявки"
		if change.Field == "places" || change.Field == "tables" {
			table := "unload_places"
			label = "Места разгрузки"
			if change.Field == "tables" {
				table = "system_tables"
				label = "Посты проезда"
				if change.Kind == "employees" {
					label = "Посты прохода"
				}
			}
			resolve := func(ids []int) string {
				if len(ids) == 0 {
					return "Не указаны"
				}
				names := make([]string, 0, len(ids))
				for _, id := range ids {
					if name, ok := catalogs[table][id]; ok {
						names = append(names, name)
					} else {
						names = append(names, "Недоступное место")
					}
				}
				return strings.Join(names, ", ")
			}
			before, after = resolve(change.Before), resolve(change.After)
		} else if change.Field != "removed" {
			continue
		}
		ack := false
		for _, value := range ref.Facts.AcknowledgedChanges {
			if value == index {
				ack = true
				break
			}
		}
		changes = append(changes, map[string]any{"index": index, "entity_id": entity.ID, "entity_kind": change.Kind, "attachment_id": entity.AttachmentID, "field": change.Field, "label": label, "entity_label": entity.Label, "before": before, "after": after, "acknowledged": ack})
		lines = append(lines, fmt.Sprintf("%s — %s: %s → %s.", entity.Label, label, before, after))
	}
	if len(changes) == 0 {
		return "", nil
	}
	data["changes"] = changes
	return strings.Join(lines, "\n"), nil
}

// The acknowledgement is independent of bell read state and survives reloads.
// Lock the notification so two tabs acknowledging different fields cannot lose data.
func (s *notificationService) acknowledgeApplicationChange(ctx context.Context, userID, id, index int) (*models.Notification, error) {
	var result models.Notification
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n models.Notification
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", id, userID).Take(&n).Error; err != nil {
			return err
		}
		ref, valid := decodeNotificationReference(n.ProjectionRef, n.Type)
		if !valid || ref.Type != NotificationTypeApplicationElementsChanged {
			return echo.NewHTTPError(400, "Это уведомление не содержит изменений заявки")
		}
		projector := *s
		projector.db = tx
		projected, err := projector.projectNotifications(ctx, userID, []models.Notification{n})
		if err != nil {
			return err
		}
		allowed, allAcknowledged := false, true
		if projected[0].Data != nil {
			var payload struct {
				Changes []struct {
					Index        int  `json:"index"`
					Acknowledged bool `json:"acknowledged"`
				} `json:"changes"`
			}
			if json.Unmarshal([]byte(*projected[0].Data), &payload) == nil {
				for _, change := range payload.Changes {
					if change.Index == index {
						allowed = true
					} else if !change.Acknowledged {
						allAcknowledged = false
					}
				}
			}
		}
		if !allowed {
			return echo.NewHTTPError(404, "Изменение недоступно")
		}
		if !slices.Contains(ref.Facts.AcknowledgedChanges, index) {
			ref.Facts.AcknowledgedChanges = append(ref.Facts.AcknowledgedChanges, index)
		}
		ref.Facts.AllChangesAcknowledged = allAcknowledged
		encoded := encodeNotificationReference(&ref)
		if encoded == nil {
			return echo.NewHTTPError(400, "Некорректное подтверждение")
		}
		if err := tx.Model(&n).Update("projection_ref", encoded).Error; err != nil {
			return err
		}
		n.ProjectionRef = encoded
		rows, err := projector.projectNotifications(ctx, userID, []models.Notification{n})
		if err != nil {
			return err
		}
		result = rows[0]
		return nil
	})
	if err == gorm.ErrRecordNotFound {
		return nil, echo.NewHTTPError(404, "Уведомление не найдено")
	}
	return &result, err
}
