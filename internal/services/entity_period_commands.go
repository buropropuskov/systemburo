package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EntityPeriodCommandService is deliberately separate from the existing service
// interfaces. Router integration must retain the normal authenticated-account
// middleware. Changes must not be exposed until expiry/read consumers support
// effective periods beyond their parent attachment.
type EntityPeriodCommandService struct {
	db          *gorm.DB
	recorder    AuditRecorder
	afterChange func(context.Context, int, ElementKind, EntityPeriodCommandResult, EntityPeriodCommandResult, string)
}

func NewEntityPeriodCommandService(db *gorm.DB, recorder AuditRecorder) *EntityPeriodCommandService {
	return &EntityPeriodCommandService{db: db, recorder: recorder}
}

type EntityPeriodInput struct {
	EntryDateFrom string `json:"entry_date_from"`
	EntryDateTo   string `json:"entry_date_to"`
	EntryTimeFrom string `json:"entry_time_from"`
	EntryTimeTo   string `json:"entry_time_to"`
}

type ChangeEntityPeriodRequest struct {
	PeriodMode       models.PeriodMode  `json:"period_mode"`
	Period           *EntityPeriodInput `json:"period"`
	Reason           string             `json:"reason"`
	ExpectedRevision string             `json:"expected_revision"`
	TableID          *int               `json:"table_id,omitempty"`
}

type EntityPeriodCommandResult struct {
	EntityID       int                    `json:"entity_id"`
	AttachmentID   int                    `json:"attachment_id"`
	ApplicationID  *int                   `json:"application_id"`
	PeriodMode     models.PeriodMode      `json:"period_mode"`
	Individual     *models.EntryPeriod    `json:"individual_period"`
	Effective      models.EffectivePeriod `json:"effective_period"`
	Revision       string                 `json:"period_revision"`
	ApprovalsReset bool                   `json:"approvals_reset"`
}

// Inspect returns an optimistic revision for an editable entity. This new
// contract is additive; it does not change PUT /applications/:id/dates.
func (s *EntityPeriodCommandService) Inspect(ctx context.Context, actorID int, kind ElementKind, entityID int, tableID *int) (*EntityPeriodCommandResult, error) {
	return s.run(ctx, actorID, kind, entityID, tableID, nil, time.Now())
}

func (s *EntityPeriodCommandService) Change(ctx context.Context, actorID int, kind ElementKind, entityID int, req ChangeEntityPeriodRequest) (*EntityPeriodCommandResult, error) {
	return s.run(ctx, actorID, kind, entityID, req.TableID, &req, time.Now())
}

