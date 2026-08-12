package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Запрет повторного использования пополняет перечень при смене пароля, поэтому у
// учётной записи, заведённой до внедрения запрета, перечень пуст: первая смена
// запоминает новый пароль, а прежний не запоминает никто, и вернуться к нему можно
// свободно. Найдено живой проверкой на стенде. Бэкфилл кладёт действующий пароль в
// перечень, чтобы правило работало и для тех, кто был заведён раньше.

// TestBackfillUsedPasswords_ProtectsPasswordOfExistingUser: главное - после
// бэкфилла работник не может вернуться к паролю, который был у него до внедрения.
func TestBackfillUsedPasswords_ProtectsPasswordOfExistingUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const before = "dovnedreniya12345"
	const after = "poslesmeny6789012"

	// Учётная запись «из прошлого»: заведена напрямую, перечня прежних паролей у
	// неё нет - ровно как у записей, живших до появления таблицы.
	u := mkBareUser(t, db, td, "backfill_old_user", before)
	require.Empty(t, usedPasswordRows(t, db, u.ID), "у записи из прошлого перечень пуст")

	require.NoError(t, database.BackfillUsedPasswords(db))

	rows := usedPasswordRows(t, db, u.ID)
	require.Len(t, rows, 1, "бэкфилл обязан внести действующий пароль")
	assert.Equal(t, currentPasswordHash(t, db, u.ID), rows[0].PasswordHash)

	token, _ := testutil.LoginUser(t, e, u.Username, before)
	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+before+`","new_password":"`+after+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	token, _ = testutil.LoginUser(t, e, u.Username, after)
	rec = testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+after+`","new_password":"`+before+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "уже использовался",
		"пароль, действовавший до внедрения, обязан быть защищён наравне с прочими")
}

// TestBackfillUsedPasswords_Idempotent: миграции прогоняются при каждом запуске
// сервера. Повторный проход не должен ни плодить записи, ни трогать тех, кто уже
// менял пароль после внедрения, - иначе действующий пароль ложился бы в перечень
// заново на каждом старте и вытеснял оттуда настоящие прежние.
func TestBackfillUsedPasswords_Idempotent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	u := mkBareUser(t, db, td, "backfill_idempotent", "ishodniyparol1234")
	require.NoError(t, database.BackfillUsedPasswords(db))
	require.NoError(t, database.BackfillUsedPasswords(db))
	assert.Len(t, usedPasswordRows(t, db, u.ID), 1, "повторный проход не добавляет записей")

	// У записи с непустым перечнем бэкфиллу делать нечего.
	withHistory := mkBareUser(t, db, td, "backfill_has_history", "drugoyparol56789")
	require.NoError(t, db.Create(&models.UsedPassword{
		UserID: withHistory.ID, PasswordHash: services.HashPassword("prezhniy901234567"),
	}).Error)
	require.NoError(t, database.BackfillUsedPasswords(db))
	assert.Len(t, usedPasswordRows(t, db, withHistory.ID), 1,
		"тем, кто уже менял пароль, бэкфилл ничего не дописывает")
}
