package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func TestEntityPeriod2665TableReadersUseEffectiveWindow(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			ctx := context.Background()
			table := models.SystemTable{Name: "period2665_reader", TableType: string(kind), IsActive: true}
			require.NoError(t, w.db.Create(&table).Error)
			for _, id := range []int{w.target, w.neighbor} {
				if kind == services.ElementEmployee {
					require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: id, TableID: table.ID}).Error)
				} else {
					require.NoError(t, w.db.Create(&models.CarTargetTable{CarID: id, TableID: table.ID}).Error)
				}
			}
			read := func() map[int]string {
				result := make(map[int]string)
				if kind == services.ElementEmployee {
					rows, err := services.NewEmployeeService(w.db, services.NewAuditRecorder(w.db)).GetActiveEmployeesForTable(ctx, table.ID)
					require.NoError(t, err)
					for _, row := range rows {
						if row.EntryDateTo != nil {
							result[row.ID] = *row.EntryDateTo
						} else {
							result[row.ID] = ""
						}
					}
				} else {
					rows, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).GetActiveCarsForTable(ctx, table.ID)
					require.NoError(t, err)
					for _, row := range rows {
						if row.EntryDateTo != nil {
							result[row.ID] = *row.EntryDateTo
						} else {
							result[row.ID] = ""
						}
					}
				}
				return result
			}
			preview, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
			require.NoError(t, err)
			changed, err := w.commands.Change(ctx, w.actor, kind, w.target, w.request(preview.Revision, 5))
			require.NoError(t, err)
			past := w.clock.AddDate(0, 0, -1).Format("2006-01-02")
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Update("entry_date_to", past).Error)
			rows := read()
			require.Equal(t, map[int]string{w.target: *changed.Effective.EntryDateTo}, rows, "individual remains visible after parent expires; inherited neighbor does not")

			// A future individual must not borrow the already-started parent window.
			future := w.clock.AddDate(0, 0, 1).Format("2006-01-02")
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Update("entry_date_from", future).Error)
			require.Empty(t, read())

			// Manual is an origin, not an unconditional bypass for a finite window.
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Updates(map[string]any{
				"is_manual": true, "application_id": nil, "entry_date_from": nil, "entry_date_to": nil, "entry_time_from": nil, "entry_time_to": nil,
			}).Error)
			rows = read()
			require.Equal(t, map[int]string{w.neighbor: ""}, rows, "only truly unbounded manual bypasses the calendar window")
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Updates(map[string]any{"entry_date_from": w.clock.AddDate(0, 0, -2).Format("2006-01-02"), "entry_date_to": past}).Error)
			require.Equal(t, map[int]string{w.neighbor: ""}, read(), "expired finite manual individual must disappear")
		})
	}
}
