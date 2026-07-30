package middleware_test

import (
	"sort"
	"strings"
	"testing"

	mw "systemburo/internal/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Здесь только проверки, которым не нужна база: DB-backed тесты гейта лежат в
// internal/handlers (единственный пакет с базой, иначе параллельные бинари дерутся
// за общую тест-БД). Сверка белого списка с реальным роутером - тоже там.

// Золотой список: расширение исключений - это расширение дыры в гейте, и оно должно
// быть осознанным, а не побочным эффектом правки соседнего кода.
func TestPDConsentGate_WhitelistIsGolden(t *testing.T) {
	want := []string{
		"GET /api/consents/gate",
		"GET /api/permissions/my",
		"GET /api/settings/data-processing/document",
		"GET /api/settings/data-processing/document/meta",
		"GET /api/settings/notifications",
		"GET /api/users/me",
		"GET /api/users/me/theme",
		"POST /api/consents/accept",
		"POST /api/events/ticket",
		"POST /api/logout",
		"POST /api/logout-all",
	}
	got := make([]string, 0, len(mw.PDConsentWhitelist))
	for key := range mw.PDConsentWhitelist {
		got = append(got, key)
	}
	sort.Strings(got)
	assert.Equal(t, want, got)
}

// Ключ белого списка собирается как "МЕТОД ПУТЬ" c.Path() - без префикса /api он бы
// не совпал ни с чем, и обязательные роуты оказались бы закрыты.
func TestPDConsentGate_WhitelistKeysCarryAPIPrefix(t *testing.T) {
	for key := range mw.PDConsentWhitelist {
		parts := strings.SplitN(key, " ", 2)
		require.Len(t, parts, 2, "ключ %q должен быть вида \"МЕТОД ПУТЬ\"", key)
		assert.True(t, strings.HasPrefix(parts[1], "/api/"), "ключ %q обязан нести префикс /api", key)
	}
}
