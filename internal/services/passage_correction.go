package services

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"systemburo/internal/models"
	"time"
	"unicode/utf8"
)

type PassageCorrectionRequest struct {
	Source              string     `json:"source"`
	TableID             *int       `json:"table_id"`
	ExpectedLastEventID *int64     `json:"expected_last_event_id"`
	Reason              string     `json:"reason"`
	ActualExitAt        *time.Time `json:"actual_exit_at"`
}

// The key is catalog-owned by the integrator; no role/grant bypass belongs here.
const passageCorrectionPermission = "detail.passage.correct"

func passageCorrectionAccess(ctx context.Context, tx *gorm.DB, actor int, kind ElementKind, id int, req PassageCorrectionRequest, locked bool) (PermissionSet, error) {
	resolver := NewPermissionResolver(tx)
	set, err := resolver.Resolve(ctx, actor)
	if err != nil {
		return set, err
	}
	if !set.Has(passageCorrectionPermission) {
		return set, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для исправления учёта")
	}
	if req.Source == "table" {
		return passageAccess(ctx, tx, actor, kind, id, req.TableID, "", locked)
	}
	if req.Source != "admin_summary" || req.TableID != nil {
		return set, echo.NewHTTPError(http.StatusBadRequest, "Некорректный источник исправления")
	}
	scopes := NewElementScopeResolver(tx, resolver)
	scope, err := scopes.Resolve(ctx, actor)
	if err != nil {
		return set, err
	}
	if !set.IsAdmin() && !set.IsSuperAdmin() && !scope.Approver {
		return set, echo.NewHTTPError(http.StatusForbidden, "Нет доступа к административной сводке")
	}
	visible, err := scopes.Visible(ctx, scope, kind, id)
	if err != nil {
		return set, err
	}
	if !visible {
		return set, echo.NewHTTPError(http.StatusForbidden, "Запись недоступна")
	}
	return set, nil
}

func (s *PassageCommandService) Correct(ctx context.Context, actor int, kind ElementKind, id int, req PassageCorrectionRequest) (*PassageResult, error) {
	return s.runCorrection(ctx, actor, kind, id, req, false)
}
func (s *PassageCommandService) RevertCorrection(ctx context.Context, actor int, kind ElementKind, id int, req PassageCorrectionRequest) (*PassageResult, error) {
	return s.runCorrection(ctx, actor, kind, id, req, true)
}

func (s *PassageCommandService) runCorrection(ctx context.Context, actor int, kind ElementKind, id int, req PassageCorrectionRequest, revert bool) (*PassageResult, error) {
	if err := passageIdentity(actor, kind, id); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil || s.recorder == nil {
		return nil, fmt.Errorf("passage service dependencies required")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 1000 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите причину длиной от 1 до 1000 символов")
	}
	if req.ExpectedLastEventID == nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Обновите состояние прохода")
	}
	var out *PassageResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := passageCorrectionAccess(ctx, tx, actor, kind, id, req, false); err != nil {
			return err
		}
		snapshot, err := lockEntityPeriodSnapshot(tx, kind, id)
		if err != nil {
			return err
		}
		if _, err := passageCorrectionAccess(ctx, tx, actor, kind, id, req, true); err != nil {
			return err
		}
		if req.Source == "table" {
			if err := passageCorrectionTableLifecycle(snapshot); err != nil {
				return err
			}
		}
		var purged bool
		if err := tx.Table(string(kind)).Select("is_purged").Where("id=?", id).Scan(&purged).Error; err != nil {
			return err
		}
		if purged {
			return echo.NewHTTPError(http.StatusForbidden, "Запись окончательно удалена")
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
		if err := passageExpected(req.ExpectedLastEventID, state.LastEventID, true); err != nil {
			return err
		}
		entityType, _ := passageEntityType(kind)
		if revert {
			if record.Action != PassageCorrectionAction || record.CreatedAt == nil {
				return echo.NewHTTPError(http.StatusConflict, "Последнее событие не является исправлением")
			}
			if now.Sub(*record.CreatedAt) > passageRevertWindow {
				return echo.NewHTTPError(http.StatusForbidden, "Исправление можно отменить в течение 15 минут")
			}
			var known bool
			if err := tx.Raw("SELECT COALESCE((details->>'entry_time_known')::boolean,FALSE) FROM audit_log WHERE id=?", record.ID).Scan(&known).Error; err != nil {
				return err
			}
			details := map[string]any{"reverts_id": record.ID, "comment": reason, "table_id": req.TableID}
			if err := s.recorder.Record(ctx, tx, entityType, &id, PassageCorrectionRevertAction, &actor, details); err != nil {
				return err
			}
			if !known && record.PreviousID == 0 {
				if err := tx.Table(string(kind)).Where("id=?", id).Update("territory_status", 1).Error; err != nil {
					return err
				}
			}
		} else {
			if !state.Open {
				return echo.NewHTTPError(http.StatusConflict, "Нет незакрытого входа")
			}
			if req.ActualExitAt != nil {
				if state.EntryAt == nil || req.ActualExitAt.Before(*state.EntryAt) || req.ActualExitAt.After(now) {
					return echo.NewHTTPError(http.StatusBadRequest, "Фактический выход должен быть между входом и текущим временем")
				}
			}
			details := map[string]any{"comment": reason, "reason": reason, "table_id": req.TableID, "actual_exit_at": req.ActualExitAt, "entry_time_known": state.EntryTimeKnown, "entry_event_id": state.LastEventID, "source": req.Source}
			details["metadata"] = map[string]any{"actual_exit_at": req.ActualExitAt, "entry_time_known": state.EntryTimeKnown, "source": req.Source}
			// Deliberately not action=exit: correction is not an observed departure.
			if err := s.recorder.Record(ctx, tx, entityType, &id, PassageCorrectionAction, &actor, details); err != nil {
				return err
			}
		}
		period, _ := models.ResolveEntityPeriod(snapshot.PeriodMode, snapshot.Own, snapshot.Parent, snapshot.Manual)
		// Revoked/archive correction never grants ordinary guard admission.
		out, err = s.finish(ctx, tx, actor, kind, id, snapshot, nil, period)
		if err != nil {
			return err
		}
		out.PassageState.CanCorrect = out.PassageState.Open
		out.PassageState.CanRevertCorrection = !revert
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.afterChange != nil {
		s.afterChange(ctx, kind, id)
	}
	return out, nil
}

// Existing binding does not reopen an explicitly withdrawn, archived or removed
// foundation. Only the separate scoped administrative source may correct it.
func passageCorrectionTableLifecycle(snapshot entityPeriodSnapshot) error {
	if snapshot.Removed || snapshot.Archived || !snapshot.Manual && (snapshot.ApplicationStatus != models.StatusInWork && snapshot.ApplicationStatus != models.StatusCompleted || snapshot.Confirmation != models.ConfirmationApproved) {
		return echo.NewHTTPError(http.StatusForbidden, "Исправление недоступного основания выполняется в административной сводке")
	}
	return nil
}
