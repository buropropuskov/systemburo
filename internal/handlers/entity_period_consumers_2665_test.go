package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// These tests call the production service readers. Dates are relative to the
// fixture's single Moscow clock, with no same-day expiry boundary assumptions.
func TestEntityPeriod2665DBConsumerWindows(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			for _, scenario := range []string{
				"individual_beyond_expired_parent",
				"individual_expired_parent_future",
				"explicit_inherit_own_null_parent_finite",
				"legacy_inherit_parent_null_end",
				"individual_null_end",
			} {
				t.Run(scenario, func(t *testing.T) {
					w := setupPeriod2665World(t, kind)
					ctx := context.Background()
					day := func(offset int) string { return w.clock.AddDate(0, 0, offset).Format("2006-01-02") }
					parentFrom, parentTo := day(-3), day(4)
					ownFrom, ownTo := day(-7), day(7)
					mode := models.PeriodIndividual
					wantRestore := true
					wantFrom, wantTo := ownFrom, ownTo
					switch scenario {
					case "individual_beyond_expired_parent":
						parentTo = day(-2)
					case "individual_expired_parent_future":
						ownTo = day(-1)
						wantRestore, wantTo = false, parentTo
					case "explicit_inherit_own_null_parent_finite":
						mode = models.PeriodInherit
						wantFrom, wantTo = parentFrom, parentTo
					case "legacy_inherit_parent_null_end":
						mode = models.PeriodInherit
						wantFrom, wantTo = parentFrom, ""
					case "individual_null_end":
						wantRestore, wantTo = false, ""
					}
					parent := map[string]any{"entry_date_from": parentFrom, "entry_date_to": parentTo, "entry_time_from": "08:00:00", "entry_time_to": "22:00:00"}
					if scenario == "legacy_inherit_parent_null_end" {
						parent["entry_date_to"] = nil
					}
					require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(parent).Error)
					own := map[string]any{"period_mode": mode, "entry_date_from": ownFrom, "entry_date_to": ownTo, "entry_time_from": "08:00:00", "entry_time_to": "22:00:00", "status": 0}
					if kind == services.ElementCar {
						own["date_removed"] = w.clock.UTC()
					} else {
						own["date_deleted"] = w.clock.UTC()
					}
					if mode == models.PeriodInherit {
						for _, field := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"} {
							own[field] = nil
						}
					}
					if scenario == "individual_null_end" {
						own["entry_date_to"] = nil
					}
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Updates(own).Error)
					db := w.db
					trash := services.NewTrashService(db, services.NewAuditRecorder(db))
					var allowed bool
					var reason string
					if kind == services.ElementCar {
						allowed, reason = trash.CanRestoreCar(ctx, w.target)
					} else {
						allowed, reason = trash.CanRestoreEmployee(ctx, w.target)
					}
					require.Equal(t, wantRestore, allowed)
					if wantRestore {
						require.Empty(t, reason)
					} else {
						require.NotEmpty(t, reason)
					}
					apps := services.NewApplicationService(db, nil, nil, nil, nil, services.NewAuditRecorder(db))
					extras, err := apps.GetRegistryExtras(ctx, []int{w.application})
					require.NoError(t, err)
					require.Len(t, extras, 1)
					got, exists := extras[w.application]
					require.True(t, exists)
					require.Equal(t, wantFrom, got.EntryDateFrom)
					require.Equal(t, wantTo, got.EntryDateTo)
					if kind == services.ElementCar {
						require.Equal(t, 2, got.CarsCount)
						require.Zero(t, got.PeopleCount)
					} else {
						require.Equal(t, 2, got.PeopleCount)
						require.Zero(t, got.CarsCount)
					}
				})
			}
		})
	}
}

func TestEntityPeriod2665DBConsumerInvalidModeConstraint(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			before := w.rowSnapshot(t, w.target)
			// Unknown mode cannot enter the actual tables. The transaction rolls
			// back the expected PostgreSQL CHECK failure; no constraint is removed.
			err := w.db.Transaction(func(tx *gorm.DB) error {
				return tx.Table(string(kind)).Where("id=?", w.target).Update("period_mode", "unknown2665").Error
			})
			require.ErrorContains(t, err, "chk_"+string(kind)+"_period_mode")
			require.Equal(t, before, w.rowSnapshot(t, w.target))
		})
	}
}

func TestEntityPeriod2665DBRegistryExtrasMixedNeighbors(t *testing.T) {
	w := setupPeriod2665World(t, services.ElementEmployee)
	ctx := context.Background()
	day := func(offset int) string { return w.clock.AddDate(0, 0, offset).Format("2006-01-02") }
	from, to, start, end, active := day(-5), day(8), "08:00:00", "22:00:00", 1
	require.NoError(t, w.db.Table("employees").Where("id=?", w.target).Updates(map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": from, "entry_date_to": to, "entry_time_from": start, "entry_time_to": end}).Error)
	carFrom, carTo := day(-2), day(4)
	carAtt := models.Attachment{ApplicationID: &w.application, AttachmentType: "cars", Status: &active, EntryDateFrom: &carFrom, EntryDateTo: &carTo, EntryTimeFrom: &start, EntryTimeTo: &end}
	require.NoError(t, w.db.Create(&carAtt).Error)
	for _, plate := range []string{"PERIOD2665-MIX-A", "PERIOD2665-MIX-B"} {
		car := models.Car{AttachmentID: carAtt.ID, CarNumber: &plate, Status: &active}
		require.NoError(t, w.db.Create(&car).Error)
	}
	// A different application must neither affect the range nor leak into the map.
	status := models.StatusInWork
	other := models.Application{SenderUserID: w.actor, Status: &status}
	var organizationID int
	require.NoError(t, w.db.Table("applications").Select("organization_id").Where("id=?", w.application).Scan(&organizationID).Error)
	other.OrganizationID = organizationID
	require.NoError(t, w.db.Create(&other).Error)
	farFrom, farTo := day(-50), day(50)
	require.NoError(t, w.db.Create(&models.Attachment{ApplicationID: &other.ID, AttachmentType: "items", Status: &active, EntryDateFrom: &farFrom, EntryDateTo: &farTo}).Error)
	apps := services.NewApplicationService(w.db, nil, nil, nil, nil, services.NewAuditRecorder(w.db))
	extras, err := apps.GetRegistryExtras(ctx, []int{w.application})
	require.NoError(t, err)
	require.Len(t, extras, 1)
	got, exists := extras[w.application]
	require.True(t, exists)
	require.Equal(t, 2, got.PeopleCount)
	require.Equal(t, 2, got.CarsCount)
	require.Equal(t, from, got.EntryDateFrom)
	require.Equal(t, to, got.EntryDateTo)
	empty, err := apps.GetRegistryExtras(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