// run owns the transaction; callers publish notifications/cache invalidation
// only after it returns success. Lock order is application -> attachment -> row
// -> optional table binding. Initial locator reads are rechecked under locks.
func (s *EntityPeriodCommandService) run(ctx context.Context, actorID int, kind ElementKind, entityID int, tableID *int, req *ChangeEntityPeriodRequest, now time.Time) (*EntityPeriodCommandResult, error) {
	if s == nil || s.db == nil || s.recorder == nil {
		return nil, fmt.Errorf("entity period command dependencies are required")
	}
	if actorID <= 0 {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	if (kind != ElementEmployee && kind != ElementCar) || entityID <= 0 || (tableID != nil && *tableID <= 0) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная запись или таблица")
	}
	var result *EntityPeriodCommandResult
	var previous EntityPeriodCommandResult
	var changeReason string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A resolver bound to this transaction avoids stale process-cache grants
		// and reads active/banned users and personal denies through the same pool.
		resolver := NewPermissionResolver(tx)
		set, err := resolver.Resolve(ctx, actorID)
		if err != nil {
			return err
		}
		if !set.Has(KeyDetailPeriodChange) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		scopes := NewElementScopeResolver(tx, resolver)
		scope, err := scopes.Resolve(ctx, actorID)
		if err != nil {
			return err
		}
		visible, err := scopes.Visible(ctx, scope, kind, entityID)
		if err != nil {
			return err
		}
		if !visible {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		snapshot, err := lockEntityPeriodSnapshot(tx, kind, entityID)
		if err != nil {
			return err
		}
		// Parent ownership can change while an initial visibility read waits for
		// a lock. Recheck the same established scope after locking the entity.
		visible, err = scopes.Visible(ctx, scope, kind, entityID)
		if err != nil {
			return err
		}
		if err := AuthorizeEntityPeriodChange(set, PeriodChangeTarget{
			Visible: visible, Manual: snapshot.Manual && snapshot.ApplicationID == nil,
			Active:   snapshot.Status != nil && *snapshot.Status == 1 && snapshot.AttachmentStatus != nil && *snapshot.AttachmentStatus == 1,
			Archived: snapshot.Archived, Removed: snapshot.Removed,
			ApplicationStatus: snapshot.ApplicationStatus,
		}); err != nil {
			return err
		}
		if err := checkEntityPeriodTableContext(tx, set, kind, entityID, tableID); err != nil {
			return err
		}
		result, err = entityPeriodResult(actorID, kind, snapshot)
		if err != nil || req == nil {
			return err
		}
		next, reason, err := validateEntityPeriodCommand(*req, snapshot, now)
		if err != nil {
			return err
		}
		if req.ExpectedRevision != result.Revision {
			return echo.NewHTTPError(http.StatusConflict, "Срок или состояние записи изменились. Обновите карточку")
		}
		if snapshot.PeriodMode == req.PeriodMode && equalEntityPeriod(snapshot.Own, next) {
			return echo.NewHTTPError(http.StatusBadRequest, "Срок не изменился")
		}
		old := *result
		previous, changeReason = old, reason
		writtenAt := canonicalEntityPeriodTime(now)
		updates := map[string]any{
			"period_mode":     req.PeriodMode,
			"entry_date_from": next.EntryDateFrom, "entry_date_to": next.EntryDateTo,
			"entry_time_from": next.EntryTimeFrom, "entry_time_to": next.EntryTimeTo,
			"updated_at": writtenAt,
		}
		write := tx.Table(string(kind)).Where("id=? AND attachment_id=?", entityID, snapshot.AttachmentID).Updates(updates)
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return echo.NewHTTPError(http.StatusConflict, "Состав записи изменился. Обновите карточку")
		}
		snapshot.Own, snapshot.PeriodMode, snapshot.UpdatedAt = next, req.PeriodMode, writtenAt
		result, err = entityPeriodResult(actorID, kind, snapshot)
		if err != nil {
			return err
		}
		oldText, newText := describeEntityPeriod(old), describeEntityPeriod(*result)
		field := "period"
		details := struct {
			FieldName *string                `json:"field_name"`
			OldValue  *string                `json:"old_value"`
			NewValue  *string                `json:"new_value"`
			Comment   *string                `json:"comment"`
			TableID   *int                   `json:"table_id,omitempty"`
			OldMode   models.PeriodMode      `json:"old_period_mode"`
			NewMode   models.PeriodMode      `json:"new_period_mode"`
			OldPeriod models.EffectivePeriod `json:"old_period"`
			NewPeriod models.EffectivePeriod `json:"new_period"`
		}{&field, &oldText, &newText, &reason, tableID, old.PeriodMode, result.PeriodMode, old.Effective, result.Effective}
		entityType := models.AuditEntityEmployee
		if kind == ElementCar {
			entityType = models.AuditEntityCar
		}
		return s.recorder.Record(ctx, tx, entityType, &entityID, models.AuditActionDatesChanged, &actorID, details)
	})
	if err != nil {
		return nil, err
	}
	if req != nil && s.afterChange != nil {
		s.afterChange(ctx, actorID, kind, previous, *result, changeReason)
	}
	return result, nil
}

