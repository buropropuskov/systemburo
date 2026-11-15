package httpx_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/httpx"

	"github.com/labstack/echo/v5"
)

func TestStatusCode(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		committed bool
		status    int
		want      int
	}{
		{"success", nil, false, 200, 200},
		{"explicit success", nil, false, 202, 202},
		{"domain", apperr.Validation("invalid"), false, 200, 400},
		{"wrapped domain", fmt.Errorf("context: %w", apperr.Conflict("conflict")), false, 200, 409},
		{"wrapped echo", fmt.Errorf("context: %w", echo.NewHTTPError(418, http.StatusText(418))), false, 200, 418},
		{"domain priority", apperr.Internal("safe", echo.ErrNotFound), false, 200, 500},
		{"generic", errors.New("private"), false, 200, 500},
		{"committed", apperr.Internal("safe"), true, 202, 202},
		{"committed failure", echo.ErrNotFound, true, 503, 503},
		{"invalid domain low", apperr.New(99, "safe"), false, 200, 500},
		{"success code in error", apperr.New(200, "safe"), false, 200, 500},
		{"redirect code in error", echo.NewHTTPError(302, http.StatusText(302)), false, 200, 500},
		{"informational response", nil, false, 100, 100},
		{"invalid echo high", echo.NewHTTPError(600, http.StatusText(600)), false, 200, 500},
		{"invalid response", nil, false, 0, 500},
		{"valid upper bound", echo.NewHTTPError(599, http.StatusText(599)), false, 200, 599},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
			response, _ := echo.UnwrapResponse(c.Response())
			response.Status, response.Committed = tt.status, tt.committed
			if got := httpx.StatusCode(c, tt.err); got != tt.want {
				t.Fatalf("status=%d, want %d", got, tt.want)
			}
			if response.Status != tt.status || response.Committed != tt.committed {
				t.Fatal("status resolution changed the response")
			}
		})
	}
}
