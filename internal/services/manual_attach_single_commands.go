package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"systemburo/internal/models"
)

// SingleManualAttachCommandService never adopts, deletes, or rewrites the source
// attachment. The old whole-attachment command remains a separate API.
type SingleManualAttachCommandService struct {
	db          *gorm.DB
	recorder    AuditRecorder
	tables      *TablesRefreshPublisher
	available   *AvailableRefreshPublisher
	afterAttach func(context.Context, int)
}

func NewSingleManualAttachCommandService(db *gorm.DB, recorder AuditRecorder, tables *TablesRefreshPublisher, available *AvailableRefreshPublisher) *SingleManualAttachCommandService {
	return &SingleManualAttachCommandService{db: db, recorder: recorder, tables: tables, available: available}
}

// SetAfterAttach configures application/cache invalidation before serving requests.
// It runs only after commit; it must not be replaced concurrently with requests.
func (s *SingleManualAttachCommandService) SetAfterAttach(fn func(context.Context, int)) {
	s.afterAttach = fn
}

type SingleManualAttachRequest struct {
	ApplicationID      *int               `json:"application_id"`
	TargetAttachmentID *int               `json:"target_attachment_id"`
	PeriodChoice       string             `json:"period_choice,omitempty"`
	SourceAttachmentID *int               `json:"source_attachment_id,omitempty"`
	Period             *EntityPeriodInput `json:"period,omitempty"`
	TableID            *int               `json:"table_id,omitempty"`
	Reason             string             `json:"reason"`
	ExpectedRevision   string             `json:"expected_revision,omitempty"`
}

type SingleManualAttachContext struct {
	Flags                *SingleManualAttachCarFlags `json:"flags,omitempty"`
	EntityID             int                         `json:"entity_id"`
	EntityKind           ElementKind                 `json:"entity_kind"`
	AttachmentID         int                         `json:"attachment_id"`
	IsManual             bool                        `json:"is_manual"`
	ApplicationID        *int                        `json:"application_id"`
	ParentPeriod         models.EntryPeriod          `json:"parent_period"`
	PeriodMode           models.PeriodMode           `json:"period_mode"`
	EffectivePeriod      models.EffectivePeriod      `json:"effective_period"`
	RequiresPeriodChoice bool                        `json:"requires_period_choice"`
	CanAssignPeriod      bool                        `json:"can_assign_period"`
}

type SingleManualAttachResult struct {
	CurrentFlags            *SingleManualAttachCarFlags `json:"current_flags,omitempty"`
	NewFlags                *SingleManualAttachCarFlags `json:"new_flags,omitempty"`
	EntityID                int                         `json:"entity_id"`
	EntityKind              ElementKind                 `json:"entity_kind"`
	OldAttachmentID         int                         `json:"old_attachment_id"`
	ApplicationID           int                         `json:"application_id"`
	DestinationAttachmentID *int                        `json:"destination_attachment_id"`
	DestinationMode         string                      `json:"destination_mode"`
	CurrentEffective        models.EffectivePeriod      `json:"current_effective"`
	NewEffective            models.EffectivePeriod      `json:"new_effective"`
	CurrentMode             models.PeriodMode           `json:"current_mode"`
	NewMode                 models.PeriodMode           `json:"new_mode"`
	NeedsPeriodChoice       bool                        `json:"needs_period_choice"`
	Revision                string                      `json:"revision"`
}

type SingleManualAttachCarFlags struct {
	IndividualRoofAccess  bool `json:"individual_roof_access"`
	IndividualFreeParking bool `json:"individual_free_parking"`
	RoofAccess            bool `json:"roof_access"`
	FreeParking           bool `json:"free_parking"`
}

func singleAttachCarFlags(own SingleManualAttachCarFlags, parent models.Attachment) SingleManualAttachCarFlags {
	own.RoofAccess = own.IndividualRoofAccess || parent.RoofAccess
	own.FreeParking = own.IndividualFreeParking || parent.FreeParking
	return own
}

