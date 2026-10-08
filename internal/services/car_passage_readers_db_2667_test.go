package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func TestPassage2667DBCarReadersRetainOrdinaryAndFact(t *testing.T) {
	for _, plate := range []string{"TEST2667", "по факту", "пофакту legacy"} {
		t.Run(plate, func(t *testing.T) {
			w := setupPassage2667World(t, services.ElementCar)
			require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.id).Update("car_number", plate).Error)
			svc := services.NewCarService(w.db, services.NewAuditRecorder(w.db))
			var rows []services.TableCarResponse
			var err error
			if plate == "TEST2667" {
				rows, err = svc.GetActiveCarsForTable(context.Background(), w.table)
			} else {
				rows, err = svc.GetFactCarsForTable(context.Background(), w.table)
			}
			require.NoError(t, err)
			require.Len(t, rows, 1)
			row := rows[0]
			require.Equal(t, w.id, row.ID)
			require.True(t, row.PassageState.Open)
			require.Equal(t, w.entryID, row.PassageState.LastEventID)
			require.True(t, row.PassageState.NeedsAttention)
			require.WithinDuration(t, w.entered, *row.PassageState.EntryAt, time.Microsecond)
			require.Equal(t, "attachment", row.EffectivePeriod.Source)
			require.NotNil(t, row.EffectivePeriod.EntryDateFrom)
			require.False(t, row.Admission.CanEnter)
			require.False(t, row.Admission.CanExit, "actorless reader must not grant exit")
			require.False(t, row.PassageState.CanCorrect)
			require.False(t, row.ServerNow.IsZero())
		})
	}
}

func TestPassage2667DBCarReaderGraceAndOrphan(t *testing.T) {
	for _, tc := range []struct {
		name    string
		age     time.Duration
		orphan  bool
		visible bool
	}{
		{"paired_recent", 4 * time.Minute, false, true},
		{"paired_expired", 6 * time.Minute, false, false},
		{"orphan_recent", time.Minute, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := setupPassage2667World(t, services.ElementCar)
			if tc.orphan {
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("id=?", w.entryID).Update("action", "update").Error)
			}
			now := time.Now().UTC()
			details, err := json.Marshal(map[string]any{"table_id": w.table})
			require.NoError(t, err)
			require.NoError(t, w.db.Create(&models.AuditLog{EntityType: models.AuditEntityCar, EntityID: &w.id, Action: "exit", CreatedAt: now.Add(-tc.age), Details: details}).Error)
			require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.id).Update("territory_status", 0).Error)
			rows, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).GetActiveCarsForTable(context.Background(), w.table)
			require.NoError(t, err)
			if !tc.visible {
				require.Empty(t, rows)
				return
			}
			require.Len(t, rows, 1)
			require.True(t, rows[0].PassageState.InExitGrace, "reset cache must not erase recorded grace")
			require.False(t, rows[0].PassageState.Open)
		})
	}
}

func TestPassage2667DBCarReaderScopeAndLifecycle(t *testing.T) {
	for _, scenario := range []string{"detached", "removed", "purged", "withdrawn", "archived"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupPassage2667World(t, services.ElementCar)
			now := time.Now().UTC()
			switch scenario {
			case "detached":
				require.NoError(t, w.db.Exec("DELETE FROM car_target_tables WHERE car_id=?", w.id).Error)
			case "removed":
				require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.id).Update("date_removed", now).Error)
			case "purged":
				require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.id).Update("is_purged", true).Error)
			case "withdrawn":
				require.NoError(t, w.db.Exec("UPDATE applications SET status=? WHERE id=(SELECT application_id FROM attachments WHERE id=?)", models.StatusWithdrawn, w.attachment).Error)
			case "archived":
				from, to := now.In(services.MoscowLocation()).AddDate(0, -3, 0).Format("2006-01-02"), now.In(services.MoscowLocation()).AddDate(0, -2, 0).Format("2006-01-02")
				require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Updates(map[string]any{"entry_date_from": from, "entry_date_to": to}).Error)
				require.NoError(t, w.db.Exec("UPDATE applications SET status=? WHERE id=(SELECT application_id FROM attachments WHERE id=?)", models.StatusCompleted, w.attachment).Error)
			}
			rows, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).GetActiveCarsForTable(context.Background(), w.table)
			require.NoError(t, err)
			require.Empty(t, rows, "open presence must not escape source guards")
			projected, err := services.LoadTablePassageResults(context.Background(), w.db, w.actor, services.ElementCar, []int{w.id}, w.table, time.Time{})
			require.NoError(t, err)
			require.Empty(t, projected)
		})
	}
}

