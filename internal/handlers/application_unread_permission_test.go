package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationService_GetUnreadCount_PermissionCheck проверяет, что
// счётчик непрочитанных учитывает права доступа: пользователь без роли
// approver/responsible/viewer не видит чужих заявок в счётчике.
func TestApplicationService_GetUnreadCount_PermissionCheck(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Два юзера: sender (создаёт заявки) и outsider (не имеет доступа ни к одной).
	sender := models.User{
		Username:       "sender_unread",
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	outsider := models.User{
		Username:       "outsider_unread",
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	approver := models.User{
		Username:       "approver_unread",
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&sender).Error)
	require.NoError(t, db.Create(&outsider).Error)
	require.NoError(t, db.Create(&approver).Error)

	// Создаём заявку, отправленную sender'ом, без responsible/viewer.
	appNumber := "APP-UR-1"
	status := models.StatusProcessing
	confirmation := models.ConfirmationPending
	app := models.Application{
		ApplicationNumber: &appNumber,
		OrganizationID:    td.OrgID,
		CompanyID:         &td.CompanyID,
		SenderUserID:      sender.ID,
		Status:            &status,
		Confirmation:      &confirmation,
		SendingDatetime:   ptrTime(time.Now().UTC()),
	}
	require.NoError(t, db.Create(&app).Error)

	// Делаем approver глобальным approver-ом (видит все заявки).
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: approver.ID}).Error)

	permSvc := services.NewPermissionService(db)
	notifSvc := services.NewNotificationService(db)
	blRecorder := services.NewAuditRecorder(db)
	vblSvc := services.NewVehicleBlacklistService(db, blRecorder)
	pblSvc := services.NewPersonBlacklistService(db, blRecorder)
	appSvc := services.NewApplicationService(db, permSvc, notifSvc, vblSvc, pblSvc, blRecorder)

	// outsider не должен видеть эту заявку в счётчике.
	resOutsider, err := appSvc.GetUnreadCount(context.Background(), outsider.Username)
	require.NoError(t, err)
	assert.Equal(t, 0, resOutsider.Count, "outsider не должен видеть чужую заявку")

	// approver видит все заявки -> 1 непрочитанная.
	resApprover, err := appSvc.GetUnreadCount(context.Background(), approver.Username)
	require.NoError(t, err)
	assert.Equal(t, 1, resApprover.Count, "approver должен видеть заявку как непрочитанную")

	// Если outsider становится viewer'ом, он должен увидеть заявку.
	require.NoError(t, db.Create(&models.ApplicationViewer{
		ApplicationID: app.ID,
		UserID:        outsider.ID,
	}).Error)

	resOutsider2, err := appSvc.GetUnreadCount(context.Background(), outsider.Username)
	require.NoError(t, err)
	assert.Equal(t, 1, resOutsider2.Count, "outsider-viewer должен видеть заявку")
}

func ptrTime(t time.Time) *time.Time { return &t }