// SingleManualAttachAttachment is intentionally metadata only: the general
// attachment-detail endpoint and its participant permissions are not bypassed.
type SingleManualAttachAttachment struct {
	ID                    int     `json:"id"`
	ApplicationID         int     `json:"application_id"`
	Status                int     `json:"status"`
	IsManual              bool    `json:"is_manual"`
	AttachmentType        string  `json:"attachment_type"`
	AttachmentName        *string `json:"attachment_name"`
	AttachmentDisplayName *string `json:"attachment_display_name"`
	EntryDateFrom         *string `json:"entry_date_from"`
	EntryDateTo           *string `json:"entry_date_to"`
	EntryTimeFrom         *string `json:"entry_time_from"`
	EntryTimeTo           *string `json:"entry_time_to"`
}

// Attachments supplies the app/target/source selectors for an editable manual
// card. No entity contents, documents, messages, sender names or audit are read.
func (s *SingleManualAttachCommandService) Attachments(ctx context.Context, actor int, kind ElementKind, id, applicationID int, tableID *int) ([]SingleManualAttachAttachment, error) {
	if err := s.identity(actor, kind, id, tableID); err != nil {
		return nil, err
	}
	if applicationID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная заявка")
	}
	out := make([]SingleManualAttachAttachment, 0)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := singleAttachPermissions(ctx, tx, actor, kind, id); err != nil {
			return err
		}
		var app struct {
			ID           int
			Status       string
			Confirmation string
			Archived     bool
		}
		archiveSQL, args := archivedApplicationCond("app")
		args = append(args, applicationID)
		read := tx.Raw("SELECT app.id,COALESCE(app.status,'') AS status,COALESCE(app.confirmation,'') AS confirmation,"+archiveSQL+" AS archived FROM applications app WHERE app.id=? FOR SHARE OF app", args...).Scan(&app)
		if read.Error != nil {
			return read.Error
		}
		if app.ID == 0 || app.Archived || app.Status != models.StatusInWork || app.Confirmation != models.ConfirmationApproved {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Привязка возможна только к активной согласованной заявке")
		}
		orphanID, err := singleAttachLocator(tx, kind, id)
		if err != nil {
			return err
		}
		var orphan models.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&orphan, orphanID).Error; err != nil {
			return err
		}
		if _, err := singleAttachRow(tx, kind, id, orphan); err != nil {
			return err
		}
		set, err := singleAttachPermissions(ctx, tx, actor, kind, id)
		if err != nil {
			return err
		}
		if err := checkEntityPeriodTableContext(tx, set, kind, id, tableID); err != nil {
			return err
		}
		return tx.Table("attachments").Select("id,application_id,status,is_manual,attachment_type,attachment_name,attachment_display_name,entry_date_from,entry_date_to,entry_time_from,entry_time_to").Where("application_id=? AND status=1 AND is_manual=FALSE", applicationID).Order("id").Scan(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SingleManualAttachCommandService) Context(ctx context.Context, actor int, kind ElementKind, id int, table *int) (*SingleManualAttachContext, error) {
	if err := s.identity(actor, kind, id, table); err != nil {
		return nil, err
	}
	var out *SingleManualAttachContext
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		set, err := singleAttachPermissions(ctx, tx, actor, kind, id)
		if err != nil {
			return err
		}
		orphanID, err := singleAttachLocator(tx, kind, id)
		if err != nil {
			return err
		}
		var orphan models.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&orphan, orphanID).Error; err != nil {
			return err
		}
		row, err := singleAttachRow(tx, kind, id, orphan)
		if err != nil {
			return err
		}
		set, err = singleAttachPermissions(ctx, tx, actor, kind, id)
		if err != nil {
			return err
		}
		if err := checkEntityPeriodTableContext(tx, set, kind, id, table); err != nil {
			return err
		}
		effective, err := models.ResolveEntityPeriod(row.PeriodMode, row.Own, row.Parent, true)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Некорректный срок записи")
		}
		out = &SingleManualAttachContext{EntityID: id, EntityKind: kind, AttachmentID: orphan.ID, IsManual: true, ParentPeriod: row.Parent, PeriodMode: row.PeriodMode, EffectivePeriod: effective, RequiresPeriodChoice: !effective.Bounded, CanAssignPeriod: set.Has(KeyDetailPeriodChange)}
		if kind == ElementCar {
			var own SingleManualAttachCarFlags
			if err := tx.Table("cars").Select("individual_roof_access,individual_free_parking").Where("id=?", id).Scan(&own).Error; err != nil {
				return err
			}
			flags := singleAttachCarFlags(own, orphan)
			out.Flags = &flags
		}
		return nil
	})
	return out, err
}

