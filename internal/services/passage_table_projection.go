package services

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Explicit aliases keep audit IDs separate from entity IDs in embedded scans.
func tablePassageProjectionSelectSQL(entity, projection string) string {
	return entity + ".id AS passage_entity_id," + entity + ".territory_status AS passage_territory_status," +
		projection + ".id AS passage_id," + projection + ".action AS passage_action," +
		projection + ".created_at AS passage_created_at," + projection + ".actor_user_id AS passage_actor_user_id," +
		projection + ".table_id AS passage_table_id," + projection + ".closes_unknown_entry AS passage_closes_unknown_entry," +
		projection + ".previous_id AS passage_previous_id," + projection + ".previous_action AS passage_previous_action," +
		projection + ".previous_at AS passage_previous_at"
}

type tablePassagePeriodRow struct {
	EntryDateFrom *string
	EntryDateTo   *string
	EntryTimeFrom *string
	EntryTimeTo   *string
	Bounded       bool
	Source        string
	ValidMode     bool
}

func (p tablePassagePeriodRow) effective() models.EffectivePeriod {
	return models.EffectivePeriod{EntryPeriod: models.EntryPeriod{
		EntryDateFrom: p.EntryDateFrom, EntryDateTo: p.EntryDateTo,
		EntryTimeFrom: p.EntryTimeFrom, EntryTimeTo: p.EntryTimeTo,
	}, Bounded: p.Bounded, Source: p.Source}
}

func tablePassagePeriodSelectSQL(p EffectivePeriodSQL) string {
	return p.DateFrom + " AS effective_entry_date_from," + p.DateTo + " AS effective_entry_date_to," +
		p.TimeFrom + " AS effective_entry_time_from," + p.TimeTo + " AS effective_entry_time_to," +
		p.Bounded + " AS effective_bounded," + p.Source + " AS effective_source," + p.ValidMode + " AS effective_valid_mode"
}

// Preserve the table's calendar-day visibility for ordinary active rows; exact
// whole-window admission is computed separately. Retention grants no scope.
func tablePassageEligibilitySQL(entity, projection string, period EffectivePeriodSQL, now time.Time) (string, []any, error) {
	retention, args, err := PassageRetentionSQL(entity, projection, now)
	if err != nil {
		return "", nil, err
	}
	condition := "((" + entity + ".status=1 AND " + period.ValidMode + " AND (" + period.Source +
		" = 'manual_unbounded' OR ?::date BETWEEN (" + period.DateFrom + ")::date AND (" + period.DateTo + ")::date)) OR " + retention + ")"
	return condition, append([]any{now.In(MoscowLocation()).Format("2006-01-02")}, args...), nil
}

// Establish the repeatable-read snapshot before capturing the response clock.
// A concurrent event committed before this statement is not a future event.
func establishPassageReadClock(tx *gorm.DB, requested time.Time) (time.Time, error) {
	var one int
	if err := tx.Raw("SELECT 1").Scan(&one).Error; err != nil {
		return time.Time{}, err
	}
	if !requested.IsZero() {
		return requested.UTC(), nil
	}
	return time.Now().UTC(), nil
}

// Both ordinary and fact fallback reads use one snapshot and one clock. The
// service has no actor: viewer admission stays denied until handler enrichment.
func (s *carService) readTableCars(ctx context.Context, tableID *int, fact bool) ([]TableCarResponse, error) {
	var out []TableCarResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := establishPassageReadClock(tx, time.Time{})
		if err != nil {
			return err
		}
		reader := *s
		reader.db = tx
		rows := make([]tableCarRow, 0)
		if !fact {
			err = reader.tableCarsBase(ctx, tableID, now).
				Where("LOWER(TRIM(c.car_number)) != ?", "по факту").Order("c.car_number").Scan(&rows).Error
		} else {
			period, buildErr := EntityEffectivePeriodSQL("c", "a")
			if buildErr != nil {
				return buildErr
			}
			err = reader.tableCarsBase(ctx, tableID, now).
				Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
				Order("organization, " + period.DateTo).Scan(&rows).Error
			if err == nil && len(rows) == 0 {
				err = reader.tableCarsBase(ctx, tableID, now).
					Where("c.car_number ILIKE ? OR c.car_number ILIKE ? OR c.car_number ILIKE ?", "%по факту%", "%пофакту%", "%факт%").
					Order("organization, " + period.DateTo).Scan(&rows).Error
			}
		}
		if err != nil {
			return err
		}
		out, err = reader.enrichTableCars(ctx, rows, now)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching table cars")
	}
	return out, nil
}

