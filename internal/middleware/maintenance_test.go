package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// stubMaintenanceService позволяет тестировать middleware без БД.
type stubMaintenanceService struct {
	enabled bool
}

func (s *stubMaintenanceService) GetStatus(ctx context.Context) (*services.MaintenanceStatus, error) {
	return &services.MaintenanceStatus{Enabled: s.enabled}, nil
}
func (s *stubMaintenanceService) GetStatusCached(ctx context.Context) *services.MaintenanceStatus {
	return &services.MaintenanceStatus{Enabled: s.enabled}
}
func (s *stubMaintenanceService) Enable(ctx context.Context, _ int, _, _, _ string) error {
	s.enabled = true
	return nil
}
func (s *stubMaintenanceService) Disable(ctx context.Context, _ int, _ string) error {
	s.enabled = false
	return nil
}
func (s *stubMaintenanceService) InvalidateCache() {}

func callMW(t *testing.T, enabled bool, typeID int) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("type_id", typeID)

	mw := MaintenanceBlock(&stubMaintenanceService{enabled: enabled})
	h := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
	if err := h(ctx); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code
		}
		return http.StatusInternalServerError
	}
	return rec.Code
}

func TestMaintenanceBlock_AllowsSuperAdmin(t *testing.T) {
	assert.Equal(t, http.StatusOK, callMW(t, true, 6))
}

func TestMaintenanceBlock_BlocksRegularWhenEnabled(t *testing.T) {
	assert.Equal(t, http.StatusServiceUnavailable, callMW(t, true, 1))
}

func TestMaintenanceBlock_AllowsRegularWhenDisabled(t *testing.T) {
	assert.Equal(t, http.StatusOK, callMW(t, false, 1))
}
