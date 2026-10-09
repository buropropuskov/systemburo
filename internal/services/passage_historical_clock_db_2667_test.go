package services_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPassage2667DBHistoricalSessionClockRetainsGuards(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			entityType := models.AuditEntityEmployee
			if kind == services.ElementCar {
				entityType = models.AuditEntityCar
			}
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Update("status", 1).Error)
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.id).Updates(map[string]any{
				"status": 1, "territory_status": nil, "territory_entry_time": nil,
			}).Error)
			require.NoError(t, w.db.Where("entity_type=? AND entity_id=? AND action=?", entityType, w.id, "entry").Delete(&models.AuditLog{}).Error)
			var attachment models.Attachment
			require.NoError(t, w.db.First(&attachment, w.attachment).Error)
			require.NotNil(t, attachment.EntryDateFrom)
			start, err := time.ParseInLocation("2006-01-02", *attachment.EntryDateFrom, services.MoscowLocation())
			require.NoError(t, err)
			entryAt := start.Add(12 * time.Hour).UTC().Truncate(time.Microsecond)
			entryRequest := services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 1}
			before := w.snapshot(t)
			ordinary := services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
			_, err = ordinary.Mark(ctx, w.actor, kind, w.id, entryRequest)
			passage2667HTTP(t, err, http.StatusUnprocessableEntity)
			require.Equal(t, before, w.snapshot(t))

			historical := w.db.Session(&gorm.Session{NowFunc: func() time.Time { return entryAt }})
			failed := services.NewPassageCommandService(historical, failingPassage2667Audit{services.NewAuditRecorder(historical)})
			_, err = failed.Mark(ctx, w.actor, kind, w.id, entryRequest)
			require.ErrorContains(t, err, "synthetic audit failure")
			require.Equal(t, before, w.snapshot(t))
			var count int64
			require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND action=?", entityType, w.id, "entry").Count(&count).Error)
			require.Zero(t, count)

			require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_active", false).Error)
			commands := services.NewPassageCommandService(historical, services.NewAuditRecorder(historical))
			_, err = commands.Mark(ctx, w.actor, kind, w.id, entryRequest)
			passage2667HTTP(t, err, http.StatusForbidden)
			require.Equal(t, before, w.snapshot(t))
			require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_active", true).Error)
			entry, err := commands.Mark(ctx, w.actor, kind, w.id, entryRequest)
			require.NoError(t, err)
			require.True(t, entry.PassageState.Open)
			require.Equal(t, entryAt, entry.ServerNow)
			require.NotNil(t, entry.PassageState.EntryAt)
			require.WithinDuration(t, entryAt, *entry.PassageState.EntryAt, time.Microsecond)
			var audit models.AuditLog
			require.NoError(t, w.db.First(&audit, entry.PassageState.LastEventID).Error)
			require.WithinDuration(t, entryAt, audit.CreatedAt, time.Microsecond)
			var cache struct {
				TerritoryEntryTime *time.Time
				UpdatedAt          time.Time
			}
			require.NoError(t, w.db.Table(string(kind)).Select("territory_entry_time,updated_at").Where("id=?", w.id).Scan(&cache).Error)
			require.NotNil(t, cache.TerritoryEntryTime)
			require.WithinDuration(t, entryAt, *cache.TerritoryEntryTime, time.Microsecond)
			require.WithinDuration(t, entryAt, cache.UpdatedAt, time.Microsecond)

			exitAt := entryAt.Add(time.Minute)
			exitDB := w.db.Session(&gorm.Session{NowFunc: func() time.Time { return exitAt }})
			exitCommands := services.NewPassageCommandService(exitDB, services.NewAuditRecorder(exitDB))
			exit, err := exitCommands.Mark(ctx, w.actor, kind, w.id, services.PassageCommandRequest{
				TableID: &w.table, TerritoryStatus: 2, ExpectedLastEventID: &entry.PassageState.LastEventID,
			})
			require.NoError(t, err)
			require.False(t, exit.PassageState.Open)
			require.Equal(t, exitAt, exit.ServerNow)
			audit = models.AuditLog{}
			require.NoError(t, w.db.First(&audit, exit.PassageState.LastEventID).Error)
			require.WithinDuration(t, exitAt, audit.CreatedAt, time.Microsecond)
			require.True(t, audit.CreatedAt.After(entryAt))
			require.True(t, w.db.NowFunc().After(exitAt))
		})
	}
}
