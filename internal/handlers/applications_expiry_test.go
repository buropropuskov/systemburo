package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckExpiredAttachments(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create the service directly for calling CheckExpiredAttachments.
	permSvc := services.NewPermissionService(db)
	notifSvc := services.NewNotificationService(db)
	blRecorder := services.NewAuditRecorder(db)
	vblSvc := services.NewVehicleBlacklistService(db, blRecorder)
	pblSvc := services.NewPersonBlacklistService(db, blRecorder)
	appSvc := services.NewApplicationService(db, permSvc, notifSvc, vblSvc, pblSvc, blRecorder)

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "expired_attachment_deactivated_with_car_history",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				uaID := seedUniqueAttachment(t, db, "cars", "exp_cars1", "Exp Cars 1")
				token := testutil.RegisterAndLogin(t, e, "exp_sender1", "pass123", 1, td.OrgID, td.CompanyID)
				appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

				// Get attachment ID
				rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)
				attachments := testutil.ParseSlice(t, rec)
				require.NotEmpty(t, attachments)
				attID := int(attachments[0]["id"].(float64))

				// Activate items so status=1 (CheckExpiredAttachments checks status=1)
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)

				// Set entry_date_to to yesterday via raw SQL to avoid GORM type coercion issues
				yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
				result := db.Exec("UPDATE attachments SET entry_date_to = ? WHERE id = ?", yesterday, attID)
				require.NoError(t, result.Error)
				require.Equal(t, int64(1), result.RowsAffected, "should update exactly 1 attachment")

				// Run expiry check
				err := appSvc.CheckExpiredAttachments(context.Background())
				require.NoError(t, err)

				// Verify attachment deactivated
				var att models.Attachment
				err = db.First(&att, attID).Error
				require.NoError(t, err)
				require.NotNil(t, att.Status)
				assert.Equal(t, 0, *att.Status, "attachment should be deactivated")

				// Verify car deactivated
				var car models.Car
				err = db.Where("attachment_id = ?", attID).First(&car).Error
				require.NoError(t, err)
				require.NotNil(t, car.Status)
				assert.Equal(t, 0, *car.Status, "car should be deactivated")

				// Деактивация по сроку пишется в audit_log (#870, срез 1.12c).
				var auditCount int64
				db.Model(&models.AuditLog{}).
					Where("entity_type = ? AND entity_id = ? AND action = 'deactivate'", models.AuditEntityCar, car.ID).
					Count(&auditCount)
				assert.Equal(t, int64(1), auditCount, "car deactivation should be in audit_log")
			},
		},
		{
			name: "all_attachments_expired_application_completed",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				uaID := seedUniqueAttachment(t, db, "cars", "exp_cars2", "Exp Cars 2")
				token := testutil.RegisterAndLogin(t, e, "exp_sender2", "pass123", 1, td.OrgID, td.CompanyID)
				appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

				// Activate items
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)

				// Set all attachments to expired
				yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
				result := db.Exec("UPDATE attachments SET entry_date_to = ? WHERE application_id = ?", yesterday, appID)
				require.NoError(t, result.Error)
				require.GreaterOrEqual(t, result.RowsAffected, int64(1), "should update at least 1 attachment")

				// Run expiry check
				err := appSvc.CheckExpiredAttachments(context.Background())
				require.NoError(t, err)

				// Verify application status = "Завершено"
				var app models.Application
				err = db.First(&app, appID).Error
				require.NoError(t, err)
				require.NotNil(t, app.Status)
				assert.Equal(t, models.StatusCompleted, *app.Status, "application should be completed when all attachments expired")
			},
		},
		{
			name: "some_attachments_expired_application_stays_active",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				uaID1 := seedUniqueAttachment(t, db, "cars", "exp_cars3a", "Exp Cars 3A")
				uaID2 := seedUniqueAttachment(t, db, "cars", "exp_cars3b", "Exp Cars 3B")
				token := testutil.RegisterAndLogin(t, e, "exp_sender3", "pass123", 1, td.OrgID, td.CompanyID)

				// Create application with 2 attachments
				body := fmt.Sprintf(`{
					"message": "multi attachment test",
					"organization": "Test Organization",
					"responsible_person": "Test Person",
					"contact_phone": "+79001234567",
					"data_approval": true,
					"attachments": [
						{
							"attachment_type": "cars",
							"attachment_name": "cars_a",
							"attachment_display_name": "Cars A",
							"unique_attachment_id": %d,
							"entry_date_from": "2026-04-01",
							"entry_date_to": "2099-12-31",
							"entry_time_from": "08:00",
							"entry_time_to": "18:00",
							"data": {"vehicles": [{"car_number": "B001BB777", "car_brand": "Honda"}]}
						},
						{
							"attachment_type": "cars",
							"attachment_name": "cars_b",
							"attachment_display_name": "Cars B",
							"unique_attachment_id": %d,
							"entry_date_from": "2026-04-01",
							"entry_date_to": "2099-12-31",
							"entry_time_from": "08:00",
							"entry_time_to": "18:00",
							"data": {"vehicles": [{"car_number": "C001CC777", "car_brand": "Mazda"}]}
						}
					]
				}`, uaID1, uaID2)
				rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code, "create: %s", rec.Body.String())
				resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
				appID := resp.ApplicationID

				// Activate all items
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)

				// Get both attachments
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)
				attachments := testutil.ParseSlice(t, rec)
				require.Len(t, attachments, 2)

				firstAttID := int(attachments[0]["id"].(float64))

				// Expire only the first attachment
				yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
				result := db.Exec("UPDATE attachments SET entry_date_to = ? WHERE id = ?", yesterday, firstAttID)
				require.NoError(t, result.Error)
				require.Equal(t, int64(1), result.RowsAffected)

				// Remember original status
				var appBefore models.Application
				err := db.First(&appBefore, appID).Error
				require.NoError(t, err)

				// Run expiry check
				err = appSvc.CheckExpiredAttachments(context.Background())
				require.NoError(t, err)

				// First attachment should be deactivated
				var att models.Attachment
				err = db.First(&att, firstAttID).Error
				require.NoError(t, err)
				require.NotNil(t, att.Status)
				assert.Equal(t, 0, *att.Status, "expired attachment should be deactivated")

				// Application should NOT be completed
				var appAfter models.Application
				err = db.First(&appAfter, appID).Error
				require.NoError(t, err)
				require.NotNil(t, appAfter.Status)
				assert.NotEqual(t, models.StatusCompleted, *appAfter.Status, "application should stay active when some attachments are still valid")
			},
		},
		{
			name: "no_expired_attachments_nothing_changes",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				uaID := seedUniqueAttachment(t, db, "cars", "exp_cars4", "Exp Cars 4")
				token := testutil.RegisterAndLogin(t, e, "exp_sender4", "pass123", 1, td.OrgID, td.CompanyID)
				appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

				// Activate items
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)

				// Get attachment to verify its state before
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code)
				attachments := testutil.ParseSlice(t, rec)
				require.NotEmpty(t, attachments)
				attID := int(attachments[0]["id"].(float64))

				// entry_date_to is 2099-12-31, which is in the future — nothing should change.
				err := appSvc.CheckExpiredAttachments(context.Background())
				require.NoError(t, err)

				// Attachment still active
				var att models.Attachment
				err = db.First(&att, attID).Error
				require.NoError(t, err)
				require.NotNil(t, att.Status)
				assert.Equal(t, 1, *att.Status, "attachment should stay active")

				// Application status unchanged
				var app models.Application
				err = db.First(&app, appID).Error
				require.NoError(t, err)
				require.NotNil(t, app.Status)
				assert.NotEqual(t, models.StatusCompleted, *app.Status, "application should not be completed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
