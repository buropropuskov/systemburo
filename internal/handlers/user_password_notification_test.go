package handlers_test

import (
	"context"
	"fmt"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserService_UpdatePassword_AdminChange_CreatesNotification проверяет, что
// при смене пароля админом (id вызывающего != целевого) у пользователя
// появляется уведомление password_changed с текстом про администратора.
func TestUserService_UpdatePassword_AdminChange_CreatesNotification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	target := models.User{
		Username:       "target_for_pass_change",
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&target).Error)

	notifSvc := services.NewNotificationService(db)
	userSvc := services.NewUserService(db, notifSvc)

	// callerUserID != target.ID -> selfChange=false -> сообщение про админа.
	callerUserID := target.ID + 1000
	err := userSvc.UpdatePassword(
		context.Background(),
		callerUserID,
		target.Username,
		models.UpdatePasswordRequest{Password: "newpassword12345"},
		nil,
	)
	require.NoError(t, err)

	notifs, err := notifSvc.GetByUserID(context.Background(), target.ID)
	require.NoError(t, err)
	require.Len(t, notifs, 1, "ожидается одно уведомление password_changed")

	n := notifs[0]
	require.NotNil(t, n.Type)
	assert.Equal(t, "password_changed", *n.Type)
	require.NotNil(t, n.Title)
	assert.Equal(t, "Пароль изменён", *n.Title)
	require.NotNil(t, n.Message)
	// Должность того, кто сменил пароль, человеку не сообщается: важен факт, а сменить
	// пароль может не только администратор. Требование владельца по итогам работы на
	// стенде (#974) - прежний текст начинался со слова «Администратор».
	assert.Equal(t, "Ваш пароль в системе был изменён.", *n.Message)
	assert.NotContains(t, *n.Message, "Администратор")
	require.NotNil(t, n.Data)
	assert.Contains(t, *n.Data, "changed_at")
	assert.Contains(t, *n.Data, fmt.Sprintf("\"changed_by_user_id\": %d", callerUserID))
}

// TestUserService_UpdatePassword_SelfChange_Notification проверяет, что при
// смене собственного пароля (id вызывающего == целевого) уведомление содержит
// текст "Ваш пароль был успешно изменён".
func TestUserService_UpdatePassword_SelfChange_Notification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	target := models.User{
		Username:       "self_pass_change",
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&target).Error)

	notifSvc := services.NewNotificationService(db)
	userSvc := services.NewUserService(db, notifSvc)

	err := userSvc.UpdatePassword(
		context.Background(),
		target.ID, // сам себе сменил
		target.Username,
		models.UpdatePasswordRequest{Password: "newpassword12345"},
		nil,
	)
	require.NoError(t, err)

	notifs, err := notifSvc.GetByUserID(context.Background(), target.ID)
	require.NoError(t, err)
	require.Len(t, notifs, 1)

	n := notifs[0]
	require.NotNil(t, n.Message)
	assert.Contains(t, *n.Message, "успешно изменён")
}
