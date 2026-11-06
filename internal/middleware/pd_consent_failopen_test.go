package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	mw "systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

type consentGateTestSettings struct {
	services.SettingsService
	err error
}

func (s consentGateTestSettings) GetPDConsentSettings(context.Context) (*models.PDConsentSettings, error) {
	return &models.PDConsentSettings{Required: true, Version: 1, Text: "Согласие"}, s.err
}

type consentGateTestRecords struct{ services.ConsentService }

func (consentGateTestRecords) ActiveVersion(context.Context, int, string) (int, error) {
	return 0, errors.New("records unavailable")
}

func TestPDConsentGate_UnavailableOnDBError(t *testing.T) {
	for _, tt := range []struct {
		name        string
		settingsErr error
	}{{"settings", errors.New("settings unavailable")}, {"accepted version", nil}} {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			gate := services.NewPDConsentGateService(consentGateTestRecords{}, consentGateTestSettings{err: tt.settingsErr}, 0)
			e.GET("/guarded", func(c echo.Context) error { t.Fatal("business handler ran after consent lookup failed"); return nil }, func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("user_id", 42); return next(c) }
			}, mw.PDConsentGate(gate))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guarded", nil))
			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			require.Empty(t, rec.Header().Get("X-PD-Consent-Required"))
			require.NotContains(t, rec.Body.String(), "consent_required")
		})
	}
}
