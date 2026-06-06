package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserService_UpdatePassword_CreatesNotification проверяет, что после
// admin-смены пароля у целевого пользователя появляется уведомление
// password_changed.
func TestUserService_UpdatePassword_CreatesNotification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	target := models.User{
		Username:       "target_for_pass_change",
		Password:       "x",
		TypeID:         1, // обычный юзер
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&target).Error)

	notifSvc := services.NewNotificationService(db)
	userSvc := services.NewUserService(db, notifSvc)

	// callerTypeID=6 = buropropuskov (admin). target.TypeID=1 -> selfChange=false
	// -> сообщение про админа.
	err := userSvc.UpdatePassword(
		context.Background(),
		6, // callerTypeID admin
		0, // callerUserID (для аудита)
		target.Username,
		models.UpdatePasswordRequest{Password: "newpassword12345"},
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
	assert.Contains(t, *n.Message, "Администратор")
	require.NotNil(t, n.Data)
	assert.Contains(t, *n.Data, "changed_at")
	assert.Contains(t, *n.Data, "changed_by_type_id")
}

// TestUserService_UpdatePassword_NonAdmin_NoNotification проверяет, что если
// вызывающий не admin (нет прав), пароль не меняется и уведомление не пишется.
func TestUserService_UpdatePassword_NonAdmin_NoNotification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	target := models.User{
		Username:       "target_nonadmin",
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
		1, // обычный юзер -> Forbidden
		0, // callerUserID (для аудита)
		target.Username,
		models.UpdatePasswordRequest{Password: "newpass12345"},
	)
	require.Error(t, err)

	notifs, err := notifSvc.GetByUserID(context.Background(), target.ID)
	require.NoError(t, err)
	assert.Empty(t, notifs)
}
