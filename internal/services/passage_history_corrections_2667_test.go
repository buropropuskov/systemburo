package services_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

type passageHistory2667Row struct {
	ID       int
	Action   string
	Reverted bool
}

func TestPassage2667DBCorrectionGlobalJournalAndFilters(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementCar, services.ElementEmployee} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			entryActor := w.actor
			// A correction-only actor must appear in journal filters even though this
			// actor has never registered an observed entry or departure.
			corrector := models.User{Username: "passage2667_history_corrector", Password: "x", TypeID: 1, IsActive: true, IsAdmin: true}
			require.NoError(t, w.db.Create(&corrector).Error)
			w.actor = corrector.ID
			recorder := services.NewAuditRecorder(w.db)
			commands := services.NewPassageCommandService(w.db, recorder)
			cars := services.NewCarService(w.db, recorder)
			people := services.NewEmployeesHistoryService(w.db)
			read := func(q models.PassageHistoryQuery, table *int) ([]passageHistory2667Row, int64) {
				t.Helper()
				out := []passageHistory2667Row{}
				if kind == services.ElementCar {
					var rows []services.AllCarsHistoryItem
					var total int64
					var err error
					if table == nil {
						rows, total, err = cars.GetAllCarsHistory(ctx, q)
					} else {
						rows, total, err = cars.GetCarsHistoryByTable(ctx, *table, q)
					}
					require.NoError(t, err)
					for _, row := range rows {
						out = append(out, passageHistory2667Row{row.ID, row.ActionType, row.Reverted})
					}
					return out, total
				}
				var rows []services.EmployeeHistoryItem
				var total int64
				var err error
				if table == nil {
					rows, total, err = people.GetAll(ctx, q)
				} else {
					rows, total, err = people.GetByTable(ctx, *table, q)
				}
				require.NoError(t, err)
				for _, row := range rows {
					out = append(out, passageHistory2667Row{row.ID, row.ActionType, row.Reverted})
				}
				return out, total
			}
			assertObservedOnly := func() {
				t.Helper()
				recent, err := services.NewStatisticsService(w.db, 0).GetRecentPassages(ctx, 20)
				require.NoError(t, err)
				rows := recent.People
				if kind == services.ElementCar {
					rows = recent.Cars
				}
				require.Len(t, rows, 1)
				require.Equal(t, "entry", rows[0].ActionType, "correction is not an observed exit")
			}
			query := models.PassageHistoryQuery{PerPage: 1, Order: "asc"}
			if kind == services.ElementCar {
				query.CarID = &w.id
			} else {
				query.EmployeeID = &w.id
			}
			assertObservedOnly()
			closed, err := commands.Correct(ctx, w.actor, kind, w.id, w.correction())
			require.NoError(t, err)
			rows, total := read(query, nil)
			require.EqualValues(t, 2, total)
			require.Len(t, rows, 1)
			require.EqualValues(t, w.entryID, rows[0].ID)
			second := query
			second.Page = 2
			rows, total = read(second, nil)
			require.EqualValues(t, 2, total)
			require.Len(t, rows, 1)
			require.EqualValues(t, closed.PassageState.LastEventID, rows[0].ID)
			require.Equal(t, services.PassageCorrectionAction, rows[0].Action)
			require.False(t, rows[0].Reverted)
			assertObservedOnly()
			req := w.correction()
			req.ExpectedLastEventID = &closed.PassageState.LastEventID
			_, err = commands.RevertCorrection(ctx, w.actor, kind, w.id, req)
			require.NoError(t, err)
			all := query
			all.PerPage = 20
			rows, total = read(all, nil)
			require.EqualValues(t, 3, total)
			require.Len(t, rows, 3)
			require.Equal(t, services.PassageCorrectionAction, rows[1].Action)
			require.True(t, rows[1].Reverted)
			require.Equal(t, services.PassageCorrectionRevertAction, rows[2].Action)
			byActor := all
			byActor.UserID = &w.actor
			rows, total = read(byActor, nil)
			require.EqualValues(t, 2, total)
			require.Len(t, rows, 2)
			byActor.UserID = &entryActor
			rows, total = read(byActor, nil)
			require.EqualValues(t, 1, total)
			require.Equal(t, "entry", rows[0].Action)
			if kind == services.ElementCar {
				options, err := cars.GetCarsHistoryFilterOptions(ctx, nil)
				require.NoError(t, err)
				ids := []int{}
				for _, u := range options.Users {
					ids = append(ids, u.ID)
				}
				require.Contains(t, ids, w.actor)
				options, err = cars.GetCarsHistoryFilterOptions(ctx, &w.table)
				require.NoError(t, err)
				ids = nil
				for _, u := range options.Users {
					ids = append(ids, u.ID)
				}
				require.Contains(t, ids, w.actor)
			} else {
				options, err := people.GetFilterOptions(ctx, nil)
				require.NoError(t, err)
				ids := []int{}
				for _, u := range options.Users {
					ids = append(ids, u.ID)
				}
				require.Contains(t, ids, w.actor)
				entityIDs := []int{}
				for _, e := range options.Employees {
					entityIDs = append(entityIDs, e.ID)
				}
				require.Contains(t, entityIDs, w.id)
				options, err = people.GetFilterOptions(ctx, &w.table)
				require.NoError(t, err)
				ids = nil
				for _, u := range options.Users {
					ids = append(ids, u.ID)
				}
				require.Contains(t, ids, w.actor)
			}
			assertObservedOnly()
			// Detachment does not erase historical events that explicitly belong to the
			// post; new administrative corrections with no table ID must not appear there
			// through a restored/fabricated binding.
			binding, column := "employee_target_tables", "employee_id"
			if kind == services.ElementCar {
				binding, column = "car_target_tables", "car_id"
			}
			require.NoError(t, w.db.Exec("DELETE FROM "+binding+" WHERE "+column+"=?", w.id).Error)
			req = w.correction()
			req.Source = "admin_summary"
			req.TableID = nil
			detached, err := commands.Correct(ctx, w.actor, kind, w.id, req)
			require.NoError(t, err)
			req.ExpectedLastEventID = &detached.PassageState.LastEventID
			_, err = commands.RevertCorrection(ctx, w.actor, kind, w.id, req)
			require.NoError(t, err)
			rows, total = read(all, nil)
			require.EqualValues(t, 5, total)
			require.Len(t, rows, 5)
			rows, total = read(all, &w.table)
			require.EqualValues(t, 3, total)
			require.Len(t, rows, 3)
			for _, row := range rows {
				require.NotEqualValues(t, detached.PassageState.LastEventID, row.ID)
			}
			projected, err := services.LoadTablePassageResults(ctx, w.db, w.actor, kind, []int{w.id}, w.table, time.Time{})
			require.NoError(t, err)
			require.Empty(t, projected, "history must never restore guard access")
			assertObservedOnly()
		})
	}
}
