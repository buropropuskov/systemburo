package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogin_BadPasswordHashIsNotWrongPassword: повреждённая запись пароля в базе
// обязана отвечать не так, как неверный пароль, и не копить лестницу блокировки.
// Иначе дефект данных выдаётся за вину человека: он вводит верный пароль,
// получает отказ, пробует ещё - и запирает себе доступ сам (#2017).
func TestLogin_BadPasswordHashIsNotWrongPassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "badhashuser", "correctpass", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "badhashuser").
		Update("password", "not-a-phc-hash").Error)

	svc := newLoginService(db)
	meta := &services.RequestMeta{IPAddress: "198.51.100.9", UserAgent: "go-test"}
	creds := models.LoginRequest{Username: "badhashuser", Password: "correctpass"}

	// Ровно столько неудач запирает вход, если считать их неудачами человека.
	attempts := failuresBeforeLock(t, db)
	for i := 0; i < attempts; i++ {
		_, err := svc.Login(context.Background(), creds, meta)
		require.Error(t, err)

		var ae *apperr.Error
		require.ErrorAs(t, err, &ae, "битый хеш обязан приходить доменной ошибкой, а не голым echo-ответом")
		assert.Equal(t, http.StatusInternalServerError, ae.Code,
			"клиент не должен думать, что дело в пароле")
		assert.NotContains(t, ae.Message, "Неверный логин или пароль")
		assert.NotContains(t, ae.Message, "Слишком много попыток")
		assert.Empty(t, ae.Headers["X-Auth-Attempts-Remaining"],
			"остаток попыток тут не при чём - попытки никто не тратил")
	}

	// Счётчик учётной записи дефект данных не двигает: до него исполнение не доходит.
	var user models.User
	require.NoError(t, db.Where("username = ?", "badhashuser").First(&user).Error)
	assert.Equal(t, 0, user.FailedLoginCount)
	assert.Nil(t, user.LockedUntil)
}

// TestLogin_BadPasswordHashJournalEntry: в журнале битый хеш обязан выглядеть
// иначе, чем неверный пароль, - собственным типом события, а не login_failed.
func TestLogin_BadPasswordHashJournalEntry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "journalbadhash", "correctpass", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "journalbadhash").
		Update("password", "$argon2id$broken").Error)

	svc := newLoginService(db)
	meta := &services.RequestMeta{IPAddress: "198.51.100.10", UserAgent: "go-test"}

	_, err := svc.Login(context.Background(),
		models.LoginRequest{Username: "journalbadhash", Password: "correctpass"}, meta)
	require.Error(t, err)

	var ev models.AuthEvent
	require.NoError(t, db.Where("username = ?", "journalbadhash").
		Where("event_type = ?", models.AuthEventLoginBadHash).First(&ev).Error,
		"битый хеш обязан оставлять собственный тип события")
	assert.False(t, ev.Success)
	assert.Nil(t, ev.UserID, "как и при аварии базы, событие не входит в личную историю пользователя")
	assert.Contains(t, ev.Detail, "invalid PHC format", "в журнале настоящая причина, не общая фраза")

	// Ни одной записи «неудачный вход» на битом хеше: именно эта подмена причины
	// и превращала дефект данных в «человек вводит неправильные пароли».
	var failedCount int64
	require.NoError(t, db.Model(&models.AuthEvent{}).
		Where("username = ? AND event_type = ?", "journalbadhash", models.AuthEventLoginFailed).
		Count(&failedCount).Error)
	assert.Zero(t, failedCount)
}