type entityPeriodSnapshot struct {
	ID                         int
	AttachmentID               int
	ApplicationID              *int
	ApplicationStatus          string
	Confirmation               string
	ApplicationStatusUpdatedAt *time.Time
	AttachmentUpdatedAt        time.Time
	AttachmentStatus           *int
	Manual                     bool
	Archived                   bool
	Removed                    bool
	Status                     *int
	UpdatedAt                  time.Time
	PeriodMode                 models.PeriodMode
	Own                        models.EntryPeriod
	Parent                     models.EntryPeriod
	ByFact                     bool
}

func lockEntityPeriodSnapshot(tx *gorm.DB, kind ElementKind, id int) (entityPeriodSnapshot, error) {
	var out entityPeriodSnapshot
	var locator struct {
		AttachmentID  int
		ApplicationID *int
	}
	lookup := tx.Raw("SELECT e.attachment_id,a.application_id FROM "+string(kind)+" e JOIN attachments a ON a.id=e.attachment_id WHERE e.id=?", id).Scan(&locator)
	if lookup.Error != nil {
		return out, lookup.Error
	}
	if lookup.RowsAffected != 1 {
		return out, echo.NewHTTPError(http.StatusNotFound, "Запись не найдена")
	}
	out.ID, out.AttachmentID, out.ApplicationID = id, locator.AttachmentID, locator.ApplicationID
	if locator.ApplicationID != nil {
		var app struct {
			Status          string
			Confirmation    string
			StatusUpdatedAt *time.Time
			Archived        bool
		}
		archiveSQL, archiveArgs := archivedApplicationCond("app")
		args := append(archiveArgs, *locator.ApplicationID)
		read := tx.Raw("SELECT COALESCE(app.status,'') AS status,COALESCE(app.confirmation,'') AS confirmation,app.status_updated_at,"+archiveSQL+" AS archived FROM applications app WHERE app.id=? FOR UPDATE OF app", args...).Scan(&app)
		if read.Error != nil {
			return out, read.Error
		}
		if read.RowsAffected != 1 {
			return out, echo.NewHTTPError(http.StatusConflict, "Основание записи изменилось")
		}
		out.ApplicationStatus, out.Confirmation, out.ApplicationStatusUpdatedAt, out.Archived = app.Status, app.Confirmation, app.StatusUpdatedAt, app.Archived
	}
	var attachment models.Attachment
	if err := tx.Raw("SELECT id,application_id,is_manual,status,entry_date_from,entry_date_to,entry_time_from,entry_time_to,updated_at FROM attachments WHERE id=? FOR UPDATE", locator.AttachmentID).Scan(&attachment).Error; err != nil {
		return out, err
	}
	if attachment.ID == 0 || !equalPeriodID(attachment.ApplicationID, locator.ApplicationID) {
		return out, echo.NewHTTPError(http.StatusConflict, "Основание записи изменилось")
	}
	out.Manual, out.AttachmentStatus, out.AttachmentUpdatedAt = attachment.IsManual, attachment.Status, attachment.UpdatedAt
	out.Parent = models.EntryPeriod{EntryDateFrom: attachment.EntryDateFrom, EntryDateTo: attachment.EntryDateTo, EntryTimeFrom: attachment.EntryTimeFrom, EntryTimeTo: attachment.EntryTimeTo}
	var row struct {
		ID            int
		AttachmentID  int
		PeriodMode    models.PeriodMode
		EntryDateFrom *string
		EntryDateTo   *string
		EntryTimeFrom *string
		EntryTimeTo   *string
		Status        *int
		UpdatedAt     time.Time
		Removed       bool
		ByFact        bool
	}
	removed := "(e.is_purged OR e.date_deleted IS NOT NULL)"
	byFact := "FALSE"
	if kind == ElementCar {
		removed = "(e.is_purged OR e.date_removed IS NOT NULL)"
		byFact = "LOWER(REPLACE(TRIM(COALESCE(e.car_number,'')), ' ', '')) = ?"
	}
	args := []any{}
	if kind == ElementCar {
		args = append(args, byFactCompactPlate())
	}
	args = append(args, id)
	read := tx.Raw("SELECT e.id,e.attachment_id,e.period_mode,e.entry_date_from,e.entry_date_to,e.entry_time_from,e.entry_time_to,e.status,e.updated_at,"+removed+" AS removed,"+byFact+" AS by_fact FROM "+string(kind)+" e WHERE e.id=? FOR UPDATE OF e", args...).Scan(&row)
	if read.Error != nil {
		return out, read.Error
	}
	if row.ID == 0 || row.AttachmentID != locator.AttachmentID {
		return out, echo.NewHTTPError(http.StatusConflict, "Запись перемещена. Обновите карточку")
	}
	out.PeriodMode, out.Status, out.UpdatedAt, out.Removed, out.ByFact = row.PeriodMode, row.Status, row.UpdatedAt, row.Removed, row.ByFact
	out.Own = models.EntryPeriod{EntryDateFrom: row.EntryDateFrom, EntryDateTo: row.EntryDateTo, EntryTimeFrom: row.EntryTimeFrom, EntryTimeTo: row.EntryTimeTo}
	return out, nil
}

