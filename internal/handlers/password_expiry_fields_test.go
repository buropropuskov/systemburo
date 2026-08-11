package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdatePassword_MovesChangedAt: смена пароля двигает дату, от которой
// считается срок действия. Без этого плановая смена (#1910) сочтёт пароль
// протухшим сразу после того, как человек его сменил сам.
func TestUpdatePassword_MovesChangedAt(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	longAgo := time.Now().Add(-200 * 24 * time.Hour)
	target := models.User{
		Username: "pwd_changed_at_user", Password: "x", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID,
		PasswordChangedAt: &longAgo, MustChangePassword: true,
	}
	require.NoError(t, db.Create(&target).Error)

	userSvc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, userSvc.UpdatePassword(context.Background(), target.ID, target.Username,
		models.UpdatePasswordRequest{Password: "brandnewpass12345"}, nil))

	var after models.User
	require.NoError(t, db.First(&after, target.ID).Error)
	require.NotNil(t, after.PasswordChangedAt)
	assert.True(t, after.PasswordChangedAt.After(longAgo), "дата смены должна сдвинуться вперёд")
	assert.WithinDuration(t, time.Now(), *after.PasswordChangedAt, time.Minute)
	assert.False(t, after.MustChangePassword, "требование сменить пароль должно сняться")
}

// TestChangeOwnPassword_MovesChangedAt: тот же учёт для самостоятельной смены -
// человек, сменивший пароль сам, не должен попасть под ближайшую плановую смену.
func TestChangeOwnPassword_MovesChangedAt(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const old = "oldpassword12345"
	token := testutil.RegisterAndLogin(t, e, "pwd_self_changed_at", old, 1, td.OrgID, td.CompanyID)

	longAgo := time.Now().Add(-100 * 24 * time.Hour)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "pwd_self_changed_at").
		Updates(map[string]any{"password_changed_at": longAgo, "must_change_password": true}).Error)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+old+`","new_password":"freshpassword678"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var after models.User
	require.NoError(t, db.Where("username = ?", "pwd_self_changed_at").First(&after).Error)
	require.NotNil(t, after.PasswordChangedAt)
	assert.True(t, after.PasswordChangedAt.After(longAgo))
	assert.False(t, after.MustChangePassword)
}

// TestCreateUser_SetsPasswordChangedAt: новый работник получает отсчёт срока с
// момента заведения учётной записи, иначе он попадёт под первую же плановую смену.
//
// Заодно фиксируется правило про первый вход: пароль заводимой учётной записи
// придумывает либо система, либо администратор - в обоих случаях не сам работник,
// поэтому свой он задаёт при первом входе. Раньше признак здесь не поднимался.
func TestCreateUser_SetsPasswordChangedAt(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userSvc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, userSvc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "freshly_created_user", Password: "createdpass12345", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))

	var created models.User
	require.NoError(t, db.Where("username = ?", "freshly_created_user").First(&created).Error)
	require.NotNil(t, created.PasswordChangedAt, "дата смены пароля должна проставляться при создании")
	assert.WithinDuration(t, time.Now(), *created.PasswordChangedAt, time.Minute)
	assert.True(t, created.MustChangePassword, "пароль при заведении задаёт не сам работник, поэтому меняется при первом входе")
}

// TestBackfillPasswordChangedAt_FillsOnlyEmpty: учётным записям, заведённым до
// появления столбца, проставляется момент внедрения, а заполненные даты не
// трогаются. Дата создания здесь не годится намеренно: по ней в день включения
// плановой смены истекли бы разом все старые учётные записи.
func TestBackfillPasswordChangedAt_FillsOnlyEmpty(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	kept := time.Now().Add(-30 * 24 * time.Hour).Truncate(time.Second)
	withDate := models.User{
		Username: "backfill_keeps_date", Password: "x", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, PasswordChangedAt: &kept,
	}
	require.NoError(t, db.Create(&withDate).Error)

	empty := models.User{
		Username: "backfill_fills_null", Password: "x", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID,
	}
	require.NoError(t, db.Create(&empty).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", empty.ID).
		Update("password_changed_at", nil).Error)

	require.NoError(t, database.BackfillPasswordChangedAt(db))

	var filled models.User
	require.NoError(t, db.First(&filled, empty.ID).Error)
	require.NotNil(t, filled.PasswordChangedAt, "пустая дата должна заполниться")
	assert.WithinDuration(t, time.Now(), *filled.PasswordChangedAt, time.Minute)

	var untouched models.User
	require.NoError(t, db.First(&untouched, withDate.ID).Error)
	require.NotNil(t, untouched.PasswordChangedAt)
	assert.WithinDuration(t, kept, *untouched.PasswordChangedAt, time.Second, "заполненную дату трогать нельзя")
}
