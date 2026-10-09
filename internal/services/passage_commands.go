package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"systemburo/internal/models"
	"time"
	"unicode/utf8"
)

type PassageCommandService struct {
	db          *gorm.DB
	recorder    AuditRecorder
	now         func() time.Time
	afterChange func(context.Context, ElementKind, int)
}

func NewPassageCommandService(db *gorm.DB, recorder AuditRecorder) *PassageCommandService {
	clock := func() time.Time { return time.Now().UTC() }
	if db != nil && db.NowFunc != nil {
		// A trusted local GORM session may supply a historical clock. Admission,
		// audit auto timestamps and cache updates must use the same clock.
		clock = func() time.Time { return db.NowFunc().UTC() }
	}
	return &PassageCommandService{db: db, recorder: recorder, now: clock}
}

// Configure before serving; publishing must never happen inside a transaction.
func (s *PassageCommandService) SetAfterChange(fn func(context.Context, ElementKind, int)) {
	s.afterChange = fn
}

type PassageCommandRequest struct {
	TableID             *int          `json:"table_id"`
	ExpectedLastEventID *int64        `json:"expected_last_event_id"`
	TerritoryStatus     int           `json:"territory_status"`
	Reason              string        `json:"reason"`
	Pass                *FactPassData `json:"pass,omitempty"`
}

func passageIdentity(actor int, kind ElementKind, id int) error {
	if actor <= 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	if _, err := passageEntityType(kind); err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректная запись")
	}
	return nil
}

func passageAccess(ctx context.Context, tx *gorm.DB, actor int, kind ElementKind, id int, tableID *int, verb string, locked bool) (PermissionSet, error) {
	resolver := NewPermissionResolver(tx)
	set, err := resolver.Resolve(ctx, actor)
	if err != nil {
		return set, err
	}
	if set.IsBanned() {
		return set, echo.NewHTTPError(http.StatusForbidden, "Нет доступа")
	}
	if tableID == nil || *tableID <= 0 {
		return set, echo.NewHTTPError(http.StatusBadRequest, "Выберите таблицу")
	}
	var table models.SystemTable
	if err := tx.Select("id,name,table_type,is_active,status").First(&table, *tableID).Error; err != nil {
		return set, echo.NewHTTPError(http.StatusForbidden, "Нет доступа к таблице")
	}
	if !table.IsActive || table.Status != "active" || kind == ElementEmployee && table.TableType != models.TableTypePeople || kind == ElementCar && table.TableType != models.TableTypeCars {
		return set, echo.NewHTTPError(http.StatusForbidden, "Нет доступа к таблице")
	}
	if !set.Has("table."+table.Name+".view") || verb != "" && !set.Has("table."+table.Name+"."+verb) {
		return set, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
	}
	if locked {
		if err := checkEntityPeriodTableContext(tx, set, kind, id, tableID); err != nil {
			return set, err
		}
	} else {
		binding := elementBindingTables[kind]
		var exists bool
		if err := tx.Raw("SELECT EXISTS(SELECT 1 FROM "+binding.table+" WHERE "+binding.column+"=? AND table_id=?)", id, *tableID).Scan(&exists).Error; err != nil {
			return set, err
		}
		if !exists {
			// Preserve the missing-record contract only after table authorization.
			// An existing record on another post remains forbidden.
			var recordExists bool
			if err := tx.Raw("SELECT EXISTS(SELECT 1 FROM "+string(kind)+" WHERE id=?)", id).Scan(&recordExists).Error; err != nil {
				return set, err
			}
			if !recordExists {
				return set, echo.NewHTTPError(http.StatusNotFound, "Запись не найдена")
			}
			return set, echo.NewHTTPError(http.StatusForbidden, "Нет доступа к записи в выбранной таблице")
		}
	}
	return set, nil
}

func loadPassageRecord(ctx context.Context, tx *gorm.DB, kind ElementKind, id int) (passageProjectionRow, error) {
	join, err := PassageProjectionSQL(kind, "e", "p")
	if err != nil {
		return passageProjectionRow{}, err
	}
	var row passageProjectionRow
	err = tx.WithContext(ctx).Raw("SELECT e.id AS entity_id,e.territory_status,p.* FROM "+string(kind)+" e "+join+" WHERE e.id=?", id).Scan(&row).Error
	return row, err
}

