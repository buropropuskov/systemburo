package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "systemburo/internal/middleware"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivateAPICache_JSONAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		status int
	}{
		{"login success", "/api/login", http.StatusOK},
		{"login failure", "/api/login", http.StatusUnauthorized},
		{"refresh failure", "/api/refresh-token", http.StatusUnauthorized},
		{"public settings", "/api/settings/public", http.StatusOK},
		{"private success", "/api/users/me", http.StatusOK},
		{"permission failure", "/api/system-tables/1/history", http.StatusForbidden},
		{"service failure", "/api/users/me", http.StatusServiceUnavailable},
		{"API root", "/api", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(mw.PrivateAPICache())
			errorHandled := false
			e.HTTPErrorHandler = func(c *echo.Context, err error) {
				errorHandled = true
				assert.Equal(t, "no-store", c.Response().Header().Get(echo.HeaderCacheControl))
				echo.DefaultHTTPErrorHandler(false)(c, err)
			}
			e.GET(tc.path, func(c *echo.Context) error {
				assert.Equal(t, "no-store", c.Response().Header().Get(echo.HeaderCacheControl))
				if tc.status != http.StatusOK {
					return echo.NewHTTPError(tc.status, "request denied")
				}
				return c.JSON(http.StatusOK, map[string]string{"value": "response"})
			})
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path+"?source=test", nil))
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, "no-store", rec.Result().Header.Get(echo.HeaderCacheControl))
			assert.Equal(t, tc.status != http.StatusOK, errorHandled)
			if errorHandled {
				assert.Contains(t, rec.Body.String(), "request denied")
			} else {
				assert.JSONEq(t, `{"value":"response"}`, rec.Body.String())
			}
		})
	}
}

func TestPrivateAPICache_PrecedesJWTRejection(t *testing.T) {
	e := echo.New()
	e.Use(mw.PrivateAPICache())
	called := false
	e.GET("/api/users/me", func(c *echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	}, mw.JWTAuth([]byte("unused-without-authorization"), nil))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/me", nil))
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "no-store", rec.Result().Header.Get(echo.HeaderCacheControl))
}

func TestPrivateAPICache_NotFound(t *testing.T) {
	e := echo.New()
	e.Use(mw.PrivateAPICache())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "no-store", rec.Result().Header.Get(echo.HeaderCacheControl))
}

func TestPrivateAPICache_LeavesNonAPIResponsesUnchanged(t *testing.T) {
	for _, tc := range []struct {
		path   string
		policy string
	}{
		{"/assets/app.hash.js", "public, max-age=31536000, immutable"},
		{"/", "no-cache"},
		{"/health", ""},
		{"/apiary/file.js", "public, max-age=60"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			e := echo.New()
			e.Use(mw.PrivateAPICache())
			e.GET(tc.path, func(c *echo.Context) error {
				assert.Equal(t, tc.policy, c.Response().Header().Get(echo.HeaderCacheControl))
				return c.String(http.StatusOK, "unchanged")
			})
			rec := httptest.NewRecorder()
			if tc.policy != "" {
				rec.Header().Set(echo.HeaderCacheControl, tc.policy)
			}
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tc.policy, rec.Result().Header.Get(echo.HeaderCacheControl))
			assert.Equal(t, "unchanged", rec.Body.String())
		})
	}
}

func TestPrivateAPICache_PreservesSSEPolicyAndFlush(t *testing.T) {
	e := echo.New()
	e.Use(mw.PrivateAPICache())
	e.GET("/api/events", func(c *echo.Context) error {
		h := c.Response().Header()
		assert.Equal(t, "no-store", h.Get(echo.HeaderCacheControl))
		h.Set(echo.HeaderCacheControl, "no-cache")
		h.Set(echo.HeaderContentType, "text/event-stream")
		h.Set(echo.HeaderConnection, "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		c.Response().WriteHeader(http.StatusOK)
		_, err := c.Response().Write([]byte(": connected\n\n"))
		if err != nil {
			return err
		}
		_ = http.NewResponseController(c.Response()).Flush()
		return nil
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/events?ticket=test", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-cache", rec.Result().Header.Get(echo.HeaderCacheControl))
	assert.Equal(t, "text/event-stream", rec.Result().Header.Get(echo.HeaderContentType))
	assert.Equal(t, "keep-alive", rec.Result().Header.Get(echo.HeaderConnection))
	assert.Equal(t, "no", rec.Result().Header.Get("X-Accel-Buffering"))
	assert.True(t, rec.Flushed)
	assert.Equal(t, ": connected\n\n", rec.Body.String())
}

func TestPrivateAPICache_PreservesFileStream(t *testing.T) {
	e := echo.New()
	e.Use(mw.PrivateAPICache())
	payload := []byte{0, 1, 2, 255, 10}
	e.GET("/api/files/1", func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="document.bin"`)
		return c.Stream(http.StatusOK, "application/octet-stream", bytes.NewReader(payload))
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files/1", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-store", rec.Result().Header.Get(echo.HeaderCacheControl))
	assert.Equal(t, "application/octet-stream", rec.Result().Header.Get(echo.HeaderContentType))
	assert.Equal(t, `attachment; filename="document.bin"`, rec.Result().Header.Get(echo.HeaderContentDisposition))
	assert.Equal(t, payload, rec.Body.Bytes())
}
