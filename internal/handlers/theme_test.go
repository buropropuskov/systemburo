package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheme_Get_NewUserIsNull(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/users/me/theme", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	resp := testutil.ParseMap(t, rec)
	val, ok := resp["theme"]
	assert.True(t, ok, "theme key must be present")
	assert.Nil(t, val, "тема не выбрана - клиент показывает светлую")

	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	assert.Nil(t, u.Theme, "DB column must be null for new user")
}

func TestTheme_Set_ThenGetReturnsTheme(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/users/me/theme", `{"theme":"dark"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "Theme saved", testutil.ParseMessage(t, rec))

	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	require.NotNil(t, u.Theme)
	assert.Equal(t, models.ThemeDark, *u.Theme)

	recGet := testutil.GET(t, e, "/users/me/theme", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recGet.Code, "body: %s", recGet.Body.String())
	assert.Equal(t, models.ThemeDark, testutil.ParseMap(t, recGet)["theme"])
}

func TestTheme_Set_OverwritesPrevious(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, "/users/me/theme", `{"theme":"dark"}`, testutil.AuthHeader(token)).Code)
	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, "/users/me/theme", `{"theme":"light"}`, testutil.AuthHeader(token)).Code)

	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	require.NotNil(t, u.Theme)
	assert.Equal(t, models.ThemeLight, *u.Theme, "последний выбор перезаписывает прежний")
}

// Неизвестный id отклоняем: в колонке должны лежать только темы, для которых у
// фронта есть палитра, иначе юзер получит пустые переменные.
func TestTheme_Set_RejectsUnknownTheme(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, "/users/me/theme", `{"theme":"dark"}`, testutil.AuthHeader(token)).Code)

	rec := testutil.PUT(t, e, "/users/me/theme", `{"theme":"neon-hacker"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "неизвестная тема должна отклоняться")

	rec = testutil.PUT(t, e, "/users/me/theme", `{"theme":""}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "пустая тема должна отклоняться")

	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	require.NotNil(t, u.Theme)
	assert.Equal(t, models.ThemeDark, *u.Theme, "отклонённый запрос не должен затирать выбранную тему")
}

// Тема - личная настройка: юзер A не видит и не меняет тему юзера B.
func TestTheme_IsPerUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userToken := testutil.RegisterAndLogin(t, e, "theme_user", "Password123!", 1, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, "/users/me/theme", `{"theme":"dark"}`, testutil.AuthHeader(userToken)).Code)

	rec := testutil.GET(t, e, "/users/me/theme", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Nil(t, testutil.ParseMap(t, rec)["theme"], "выбор одного юзера не должен протекать другому")
}

func TestTheme_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	assert.Equal(t, http.StatusUnauthorized, testutil.GET(t, e, "/users/me/theme", nil).Code)
	assert.Equal(t, http.StatusUnauthorized,
		testutil.PUT(t, e, "/users/me/theme", `{"theme":"dark"}`, nil).Code)
}
