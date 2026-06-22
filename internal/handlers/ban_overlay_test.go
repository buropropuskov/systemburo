package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/middleware"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// BanCheck пропускает забаненного на /api/permissions/my (allowlist) - оттуда
// фронт узнаёт статус и показывает плашку блокировки. Прочие protected-роуты
// остаются 403 (окно access-токена закрыто).
func TestBanCheck_AllowlistPermissionsMy(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	userID, _, cleanup := setupMWUser(t, db, false, true) // забанен
	defer cleanup()

	banCheck := middleware.BanCheck(services.NewBanCheckService(db, time.Minute))
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", userID)
			return next(c)
		}
	}
	ok := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }

	e := echo.New()
	e.GET("/api/permissions/my", ok, inject, banCheck)
	e.GET("/api/users/me", ok, inject, banCheck)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/permissions/my", nil))
	assert.Equal(t, http.StatusOK, rec.Code, "/permissions/my доступен забаненному (для плашки)")

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/me", nil))
	assert.Equal(t, http.StatusForbidden, rec.Code, "/users/me остаётся заблокированным")
}