// LoadTablePassageResults decorates already selected IDs with fresh table scope,
// managed rights, state and admission in one batch. Missing IDs MUST be removed
// from the HTTP response: a binding/lifecycle may have changed since selection.
// Handlers pass zero now to capture a fresh clock after snapshot establishment.
// A nonzero clock is reserved for deterministic internal/test reads.
func LoadTablePassageResults(ctx context.Context, db *gorm.DB, actor int, kind ElementKind, ids []int, tableID int, now time.Time) (map[int]PassageResult, error) {
	out := make(map[int]PassageResult)
	if err := passageIdentity(actor, kind, 1); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, fmt.Errorf("passage table database required")
	}
	if tableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Выберите таблицу")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная запись")
		}
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		captured, err := establishPassageReadClock(tx, now)
		if err != nil {
			return err
		}
		set, err := NewPermissionResolver(tx).Resolve(ctx, actor)
		if err != nil {
			return err
		}
		var table models.SystemTable
		read := tx.Select("id,name,table_type,is_active,status").First(&table, tableID)
		if read.Error != nil && read.Error != gorm.ErrRecordNotFound {
			return read.Error
		}
		if read.Error != nil || set.IsBanned() || !table.IsActive || table.Status != "active" ||
			(kind == ElementCar && table.TableType != models.TableTypeCars) ||
			(kind == ElementEmployee && table.TableType != models.TableTypePeople) || !set.Has("table."+table.Name+".view") {
			return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к таблице")
		}
		if len(ids) == 0 {
			return nil
		}
		projection, err := PassageProjectionSQL(kind, "e", "p")
		if err != nil {
			return err
		}
		period, err := EntityEffectivePeriodSQL("e", "a")
		if err != nil {
			return err
		}
		binding := elementBindingTables[kind]
		eligibility, eligibilityArgs, err := tablePassageEligibilitySQL("e", "p", period, captured)
		if err != nil {
			return err
		}
		removed := "e.date_removed IS NULL"
		if kind == ElementEmployee {
			removed = "e.date_deleted IS NULL"
		}
		archive, archiveArgs := archivedApplicationCond("app")
		var rows []struct {
			Projection       passageProjectionRow  `gorm:"embedded;embeddedPrefix:passage_"`
			Period           tablePassagePeriodRow `gorm:"embedded;embeddedPrefix:effective_"`
			Status           *int
			AttachmentStatus *int
		}
		query := tx.Table(string(kind)+" e").
			Joins("JOIN attachments a ON a.id=e.attachment_id").
			Joins("LEFT JOIN applications app ON app.id=a.application_id").Joins(projection).
			Where("e.id IN ? AND NOT e.is_purged AND "+removed, ids).
			Where("EXISTS(SELECT 1 FROM "+binding.table+" b WHERE b."+binding.column+"=e.id AND b.table_id=?)", tableID).
			Where(eligibility, eligibilityArgs...).
			Where("a.is_manual OR (app.confirmation=? AND app.status IN ? AND NOT "+archive+")",
				append([]any{models.ConfirmationApproved, []string{models.StatusInWork, models.StatusCompleted}}, archiveArgs...)...)
		if err := query.Select(tablePassageProjectionSelectSQL("e", "p") + "," + tablePassagePeriodSelectSQL(period) + ",e.status,a.status AS attachment_status").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			state, err := projectPassageRow(row.Projection, captured)
			if err != nil {
				return err
			}
			effective := row.Period.effective()
			valid, reason := passageWindow(effective, captured)
			if !row.Period.ValidMode {
				valid, reason = false, "invalid_period"
			}
			active := row.Status != nil && *row.Status == 1 && row.AttachmentStatus != nil && *row.AttachmentStatus == 1
			state.CanCorrect = state.Open && set.Has(passageCorrectionPermission)
			state.CanRevertCorrection = row.Projection.Action == PassageCorrectionAction && row.Projection.CreatedAt != nil && captured.Sub(*row.Projection.CreatedAt) <= passageRevertWindow && set.Has(passageCorrectionPermission)
			out[row.Projection.EntityID] = PassageResult{EntityID: row.Projection.EntityID, EntityKind: kind,
				PassageState: state, EffectivePeriod: effective, ServerNow: captured,
				Admission: PassageAdmission{CanEnter: valid && active && !state.Open && set.Has("table."+table.Name+".entry"),
					CanExit: state.Open && set.Has("table."+table.Name+".exit"), Reason: reason}}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return out, nil
}