func checkEntityPeriodTableContext(tx *gorm.DB, set PermissionSet, kind ElementKind, id int, tableID *int) error {
	if tableID == nil {
		return nil
	}
	binding := elementBindingTables[kind]
	var row struct {
		TableID int
		Name    string
	}
	read := tx.Raw("SELECT b.table_id,st.name FROM "+binding.table+" b JOIN system_tables st ON st.id=b.table_id WHERE b."+binding.column+"=? AND b.table_id=? FOR SHARE OF b,st", id, *tableID).Scan(&row)
	if read.Error != nil {
		return read.Error
	}
	if row.TableID == 0 || !set.Has("table."+row.Name+".view") {
		return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к записи в выбранной таблице")
	}
	return nil
}

func validateEntityPeriodCommand(req ChangeEntityPeriodRequest, current entityPeriodSnapshot, now time.Time) (models.EntryPeriod, string, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 1000 {
		return models.EntryPeriod{}, "", echo.NewHTTPError(http.StatusBadRequest, "Укажите причину длиной от 1 до 1000 символов")
	}
	if len(req.ExpectedRevision) != sha256.Size*2 {
		return models.EntryPeriod{}, "", echo.NewHTTPError(http.StatusBadRequest, "Обновите карточку перед изменением срока")
	}
	if _, err := hex.DecodeString(req.ExpectedRevision); err != nil {
		return models.EntryPeriod{}, "", echo.NewHTTPError(http.StatusBadRequest, "Некорректная версия срока")
	}
	var next models.EntryPeriod
	switch req.PeriodMode {
	case models.PeriodIndividual:
		if req.Period == nil {
			return next, "", echo.NewHTTPError(http.StatusBadRequest, "Укажите индивидуальный срок")
		}
		p, err := parseApplicationPeriod(ChangeApplicationDatesRequest{EntryDateFrom: req.Period.EntryDateFrom, EntryDateTo: req.Period.EntryDateTo, EntryTimeFrom: req.Period.EntryTimeFrom, EntryTimeTo: req.Period.EntryTimeTo}, now)
		if err != nil {
			return next, "", err
		}
		next = models.EntryPeriod{EntryDateFrom: &p.DateFrom, EntryDateTo: &p.DateTo, EntryTimeFrom: &p.TimeFrom, EntryTimeTo: &p.TimeTo}
	case models.PeriodInherit:
		if req.Period != nil {
			return next, "", echo.NewHTTPError(http.StatusBadRequest, "При наследовании отдельный срок не передаётся")
		}
	default:
		return next, "", echo.NewHTTPError(http.StatusBadRequest, "Неизвестный режим срока")
	}
	effective, err := models.ResolveEntityPeriod(req.PeriodMode, next, current.Parent, current.Manual && current.ApplicationID == nil)
	if err != nil {
		return next, "", echo.NewHTTPError(http.StatusBadRequest, "Срок основания некорректен")
	}
	if !effective.Bounded && effective.Source != "manual_unbounded" {
		return next, "", echo.NewHTTPError(http.StatusBadRequest, "Укажите конечный срок основания")
	}
	if effective.Bounded {
		from, to := derefStr(effective.EntryDateFrom), derefStr(effective.EntryDateTo)
		timeFrom, timeTo := derefStr(effective.EntryTimeFrom), derefStr(effective.EntryTimeTo)
		if strings.TrimSpace(timeFrom) == "" {
			timeFrom = "00:00:00"
		}
		if strings.TrimSpace(timeTo) == "" {
			timeTo = "23:59:59"
		}
		if _, err := parseApplicationPeriod(ChangeApplicationDatesRequest{EntryDateFrom: from, EntryDateTo: to, EntryTimeFrom: timeFrom, EntryTimeTo: timeTo}, now); err != nil {
			return next, "", err
		}
	}
	if current.ByFact && (!effective.Bounded || derefStr(effective.EntryDateTo) > ByFactMaxDate(now)) {
		return next, "", echo.NewHTTPError(http.StatusBadRequest, byFactDeadlineHint(now))
	}
	return next, reason, nil
}

