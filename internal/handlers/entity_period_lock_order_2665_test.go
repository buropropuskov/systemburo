package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

type period2665LockContextKey struct{}

// The reset holds employee row locks before a period writer requests its first
// entity lock. Release it even when the observed order is wrong: this checks the
// ordering without deliberately making PostgreSQL detect a deadlock.
func TestEntityPeriod2665DBMixedLocksReset(t *testing.T) {
	for _, scenario := range []string{"expiry", "group_change"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, services.ElementEmployee)
			ids := []int{w.attachment, w.secondAttachment}
			for _, table := range []string{"employees", "cars"} {
				require.NoError(t, w.db.Table(table).Where("attachment_id IN ?", ids).Update("territory_status", 2).Error)
			}
			req := w.request(ids, services.IndividualPolicyReplace)
			if scenario == "expiry" {
				require.NoError(t, w.db.Table("applications").Where("id=?", w.application).Update("status", models.StatusInWork).Error)
				past := map[string]any{"entry_date_from": w.clock.AddDate(0, 0, -3).Format("2006-01-02"), "entry_date_to": w.clock.AddDate(0, 0, -2).Format("2006-01-02")}
				require.NoError(t, w.db.Table("attachments").Where("id IN ?", ids).Updates(past).Error)
				for _, table := range []string{"employees", "cars"} {
					require.NoError(t, w.db.Table(table).Where("attachment_id IN ? AND period_mode=?", ids, models.PeriodIndividual).Updates(past).Error)
				}
			} else {
				preview, err := w.service.Preview(context.Background(), w.actor, w.application, req)
				require.NoError(t, err)
				req.ExpectedRevision = preview.Revision
			}
			votes := w.votesAndApplication(t)
			// Reset legitimately changes updated_at, which is part of the group
			// revision. Compare period facts separately from reset-owned fields.
			periodFacts := func() []string {
				out := []string{}
				for _, table := range []string{"attachments", "employees", "cars"} {
					filter, mode := "attachment_id IN ?", "period_mode,"
					if table == "attachments" {
						filter, mode = "id IN ?", ""
					}
					var facts string
					require.NoError(t, w.db.Raw("SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]'::jsonb)::text FROM (SELECT id,"+mode+"entry_date_from,entry_date_to,entry_time_from,entry_time_to FROM "+table+" WHERE "+filter+") p", ids).Scan(&facts).Error)
					out = append(out, facts)
				}
				return out
			}
			periodsBefore, auditsBefore := periodFacts(), w.auditCount(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			held, first, release := make(chan struct{}, 1), make(chan string, 1), make(chan struct{})
			var unblockOnce, firstOnce, heldOnce sync.Once
			unblock := func() { unblockOnce.Do(func() { close(release) }) }
			defer unblock()
			const resetCallback = "period2665_reset_employee_barrier"
			const writerCallback = "period2665_writer_first_entity_lock"
			require.NoError(t, w.db.Callback().Update().After("gorm:update").Register(resetCallback, func(tx *gorm.DB) {
				if tx.Statement.Context.Value(period2665LockContextKey{}) != "reset" || tx.Statement.Table != "employees" || tx.Error != nil {
					return
				}
				heldOnce.Do(func() {
					held <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						tx.AddError(ctx.Err())
					}
				})
			}))
			require.NoError(t, w.db.Callback().Row().Before("gorm:row").Register(writerCallback, func(tx *gorm.DB) {
				if tx.Statement.Context.Value(period2665LockContextKey{}) != "writer" {
					return
				}
				sql := tx.Statement.SQL.String()
				if !strings.Contains(sql, "FOR UPDATE") {
					return
				}
				for _, table := range []string{"employees", "cars"} {
					if strings.Contains(sql, "FROM "+table+" ") {
						firstOnce.Do(func() { first <- table })
						return
					}
				}
			}))
			t.Cleanup(func() {
				require.NoError(t, w.db.Callback().Update().Remove(resetCallback))
				require.NoError(t, w.db.Callback().Row().Remove(writerCallback))
			})
			type resetResult struct {
				employees, cars int64
				err             error
			}
			resetDone, writerDone := make(chan resetResult, 1), make(chan error, 1)
			var workers sync.WaitGroup
			defer func() { cancel(); unblock(); workers.Wait() }()
			workers.Add(1)
			go func() {
				defer workers.Done()
				e, c, err := services.NewTerritoryResetService(w.db).ResetExitedStatuses(context.WithValue(ctx, period2665LockContextKey{}, "reset"))
				resetDone <- resetResult{e, c, err}
			}()
			select {
			case <-held:
			case <-ctx.Done():
				t.Fatal("reset did not acquire employee locks")
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				writerCtx := context.WithValue(ctx, period2665LockContextKey{}, "writer")
				var err error
				if scenario == "expiry" {
					err = period2665ApplicationService(w.period2665World).CheckExpiredAttachments(writerCtx)
				} else {
					_, err = w.service.Change(writerCtx, w.actor, w.application, req)
				}
				writerDone <- err
			}()
			var firstTable string
			select {
			case firstTable = <-first:
			case <-ctx.Done():
				unblock()
				t.Fatal("writer did not reach entity lock")
			}
			unblock()
			require.Equal(t, "employees", firstTable)
			select {
			case result := <-resetDone:
				require.NoError(t, result.err)
				require.EqualValues(t, 2, result.employees)
				require.EqualValues(t, 2, result.cars)
			case <-ctx.Done():
				t.Fatal("reset did not finish")
			}
			select {
			case err := <-writerDone:
				if scenario == "group_change" {
					period2665RequireHTTPError(t, err, http.StatusConflict)
				} else {
					require.NoError(t, err)
				}
			case <-ctx.Done():
				t.Fatal("writer did not finish")
			}
			if scenario == "group_change" {
				require.Equal(t, periodsBefore, periodFacts(), "stale group command must not write any period or mode")
				require.Equal(t, auditsBefore, w.auditCount(t), "reset has no audit and stale group command must roll back without audit")
				require.Equal(t, votes, w.votesAndApplication(t))
				preview, err := w.service.Preview(ctx, w.actor, w.application, req)
				require.NoError(t, err)
				require.NotEqual(t, req.ExpectedRevision, preview.Revision, "reset invalidates the original snapshot")
				req.ExpectedRevision = preview.Revision
				_, err = w.service.Change(ctx, w.actor, w.application, req)
				require.NoError(t, err, "an explicitly refreshed command succeeds after reset")
			}
			for _, table := range []string{"employees", "cars"} {
				var rows []struct {
					Status          int
					TerritoryStatus int
				}
				require.NoError(t, w.db.Table(table).Select("status,territory_status").Where("attachment_id IN ?", ids).Scan(&rows).Error)
				require.Len(t, rows, 2)
				for _, row := range rows {
					require.Zero(t, row.TerritoryStatus)
					if scenario == "expiry" {
						require.Zero(t, row.Status)
					} else {
						require.Equal(t, 1, row.Status)
					}
				}
				entityType := models.AuditEntityEmployee
				if table == "cars" {
					entityType = models.AuditEntityCar
				}
				action := models.AuditActionDatesChanged
				if scenario == "expiry" {
					action = "deactivate"
				}
				var audits int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND action=?", entityType, action).Count(&audits).Error)
				require.EqualValues(t, 2, audits)
			}
			if scenario == "group_change" {
				require.Equal(t, votes, w.votesAndApplication(t))
				var attachments []models.Attachment
				require.NoError(t, w.db.Where("id IN ?", ids).Find(&attachments).Error)
				for _, a := range attachments {
					require.Equal(t, req.Period.EntryDateFrom, *a.EntryDateFrom)
					require.Equal(t, req.Period.EntryDateTo, *a.EntryDateTo)
					require.Equal(t, req.Period.EntryTimeFrom, (*a.EntryTimeFrom)[:5])
					require.Equal(t, req.Period.EntryTimeTo, (*a.EntryTimeTo)[:5])
				}
			}
		})
	}
}
