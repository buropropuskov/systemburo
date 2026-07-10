package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type authEventsResp struct {
	Success bool                         `json:"success"`
	Data    models.AuthEventPageResponse `json:"data"`
}

func parseAuthEvents(t *testing.T, body []byte) authEventsResp {
	t.Helper()
	var r authEventsResp
	require.NoError(t, json.Unmarshal(body, &r))
	return r
}

func seedAuthEvent(t *testing.T, db *gorm.DB, userID int, username, eventType string, success bool, at time.Time, detail string) {
	t.Helper()
	ev := models.AuthEvent{
		UserID:    &userID,
		Username:  username,
		EventType: eventType,
		Success:   success,
		IPAddress: "192.168.1.42",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0) Chrome/120",
		Detail:    detail,
		CreatedAt: at,
	}
	require.NoError(t, db.Create(&ev).Error)
}

// seedLoginHistory наполняет auth_events детерминированным набором для целевого юзера.
// Возвращает user_id и хронологический порядок (старые -> новые).
func seedLoginHistory(t *testing.T, db *gorm.DB, username string) int {
	t.Helper()
	var u models.User
	require.NoError(t, db.Where("username = ?", username).First(&u).Error)

	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	seedAuthEvent(t, db, u.ID, username, models.AuthEventLoginSuccess, true, base, "")
	seedAuthEvent(t, db, u.ID, username, models.AuthEventLoginFailed, false, base.Add(5*time.Minute), "wrong password")
	seedAuthEvent(t, db, u.ID, username, models.AuthEventLoginSuccess, true, base.AddDate(0, 0, 1), "")
	seedAuthEvent(t, db, u.ID, username, models.AuthEventLogout, true, base.AddDate(0, 0, 1).Add(8*time.Hour), "")
	seedAuthEvent(t, db, u.ID, username, models.AuthEventLoginFailed, false, base.AddDate(0, 0, 2).Add(12*time.Hour), "wrong password")
	seedAuthEvent(t, db, u.ID, username, models.AuthEventAccountLocked, false, base.AddDate(0, 0, 2).Add(12*time.Hour+time.Minute), "10 failed attempts")
	return u.ID
}

func TestAuthEvents_ListForUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	testutil.RegisterUser(t, e, "loguser", "password123", 1, td.OrgID, td.CompanyID)
	seedLoginHistory(t, db, "loguser")

	rec := testutil.GET(t, e, "/users/loguser/auth-events", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := parseAuthEvents(t, rec.Body.Bytes())
	require.Len(t, resp.Data.Items, 6)
	assert.Equal(t, int64(6), resp.Data.Total)
	// Новые сверху: последним засеян account_locked -> он первый.
	assert.Equal(t, models.AuthEventAccountLocked, resp.Data.Items[0].EventType)
	assert.False(t, resp.Data.Items[0].Success)
	assert.Equal(t, "10 failed attempts", resp.Data.Items[0].Detail)
	// created_at строго убывает.
	for i := 1; i < len(resp.Data.Items); i++ {
		assert.Falsef(t, resp.Data.Items[i].CreatedAt.After(resp.Data.Items[i-1].CreatedAt),
			"порядок должен быть новые->старые на позиции %d", i)
	}
	// Персональные поля (ip/ua) отдаются, user_id/username не протекают в DTO.
	assert.Equal(t, "192.168.1.42", resp.Data.Items[0].IPAddress)
	assert.NotEmpty(t, resp.Data.Items[0].UserAgent)
}

func TestAuthEvents_FilterByCategory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	testutil.RegisterUser(t, e, "loguser", "password123", 1, td.OrgID, td.CompanyID)
	seedLoginHistory(t, db, "loguser")

	// failed -> только login_failed (2 шт).
	rec := testutil.GET(t, e, "/users/loguser/auth-events?category=failed", h)
	resp := parseAuthEvents(t, rec.Body.Bytes())
	require.Len(t, resp.Data.Items, 2)
	assert.Equal(t, int64(2), resp.Data.Total)
	for _, it := range resp.Data.Items {
		assert.Equal(t, models.AuthEventLoginFailed, it.EventType)
	}

	// locked -> login_locked + account_locked (1 шт: account_locked).
	rec = testutil.GET(t, e, "/users/loguser/auth-events?category=locked", h)
	resp = parseAuthEvents(t, rec.Body.Bytes())
	require.Len(t, resp.Data.Items, 1)
	assert.Equal(t, models.AuthEventAccountLocked, resp.Data.Items[0].EventType)

	// login -> только login_success (2 шт).
	rec = testutil.GET(t, e, "/users/loguser/auth-events?category=login", h)
	resp = parseAuthEvents(t, rec.Body.Bytes())
	require.Len(t, resp.Data.Items, 2)
	for _, it := range resp.Data.Items {
		assert.Equal(t, models.AuthEventLoginSuccess, it.EventType)
	}

	// Неизвестная категория -> без фильтра по типу, все 6.
	rec = testutil.GET(t, e, "/users/loguser/auth-events?category=bogus", h)
	resp = parseAuthEvents(t, rec.Body.Bytes())
	assert.Len(t, resp.Data.Items, 6)
}

func TestAuthEvents_FilterByPeriod(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	testutil.RegisterUser(t, e, "loguser", "password123", 1, td.OrgID, td.CompanyID)
	seedLoginHistory(t, db, "loguser")

	// Только события 2026-07-01 (2 шт: login_success + login_failed).
	rec := testutil.GET(t, e, "/users/loguser/auth-events?from=2026-07-01&to=2026-07-01", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := parseAuthEvents(t, rec.Body.Bytes())
	assert.Len(t, resp.Data.Items, 2, "to включительно должно захватывать весь день")
	assert.Equal(t, int64(2), resp.Data.Total)

	// Некорректная дата -> 400.
	rec = testutil.GET(t, e, "/users/loguser/auth-events?from=01.07.2026", h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthEvents_Pagination(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	testutil.RegisterUser(t, e, "loguser", "password123", 1, td.OrgID, td.CompanyID)
	seedLoginHistory(t, db, "loguser")

	rec := testutil.GET(t, e, "/users/loguser/auth-events?limit=2&page=1", h)
	resp := parseAuthEvents(t, rec.Body.Bytes())
	require.Len(t, resp.Data.Items, 2)
	assert.Equal(t, int64(6), resp.Data.Total)
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 2, resp.Data.Limit)
	firstPageTop := resp.Data.Items[0].ID

	rec = testutil.GET(t, e, "/users/loguser/auth-events?limit=2&page=2", h)
	resp = parseAuthEvents(t, rec.Body.Bytes())
	require.Len(t, resp.Data.Items, 2)
	assert.NotEqual(t, firstPageTop, resp.Data.Items[0].ID, "вторая страница - другие записи")
}

func TestAuthEvents_UnknownUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/users/nonexistent/auth-events", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAuthEvents_RequiresUsersPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterUser(t, e, "loguser", "password123", 1, td.OrgID, td.CompanyID)

	// Без токена -> 401.
	rec := testutil.GET(t, e, "/users/loguser/auth-events", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Обычный пользователь без page.admin.users -> 403.
	token := testutil.RegisterAndLogin(t, e, "regular_log", "password123", 1, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/users/loguser/auth-events", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
