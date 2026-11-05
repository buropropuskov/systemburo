package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

type notificationIntentApplication struct {
	ID                int
	ApplicationNumber *string
	OrganizationID    *int
	CompanyID         *int
	SenderUserID      *int
	Message           *string
}

func notificationIntentAudience(ctx context.Context, db *gorm.DB, applicationID, actorID int) ([]int, error) {
	groups := make([][]int, 0, 3)
	for _, table := range []string{"application_responsible_users", "application_viewers", "application_approvers"} {
		query := db.WithContext(ctx).Table(table).Distinct("user_id")
		if table != "application_approvers" {
			query = query.Where("application_id = ?", applicationID)
		}
		var ids []int
		if err := query.Pluck("user_id", &ids).Error; err != nil {
			return nil, err
		}
		groups = append(groups, ids)
	}
	ids := mergeUniqueIDs(actorID, groups...)
	sort.Ints(ids)
	return ids, nil
}

func notificationDraft(userID int, kind, title, message string, data *string) NotificationDraft {
	return NotificationDraft{UserID: userID, Type: kind, Title: title, Message: message, Data: data, GroupKey: autoNotificationGroupKey(kind, data)}
}

func (w *NotificationIntentWorker) buildIntentDrafts(ctx context.Context, tx *gorm.DB, intent models.NotificationIntent, app notificationIntentApplication, rights map[int]bool) ([]NotificationDraft, error) {
	switch intent.Kind {
	case notificationIntentSubmit:
		return w.buildSubmitNotificationDrafts(ctx, tx, intent, app, rights)
	case notificationIntentSupplement:
		return buildSupplementNotificationDrafts(ctx, tx, intent, app)
	default:
		return nil, fmt.Errorf("unsupported notification intent kind")
	}
}

