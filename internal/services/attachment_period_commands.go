package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	IndividualPolicyPreserve = "preserve"
	IndividualPolicyReplace  = "replace"
)

// AttachmentPeriodCommandService is additive and deliberately has no routes.
// The integration owner publishes notifications/invalidation after Change commits.
type AttachmentPeriodCommandService struct {
	db          *gorm.DB
	recorder    AuditRecorder
	afterChange func(context.Context, int, AttachmentPeriodCommandResult, string)
}

func NewAttachmentPeriodCommandService(db *gorm.DB, recorder AuditRecorder) *AttachmentPeriodCommandService {
	return &AttachmentPeriodCommandService{db: db, recorder: recorder}
}

type ChangeAttachmentPeriodRequest struct {
	AttachmentIDs    []int              `json:"attachment_ids"`
	Period           *EntityPeriodInput `json:"period"`
	IndividualPolicy string             `json:"individual_policy"`
	Reason           string             `json:"reason"`
	ExpectedRevision string             `json:"expected_revision"`
}

// IDs are the preview labels: no names, documents or plates leave this service.
type AttachmentPeriodPreviewItem struct {
	AttachmentID          int                `json:"attachment_id"`
	OldPeriod             models.EntryPeriod `json:"old_period"`
	EmployeeIDs           []int              `json:"employee_ids"`
	CarIDs                []int              `json:"car_ids"`
	IndividualEmployeeIDs []int              `json:"individual_employee_ids"`
	IndividualCarIDs      []int              `json:"individual_car_ids"`
}

type AttachmentPeriodCommandResult struct {
	ApplicationID        int                           `json:"application_id"`
	Attachments          []AttachmentPeriodPreviewItem `json:"attachments"`
	AttachmentIDs        []int                         `json:"attachment_ids"`
	ChangedAttachmentIDs []int                         `json:"changed_attachment_ids"`
	ChangedEmployeeIDs   []int                         `json:"changed_employee_ids"`
	ChangedCarIDs        []int                         `json:"changed_car_ids"`
	AttachmentCount      int                           `json:"attachment_count"`
	EmployeeCount        int                           `json:"employee_count"`
	CarCount             int                           `json:"car_count"`
	IndividualCount      int                           `json:"individual_count"`
	IndividualPolicy     string                        `json:"individual_policy"`
	NewPeriod            models.EntryPeriod            `json:"new_period"`
	Revision             string                        `json:"period_revision"`
	ApprovalsReset       bool                          `json:"approvals_reset"`
}

func (s *AttachmentPeriodCommandService) Preview(ctx context.Context, actorID, applicationID int, req ChangeAttachmentPeriodRequest) (*AttachmentPeriodCommandResult, error) {
	return s.run(ctx, actorID, applicationID, req, false, time.Now())
}

func (s *AttachmentPeriodCommandService) Change(ctx context.Context, actorID, applicationID int, req ChangeAttachmentPeriodRequest) (*AttachmentPeriodCommandResult, error) {
	return s.run(ctx, actorID, applicationID, req, true, time.Now())
}

type attachmentPeriodRow struct {
	ID             int
	ApplicationID  *int
	AttachmentType string
	Status         *int
	IsManual       bool
	UpdatedAt      time.Time
	EntryDateFrom  *string
	EntryDateTo    *string
	EntryTimeFrom  *string
	EntryTimeTo    *string
}

func (r attachmentPeriodRow) period() models.EntryPeriod {
	return models.EntryPeriod{EntryDateFrom: r.EntryDateFrom, EntryDateTo: r.EntryDateTo, EntryTimeFrom: r.EntryTimeFrom, EntryTimeTo: r.EntryTimeTo}
}

type attachmentPeriodEntity struct {
	ID            int
	AttachmentID  int
	PeriodMode    models.PeriodMode
	Status        *int
	Removed       bool
	ByFact        bool
	UpdatedAt     time.Time
	EntryDateFrom *string
	EntryDateTo   *string
	EntryTimeFrom *string
	EntryTimeTo   *string
}

func (r attachmentPeriodEntity) period() models.EntryPeriod {
	return models.EntryPeriod{EntryDateFrom: r.EntryDateFrom, EntryDateTo: r.EntryDateTo, EntryTimeFrom: r.EntryTimeFrom, EntryTimeTo: r.EntryTimeTo}
}