func (s *SingleManualAttachCommandService) Preview(ctx context.Context, actor int, kind ElementKind, id int, req SingleManualAttachRequest) (*SingleManualAttachResult, error) {
	return s.run(ctx, actor, kind, id, req, false)
}
func (s *SingleManualAttachCommandService) Attach(ctx context.Context, actor int, kind ElementKind, id int, req SingleManualAttachRequest) (*SingleManualAttachResult, error) {
	return s.run(ctx, actor, kind, id, req, true)
}

func (s *SingleManualAttachCommandService) identity(actor int, kind ElementKind, id int, table *int) error {
	if s == nil || s.db == nil || s.recorder == nil {
		return fmt.Errorf("single manual attach dependencies are required")
	}
	if actor <= 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	if (kind != ElementEmployee && kind != ElementCar) || id <= 0 || (table != nil && *table <= 0) {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректная запись или таблица")
	}
	return nil
}

func (s *SingleManualAttachCommandService) run(ctx context.Context, actor int, kind ElementKind, id int, req SingleManualAttachRequest, execute bool) (*SingleManualAttachResult, error) {
	if err := s.identity(actor, kind, id, req.TableID); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 1000 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите причину длиной от 1 до 1000 символов")
	}
	if execute {
		if raw, err := hex.DecodeString(req.ExpectedRevision); err != nil || len(raw) != sha256.Size {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Обновите данные перед привязкой")
		}
	}
	bulk := AttachToApplicationRequest{ApplicationID: req.ApplicationID, TargetAttachmentID: req.TargetAttachmentID, PeriodChoice: req.PeriodChoice, SourceAttachmentID: req.SourceAttachmentID, Period: req.Period}
	var out *SingleManualAttachResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := singleAttachPermissions(ctx, tx, actor, kind, id); err != nil {
			return err
		}
		orphanID, err := singleAttachLocator(tx, kind, id)
		if err != nil {
			return err
		}
		// Reject a repeated attach as a stale binding before validating target IDs
		// relative to the located parent. This is only a locator precheck: the
		// same manual/orphan invariant is checked again under attachment/row locks.
		var initial models.Attachment
		if err := tx.Select("id", "is_manual", "application_id").First(&initial, orphanID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusConflict, "Основание записи изменилось")
			}
			return err
		}
		if !initial.IsManual || initial.ApplicationID != nil {
			return echo.NewHTTPError(http.StatusConflict, "Запись больше не является ручной")
		}
		if err := validateManualAttachRequest(orphanID, bulk); err != nil {
			return err
		}
		appID := 0
		if req.ApplicationID != nil {
			appID = *req.ApplicationID
		} else {
			var target models.Attachment
			if err := tx.Select("id", "application_id").First(&target, *req.TargetAttachmentID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return echo.NewHTTPError(http.StatusUnprocessableEntity, "Целевое вложение не найдено")
				}
				return err
			}
			if target.ApplicationID == nil {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Целевое вложение не принадлежит заявке")
			}
			appID = *target.ApplicationID
		}
		// All competing period/group/cron writers use app -> sorted attachments ->
		// entity -> table binding. Never relocate and lock a different app after this.
		var app struct {
			ID              int
			Status          string
			Confirmation    string
			StatusUpdatedAt *time.Time
			Archived        bool
		}
		archiveSQL, archiveArgs := archivedApplicationCond("app")
		args := append(archiveArgs, appID)
		read := tx.Raw("SELECT app.id,COALESCE(app.status,'') AS status,COALESCE(app.confirmation,'') AS confirmation,app.status_updated_at,"+archiveSQL+" AS archived FROM applications app WHERE app.id=? FOR UPDATE OF app", args...).Scan(&app)
		if read.Error != nil {
			return read.Error
		}
		if app.ID == 0 || app.Archived || app.Status != models.StatusInWork || app.Confirmation != models.ConfirmationApproved {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Привязка возможна только к активной согласованной заявке")
		}
		ids := []int{orphanID}
		if req.TargetAttachmentID != nil {
			ids = append(ids, *req.TargetAttachmentID)
		}
		if req.SourceAttachmentID != nil {
			ids = append(ids, *req.SourceAttachmentID)
		}
		slices.Sort(ids)
		ids = slices.Compact(ids)
		var attachments []models.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&attachments).Error; err != nil {
			return err
		}
		locked := map[int]models.Attachment{}
		for _, a := range attachments {
			locked[a.ID] = a
		}
		orphan, ok := locked[orphanID]
		if !ok {
			return echo.NewHTTPError(http.StatusConflict, "Основание записи изменилось")
		}
		row, err := singleAttachRow(tx, kind, id, orphan)
		if err != nil {
			return err
		}
		set, err := singleAttachPermissions(ctx, tx, actor, kind, id)
		if err != nil {
			return err
		}
		if err := checkEntityPeriodTableContext(tx, set, kind, id, req.TableID); err != nil {
			return err
		}
		current, err := models.ResolveEntityPeriod(row.PeriodMode, row.Own, row.Parent, true)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Некорректный срок записи")
		}
		needsChoice := !current.Bounded
		if needsChoice && !set.Has(KeyDetailPeriodChange) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для назначения срока")
		}
		var target, source *models.Attachment
		if req.TargetAttachmentID != nil {
			a, ok := locked[*req.TargetAttachmentID]
			if !ok || a.ApplicationID == nil || *a.ApplicationID != appID || a.IsManual {
				return echo.NewHTTPError(http.StatusConflict, "Целевое вложение изменилось")
			}
			if a.Status == nil || *a.Status != 1 || a.AttachmentType != orphan.AttachmentType {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Тип или состояние целевого вложения не подходит")
			}
			target = &a
		}
		if req.SourceAttachmentID != nil {
			a, ok := locked[*req.SourceAttachmentID]
			if !ok || a.ApplicationID == nil || *a.ApplicationID != appID || a.IsManual || a.Status == nil || *a.Status != 1 {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Источник срока должен быть активным вложением выбранной заявки")
			}
			source = &a
		}
		// Lock only this car's unload facts. Never copy attachment-level source union:
		// that union may contain places belonging to neighboring manual cars.
		var unload []models.CarUnloadPlace
		if kind == ElementCar {
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("car_id=?", id).Order("unload_place_id,id").Find(&unload).Error; err != nil {
				return err
			}
		}
		var targetPlaces []models.AttachmentUnloadPlace
		if kind == ElementCar && target != nil {
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("attachment_id=?", target.ID).Order("unload_place_id,id").Find(&targetPlaces).Error; err != nil {
				return err
			}
		}
		if err := singleManualAttachTargetCompatibility(kind, orphan, target); err != nil {
			return err
		}
		now := canonicalEntityPeriodTime(time.Now())
		chosen, err := manualAttachChosenWindow(bulk, needsChoice, source, now)
		if err != nil {
			return err
		}
		parent := current.EntryPeriod
		if target != nil {
			parent = manualAttachWindow(*target)
		} else if needsChoice {
			parent = chosen
		}
		nextMode, nextOwn, _, err := manualAttachNextPeriod(row.PeriodMode, row.Own, row.Parent, parent, chosen, bulk, needsChoice, target == nil)
		if err != nil {
			return err
		}
		if row.ByFact {
			if err := manualAttachValidateByFact("По факту", row.PeriodMode, row.Own, row.Parent, nextMode, nextOwn, parent, now); err != nil {
				return err
			}
		}
		next, err := models.ResolveEntityPeriod(nextMode, nextOwn, parent, false)
		if err != nil || !next.Bounded {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Не задан конечный срок привязанной записи")
		}
		// Snapshot is selected-only: unrelated sibling changes do not invalidate it.
		var ownFlags SingleManualAttachCarFlags
		if kind == ElementCar {
			if err := tx.Table("cars").Select("individual_roof_access,individual_free_parking").Where("id=?", id).Scan(&ownFlags).Error; err != nil {
				return err
			}
		}
		revision, err := singleAttachRevision(actor, kind, row, orphan, target, source, app.ID, app.Status, app.Confirmation, app.StatusUpdatedAt, req, unload, targetPlaces, ownFlags)
		if err != nil {
			return err
		}
		out = &SingleManualAttachResult{EntityID: id, EntityKind: kind, OldAttachmentID: orphanID, ApplicationID: appID, DestinationMode: "new_attachment", CurrentEffective: current, NewEffective: next, CurrentMode: row.PeriodMode, NewMode: nextMode, NeedsPeriodChoice: needsChoice, Revision: revision}
		if kind == ElementCar {
			currentFlags := singleAttachCarFlags(ownFlags, orphan)
			nextOwn := ownFlags
			nextOwn.IndividualRoofAccess = ownFlags.IndividualRoofAccess || orphan.RoofAccess
			nextOwn.IndividualFreeParking = ownFlags.IndividualFreeParking || orphan.FreeParking
			destination := orphan
			if target != nil {
				destination = *target
			}
			nextFlags := singleAttachCarFlags(nextOwn, destination)
			out.CurrentFlags, out.NewFlags = &currentFlags, &nextFlags
		}
		if target != nil {
			dest := target.ID
			out.DestinationAttachmentID = &dest
			out.DestinationMode = "existing_attachment"
		}
		if !execute {
			return nil
		}
		if revision != req.ExpectedRevision {
			return echo.NewHTTPError(http.StatusConflict, "Данные изменились. Обновите их перед привязкой")
		}
		if target == nil {
			active := 1
			created := models.Attachment{ApplicationID: &appID, AttachmentType: orphan.AttachmentType, AttachmentName: orphan.AttachmentName, AttachmentDisplayName: orphan.AttachmentDisplayName, UniqueAttachmentID: orphan.UniqueAttachmentID, RoofAccess: orphan.RoofAccess, FreeParking: orphan.FreeParking, EntryDateFrom: parent.EntryDateFrom, EntryDateTo: parent.EntryDateTo, EntryTimeFrom: parent.EntryTimeFrom, EntryTimeTo: parent.EntryTimeTo, Status: &active, CreatedByUserID: &actor, CreatedAt: now, UpdatedAt: now}
			if err := tx.Omit(clause.Associations).Create(&created).Error; err != nil {
				return err
			}
			target = &created
			dest := created.ID
			out.DestinationAttachmentID = &dest
		}
		updates := map[string]any{"attachment_id": target.ID, "period_mode": nextMode, "entry_date_from": nextOwn.EntryDateFrom, "entry_date_to": nextOwn.EntryDateTo, "entry_time_from": nextOwn.EntryTimeFrom, "entry_time_to": nextOwn.EntryTimeTo, "updated_at": now}
		if out.NewFlags != nil {
			updates["individual_roof_access"] = out.NewFlags.IndividualRoofAccess
			updates["individual_free_parking"] = out.NewFlags.IndividualFreeParking
		}
		write := tx.Table(string(kind)).Where("id=? AND attachment_id=?", id, orphanID).Updates(updates)
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return echo.NewHTTPError(http.StatusConflict, "Запись перемещена другим запросом")
		}
		if kind == ElementCar && req.TargetAttachmentID != nil {
			// Existing attachment owns the common places. Only this car adopts them;
			// neither target union nor source attachment/sibling associations change.
			if err := tx.Where("car_id=?", id).Delete(&models.CarUnloadPlace{}).Error; err != nil {
				return err
			}
			for _, place := range targetPlaces {
				association := models.CarUnloadPlace{CarID: id, UnloadPlaceID: place.UnloadPlaceID, OrderIndex: place.OrderIndex}
				if err := tx.Omit(clause.Associations).Create(&association).Error; err != nil {
					return err
				}
			}
			// This legacy text is returned beside normalized unload_places. Keeping the
			// old manual text would falsely display a place the car no longer uses.
			if err := tx.Table("cars").Where("id=?", id).Update("unload_place", nil).Error; err != nil {
				return err
			}
		} else {
			for _, place := range unload {
				if err := tx.Exec("INSERT INTO attachment_unload_places(attachment_id,unload_place_id,order_index,created_at) VALUES(?,?,?,?) ON CONFLICT(attachment_id,unload_place_id) DO NOTHING", target.ID, place.UnloadPlaceID, place.OrderIndex, now).Error; err != nil {
					return err
				}
			}
		}
		entityType := models.AuditEntityEmployee
		if kind == ElementCar {
			entityType = models.AuditEntityCar
		}
		comment := "Привязана открытая запись к заявке"
		details := struct {
			OldFlags        *SingleManualAttachCarFlags `json:"old_flags,omitempty"`
			NewFlags        *SingleManualAttachCarFlags `json:"new_flags,omitempty"`
			Comment         string                      `json:"comment"`
			Reason          string                      `json:"reason"`
			OldAttachmentID int                         `json:"old_attachment_id"`
			NewAttachmentID int                         `json:"new_attachment_id"`
			ApplicationID   int                         `json:"application_id"`
			OldMode         models.PeriodMode           `json:"old_period_mode"`
			NewMode         models.PeriodMode           `json:"new_period_mode"`
			OldPeriod       models.EffectivePeriod      `json:"old_period"`
			NewPeriod       models.EffectivePeriod      `json:"new_period"`
		}{out.CurrentFlags, out.NewFlags, comment, reason, orphanID, target.ID, appID, row.PeriodMode, nextMode, current, next}
		if err := s.recorder.Record(ctx, tx, entityType, &id, "update", &actor, details); err != nil {
			return err
		}
		if row.PeriodMode != nextMode || current.Bounded != next.Bounded || !equalEntityPeriod(current.EntryPeriod, next.EntryPeriod) {
			field := "period"
			oldValue := describeEntityPeriod(EntityPeriodCommandResult{PeriodMode: row.PeriodMode, Effective: current})
			newValue := describeEntityPeriod(EntityPeriodCommandResult{PeriodMode: nextMode, Effective: next})
			periodDetails := struct {
				FieldName *string                `json:"field_name"`
				OldValue  *string                `json:"old_value"`
				NewValue  *string                `json:"new_value"`
				Comment   *string                `json:"comment"`
				OldMode   models.PeriodMode      `json:"old_period_mode"`
				NewMode   models.PeriodMode      `json:"new_period_mode"`
				OldPeriod models.EffectivePeriod `json:"old_period"`
				NewPeriod models.EffectivePeriod `json:"new_period"`
			}{&field, &oldValue, &newValue, &reason, row.PeriodMode, nextMode, current, next}
			return s.recorder.Record(ctx, tx, entityType, &id, models.AuditActionDatesChanged, &actor, periodDetails)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if execute {
		if s.tables != nil {
			if kind == ElementCar {
				s.tables.NotifyCarsChangedBatch(ctx, []int{id})
			} else {
				s.tables.NotifyEmployeesChangedBatch(ctx, []int{id})
			}
		}
		if s.available != nil {
			s.available.NotifyAvailableChanged(ctx)
		}
		if s.afterAttach != nil {
			s.afterAttach(ctx, out.ApplicationID)
		}
	}
	return out, nil
}

