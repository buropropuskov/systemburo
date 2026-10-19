package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestArchiveOriginal_UnavailablePolicyDoesNotAllowOriginal(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler
	e.GET("/", func(c echo.Context) error { return requireArchiveOriginal(c, nil) })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.NotContains(t, rec.Header().Get("Content-Type"), "application/zip")
}