func TestPassage2667DBTableAdmissionFreshRights(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementCar, services.ElementEmployee} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			read := func() map[int]services.PassageResult {
				result, err := services.LoadTablePassageResults(ctx, w.db, w.actor, kind, []int{w.id}, w.table, time.Time{})
				require.NoError(t, err)
				return result
			}
			rows := read()
			require.Len(t, rows, 1)
			require.True(t, rows[w.id].Admission.CanExit)
			require.False(t, rows[w.id].Admission.CanEnter)
			require.True(t, rows[w.id].PassageState.CanCorrect)
			deny := models.UserPermissionOverride{UserID: w.actor, PermissionKey: "table.passage2667_post.exit", Value: "deny", GrantedAt: time.Now().UTC()}
			require.NoError(t, w.db.Create(&deny).Error)
			require.False(t, read()[w.id].Admission.CanExit)
			deny.PermissionKey = "table.passage2667_post.view"
			require.NoError(t, w.db.Create(&deny).Error)
			_, err := services.LoadTablePassageResults(ctx, w.db, w.actor, kind, []int{w.id}, w.table, time.Time{})
			passage2667HTTP(t, err, http.StatusForbidden)
		})
	}
}

func TestPassage2667DBCarReaderWholeIndividualAndManual(t *testing.T) {
	for _, scenario := range []string{"individual", "future_individual", "manual_unbounded"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupPassage2667World(t, services.ElementCar)
			ctx := context.Background()
			now := time.Now().UTC()
			require.NoError(t, w.db.Model(&models.AuditLog{}).Where("id=?", w.entryID).Update("action", "update").Error)
			require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.id).Updates(map[string]any{"status": 1, "territory_status": 0}).Error)
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Update("status", 1).Error)
			source := "individual"
			if scenario == "manual_unbounded" {
				source = "manual_unbounded"
				require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Updates(map[string]any{
					"is_manual": true, "application_id": nil, "entry_date_from": nil, "entry_date_to": nil, "entry_time_from": nil, "entry_time_to": nil,
				}).Error)
			} else {
				start, end := now.Add(-time.Hour).In(services.MoscowLocation()), now.Add(time.Hour).In(services.MoscowLocation())
				if scenario == "future_individual" {
					start, end = start.AddDate(0, 0, 2), end.AddDate(0, 0, 2)
				}
				require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.id).Updates(map[string]any{
					"period_mode":     models.PeriodIndividual,
					"entry_date_from": start.Format("2006-01-02"), "entry_date_to": end.Format("2006-01-02"),
					"entry_time_from": start.Format("15:04:05"), "entry_time_to": end.Format("15:04:05"),
				}).Error)
			}
			rows, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).GetActiveCarsForTable(ctx, w.table)
			require.NoError(t, err)
			if scenario == "future_individual" {
				require.Empty(t, rows)
				return
			}
			require.Len(t, rows, 1)
			require.Equal(t, source, rows[0].EffectivePeriod.Source)
			require.False(t, rows[0].PassageState.Open)
			if scenario == "individual" {
				require.NotNil(t, rows[0].EffectivePeriod.EntryTimeFrom)
				require.NotNil(t, rows[0].EffectivePeriod.EntryTimeTo)
			}
			fresh, err := services.LoadTablePassageResults(ctx, w.db, w.actor, services.ElementCar, []int{w.id}, w.table, time.Time{})
			require.NoError(t, err)
			require.True(t, fresh[w.id].Admission.CanEnter, "whole own/manual window must replace expired parent")
			require.False(t, fresh[w.id].Admission.CanExit)
		})
	}
}
