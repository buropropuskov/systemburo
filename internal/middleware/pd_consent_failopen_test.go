package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/config"
	mw "systemburo/internal/middleware"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Гейт согласия висит на КАЖДОМ protected-запросе, поэтому сбой чтения настроек не
// должен класть весь API: временная недоступность базы запрёт всех разом, включая
// тех, кто согласие давно дал. Поведение зеркалит проверку блокировки - пропускаем и
// пишем в журнал.
func TestPDConsentGate_FailOpenOnDBError(t *testing.T) {
	// Соединение никуда не ведёт: PingOnConnect отключён, поэтому gorm.Open проходит,
	// а любой запрос падает - ровно то состояние, которое имитируем.
	brokenDB, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "postgres://nobody:nobody@127.0.0.1:1/nonexistent"}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true},
	)
	require.NoError(t, err)

	gate := services.NewPDConsentGateService(
		services.NewConsentService(brokenDB),
		services.NewSettingsService(brokenDB, &config.Config{}),
		0,
	)

	// Без этой проверки тест был бы зелёным вхолостую: при рабочей базе и настройках
	// по умолчанию гейт тоже пропускает, и 200 ничего не доказывал бы.
	_, reqErr := gate.Requirement(context.Background())
	require.Error(t, reqErr, "сломанное соединение обязано давать ошибку чтения настроек")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/guarded", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/guarded")
	c.Set("user_id", 42)

	handler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	err = mw.PDConsentGate(gate)(handler)(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code, "сбой базы обязан пропускать, а не запирать систему")
	assert.Empty(t, rec.Header().Get("X-PD-Consent-Required"))
}