func singleAttachPermissions(ctx context.Context, tx *gorm.DB, actor int, kind ElementKind, id int) (PermissionSet, error) {
	resolver := NewPermissionResolver(tx)
	set, err := resolver.Resolve(ctx, actor)
	if err != nil {
		return set, err
	}
	if !set.Has(KeyPageAdmin) {
		return set, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
	}
	scopes := NewElementScopeResolver(tx, resolver)
	scope, err := scopes.Resolve(ctx, actor)
	if err != nil {
		return set, err
	}
	visible, err := scopes.Visible(ctx, scope, kind, id)
	if err != nil {
		return set, err
	}
	if !visible {
		return set, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
	}
	return set, nil
}

func singleAttachLocator(tx *gorm.DB, kind ElementKind, id int) (int, error) {
	var row struct{ AttachmentID int }
	read := tx.Table(string(kind)).Select("attachment_id").Where("id=?", id).Scan(&row)
	if read.Error != nil {
		return 0, read.Error
	}
	if read.RowsAffected != 1 || row.AttachmentID <= 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Запись не найдена")
	}
	return row.AttachmentID, nil
}

func singleAttachRow(tx *gorm.DB, kind ElementKind, id int, orphan models.Attachment) (entityPeriodSnapshot, error) {
	var out entityPeriodSnapshot
	expectedType := "people"
	removed := "(e.is_purged OR e.date_deleted IS NOT NULL)"
	if kind == ElementCar {
		expectedType = "cars"
		removed = "(e.is_purged OR e.date_removed IS NOT NULL)"
	}
	if !orphan.IsManual || orphan.ApplicationID != nil {
		return out, echo.NewHTTPError(http.StatusConflict, "Запись больше не является ручной")
	}
	if orphan.AttachmentType != expectedType || orphan.Status == nil || *orphan.Status != 1 {
		return out, echo.NewHTTPError(http.StatusUnprocessableEntity, "Ручное основание неактивно или несовместимо")
	}
	var row struct {
		ID, AttachmentID                                       int
		PeriodMode                                             models.PeriodMode
		EntryDateFrom, EntryDateTo, EntryTimeFrom, EntryTimeTo *string
		Status                                                 *int
		UpdatedAt                                              time.Time
		Removed, ByFact                                        bool
	}
	byFact := "FALSE"
	args := []any{}
	if kind == ElementCar {
		byFact = "LOWER(REPLACE(TRIM(COALESCE(e.car_number,'')), ' ', '')) = ?"
		args = append(args, byFactCompactPlate())
	}
	args = append(args, id)
	read := tx.Raw("SELECT e.id,e.attachment_id,e.period_mode,e.entry_date_from,e.entry_date_to,e.entry_time_from,e.entry_time_to,e.status,e.updated_at,"+removed+" AS removed,"+byFact+" AS by_fact FROM "+string(kind)+" e WHERE e.id=? FOR UPDATE OF e", args...).Scan(&row)
	if read.Error != nil {
		return out, read.Error
	}
	if row.ID == 0 || row.AttachmentID != orphan.ID {
		return out, echo.NewHTTPError(http.StatusConflict, "Основание записи изменилось")
	}
	if row.Removed || row.Status == nil || *row.Status != 1 {
		return out, echo.NewHTTPError(http.StatusUnprocessableEntity, "Запись неактивна или удалена")
	}
	out = entityPeriodSnapshot{ID: id, AttachmentID: orphan.ID, Manual: true, AttachmentStatus: orphan.Status, AttachmentUpdatedAt: orphan.UpdatedAt, Status: row.Status, UpdatedAt: row.UpdatedAt, PeriodMode: row.PeriodMode, Own: models.EntryPeriod{EntryDateFrom: row.EntryDateFrom, EntryDateTo: row.EntryDateTo, EntryTimeFrom: row.EntryTimeFrom, EntryTimeTo: row.EntryTimeTo}, Parent: manualAttachWindow(orphan), ByFact: row.ByFact}
	return out, nil
}

