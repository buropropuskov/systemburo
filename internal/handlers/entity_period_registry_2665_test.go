package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

// The application remains in Work. Each registry identity has exactly one
// candidate pass, so an active neighbor cannot conceal a wrong source window.
func TestEntityPeriod2665DBActualRegistryWindows(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"individual_beyond_expired_parent", "individual_expired_parent_future", "inherit_own_null", "legacy_parent_null_end", "individual_null_end"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupPeriod2665World(t, kind)
				day := func(n int) string { return w.clock.AddDate(0, 0, n).Format("2006-01-02") }
				parentEnd, ownEnd, wantEnd := day(4), day(7), day(7)
				mode, want := models.PeriodIndividual, true
				switch scenario {
				case "individual_beyond_expired_parent":
					parentEnd = day(-2)
				case "individual_expired_parent_future":
					ownEnd, want = day(-1), false
				case "inherit_own_null":
					mode, wantEnd = models.PeriodInherit, parentEnd
				case "legacy_parent_null_end":
					mode, wantEnd = models.PeriodInherit, ""
				case "individual_null_end":
					want = false
				}
				parent := map[string]any{"entry_date_from": day(-4), "entry_date_to": parentEnd}
				if scenario == "legacy_parent_null_end" {
					parent["entry_date_to"] = nil
				}
				require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(parent).Error)
				own := map[string]any{"period_mode": mode, "entry_date_from": day(-3), "entry_date_to": ownEnd, "entry_time_from": "09:00:00", "entry_time_to": "21:00:00"}
				if mode == models.PeriodInherit {
					for _, field := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"} {
						own[field] = nil
					}
				}
				if scenario == "individual_null_end" {
					own["entry_date_to"] = nil
				}
				require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Updates(own).Error)
				registryIDs := periodRegistry2665Identities(t, w)
				ctx := context.Background()
				if kind == services.ElementCar {
					rows, err := services.NewUniqueCarService(w.db).GetAll(ctx, "period2665_actor", "")
					require.NoError(t, err)
					require.Len(t, rows, 2)
					for _, row := range rows {
						isTarget := row.ID == registryIDs[0]
						require.Contains(t, registryIDs, row.ID)
						active := want
						if !isTarget {
							active = scenario != "individual_beyond_expired_parent"
						}
						require.Equal(t, active, row.Status)
						if isTarget {
							periodRegistry2665AssertActive(t, active, wantEnd, row.ActiveEntryDateTo, row.ActiveCarID, row.ActiveApplicationID, w)
						}
						if isTarget && active && mode == models.PeriodIndividual {
							require.Equal(t, "09:00:00", *row.ActiveEntryTimeFrom)
							require.Equal(t, "21:00:00", *row.ActiveEntryTimeTo)
						}
					}
				} else {
					rows, err := services.NewUniqueEmployeeService(w.db).GetAll(ctx, "period2665_actor", "")
					require.NoError(t, err)
					require.Len(t, rows, 2)
					for _, row := range rows {
						isTarget := row.ID == registryIDs[0]
						require.Contains(t, registryIDs, row.ID)
						active := want
						if !isTarget {
							active = scenario != "individual_beyond_expired_parent"
						}
						require.Equal(t, active, row.Status)
						if isTarget {
							periodRegistry2665AssertActive(t, active, wantEnd, row.ActiveEntryDateTo, row.ActiveEmployeeID, row.ActiveApplicationID, w)
						}
						if isTarget && active && mode == models.PeriodIndividual {
							require.Equal(t, "09:00:00 - 21:00:00", *row.ActivePassTime)
						}
					}
				}
			})
		}
	}
}

func periodRegistry2665Identities(t *testing.T, w period2665World) []int {
	t.Helper()
	ids := make([]int, 0, 2)
	for i, entityID := range []int{w.target, w.neighbor} {
		if w.kind == services.ElementCar {
			var car models.Car
			require.NoError(t, w.db.First(&car, entityID).Error)
			unique := models.UniqueCar{Number: car.CarNumber, UserID: &w.actor}
			require.NoError(t, w.db.Create(&unique).Error)
			ids = append(ids, unique.ID)
		} else {
			passport := fmt.Sprintf("9900%06d", i+1)
			var employee models.Employee
			require.NoError(t, w.db.First(&employee, entityID).Error)
			employee.PassportSeriesNumber = &passport
			require.NoError(t, w.db.Save(&employee).Error)
			// Separate cleartext variable: model hooks encrypt the pass and
			// registry independently, deriving their real shared HMAC identity.
			unique := models.UniqueEmployee{PassportSeriesNumber: &passport, UserID: &w.actor}
			require.NoError(t, w.db.Create(&unique).Error)
			ids = append(ids, unique.ID)
		}
	}
	return ids
}

