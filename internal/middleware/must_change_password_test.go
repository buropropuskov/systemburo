package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Здесь только проверки, которым не нужна база. DB-backed тесты гейта лежат в
// internal/handlers - единственном пакете с базой (параллельные бинари дерутся за
// общую тест-БД). Там же сверка белого списка с реальным роутером.

// Золотой список: каждое исключение - это путь, открытый человеку с паролем из
// письма. Расширение должно быть осознанным, а не побочным эффектом правки рядом.
func TestMustChangePassword_WhitelistIsGolden(t *testing.T) {
	want := []string{
		"GET /api/consents/gate",
		"GET /api/permissions/my",
		"GET /api/settings/data-processing/document",
		"GET /api/settings/data-processing/document/meta",
		"GET /api/settings/notifications",
		"GET /api/settings/password-policy",
		"GET /api/users/me",
		"GET /api/users/me/theme",
		"POST /api/consents/accept",
		"POST /api/logout",
		"POST /api/logout-all",
		"PUT /api/users/me/password",
	}
	got := make([]string, 0, len(mw.MustChangePasswordWhitelist))
	for key := range mw.MustChangePasswordWhitelist {
		got = append(got, key)
	}
	sort.Strings(got)
	assert.Equal(t, want, got)
}

// Ключ собирается как "МЕТОД ПУТЬ" из c.Path() - без префикса /api он не совпал бы
// ни с чем, и сама смена пароля оказалась бы закрыта своим же гейтом.
func TestMustChangePassword_WhitelistKeysCarryAPIPrefix(t *testing.T) {
	for key := range mw.MustChangePasswordWhitelist {
		parts := strings.SplitN(key, " ", 2)
		require.Len(t, parts, 2, "ключ %q должен быть вида \"МЕТОД ПУТЬ\"", key)
		assert.True(t, strings.HasPrefix(parts[1], "/api/"), "ключ %q обязан нести префикс /api", key)
	}
}

// Гейт висит на КАЖДОМ protected-запросе, поэтому сбой чтения флага не должен
// класть весь API: временная недоступность базы иначе запрёт всех разом, включая
// тех, кто пароль давно сменил. Поведение зеркалит проверку блокировки.
func TestMustChangePassword_FailOpenOnDBError(t *testing.T) {
	// Соединение никуда не ведёт: PingOnConnect отключён, поэтому gorm.Open проходит,
	// а любой запрос падает - ровно то состояние, которое имитируем.
	brokenDB, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "postgres://nobody:nobody@127.0.0.1:1/nonexistent"}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true},
	)
	require.NoError(t, err)

	gate := services.NewPasswordChangeGateService(brokenDB, 0)

	// Без этой проверки тест был бы зелёным вхолостую: при рабочей базе и снятом
	// флаге гейт тоже пропускает, и 200 ничего не доказывал бы.
	_, reqErr := gate.Required(context.Background(), 42)
	require.Error(t, reqErr, "сломанное соединение обязано давать ошибку чтения флага")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/guarded", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/guarded")
	c.Set("user_id", 42)

	handler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	err = mw.MustChangePassword(gate)(handler)(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code, "сбой базы обязан пропускать, а не запирать систему")
	assert.Empty(t, rec.Header().Get("X-Password-Change-Required"))
}

// Без user_id в контексте (гейт по ошибке навесили до JWTAuth) в базу не ходим и
// запрос не режем: иначе публичные роуты отдавали бы требование сменить пароль
// неизвестно кому.
func TestMustChangePassword_SkipsAnonymous(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/guarded", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/guarded")

	handler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	// nil-сервис: если middleware полезет за флагом, тест упадёт паникой - это и
	// есть проверка, что до чтения дело не доходит.
	require.NoError(t, mw.MustChangePassword(nil)(handler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}
