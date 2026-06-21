package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
)

// Раздел «Мониторинг запросов» целиком admin-only (page.admin) -- авторизация на
// роут-middleware (Ф5, ранее service checkAdmin).

func TestRequestLogs_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "rluser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	for _, path := range []string{"/request-logs", "/request-logs/stats", "/request-logs/users", "/request-logs/export"} {
		rec := testutil.GET(t, e, path, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "non-admin must be forbidden on %s", path)
	}
}

func TestRequestLogs_Admin_Ok(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/request-logs", h)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/request-logs/stats", h)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequestLogs_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/request-logs", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
