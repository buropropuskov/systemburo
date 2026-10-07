package handlers_test

import (
	"strings"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// Эти входы не проходят protected-группу. Каждый имеет отдельный контракт;
// uploads использует FileAccess, остальные классифицированы как public в S1.
var securityGateEntrances = map[string]string{
	"GET /health":                    "публичная проверка живости",
	"POST /api/login":                "проверка пароля и состояния при входе",
	"POST /api/refresh-token":        "состояние пользователя и запись refresh-cookie",
	"GET /api/user-types":            "справочник до входа",
	"GET /api/settings/contacts":     "публичные контакты",
	"GET /api/settings/maintenance":  "публичный статус техработ",
	"GET /api/events":                "одноразовый SSE-билет, затем ограниченная жизнь потока",
	"GET /api/file-archive/download": "одноразовый билет на конкретный период",
	"GET /api/uploads*":              "FileAccess и проверка принадлежности скана",
}

func securityProtectedRoutes(t *testing.T, e *echo.Echo) []string {
	t.Helper()
	actual := registeredRoutes(e.Routes())
	for key := range actual {
		_, ok := routeAccessRegistry[key]
		require.True(t, ok, "маршрут без классификации S1: %s", key)
	}
	for key, access := range routeAccessRegistry {
		require.True(t, actual[key], "устаревшая классификация: %s", key)
		if access.class == accessPublic {
			require.NotEmpty(t, securityGateEntrances[key], "отдельный вход не разобран: %s", key)
		}
	}
	for key, reason := range securityGateEntrances {
		require.True(t, actual[key], "устаревший отдельный вход: %s", key)
		require.NotEmpty(t, reason)
		if key != "GET /api/uploads*" {
			require.Equal(t, accessPublic, routeAccessRegistry[key].class, key)
		}
	}
	for name, whitelist := range map[string]map[string]bool{
		"consent":  mw.PDConsentWhitelist,
		"password": mw.MustChangePasswordWhitelist,
	} {
		for key, enabled := range whitelist {
			require.True(t, enabled, "%s: неактивное исключение %s", name, key)
			require.True(t, actual[key], "%s: несуществующее исключение %s", name, key)
			require.Empty(t, securityGateEntrances[key], "%s: исключение вне protected %s", name, key)
		}
	}
	var result []string
	for _, key := range sortedKeys(actual) {
		if securityGateEntrances[key] == "" {
			require.True(t, strings.HasPrefix(strings.SplitN(key, " ", 2)[1], "/api/"), key)
			result = append(result, key)
		}
	}
	require.NotEmpty(t, result)
	return result
}

func TestSecurityGates_Registry(t *testing.T) {
	e, _, _, cleanup := testutil.SetupTestAppWithAllGates(t)
	defer cleanup()
	routes := securityProtectedRoutes(t, e)
	t.Logf("protected=%d, separate entrances=%d", len(routes), len(securityGateEntrances))
}
