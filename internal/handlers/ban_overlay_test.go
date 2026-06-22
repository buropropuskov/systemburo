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

// BanCheck оставляет забаненному доступ ТОЛЬКО на чтение: GET-ручки (включая
// /permissions/my для плашки и /users/me для ФИО в кабинете) проходят, а любая
// мутация (POST/PUT/DELETE) -- 403. Так кабинет грузится read-only под плашкой.
func TestBanCheck_ReadOnlyForBanned(t *testing.T) {
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
	e.POST("/api/applications", ok, inject, banCheck)
	e.PUT("/api/users/me", ok, inject, banCheck)
	e.DELETE("/api/notifications", ok, inject, banCheck)

	for _, p := range []string{"/api/permissions/my", "/api/users/me"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		assert.Equal(t, http.StatusOK, rec.Code, "GET %s доступен забаненному (read-only кабинет)", p)
	}

	mutations := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/applications"},
		{http.MethodPut, "/api/users/me"},
		{http.MethodDelete, "/api/notifications"},
	}
	for _, m := range mutations {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(m.method, m.path, nil))
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s заблокирован для забаненного", m.method, m.path)
	}
}