func periodRegistry2665AssertActive(t *testing.T, active bool, end string, date *string, entity, app *int, w period2665World) {
	t.Helper()
	if !active {
		require.Nil(t, date)
		require.Nil(t, entity)
		require.Nil(t, app)
		return
	}
	if end == "" {
		require.Nil(t, date)
	} else {
		require.NotNil(t, date)
		require.Equal(t, end, *date)
	}
	require.NotNil(t, entity)
	require.NotNil(t, app)
	require.Equal(t, w.target, *entity)
	require.Equal(t, w.application, *app)
}

func TestEntityPeriod2665DBActualCheckActiveCar(t *testing.T) {
	for _, scenario := range []string{"individual_beyond_parent", "individual_expired", "inherit_own_null", "inherit_parent_null", "individual_null_end", "manual_finite", "manual_unbounded"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupPeriod2665World(t, services.ElementCar)
			day := func(n int) string { return w.clock.AddDate(0, 0, n).Format("2006-01-02") }
			parent := map[string]any{"entry_date_from": day(-4), "entry_date_to": day(-2)}
			own := map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": day(-3), "entry_date_to": day(7), "entry_time_from": "09:00:00", "entry_time_to": "21:00:00", "car_brand": "Period2665Brand"}
			want, end := true, day(7)
			switch scenario {
			case "individual_expired":
				parent["entry_date_to"], own["entry_date_to"], want = day(4), day(-1), false
			case "inherit_own_null", "inherit_parent_null":
				own["period_mode"] = models.PeriodInherit
				for _, field := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"} {
					own[field] = nil
				}
				parent["entry_date_to"], end = day(4), day(4)
				if scenario == "inherit_parent_null" {
					parent["entry_date_to"], want = nil, false
				}
			case "individual_null_end":
				own["entry_date_to"], want = nil, false
			case "manual_finite", "manual_unbounded":
				// Preserve the existing application JOIN: orphan manual records
				// must not unexpectedly become duplicate application passes.
				parent["application_id"], want = nil, false
				parent["is_manual"] = true
				if scenario == "manual_unbounded" {
					own["period_mode"] = models.PeriodInherit
					for _, field := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"} {
						own[field], parent[field] = nil, nil
					}
				}
			}
			require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(parent).Error)
			require.NoError(t, w.db.Table("cars").Where("id=?", w.target).Updates(own).Error)
			var app models.Application
			require.NoError(t, w.db.First(&app, w.application).Error)
			got, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).CheckActiveCar(context.Background(), services.CheckActiveCarRequest{CarNumber: "TEST2665-0", CarBrand: "Period2665Brand", OrganizationID: &app.OrganizationID})
			require.NoError(t, err)
			require.Equal(t, want, got.Active)
			if want {
				require.NotNil(t, got.CarID)
				require.Equal(t, w.target, *got.CarID)
				require.NotNil(t, got.EntryDateTo)
				require.Equal(t, end, *got.EntryDateTo)
			} else {
				require.Nil(t, got.CarID)
				require.Nil(t, got.ApplicationID)
			}
		})
	}
}

func TestEntityPeriod2665DBActualByFactGuard(t *testing.T) {
	w := setupPeriod2665World(t, services.ElementCar)
	require.NoError(t, w.db.Table("cars").Where("id=?", w.target).UpdateColumn("car_number", "По факту").Error)
	ctx := context.Background()
	preview, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
	require.NoError(t, err)
	before, parent := w.rowSnapshot(t, w.target), w.parentAndVotesSnapshot(t)
	_, err = w.commands.Change(ctx, w.actor, w.kind, w.target, w.request(preview.Revision, 7))
	period2665RequireHTTPError(t, err, http.StatusBadRequest)
	require.Equal(t, before, w.rowSnapshot(t, w.target))
	require.Equal(t, parent, w.parentAndVotesSnapshot(t))
	var count int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND action=?", models.AuditEntityCar, w.target, models.AuditActionDatesChanged).Count(&count).Error)
	require.Zero(t, count)
	request := w.request(preview.Revision, 1)
	request.Period.EntryDateTo = services.ByFactMaxDate(w.clock)
	changed, err := w.commands.Change(ctx, w.actor, w.kind, w.target, request)
	require.NoError(t, err)
	require.Equal(t, request.Period.EntryDateTo, *changed.Effective.EntryDateTo)
	require.Equal(t, parent, w.parentAndVotesSnapshot(t))
}

