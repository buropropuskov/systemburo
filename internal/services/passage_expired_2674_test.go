package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

func TestPassage2674ExpiredFilterAndCounts(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementCar, services.ElementEmployee} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			// The second open row has its own future period despite the expired parent.
			now := time.Now().In(services.MoscowLocation())
			from := now.AddDate(0, 0, -1).Format("2006-01-02")
			to := now.AddDate(0, 0, 2).Format("2006-01-02")
			inside, active := 1, 1
			if kind == services.ElementCar {
				car := models.Car{AttachmentID: w.attachment, Status: &active, TerritoryStatus: &inside}
				require.NoError(t, w.db.Create(&car).Error)
				require.NoError(t, w.db.Exec("UPDATE cars SET period_mode='individual', entry_date_from=?, entry_date_to=? WHERE id=?", from, to, car.ID).Error)
				require.NoError(t, w.db.Exec("INSERT INTO car_target_tables(car_id,table_id) VALUES(?,?)", car.ID, w.table).Error)
			} else {
				person := models.Employee{AttachmentID: &w.attachment, Status: &active, TerritoryStatus: &inside}
				require.NoError(t, w.db.Create(&person).Error)
				require.NoError(t, w.db.Exec("UPDATE employees SET period_mode='individual', entry_date_from=?, entry_date_to=? WHERE id=?", from, to, person.ID).Error)
				require.NoError(t, w.db.Exec("INSERT INTO employee_target_tables(employee_id,table_id) VALUES(?,?)", person.ID, w.table).Error)
			}
			list := services.NewPassageOpenService(w.db)
			all, err := list.List(context.Background(), w.actor, kind, &w.table, services.PassageOpenFilter{})
			require.NoError(t, err)
			require.EqualValues(t, 2, all.Total)
			require.EqualValues(t, 2, all.Counts.AllOpen)
			require.EqualValues(t, 1, all.Counts.Expired)
			expired, err := list.List(context.Background(), w.actor, kind, &w.table, services.PassageOpenFilter{ExpiredOnly: true, PerPage: 1})
			require.NoError(t, err)
			require.EqualValues(t, 1, expired.Total)
			require.Len(t, expired.Items, 1)
			require.Equal(t, w.id, expired.Items[0].EntityID)
			require.False(t, expired.Items[0].Admission.CanEnter)
			require.True(t, expired.Items[0].Admission.CanExit)
			empty, err := list.List(context.Background(), w.actor, kind, &w.table, services.PassageOpenFilter{ExpiredOnly: true, Search: "no-match-2674"})
			require.NoError(t, err)
			require.Zero(t, empty.Total)
			require.Zero(t, empty.Counts.AllOpen)
			require.Zero(t, empty.Counts.Expired)
		})
	}
}
