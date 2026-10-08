package services

import (
	"context"
	"database/sql"
	"fmt"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"time"
)

type passageCurrentItem struct {
	EntityID        int
	TerritoryStatus int
	EntryTime       *string
	LastExitTime    *string
	CanRevert       bool
	LastMarkTableID *int
	PassageState    PassageState
	EffectivePeriod models.EffectivePeriod
	ServerNow       time.Time
}

// A status read never trusts daily-reset cache columns over live passage events.
// The supplied scope is intersected with freshly resolved existing visibility.
func loadPassageCurrent(ctx context.Context, db *gorm.DB, viewerID int, kind ElementKind, supplied ElementScope) ([]passageCurrentItem, error) {
	if err := passageIdentity(viewerID, kind, 1); err != nil {
		return nil, err
	}
	items := []passageCurrentItem{}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resolver := NewPermissionResolver(tx)
		set, err := resolver.Resolve(ctx, viewerID)
		if err != nil {
			return err
		}
		scopes := NewElementScopeResolver(tx, resolver)
		fresh, err := scopes.Resolve(ctx, viewerID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		visible, args := supplied.Predicate(kind, "e", "a", "app")
		freshSQL, freshArgs := fresh.Predicate(kind, "e", "a", "app")
		join, err := PassageProjectionSQL(kind, "e", "p")
		if err != nil {
			return err
		}
		period, err := EntityEffectivePeriodSQL("e", "a")
		if err != nil {
			return err
		}
		retention, retentionArgs, err := PassageRetentionSQL("e", "p", now)
		if err != nil {
			return err
		}
		entityType, _ := passageEntityType(kind)
		archive, archiveArgs := archivedApplicationCond("app")
		removed := "e.date_deleted IS NOT NULL"
		if kind == ElementCar {
			removed = "e.date_removed IS NOT NULL"
		}
		var rows []struct {
			Projection                                             passageProjectionRow `gorm:"embedded"`
			LastExitAt                                             *time.Time
			EntryDateFrom, EntryDateTo, EntryTimeFrom, EntryTimeTo *string
			Bounded                                                bool
			PeriodSource                                           string
			Manual                                                 bool
			Archived                                               bool
			Removed                                                bool
			ApplicationStatus                                      string
			Confirmation                                           string
		}
		selectSQL := "e.id AS entity_id,e.territory_status,p.*," + period.DateFrom + " AS entry_date_from," + period.DateTo + " AS entry_date_to," + period.TimeFrom + " AS entry_time_from," + period.TimeTo + " AS entry_time_to," + period.Source + " AS period_source," + period.Bounded + " AS bounded,a.is_manual AS manual," + removed + " AS removed," + archive + " AS archived,COALESCE(app.status,'') AS application_status,COALESCE(app.confirmation,'') AS confirmation,observed.created_at AS last_exit_at"
		observed := fmt.Sprintf("LEFT JOIN LATERAL(SELECT h.created_at FROM audit_log h WHERE h.entity_type='%s' AND h.entity_id=e.id AND h.action='exit' AND %s ORDER BY h.created_at DESC,h.id DESC LIMIT 1) observed ON TRUE", entityType, PassageLivePredicate("h"))
		q := tx.Table(string(kind)+" e").Select(selectSQL, archiveArgs...).Joins("LEFT JOIN attachments a ON a.id=e.attachment_id").Joins("LEFT JOIN applications app ON app.id=a.application_id").Joins(join).Joins(observed).Where("NOT e.is_purged").Where(visible, args...).Where(freshSQL, freshArgs...)
		q = q.Where("(e.status=1 OR "+retention+" OR (p.action=? AND p.created_at>=?))", append(retentionArgs, PassageCorrectionAction, now.Add(-passageRevertWindow))...)
		q = q.Where("NOT ("+removed+") AND (a.is_manual OR (app.status IN ? AND app.confirmation=? AND NOT ("+archive+")))", append([]any{[]string{models.StatusInWork, models.StatusCompleted}, models.ConfirmationApproved}, archiveArgs...)...)
		if err := q.Order("e.id ASC").Scan(&rows).Error; err != nil {
			return err
		}
		// One batch of live table bindings serves every status row; no per-row query.
		var bindings []struct {
			EntityID int
			TableID  int
			Name     string
		}
		binding := elementBindingTables[kind]
		ids := make([]int, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.Projection.EntityID)
		}
		if len(ids) > 0 {
			if err := tx.Table(binding.table+" b").Select("b."+binding.column+" AS entity_id,b.table_id,st.name").Joins("JOIN system_tables st ON st.id=b.table_id").Where("b."+binding.column+" IN ? AND st.is_active AND st.status='active'", ids).Scan(&bindings).Error; err != nil {
				return err
			}
		}
		perEntity := map[int][]struct {
			TableID int
			Name    string
		}{}
		for _, b := range bindings {
			perEntity[b.EntityID] = append(perEntity[b.EntityID], struct {
				TableID int
				Name    string
			}{b.TableID, b.Name})
		}
		for _, row := range rows {
			p := row.Projection
			state, err := projectPassageRow(p, now)
			if err != nil {
				return err
			}
			effective := models.EffectivePeriod{EntryPeriod: models.EntryPeriod{EntryDateFrom: row.EntryDateFrom, EntryDateTo: row.EntryDateTo, EntryTimeFrom: row.EntryTimeFrom, EntryTimeTo: row.EntryTimeTo}, Bounded: row.Bounded, Source: row.PeriodSource}
			lifecycle := !row.Removed && !row.Archived && (row.Manual || (row.ApplicationStatus == models.StatusInWork || row.ApplicationStatus == models.StatusCompleted) && row.Confirmation == models.ConfirmationApproved)
			samePostRight := false
			for _, b := range perEntity[p.EntityID] {
				if p.TableID != nil && *p.TableID != b.TableID {
					continue
				}
				if set.Has("table."+b.Name+".view") && (p.Action == "entry" || p.Action == "exit") && set.Has("table."+b.Name+"."+p.Action) {
					samePostRight = true
					break
				}
			}
			canRevert := lifecycle && samePostRight && p.CreatedAt != nil && (set.IsAdmin() || set.IsSuperAdmin() || p.ActorUserID != nil && *p.ActorUserID == viewerID && now.Sub(*p.CreatedAt) <= passageRevertWindow)
			correctionAuthority := lifecycle || set.IsAdmin() || set.IsSuperAdmin() || fresh.Approver
			state.CanCorrect = state.Open && correctionAuthority && set.Has(passageCorrectionPermission)
			state.CanRevertCorrection = correctionAuthority && p.Action == PassageCorrectionAction && p.CreatedAt != nil && now.Sub(*p.CreatedAt) <= passageRevertWindow && set.Has(passageCorrectionPermission)
			status := 0
			if state.Open {
				status = 1
			} else if state.HasEvent {
				status = 2
			}
			items = append(items, passageCurrentItem{EntityID: p.EntityID, TerritoryStatus: status, EntryTime: FormatUTCPtr(state.EntryAt), LastExitTime: FormatUTCPtr(row.LastExitAt), CanRevert: canRevert, LastMarkTableID: p.TableID, PassageState: state, EffectivePeriod: effective, ServerNow: now})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return items, err
}