func entityPeriodResult(actorID int, kind ElementKind, snapshot entityPeriodSnapshot) (*EntityPeriodCommandResult, error) {
	effective, err := models.ResolveEntityPeriod(snapshot.PeriodMode, snapshot.Own, snapshot.Parent, snapshot.Manual && snapshot.ApplicationID == nil)
	if err != nil {
		return nil, err
	}
	revision, err := entityPeriodRevision(actorID, kind, snapshot)
	if err != nil {
		return nil, err
	}
	result := &EntityPeriodCommandResult{EntityID: snapshot.ID, AttachmentID: snapshot.AttachmentID, ApplicationID: snapshot.ApplicationID, PeriodMode: snapshot.PeriodMode, Effective: effective, Revision: revision}
	if snapshot.PeriodMode == models.PeriodIndividual {
		own := snapshot.Own
		result.Individual = &own
	}
	return result, nil
}

// Revision is an optimistic consistency hash, not a bearer token or an access
// grant. Only new endpoints accept it. No names, documents or plates are hashed.
func entityPeriodRevision(actorID int, kind ElementKind, snapshot entityPeriodSnapshot) (string, error) {
	// PostgreSQL timestamps and pgx's wire encoding retain microseconds. Hash
	// their canonical UTC representation so Change and a later Inspect agree.
	snapshot.UpdatedAt = canonicalEntityPeriodTime(snapshot.UpdatedAt)
	snapshot.AttachmentUpdatedAt = canonicalEntityPeriodTime(snapshot.AttachmentUpdatedAt)
	if snapshot.ApplicationStatusUpdatedAt != nil {
		stamp := canonicalEntityPeriodTime(*snapshot.ApplicationStatusUpdatedAt)
		snapshot.ApplicationStatusUpdatedAt = &stamp
	}
	raw, err := json.Marshal(struct {
		ActorID  int
		Kind     ElementKind
		Snapshot entityPeriodSnapshot
	}{actorID, kind, snapshot})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalEntityPeriodTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func equalPeriodID(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func equalEntityPeriod(a, b models.EntryPeriod) bool {
	return derefStr(a.EntryDateFrom) == derefStr(b.EntryDateFrom) && derefStr(a.EntryDateTo) == derefStr(b.EntryDateTo) && derefStr(a.EntryTimeFrom) == derefStr(b.EntryTimeFrom) && derefStr(a.EntryTimeTo) == derefStr(b.EntryTimeTo)
}
func describeEntityPeriod(result EntityPeriodCommandResult) string {
	if !result.Effective.Bounded {
		return string(result.PeriodMode) + ": без ограничения срока"
	}
	return string(result.PeriodMode) + ": " + formatApplicationPeriod(applicationPeriod{DateFrom: derefStr(result.Effective.EntryDateFrom), DateTo: derefStr(result.Effective.EntryDateTo), TimeFrom: derefStr(result.Effective.EntryTimeFrom), TimeTo: derefStr(result.Effective.EntryTimeTo)})
}
