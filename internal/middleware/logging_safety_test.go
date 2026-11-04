package middleware

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRequestLogWriterUsesFinalErrorStatusAndRoute(t *testing.T) {
	for _, path := range []string{"/api/users/synthetic-private-name", "/api/search"} {
		t.Run(path, func(t *testing.T) {
			writer := &RequestLogWriter{queue: make(chan models.RequestLogs, 1)}
			e := echo.New()
			e.Use(writer.Middleware())
			route := path
			if strings.Contains(path, "/users/") {
				route = "/api/users/:username"
			}
			e.GET(route, func(c echo.Context) error {
				return echo.NewHTTPError(http.StatusForbidden, "synthetic refusal")
			})
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?document=synthetic-secret", nil))
			require.Equal(t, http.StatusForbidden, rec.Code)
			select {
			case entry := <-writer.queue:
				require.Equal(t, http.StatusForbidden, *entry.ResponseStatus)
				require.Equal(t, route, *entry.URL)
				require.NotContains(t, *entry.URL, "synthetic")
			default:
				t.Fatal("refusal must remain visible in the request log")
			}
		})
	}
}

func TestPDAuditStatusUsesDomainAndCommittedResponse(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	require.Equal(t, http.StatusForbidden, pdStatusCode(c, apperr.Forbidden("synthetic")))
	require.Equal(t, http.StatusInternalServerError, pdStatusCode(c, errors.New("synthetic")))
	require.NoError(t, c.NoContent(http.StatusAccepted))
	require.Equal(t, http.StatusAccepted, pdStatusCode(c, apperr.Forbidden("synthetic")))
}

func TestDiagnosticLoggerExcludesRequestValues(t *testing.T) {
	old := slog.Default()
	var out bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&out, nil)))
	defer slog.SetDefault(old)
	e := echo.New()
	e.Use(RequestID(), DiagnosticLogger())
	e.GET("/api/users/:username", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusForbidden, "synthetic-error-marker")
	})
	req := httptest.NewRequest(http.MethodGet, "/api/users/synthetic-name-marker?ticket=synthetic-query-marker", nil)
	req.Header.Set("Authorization", "synthetic-header-marker")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, out.String(), "status=403")
	require.Contains(t, out.String(), "route=/api/users/:username")
	require.Contains(t, out.String(), rec.Header().Get(HeaderRequestID))
	for _, marker := range []string{"synthetic-name-marker", "synthetic-query-marker", "synthetic-header-marker", "synthetic-error-marker"} {
		require.NotContains(t, out.String(), marker)
	}
}

func TestRequestIDBoundsUntrustedInput(t *testing.T) {
	id := uuid.New().String()
	for _, value := range []string{id, strings.ReplaceAll(id, "-", ""), "synthetic-request-secret", strings.Repeat("a", 1000), ""} {
		e := echo.New()
		e.Use(RequestID())
		e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(HeaderRequestID, value)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		actual := rec.Header().Get(HeaderRequestID)
		_, err := uuid.Parse(actual)
		require.NoError(t, err)
		if validRequestID(value) {
			require.Equal(t, value, actual)
		} else {
			require.NotEqual(t, value, actual)
		}
	}
}
