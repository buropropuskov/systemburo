package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func period2665ApplicationService(w period2665World) services.ApplicationService {
	recorder := services.NewAuditRecorder(w.db)
	return services.NewApplicationService(w.db, services.NewPermissionService(w.db), services.NewNotificationService(w.db),
		services.NewVehicleBlacklistService(w.db, recorder), services.NewPersonBlacklistService(w.db, recorder), recorder)
}

func TestEntityPeriod2665ExpiryKeepsLongerIndividual(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			ctx := context.Background()
			preview, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
			require.NoError(t, err)
			changed, err := w.commands.Change(ctx, w.actor, kind, w.target, w.request(preview.Revision, 5))
			require.NoError(t, err)
			past := w.clock.AddDate(0, 0, -1).Format("2006-01-02")
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Update("entry_date_to", past).Error)
			service := period2665ApplicationService(w)
			require.NoError(t, service.CheckExpiredAttachments(ctx))
			var target, neighbor struct{ Status int }
			require.NoError(t, w.db.Table(string(kind)).Select("status").Where("id=?", w.target).Scan(&target).Error)
			require.NoError(t, w.db.Table(string(kind)).Select("status").Where("id=?", w.neighbor).Scan(&neighbor).Error)
			require.Equal(t, 1, target.Status)
			require.Zero(t, neighbor.Status)
			var attachment models.Attachment
			var app models.Application
			require.NoError(t, w.db.First(&attachment, w.attachment).Error)
			require.NoError(t, w.db.First(&app, w.application).Error)
			require.Equal(t, 1, *attachment.Status)
			require.Equal(t, past, *attachment.EntryDateTo, "parent must not be prolonged to the individual end")
			require.Equal(t, models.StatusInWork, *app.Status)
			require.Equal(t, *changed.Effective.EntryDateTo, w.clock.AddDate(0, 0, 5).Format("2006-01-02"))
			var approved, pending int64
			require.NoError(t, w.db.Model(&models.ApplicationResponsibleUser{}).Where("application_id=? AND approval_status='approved'", w.application).Count(&approved).Error)
			require.NoError(t, w.db.Model(&models.ApplicationResponsibleUser{}).Where("application_id=? AND approval_status='pending'", w.application).Count(&pending).Error)
			require.EqualValues(t, 1, approved)
			require.EqualValues(t, 1, pending)

			// Expire the last individual; the next pass completes exactly once.
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Update("entry_date_to", past).Error)
			require.NoError(t, service.CheckExpiredAttachments(ctx))
			require.NoError(t, w.db.First(&app, w.application).Error)
			require.Equal(t, models.StatusCompleted, *app.Status)
			var countBefore, countAfter int64
			require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&countBefore).Error)
			require.NoError(t, service.CheckExpiredAttachments(ctx))
			require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&countAfter).Error)
			require.Equal(t, countBefore, countAfter, "repeated cron must not add transitions")
		})
	}
}

func TestEntityPeriod2665ExpiryLastShorterIndividual(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, expiresNow := range []bool{true, false} {
			t.Run(string(kind)+map[bool]string{true: "/expires", false: "/already_disabled"}[expiresNow], func(t *testing.T) {
				w := setupPeriod2665World(t, kind)
				pastFrom, pastTo := w.clock.AddDate(0, 0, -2).Format("2006-01-02"), w.clock.AddDate(0, 0, -1).Format("2006-01-02")
				require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.neighbor).Update("status", 0).Error)
				status := 0
				if expiresNow {
					status = 1
				}
				require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.target).Updates(map[string]any{
					"period_mode": models.PeriodIndividual, "entry_date_from": pastFrom, "entry_date_to": pastTo,
					"entry_time_from": "08:00:00", "entry_time_to": "22:00:00", "status": status,
				}).Error)
				require.NoError(t, period2665ApplicationService(w).CheckExpiredAttachments(context.Background()))
				var attachment models.Attachment
				var app models.Application
				require.NoError(t, w.db.First(&attachment, w.attachment).Error)
				require.NoError(t, w.db.First(&app, w.application).Error)
				if expiresNow {
					require.Zero(t, *attachment.Status, "last actual admission ended before the parent window")
					require.Equal(t, models.StatusCompleted, *app.Status)
				} else {
					require.Equal(t, 1, *attachment.Status, "manual deactivation is not an automatic expiry event")
					require.Equal(t, models.StatusInWork, *app.Status)
				}
			})
		}
	}
}