// Template mismatch remains explicit; car flags are preserved on the selected
// row and combined with target permissions without changing target or siblings.
func singleManualAttachTargetCompatibility(kind ElementKind, orphan models.Attachment, target *models.Attachment) error {
	if target == nil {
		return nil
	}
	if !equalPeriodID(orphan.UniqueAttachmentID, target.UniqueAttachmentID) || (kind != ElementCar && (orphan.RoofAccess != target.RoofAccess || orphan.FreeParking != target.FreeParking)) {
		return echo.NewHTTPError(http.StatusConflict, "Признаки вложений различаются. Создайте отдельное вложение")
	}
	return nil
}

func singleAttachRevision(actor int, kind ElementKind, row entityPeriodSnapshot, orphan models.Attachment, target, source *models.Attachment, appID int, status, confirmation string, changed *time.Time, req SingleManualAttachRequest, unload []models.CarUnloadPlace, targetPlaces []models.AttachmentUnloadPlace, ownFlags SingleManualAttachCarFlags) (string, error) {
	type attachmentFacts struct {
		ID                              int
		ApplicationID                   *int
		Type                            string
		Manual                          bool
		Status                          *int
		Period                          models.EntryPeriod
		Roof, Parking                   bool
		Template, Organization, Company *int
		Updated                         time.Time
	}
	facts := func(a *models.Attachment) *attachmentFacts {
		if a == nil {
			return nil
		}
		return &attachmentFacts{a.ID, a.ApplicationID, a.AttachmentType, a.IsManual, a.Status, manualAttachWindow(*a), a.RoofAccess, a.FreeParking, a.UniqueAttachmentID, a.OrganizationID, a.CompanyID, canonicalEntityPeriodTime(a.UpdatedAt)}
	}
	row.UpdatedAt = canonicalEntityPeriodTime(row.UpdatedAt)
	row.AttachmentUpdatedAt = canonicalEntityPeriodTime(row.AttachmentUpdatedAt)
	if changed != nil {
		stamp := canonicalEntityPeriodTime(*changed)
		changed = &stamp
	}
	req.ExpectedRevision = ""
	req.Reason = strings.TrimSpace(req.Reason)
	type unloadFact struct {
		ID, Place int
		Order     *int
	}
	places := make([]unloadFact, 0, len(unload))
	for _, p := range unload {
		places = append(places, unloadFact{p.ID, p.UnloadPlaceID, p.OrderIndex})
	}
	destinationPlaces := make([]unloadFact, 0, len(targetPlaces))
	for _, p := range targetPlaces {
		destinationPlaces = append(destinationPlaces, unloadFact{p.ID, p.UnloadPlaceID, p.OrderIndex})
	}
	raw, err := json.Marshal(struct {
		OwnFlags               SingleManualAttachCarFlags
		Actor                  int
		Kind                   ElementKind
		Row                    entityPeriodSnapshot
		Orphan, Target, Source *attachmentFacts
		ApplicationID          int
		Status, Confirmation   string
		Changed                *time.Time
		Request                SingleManualAttachRequest
		Unload, TargetPlaces   []unloadFact
	}{ownFlags, actor, kind, row, facts(&orphan), facts(target), facts(source), appID, status, confirmation, changed, req, places, destinationPlaces})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