// Every DB read here uses the materialization transaction. No request payload or
// personal snapshot is copied to the durable intent; texts use committed data.
func (w *NotificationIntentWorker) buildSubmitNotificationDrafts(ctx context.Context, tx *gorm.DB, intent models.NotificationIntent, app notificationIntentApplication, rights map[int]bool) ([]NotificationDraft, error) {
	consentMasks, err := loadConsentMasks(ctx, tx)
	if err != nil {
		return nil, err
	}
	number := optionalString(app.ApplicationNumber)
	organization, err := notificationDirectoryName(ctx, tx, "organizations", app.OrganizationID)
	if err != nil {
		return nil, err
	}
	if organization == "" {
		organization, err = notificationDirectoryName(ctx, tx, "companies", app.CompanyID)
		if err != nil {
			return nil, err
		}
	}
	sender, masked := consentMasks[intent.ActorUserID]
	if !masked {
		var author struct {
			Username                        string
			LastName, FirstName, MiddleName *string
		}
		err = tx.WithContext(ctx).Table("users").Select("username,last_name,first_name,middle_name").Where("id = ?", intent.ActorUserID).Take(&author).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		sender = formatFullName(author.LastName, author.FirstName, author.MiddleName)
		if strings.TrimSpace(sender) == "" {
			sender = author.Username
		}
	}
	payload := map[string]any{"application_id": app.ID, "application_number": number}
	if organization != "" {
		payload["organization"] = organization
	}
	if sender != "" {
		payload["sender_name"] = sender
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data := string(encoded)
	drafts := []NotificationDraft{notificationDraft(intent.ActorUserID, NotificationTypeApplicationCreated, "Заявка отправлена", fmt.Sprintf("Ваша заявка %s отправлена и ожидает согласования.", number), &data)}
	var responsible []int
	if err := tx.WithContext(ctx).Table("application_responsible_users").Where("application_id = ? AND required_approval = true", app.ID).Order("user_id").Pluck("user_id", &responsible).Error; err != nil {
		return nil, err
	}
	for _, id := range responsible {
		if id != intent.ActorUserID {
			drafts = append(drafts, notificationDraft(id, NotificationTypeApplicationApprovalRequired, "Требуется согласование", fmt.Sprintf("Поступила новая заявка %s на согласование.", number), &data))
		}
	}
	var approvers []int
	if err := tx.WithContext(ctx).Table("application_approvers").Where("user_id <> ?", intent.ActorUserID).Order("user_id").Pluck("user_id", &approvers).Error; err != nil {
		return nil, err
	}
	var fileNames []string
	if err := tx.WithContext(ctx).Table("application_files").Where("application_id = ?", app.ID).Order("id").Pluck("file_name", &fileNames).Error; err != nil {
		return nil, err
	}
	note := pendingAcceptanceNote{number: number, organization: organization, sender: sender, messageText: optionalString(app.Message), fileNames: fileNames}
	for _, id := range approvers {
		drafts = append(drafts, notificationDraft(id, NotificationTypeApplicationPendingAcceptance, "Новая заявка", note.message(), &data))
	}
	notices, err := notificationPendingDirectories(ctx, tx, intent, app)
	if err != nil {
		return nil, err
	}
	if len(notices) == 0 {
		return drafts, nil
	}
	audience, err := notificationIntentAudience(ctx, tx, app.ID, intent.ActorUserID)
	if err != nil {
		return nil, err
	}
	directoryPayload, err := json.Marshal(map[string]any{"application_id": app.ID, "application_number": number})
	if err != nil {
		return nil, err
	}
	directoryData := string(directoryPayload)
	for _, id := range audience {
		if id == intent.ActorUserID {
			continue
		}
		allowed, known := rights[id]
		if !known {
			return nil, fmt.Errorf("notification moderation audience changed")
		}
		if !allowed {
			continue
		}
		for _, notice := range notices {
			title := "Новая " + strings.ToLower(notice.label) + " на проверке"
			message := fmt.Sprintf("В заявке %s указана «%s». Проверьте наименование и разберите запись.", number, notice.name)
			drafts = append(drafts, notificationDraft(id, NotificationTypeDirectoryPending, title, message, &directoryData))
		}
	}
	return drafts, nil
}

func notificationDirectoryName(ctx context.Context, tx *gorm.DB, table string, id *int) (string, error) {
	if id == nil || *id <= 0 {
		return "", nil
	}
	var row struct{ Name string }
	err := tx.WithContext(ctx).Table(table).Select("name").Where("id = ?", *id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return row.Name, err
}

func notificationPendingDirectories(ctx context.Context, tx *gorm.DB, intent models.NotificationIntent, app notificationIntentApplication) ([]pendingDirectoryNotice, error) {
	var notices []pendingDirectoryNotice
	for _, item := range []struct {
		table, label     string
		pending, current *int
	}{
		{"organizations", "Организация", intent.PendingOrganizationID, app.OrganizationID},
		{"companies", "Компания", intent.PendingCompanyID, app.CompanyID},
	} {
		if item.pending == nil || item.current == nil || *item.pending != *item.current {
			continue
		}
		var row struct {
			Name             string
			ModerationStatus string
		}
		err := tx.WithContext(ctx).Table(item.table).Select("name,moderation_status").Where("id = ?", *item.pending).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if row.ModerationStatus == models.ModerationPending {
			notices = append(notices, pendingDirectoryNotice{label: item.label, name: row.Name})
		}
	}
	return notices, nil
}

func buildSupplementNotificationDrafts(ctx context.Context, tx *gorm.DB, intent models.NotificationIntent, app notificationIntentApplication) ([]NotificationDraft, error) {
	if intent.SupplementID == nil {
		return nil, fmt.Errorf("notification supplement identity is required")
	}
	var supplement struct{ ID, Number int }
	err := tx.WithContext(ctx).Table("application_supplements").Select("id,number").Where("id = ? AND application_id = ?", *intent.SupplementID, app.ID).Take(&supplement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	table, key, id := "application_responsible_users", "application_id", app.ID
	if intent.InWork {
		table, key, id = "application_supplement_approvals", "supplement_id", supplement.ID
	}
	var recipients []int
	if err := tx.WithContext(ctx).Table(table).Where(key+" = ? AND required_approval = true", id).Order("user_id").Pluck("user_id", &recipients).Error; err != nil {
		return nil, err
	}
	number := optionalString(app.ApplicationNumber)
	encoded, err := json.Marshal(map[string]any{"application_id": app.ID, "application_number": number, "supplement_number": supplement.Number})
	if err != nil {
		return nil, err
	}
	data := string(encoded)
	message := fmt.Sprintf("В заявку %s добавлены новые строки - требуется согласование.", number)
	if intent.InWork {
		message = fmt.Sprintf("По заявке %s подано дополнение №%d - требуется согласование.", number, supplement.Number)
	}
	var drafts []NotificationDraft
	for _, id := range recipients {
		if id != intent.ActorUserID {
			drafts = append(drafts, notificationDraft(id, NotificationTypeApplicationApprovalRequired, "Требуется согласование", message, &data))
		}
	}
	return drafts, nil
}
