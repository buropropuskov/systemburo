package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

// Actual commands establish the mode before soft deletion. The list must show
// the same effective dates, rather than cleared own fields or an old parent.
func TestEntityPeriod2665DBTrashListsEffectivePeriod(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"inherit_cleared", "individual_beyond_parent"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupPeriod2665World(t, kind)
				ctx := context.Background()
				initial, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
				require.NoError(t, err)
				req := w.request(initial.Revision, 7)
				req.Period.EntryTimeFrom, req.Period.EntryTimeTo = "10:15", "20:45"
				changed, err := w.commands.Change(ctx, w.actor, kind, w.target, req)
				require.NoError(t, err)
				if scenario == "inherit_cleared" {
					changed, err = w.commands.Change(ctx, w.actor, kind, w.target, services.ChangeEntityPeriodRequest{
						PeriodMode: models.PeriodInherit, Reason: "Return to attachment period", ExpectedRevision: changed.Revision,
					})
					require.NoError(t, err)
					var populated int64
					require.NoError(t, w.db.Table(string(kind)).Where("id=? AND (entry_date_from IS NOT NULL OR entry_date_to IS NOT NULL OR entry_time_from IS NOT NULL OR entry_time_to IS NOT NULL)", w.target).Count(&populated).Error)
					require.Zero(t, populated)
				} else {
					require.Equal(t, w.clock.AddDate(0, 0, 7).Format("2006-01-02"), *changed.Effective.EntryDateTo)
				}

				tableType, entityType, removedField := "people", models.AuditEntityEmployee, "date_deleted"
				if kind == services.ElementCar {
					tableType, entityType, removedField = "cars", models.AuditEntityCar, "date_removed"
				}
				table := models.SystemTable{Name: "period2665_trash_" + tableType, TableType: tableType, IsActive: true}
				require.NoError(t, w.db.Create(&table).Error)
				require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Updates(map[string]any{
					"status": 0, removedField: w.clock.UTC(), "is_purged": false,
				}).Error)
				recorder := services.NewAuditRecorder(w.db)
				require.NoError(t, recorder.Record(ctx, nil, entityType, &w.target, "delete", &w.actor, map[string]any{"table_id": table.ID}))
				beforeRow, beforeParents := w.rowSnapshot(t, w.target), w.parentAndVotesSnapshot(t)
				var auditBefore int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&auditBefore).Error)

				trash := services.NewTrashService(w.db, recorder)
				var items []models.TrashItem
				if kind == services.ElementCar {
					items, err = trash.ListCarsTrash(ctx, table.ID, models.TrashFilter{})
				} else {
					items, err = trash.ListEmployeesTrash(ctx, table.ID, models.TrashFilter{})
				}
				require.NoError(t, err)
				require.Len(t, items, 1)
				require.Equal(t, w.target, items[0].ID)
				require.Equal(t, changed.Effective.EntryDateTo, items[0].EntryDateTo)
				require.Equal(t, changed.Effective.EntryTimeFrom, items[0].EntryTimeFrom)
				require.Equal(t, changed.Effective.EntryTimeTo, items[0].EntryTimeTo)
				require.Equal(t, beforeRow, w.rowSnapshot(t, w.target))
				require.Equal(t, beforeParents, w.parentAndVotesSnapshot(t))
				var auditAfter int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&auditAfter).Error)
				require.Equal(t, auditBefore, auditAfter, "reading trash must not mutate audit storage")
			})
		}
	}
}