func TestEntityPeriod2665DBActualGroupByFactChangedOnly(t *testing.T) {
	for _, policy := range []string{"preserve", "replace"} {
		t.Run(policy, func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, services.ElementCar)
			require.NoError(t, w.db.Table("cars").Where("id=?", w.neighbor).UpdateColumn("car_number", "По факту").Error)
			before, auditBefore := w.snapshot(t), w.auditCount(t)
			individualBefore := w.entitySnapshot(t, services.ElementCar, w.neighbor)
			request := w.request([]int{w.attachment}, policy)
			preview, err := w.service.Preview(context.Background(), w.actor, w.application, request)
			if policy == "replace" {
				period2665RequireHTTPError(t, err, http.StatusBadRequest)
				require.Equal(t, before, w.snapshot(t))
				require.Equal(t, auditBefore, w.auditCount(t))
				return
			}
			require.NoError(t, err, "preserved individual ByFact row is not subject to the new parent window")
			require.Equal(t, before, w.snapshot(t), "preview writes neither entities nor votes")
			request.ExpectedRevision = preview.Revision
			_, err = w.service.Change(context.Background(), w.actor, w.application, request)
			require.NoError(t, err)
			require.Equal(t, individualBefore, w.entitySnapshot(t, services.ElementCar, w.neighbor))
			var end string
			require.NoError(t, w.db.Table("attachments").Select("entry_date_to").Where("id=?", w.attachment).Scan(&end).Error)
			require.Equal(t, request.Period.EntryDateTo, end)
		})
	}
}

// PostgreSQL now() is fixed by this rollback transaction, avoiding a midnight
// race between constructing the +3 calendar-day fixture and the notify query.
func TestEntityPeriod2665DBActualExpiryNotifyEffectiveMaximum(t *testing.T) {
	for _, scenario := range []string{"individual_beyond_expired_parent", "later_individual_suppresses_parent_warning", "unbounded_inherit_suppresses_warning"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupPeriod2665World(t, services.ElementCar)
			tx := w.db.Begin()
			require.NoError(t, tx.Error)
			t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })
			var today string
			require.NoError(t, tx.Raw("SELECT (now() AT TIME ZONE 'Europe/Moscow')::date::text").Scan(&today).Error)
			clock, err := time.Parse("2006-01-02", today)
			require.NoError(t, err)
			day := func(n int) string { return clock.AddDate(0, 0, n).Format("2006-01-02") }
			parentEnd, ownEnd, want := day(-2), day(3), 1
			if scenario == "later_individual_suppresses_parent_warning" {
				parentEnd, ownEnd, want = day(1), day(7), 0
			}
			parent := map[string]any{"entry_date_from": day(-4), "entry_date_to": parentEnd}
			if scenario == "unbounded_inherit_suppresses_warning" {
				parent["entry_date_to"], want = nil, 0
			}
			require.NoError(t, tx.Table("attachments").Where("id=?", w.attachment).Updates(parent).Error)
			require.NoError(t, tx.Table("cars").Where("id=?", w.target).Updates(map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": day(-3), "entry_date_to": ownEnd, "entry_time_from": "08:00:00", "entry_time_to": "22:00:00"}).Error)
			spy := &periodNotification2665Spy{}
			require.NoError(t, services.NewExpiryNotifyService(tx, spy).NotifyExpiringSoon(context.Background()))
			require.Len(t, spy.calls, want)
			if want != 0 {
				call := spy.calls[0]
				require.Equal(t, w.actor, call.userID)
				require.Equal(t, services.NotificationTypeApplicationExpiring, call.kind)
				var data map[string]any
				require.NoError(t, json.Unmarshal([]byte(call.data), &data))
				require.Equal(t, float64(w.application), data["application_id"])
				require.Equal(t, ownEnd, data["entry_date_to"])
				require.Equal(t, float64(3), data["days_left"])
			}
		})
	}
}