func passageExpected(expected *int64, actual int64, required bool) error {
	if expected == nil {
		if required {
			return echo.NewHTTPError(http.StatusBadRequest, "Обновите состояние прохода")
		}
		return nil
	}
	if *expected < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректная отметка")
	}
	if *expected != actual {
		return echo.NewHTTPError(http.StatusConflict, "Отметка изменилась. Обновите таблицу")
	}
	return nil
}

func passageLifecycle(snapshot entityPeriodSnapshot, period models.EffectivePeriod, now time.Time) (bool, string) {
	if snapshot.Removed {
		return false, "removed"
	}
	if snapshot.Archived {
		return false, "archived"
	}
	if !snapshot.Manual {
		if snapshot.ApplicationStatus == models.StatusWithdrawn {
			return false, "withdrawn"
		}
		if snapshot.ApplicationStatus != models.StatusInWork && snapshot.ApplicationStatus != models.StatusCompleted {
			return false, "inactive"
		}
		if snapshot.Confirmation != models.ConfirmationApproved {
			return false, "inactive"
		}
	}
	valid, reason := passageWindow(period, now)
	if !valid {
		return false, reason
	}
	if snapshot.Status == nil || *snapshot.Status != 1 || snapshot.AttachmentStatus == nil || *snapshot.AttachmentStatus != 1 {
		return false, "inactive"
	}
	return true, ""
}

func (s *PassageCommandService) Mark(ctx context.Context, actor int, kind ElementKind, id int, req PassageCommandRequest) (*PassageResult, error) {
	action, err := passageActionFor(req.TerritoryStatus)
	if err != nil {
		return nil, err
	}
	return s.runOrdinary(ctx, actor, kind, id, req, action, false)
}

func (s *PassageCommandService) Revert(ctx context.Context, actor int, kind ElementKind, id int, req PassageCommandRequest) (*PassageResult, error) {
	action, err := passageActionFor(req.TerritoryStatus)
	if err != nil {
		return nil, err
	}
	return s.runOrdinary(ctx, actor, kind, id, req, action, true)
}

