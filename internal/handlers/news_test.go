package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Активные новости/объявления доступны любому авторизованному; управление (all/CUD)
// гейтится page.admin на роут-middleware (Ф5, ранее service checkAdmin).

func TestNews_GetActive_AnyAuthenticated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "newsreader", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/news", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNews_GetAll_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "newsuser", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/news/all", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/news/all", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNews_Create_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "newsnonadmin", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/news", `{"title":"Запрещено"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestNews_Create_Admin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/news", `{"title":"Новость от админа"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusCreated, rec.Code)

	// созданная новость видна в админском списке
	rec = testutil.GET(t, e, "/news/all", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.ParseSlice(t, rec)
	found := false
	for _, item := range list {
		if item["title"] == "Новость от админа" {
			found = true
			break
		}
	}
	assert.True(t, found, "созданная новость не найдена в /news/all")
}

func TestAnnouncements_GetActive_AnyAuthenticated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "annreader", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/announcements/active", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAnnouncements_Create_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "annnonadmin", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/announcements", `{"title":"Запрещено"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAnnouncements_Create_Admin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/announcements", `{"title":"Объявление от админа"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusCreated, rec.Code)
}
