package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	mw "systemburo/internal/middleware"
	"systemburo/internal/services"
)

func TestSecurityS2_BanCheckDBErrorAndLogout(t *testing.T) {
	brokenDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "postgres://nobody:nobody@127.0.0.1:1/nonexistent"}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true})
	require.NoError(t, err)
	sqlDB, err := brokenDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close()) // deterministic failure; no connection attempt is needed
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/users/me", http.StatusServiceUnavailable},
		{http.MethodGet, "/api/applications", http.StatusServiceUnavailable},
		{http.MethodPost, "/api/logout", http.StatusNoContent},
		{http.MethodPost, "/api/logout-all", http.StatusNoContent},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			e := echo.New()
			called := false
			inject := func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("user_id", 42); return next(c) }
			}
			e.Add(tc.method, tc.path, func(c echo.Context) error { called = true; return c.NoContent(http.StatusNoContent) }, inject,
				mw.BanCheck(services.NewBanCheckService(brokenDB, time.Hour)))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			require.Equal(t, tc.want == http.StatusNoContent, called)
		})
	}
}

func TestSecurityS2_BannedWhitelistGolden(t *testing.T) {
	want := map[string]bool{
		"GET /api/users/me":                               true,
		"GET /api/users/current":                          true,
		"GET /api/user-data":                              true,
		"GET /api/users/me/theme":                         true,
		"GET /api/permissions/my":                         true,
		"GET /api/applications/user":                      true,
		"GET /api/applications/user/status-updates-count": true,
		"GET /api/applications/:id":                       true,
		"GET /api/applications/:id/details":               true,
	}
	require.Equal(t, want, mw.BannedReadWhitelist)
}