type attachmentPeriodSnapshot struct {
	ApplicationID   int
	Status          string
	Confirmation    string
	StatusUpdatedAt *time.Time
	Archived        bool
	Attachments     []attachmentPeriodRow
	Cars            []attachmentPeriodEntity
	Employees       []attachmentPeriodEntity
}

type attachmentPeriodWrite struct {
	Kind         ElementKind
	ID           int
	AttachmentID int
	Mode         models.PeriodMode
	OldOwn       models.EntryPeriod
	NewOwn       models.EntryPeriod
	OldEffective models.EntryPeriod
	NewEffective models.EntryPeriod
	WriteOwn     bool
}

func validateAttachmentPeriodRequest(req ChangeAttachmentPeriodRequest, change bool, now time.Time) (ChangeAttachmentPeriodRequest, models.EntryPeriod, error) {
	bad := func(message string) (ChangeAttachmentPeriodRequest, models.EntryPeriod, error) {
		return req, models.EntryPeriod{}, echo.NewHTTPError(http.StatusBadRequest, message)
	}
	if len(req.AttachmentIDs) == 0 {
		return bad("Выберите вложения")
	}
	req.AttachmentIDs = append([]int(nil), req.AttachmentIDs...)
	sort.Ints(req.AttachmentIDs)
	for i, id := range req.AttachmentIDs {
		if id <= 0 || (i > 0 && req.AttachmentIDs[i-1] == id) {
			return bad("Список вложений должен содержать уникальные положительные ID")
		}
	}
	if req.IndividualPolicy == "" {
		req.IndividualPolicy = IndividualPolicyPreserve
	}
	if req.IndividualPolicy != IndividualPolicyPreserve && req.IndividualPolicy != IndividualPolicyReplace {
		return bad("Неизвестная политика индивидуальных сроков")
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 1000 {
		return bad("Укажите причину длиной от 1 до 1000 символов")
	}
	if change {
		if len(req.ExpectedRevision) != sha256.Size*2 {
			return bad("Обновите предварительный просмотр")
		}
		if _, err := hex.DecodeString(req.ExpectedRevision); err != nil {
			return bad("Некорректная версия срока")
		}
	}
	if req.Period == nil {
		return bad("Укажите полный срок")
	}
	p, err := parseApplicationPeriod(ChangeApplicationDatesRequest{EntryDateFrom: req.Period.EntryDateFrom, EntryDateTo: req.Period.EntryDateTo, EntryTimeFrom: req.Period.EntryTimeFrom, EntryTimeTo: req.Period.EntryTimeTo}, now)
	if err != nil {
		return req, models.EntryPeriod{}, err
	}
	// Canonical request is also the actor-bound revision input. ExpectedRevision
	// is excluded from hashing; request order and HH:MM vs HH:MM:SS are immaterial.
	req.Period = &EntityPeriodInput{EntryDateFrom: p.DateFrom, EntryDateTo: p.DateTo, EntryTimeFrom: p.TimeFrom, EntryTimeTo: p.TimeTo}
	return req, models.EntryPeriod{EntryDateFrom: &p.DateFrom, EntryDateTo: &p.DateTo, EntryTimeFrom: &p.TimeFrom, EntryTimeTo: &p.TimeTo}, nil
}

func attachmentPeriodVisible(ctx context.Context, tx *gorm.DB, set PermissionSet, actorID, applicationID int) (bool, error) {
	query := tx.WithContext(ctx).Table("applications AS a").Where("a.id=?", applicationID)
	if !set.IsSuperAdmin() {
		apps := &applicationService{db: tx}
		approver, err := apps.isApprover(ctx, actorID)
		if err != nil {
			return false, err
		}
		query = applyApplicationAccessFilter(query, actorID, approver)
	}
	var count int64
	err := query.Count(&count).Error
	return count == 1, err
}

func lockAttachmentPeriodSnapshot(tx *gorm.DB, applicationID int, ids []int) (attachmentPeriodSnapshot, error) {
	out := attachmentPeriodSnapshot{ApplicationID: applicationID}
	var app struct {
		Status          string
		Confirmation    string
		StatusUpdatedAt *time.Time
		Archived        bool
	}
	archiveSQL, archiveArgs := archivedApplicationCond("app")
	args := append(archiveArgs, applicationID)
	read := tx.Raw("SELECT COALESCE(app.status,'') AS status, COALESCE(app.confirmation,'') AS confirmation,app.status_updated_at,"+archiveSQL+" AS archived FROM applications app WHERE app.id=? FOR UPDATE OF app", args...).Scan(&app)
	if read.Error != nil {
		return out, read.Error
	}
	if read.RowsAffected != 1 {
		return out, echo.NewHTTPError(http.StatusNotFound, "Заявка не найдена")
	}
	out.Status, out.Confirmation, out.StatusUpdatedAt, out.Archived = app.Status, app.Confirmation, app.StatusUpdatedAt, app.Archived
	if err := tx.Raw("SELECT id,application_id,attachment_type,status,is_manual,updated_at,entry_date_from,entry_date_to,entry_time_from,entry_time_to FROM attachments WHERE id IN ? ORDER BY id FOR UPDATE", ids).Scan(&out.Attachments).Error; err != nil {
		return out, err
	}
	if len(out.Attachments) != len(ids) {
		return out, echo.NewHTTPError(http.StatusForbidden, "Недоступные вложения")
	}
	for i, row := range out.Attachments {
		if row.ID != ids[i] || row.ApplicationID == nil || *row.ApplicationID != applicationID {
			return out, echo.NewHTTPError(http.StatusForbidden, "Недоступные вложения")
		}
	}
	// Same fixed order as the expiry writer and legacy territory reset:
	// application -> attachments -> employees -> cars. Include history rows.
	for _, target := range []struct {
		kind ElementKind
		rows *[]attachmentPeriodEntity
	}{{ElementEmployee, &out.Employees}, {ElementCar, &out.Cars}} {
		removed, byFact := "(e.is_purged OR e.date_deleted IS NOT NULL)", "FALSE"
		var entityArgs []any
		if target.kind == ElementCar {
			removed = "(e.is_purged OR e.date_removed IS NOT NULL)"
			byFact = "LOWER(REPLACE(TRIM(COALESCE(e.car_number,'')), ' ', '')) = ?"
			entityArgs = append(entityArgs, byFactCompactPlate())
		}
		entityArgs = append(entityArgs, ids)
		if err := tx.Raw("SELECT e.id,e.attachment_id,e.period_mode,e.status,e.updated_at,e.entry_date_from,e.entry_date_to,e.entry_time_from,e.entry_time_to,"+removed+" AS removed,"+byFact+" AS by_fact FROM "+string(target.kind)+" e WHERE e.attachment_id IN ? ORDER BY e.id FOR UPDATE OF e", entityArgs...).Scan(target.rows).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}

func planAttachmentPeriodWrites(snapshot attachmentPeriodSnapshot, next models.EntryPeriod, policy string, now time.Time) ([]int, []attachmentPeriodWrite, error) {
	parents := make(map[int]models.EntryPeriod, len(snapshot.Attachments))
	var changedParents []int
	for _, row := range snapshot.Attachments {
		parents[row.ID] = row.period()
		if !equalEntityPeriod(row.period(), next) {
			changedParents = append(changedParents, row.ID)
		}
	}
	var writes []attachmentPeriodWrite
	for _, target := range []struct {
		kind ElementKind
		rows []attachmentPeriodEntity
	}{{ElementCar, snapshot.Cars}, {ElementEmployee, snapshot.Employees}} {
		for _, row := range target.rows {
			mode := row.PeriodMode
			if mode == "" {
				mode = models.PeriodInherit
			}
			if mode != models.PeriodInherit && mode != models.PeriodIndividual {
				return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим срока записи")
			}
			oldEffective, err := models.ResolveEntityPeriod(mode, row.period(), parents[row.AttachmentID], false)
			if err != nil {
				return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный индивидуальный срок записи")
			}
			newOwn := row.period()
			if mode == models.PeriodIndividual && policy == IndividualPolicyReplace || mode == models.PeriodInherit && target.kind == ElementCar {
				newOwn = next
			}
			newEffective, err := models.ResolveEntityPeriod(mode, newOwn, next, false)
			if err != nil {
				return nil, nil, err
			}
			effectiveChanged := !equalEntityPeriod(oldEffective.EntryPeriod, newEffective.EntryPeriod)
			ownChanged := !equalEntityPeriod(row.period(), newOwn)
			if !effectiveChanged && !ownChanged {
				continue
			}
			if target.kind == ElementCar && row.ByFact && !row.Removed && effectiveChanged && derefStr(newEffective.EntryDateTo) > ByFactMaxDate(now) {
				return nil, nil, echo.NewHTTPError(http.StatusBadRequest, byFactDeadlineHint(now))
			}
			writes = append(writes, attachmentPeriodWrite{Kind: target.kind, ID: row.ID, AttachmentID: row.AttachmentID, Mode: mode, OldOwn: row.period(), NewOwn: newOwn, OldEffective: oldEffective.EntryPeriod, NewEffective: newEffective.EntryPeriod, WriteOwn: ownChanged})
		}
	}
	return changedParents, writes, nil
}

func attachmentPeriodRevision(actorID int, snapshot attachmentPeriodSnapshot, req ChangeAttachmentPeriodRequest) (string, error) {
	req.ExpectedRevision = ""
	// Free text can contain personal data and does not change the affected set.
	req.Reason = ""
	if snapshot.StatusUpdatedAt != nil {
		stamp := canonicalEntityPeriodTime(*snapshot.StatusUpdatedAt)
		snapshot.StatusUpdatedAt = &stamp
	}
	snapshot.Attachments = append([]attachmentPeriodRow(nil), snapshot.Attachments...)
	snapshot.Cars = append([]attachmentPeriodEntity(nil), snapshot.Cars...)
	snapshot.Employees = append([]attachmentPeriodEntity(nil), snapshot.Employees...)
	for i := range snapshot.Attachments {
		snapshot.Attachments[i].UpdatedAt = canonicalEntityPeriodTime(snapshot.Attachments[i].UpdatedAt)
	}
	for _, rows := range [][]attachmentPeriodEntity{snapshot.Cars, snapshot.Employees} {
		for i := range rows {
			rows[i].UpdatedAt = canonicalEntityPeriodTime(rows[i].UpdatedAt)
		}
	}
	raw, err := json.Marshal(struct {
		ActorID  int
		Snapshot attachmentPeriodSnapshot
		Request  ChangeAttachmentPeriodRequest
	}{actorID, snapshot, req})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func attachmentPeriodResult(actorID int, snapshot attachmentPeriodSnapshot, req ChangeAttachmentPeriodRequest, next models.EntryPeriod, parents []int, writes []attachmentPeriodWrite) (*AttachmentPeriodCommandResult, error) {
	revision, err := attachmentPeriodRevision(actorID, snapshot, req)
	if err != nil {
		return nil, err
	}
	out := &AttachmentPeriodCommandResult{ApplicationID: snapshot.ApplicationID, Attachments: []AttachmentPeriodPreviewItem{}, AttachmentIDs: append([]int{}, req.AttachmentIDs...), ChangedAttachmentIDs: append([]int{}, parents...), ChangedEmployeeIDs: []int{}, ChangedCarIDs: []int{}, AttachmentCount: len(snapshot.Attachments), IndividualPolicy: req.IndividualPolicy, NewPeriod: next, Revision: revision}
	for _, row := range snapshot.Attachments {
		item := AttachmentPeriodPreviewItem{AttachmentID: row.ID, OldPeriod: row.period(), EmployeeIDs: []int{}, CarIDs: []int{}, IndividualEmployeeIDs: []int{}, IndividualCarIDs: []int{}}
		for _, target := range []struct {
			rows            []attachmentPeriodEntity
			ids, individual *[]int
		}{{snapshot.Cars, &item.CarIDs, &item.IndividualCarIDs}, {snapshot.Employees, &item.EmployeeIDs, &item.IndividualEmployeeIDs}} {
			for _, entity := range target.rows {
				if entity.AttachmentID == row.ID {
					*target.ids = append(*target.ids, entity.ID)
					if entity.PeriodMode == models.PeriodIndividual {
						*target.individual = append(*target.individual, entity.ID)
					}
				}
			}
		}
		out.CarCount += len(item.CarIDs)
		out.EmployeeCount += len(item.EmployeeIDs)
		out.IndividualCount += len(item.IndividualCarIDs) + len(item.IndividualEmployeeIDs)
		out.Attachments = append(out.Attachments, item)
	}
	for _, write := range writes {
		if write.Kind == ElementCar {
			out.ChangedCarIDs = append(out.ChangedCarIDs, write.ID)
		} else {
			out.ChangedEmployeeIDs = append(out.ChangedEmployeeIDs, write.ID)
		}
	}
	return out, nil
}

func (s *AttachmentPeriodCommandService) run(ctx context.Context, actorID, applicationID int, input ChangeAttachmentPeriodRequest, change bool, now time.Time) (*AttachmentPeriodCommandResult, error) {
	if s == nil || s.db == nil || s.recorder == nil {
		return nil, fmt.Errorf("attachment period command dependencies are required")
	}
	if actorID <= 0 {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	if applicationID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная заявка")
	}
	req, next, err := validateAttachmentPeriodRequest(input, change, now)
	if err != nil {
		return nil, err
	}
	var result *AttachmentPeriodCommandResult
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		set, err := NewPermissionResolver(tx).Resolve(ctx, actorID)
		if err != nil {
			return err
		}
		if !set.Has(KeyApplicationPeriodChange) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		visible, err := attachmentPeriodVisible(ctx, tx, set, actorID, applicationID)
		if err != nil {
			return err
		}
		if !visible {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		snapshot, err := lockAttachmentPeriodSnapshot(tx, applicationID, req.AttachmentIDs)
		if err != nil {
			return err
		}
		set, err = NewPermissionResolver(tx).Resolve(ctx, actorID)
		if err != nil {
			return err
		}
		if !set.Has(KeyApplicationPeriodChange) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		visible, err = attachmentPeriodVisible(ctx, tx, set, actorID, applicationID)
		if err != nil {
			return err
		}
		if !visible {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		// Access is checked first. A changed lifecycle/composition then produces
		// the same stale-preview conflict as a changed period, before planning.
		if change {
			revision, err := attachmentPeriodRevision(actorID, snapshot, req)
			if err != nil {
				return err
			}
			if req.ExpectedRevision != revision {
				return echo.NewHTTPError(http.StatusConflict, "Срок, состояние или состав изменились. Обновите предварительный просмотр")
			}
		}
		if err := AuthorizeAttachmentPeriodChange(set, PeriodChangeTarget{Visible: visible, Archived: snapshot.Archived, ApplicationStatus: snapshot.Status}); err != nil {
			return err
		}
		if err := validateAttachmentPeriodConfirmation(snapshot.Confirmation); err != nil {
			return err
		}
		parents, writes, err := planAttachmentPeriodWrites(snapshot, next, req.IndividualPolicy, now)
		if err != nil {
			return err
		}
		result, err = attachmentPeriodResult(actorID, snapshot, req, next, parents, writes)
		if err != nil || !change {
			return err
		}
		if len(parents) == 0 && len(writes) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Срок не изменился")
		}
		before := *result
		writtenAt := canonicalEntityPeriodTime(now)
		for i := range snapshot.Attachments {
			row := &snapshot.Attachments[i]
			if equalEntityPeriod(row.period(), next) {
				continue
			}
			write := tx.Table("attachments").Where("id=? AND application_id=?", row.ID, applicationID).Updates(attachmentPeriodUpdates(next, writtenAt))
			if write.Error != nil {
				return write.Error
			}
			if write.RowsAffected != 1 {
				return echo.NewHTTPError(http.StatusConflict, "Состав вложений изменился")
			}
			row.EntryDateFrom, row.EntryDateTo, row.EntryTimeFrom, row.EntryTimeTo, row.UpdatedAt = next.EntryDateFrom, next.EntryDateTo, next.EntryTimeFrom, next.EntryTimeTo, writtenAt
		}
		for _, write := range writes {
			if write.WriteOwn {
				updated := tx.Table(string(write.Kind)).Where("id=? AND attachment_id=?", write.ID, write.AttachmentID).Updates(attachmentPeriodUpdates(write.NewOwn, writtenAt))
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return echo.NewHTTPError(http.StatusConflict, "Состав записей изменился")
				}
				rows := snapshot.Employees
				if write.Kind == ElementCar {
					rows = snapshot.Cars
				}
				for i := range rows {
					if rows[i].ID == write.ID {
						rows[i].EntryDateFrom, rows[i].EntryDateTo, rows[i].EntryTimeFrom, rows[i].EntryTimeTo, rows[i].UpdatedAt = write.NewOwn.EntryDateFrom, write.NewOwn.EntryDateTo, write.NewOwn.EntryTimeFrom, write.NewOwn.EntryTimeTo, writtenAt
					}
				}
			}
			oldText, newText := attachmentPeriodText(write.OldEffective), attachmentPeriodText(write.NewEffective)
			entityType := models.AuditEntityEmployee
			if write.Kind == ElementCar {
				entityType = models.AuditEntityCar
			}
			details := struct {
				OldValue     string             `json:"old_value"`
				NewValue     string             `json:"new_value"`
				Comment      string             `json:"comment"`
				AttachmentID int                `json:"attachment_id"`
				OldMode      models.PeriodMode  `json:"old_period_mode"`
				NewMode      models.PeriodMode  `json:"new_period_mode"`
				OldPeriod    models.EntryPeriod `json:"old_period"`
				NewPeriod    models.EntryPeriod `json:"new_period"`
			}{oldText, newText, req.Reason, write.AttachmentID, write.Mode, write.Mode, write.OldEffective, write.NewEffective}
			if err := s.recorder.Record(ctx, tx, entityType, &write.ID, models.AuditActionDatesChanged, &actorID, details); err != nil {
				return err
			}
		}
		result, err = attachmentPeriodResult(actorID, snapshot, req, next, parents, writes)
		if err != nil {
			return err
		}
		oldPeriods := make([]applicationPeriod, 0, len(before.Attachments))
		for _, item := range before.Attachments {
			oldPeriods = append(oldPeriods, applicationPeriod{DateFrom: derefStr(item.OldPeriod.EntryDateFrom), DateTo: derefStr(item.OldPeriod.EntryDateTo), TimeFrom: derefStr(item.OldPeriod.EntryTimeFrom), TimeTo: derefStr(item.OldPeriod.EntryTimeTo)})
		}
		details := struct {
			OldValue             string                        `json:"old_value"`
			NewValue             string                        `json:"new_value"`
			Comment              string                        `json:"comment"`
			IndividualPolicy     string                        `json:"individual_policy"`
			Attachments          []AttachmentPeriodPreviewItem `json:"attachments"`
			ChangedAttachmentIDs []int                         `json:"changed_attachment_ids"`
			ChangedEmployeeIDs   []int                         `json:"changed_employee_ids"`
			ChangedCarIDs        []int                         `json:"changed_car_ids"`
		}{formatApplicationPeriods(oldPeriods), attachmentPeriodText(next), req.Reason, req.IndividualPolicy, before.Attachments, parents, result.ChangedEmployeeIDs, result.ChangedCarIDs}
		return s.recorder.Record(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionDatesChanged, &actorID, details)
	})
	if err != nil {
		return nil, err
	}
	if change && s.afterChange != nil {
		s.afterChange(ctx, actorID, *result, req.Reason)
	}
	return result, nil
}

// Preserve the existing date writer's final-confirmation gate.
func validateAttachmentPeriodConfirmation(confirmation string) error {
	if confirmation != "" && confirmation != models.ConfirmationPending && confirmation != models.ConfirmationApproved {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("По заявке уже есть итог согласования («%s»): срок менять нельзя", confirmation))
	}
	return nil
}

func attachmentPeriodUpdates(period models.EntryPeriod, writtenAt time.Time) map[string]any {
	return map[string]any{"entry_date_from": period.EntryDateFrom, "entry_date_to": period.EntryDateTo, "entry_time_from": period.EntryTimeFrom, "entry_time_to": period.EntryTimeTo, "updated_at": writtenAt}
}

func attachmentPeriodText(period models.EntryPeriod) string {
	return formatApplicationPeriod(applicationPeriod{DateFrom: derefStr(period.EntryDateFrom), DateTo: derefStr(period.EntryDateTo), TimeFrom: derefStr(period.EntryTimeFrom), TimeTo: derefStr(period.EntryTimeTo)})
}