func (s *PassageCommandService) runOrdinary(ctx context.Context, actor int, kind ElementKind, id int, req PassageCommandRequest, action string, revert bool) (*PassageResult, error) {
	if err := passageIdentity(actor, kind, id); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil || s.recorder == nil {
		return nil, fmt.Errorf("passage service dependencies required")
	}
	if revert && (strings.TrimSpace(req.Reason) == "" || utf8.RuneCountInString(req.Reason) > 1000) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите причину отмены")
	}
	var out *PassageResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := passageAccess(ctx, tx, actor, kind, id, req.TableID, action, false); err != nil {
			return err
		}
		// Shared lock helper preserves app -> attachment -> selected entity order
		// and revalidates locator after concurrent reparent/expiry/group writers.
		snapshot, err := lockEntityPeriodSnapshot(tx, kind, id)
		if err != nil {
			return err
		}
		if _, err := passageAccess(ctx, tx, actor, kind, id, req.TableID, action, true); err != nil {
			return err
		}
		now := s.now().UTC()
		record, err := loadPassageRecord(ctx, tx, kind, id)
		if err != nil {
			return err
		}
		state, err := projectPassageRow(record, now)
		if err != nil {
			return err
		}
		if err := passageExpected(req.ExpectedLastEventID, state.LastEventID, false); err != nil {
			return err
		}
		period, periodErr := models.ResolveEntityPeriod(snapshot.PeriodMode, snapshot.Own, snapshot.Parent, snapshot.Manual)
		if snapshot.Removed || snapshot.Archived || !snapshot.Manual && (snapshot.ApplicationStatus != models.StatusInWork && snapshot.ApplicationStatus != models.StatusCompleted || snapshot.Confirmation != models.ConfirmationApproved) {
			return echo.NewHTTPError(http.StatusForbidden, "Основание недоступно для отметок поста")
		}
		entityType, _ := passageEntityType(kind)
		if revert {
			if record.Action != action {
				return echo.NewHTTPError(http.StatusConflict, "Последняя отметка изменилась")
			}
			if record.TableID != nil && *record.TableID != *req.TableID {
				return echo.NewHTTPError(http.StatusConflict, "Отмените отметку на том же посту")
			}
			admin, err := isPassageRevertAdmin(ctx, tx, actor)
			if err != nil {
				return err
			}
			if !admin && (record.ActorUserID == nil || *record.ActorUserID != actor || record.CreatedAt == nil || now.Sub(*record.CreatedAt) > passageRevertWindow) {
				return echo.NewHTTPError(http.StatusForbidden, "Свою отметку можно отменить в течение 15 минут")
			}
			reason := strings.TrimSpace(req.Reason)
			details := passageRevertDetails{RevertsID: int(record.ID), Comment: &reason, TableID: req.TableID}
			if subject := passageEntitySubject(ctx, tx, entityType, id); subject != "" {
				details.Subject = &subject
			}
			if err := s.recorder.Record(ctx, tx, entityType, &id, passageRevertActionFor(action), &actor, details); err != nil {
				return err
			}
			if action == "entry" && record.PreviousID == 0 {
				if err := tx.Table(string(kind)).Where("id=?", id).Update("territory_status", nil).Error; err != nil {
					return err
				}
			}
			if action == "exit" && record.ClosesUnknownEntry && record.PreviousID == 0 {
				if err := tx.Table(string(kind)).Where("id=?", id).Update("territory_status", 1).Error; err != nil {
					return err
				}
			}
		} else {
			if action == "entry" {
				valid, reason := passageLifecycle(snapshot, period, now)
				if periodErr != nil || !valid {
					return echo.NewHTTPError(http.StatusUnprocessableEntity, "Вход запрещён: "+reason)
				}
				if state.Open {
					return echo.NewHTTPError(http.StatusConflict, "Вход уже открыт")
				}
			} else if !state.Open {
				return echo.NewHTTPError(http.StatusConflict, "Нет открытого входа")
			}
			details := map[string]any{"table_id": *req.TableID, "comment": "Отметка прохода"}
			if action == "exit" && state.Open && !state.EntryTimeKnown {
				details["closes_unknown_entry"] = true
			}
			if subject := passageEntitySubject(ctx, tx, entityType, id); subject != "" {
				details["subject"] = subject
			}
			if action == "entry" && req.Pass != nil && strings.TrimSpace(req.Pass.Number) != "" {
				raw, err := json.Marshal(req.Pass)
				if err != nil {
					return err
				}
				details["metadata"] = json.RawMessage(raw)
			}
			if err := s.recorder.Record(ctx, tx, entityType, &id, action, &actor, details); err != nil {
				return err
			}
		}
		var errResult error
		out, errResult = s.finish(ctx, tx, actor, kind, id, snapshot, req.TableID, period)
		return errResult
	})
	if err != nil {
		return nil, err
	}
	if s.afterChange != nil {
		s.afterChange(ctx, kind, id)
	}
	return out, nil
}

func (s *PassageCommandService) finish(ctx context.Context, tx *gorm.DB, actor int, kind ElementKind, id int, snapshot entityPeriodSnapshot, tableID *int, period models.EffectivePeriod) (*PassageResult, error) {
	record, err := loadPassageRecord(ctx, tx, kind, id)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if record.CreatedAt != nil && record.CreatedAt.After(now) {
		now = record.CreatedAt.UTC()
	}
	state, err := projectPassageRow(record, now)
	if err != nil {
		return nil, err
	}
	status := PassageTerritoryStatus(state, record.TerritoryStatus)
	entryAt := state.EntryAt
	// The legacy cache retains the last effective entry even after departure.
	// It does not make the closed passage open or create a new grace window.
	if !state.Open && record.PreviousAction == "entry" {
		entryAt = record.PreviousAt
	}
	updates := map[string]any{"territory_status": status, "territory_entry_time": entryAt, "updated_at": now}
	if err := tx.Table(string(kind)).Where("id=?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	admission := PassageAdmission{}
	if tableID != nil {
		set, err := passageAccess(ctx, tx, actor, kind, id, tableID, "", true)
		if err != nil {
			return nil, err
		}
		var name string
		if err := tx.Table("system_tables").Select("name").Where("id=?", *tableID).Scan(&name).Error; err != nil {
			return nil, err
		}
		valid, reason := passageLifecycle(snapshot, period, now)
		admission = PassageAdmission{CanEnter: valid && !state.Open && set.Has("table."+name+".entry"), CanExit: state.Open && set.Has("table."+name+".exit"), Reason: reason}
	}
	return &PassageResult{EntityID: id, EntityKind: kind, PassageState: state, EffectivePeriod: period, Admission: admission, ServerNow: now}, nil
}
