package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/handlers"
	"systemburo/internal/httpx"
	"systemburo/internal/logmask"
	mw "systemburo/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestSafeLoggingPipelineRecoversWithoutPrivateValues(t *testing.T) {
	old := slog.Default()
	var out bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{ReplaceAttr: logmask.ReplaceAttr})))
	defer slog.SetDefault(old)
	e := echo.New()
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	e.Use(mw.PrivateAPICache(), mw.RequestID(), mw.DiagnosticLogger(), mw.SafeRecover())
	e.GET("/api/synthetic/:name", func(c echo.Context) error {
		require.Equal(t, c.Response().Header().Get("X-Request-ID"), httpx.RequestID(c.Request().Context()))
		panic("PRIVATE_PANIC_MARKER")
	})
	req := httptest.NewRequest(http.MethodGet, "/api/synthetic/PRIVATE_PATH_MARKER?token=PRIVATE_QUERY_MARKER", nil)
	req.Header.Set("Authorization", "Bearer PRIVATE_HEADER_MARKER")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	require.NotEmpty(t, rec.Header().Get("X-Request-ID"))
	require.Contains(t, out.String(), "status=500")
	require.Contains(t, out.String(), rec.Header().Get("X-Request-ID"))
	for _, marker := range []string{"PRIVATE_PANIC_MARKER", "PRIVATE_PATH_MARKER", "PRIVATE_QUERY_MARKER", "PRIVATE_HEADER_MARKER"} {
		require.NotContains(t, out.String(), marker)
		require.NotContains(t, rec.Body.String(), marker)
	}
}
