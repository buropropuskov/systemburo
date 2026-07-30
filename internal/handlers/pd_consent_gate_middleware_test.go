package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Гейт согласия на обработку ПД (#1567): пока пользователь не согласился с текущей
// редакцией текста, protected-API отвечает 403 с маркером consent_required. Гейт
// навешивается только через SetupTestAppWithConsentGate - в остальных тестах он nil,
// иначе они получали бы 403 вместо своих ответов.
//
// Тесты лежат в handlers, а не рядом с middleware: любой DB-backed Go-тест обязан
// быть в этом пакете - `go test ./...` гоняет пакеты параллельными бинарями, и
// второй бинарь с базой дерётся с этим за общую auto_registry_test (Seed падает на
// duplicate key). Скоуп-прогон одного пакета такую гонку не показывает.

const (
	consentSettingsPath = "/settings/pd-consent"
	// Произвольная protected-ручка вне белого списка: на ней и проверяем, что
	// доступ реально закрыт. Читающая и безобидная - бан её пропускает, так что
	// отказ на ней означает именно гейт согласия.
	guardedPath = "/citizenships"

	testPassword = "password123456789012345678901234"
)

func enableConsentSettings(t *testing.T, e *echo.Echo, admin, html string) {
	t.Helper()
	rec := testutil.PUT(t, e, consentSettingsPath+"/text", fmt.Sprintf(`{"text":%q}`, html), testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.PUT(t, e, consentSettingsPath+"/required", `{"required":true}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func assertConsentBlocked(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Equal(t, "1", rec.Header().Get("X-PD-Consent-Required"),
		"фронт отличает требование согласия от нехватки прав по маркеру ответа")
	assert.Contains(t, rec.Body.String(), `"consent_required":true`)
}

// setupGate поднимает приложение с гейтом, включает запрос согласия и возвращает
// токены супер-администратора и обычного пользователя.
func setupGate(t *testing.T, html string) (*echo.Echo, *gorm.DB, string, string, func()) {
	t.Helper()
	e, db, cleanup := testutil.SetupTestAppWithConsentGate(t)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsentSettings(t, e, admin, html)
	user := testutil.RegisterAndLogin(t, e, "consent_gate_user", testPassword, 1, td.OrgID, td.CompanyID)
	return e, db, admin, user, cleanup
}

func userIDByName(t *testing.T, db *gorm.DB, username string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw("SELECT id FROM users WHERE username = ?", username).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func TestPDConsentGate_BlocksUntilAccepted(t *testing.T) {
	e, _, _, user, cleanup := setupGate(t, "<p>Согласие на обработку данных</p>")
	defer cleanup()

	assertConsentBlocked(t, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)))

	// Доступ обязан открыться сразу после записи согласия, а не по истечении TTL
	// кэша: иначе фронт уже снял окно, а API продолжает отказывать.
	rec := testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusOK, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)).Code)
}

// Без этих ручек окно согласия - тупик: нечего показать, нечем согласиться и
// невозможно выйти. Проверяем именно под активным гейтом.
func TestPDConsentGate_AllowsConsentFlowAndLogout(t *testing.T) {
	e, _, _, user, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	assert.Equal(t, http.StatusOK, testutil.GET(t, e, gatePath, testutil.AuthHeader(user)).Code,
		"состояние гейта обязано читаться, иначе фронту нечего показать")
	assert.Equal(t, http.StatusOK, testutil.GET(t, e, "/permissions/my", testutil.AuthHeader(user)).Code)
	assert.Equal(t, http.StatusOK, testutil.GET(t, e, "/users/me", testutil.AuthHeader(user)).Code)
	assert.Equal(t, http.StatusOK, testutil.POST(t, e, "/events/ticket", `{}`, testutil.AuthHeader(user)).Code,
		"после согласия токен не меняется, поток сам не переподключится - билет нужен сразу")
	assert.Equal(t, http.StatusOK, testutil.POST(t, e, "/logout", `{}`, testutil.AuthHeader(user)).Code,
		"выход обязан работать: окно без выхода - тупик")
}

func TestPDConsentGate_SuperAdminPasses(t *testing.T) {
	e, _, admin, _, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	// Аварийная дверь: с ошибочной настройкой систему всё равно надо чинить через
	// интерфейс, поэтому супер-администратора гейт не закрывает.
	assert.Equal(t, http.StatusOK, testutil.GET(t, e, guardedPath, testutil.AuthHeader(admin)).Code)
}

func TestPDConsentGate_VersionBumpClosesAccessAgain(t *testing.T) {
	e, _, admin, user, cleanup := setupGate(t, "<p>Первая редакция</p>")
	defer cleanup()

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)
	require.Equal(t, http.StatusOK, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)).Code)

	rec := testutil.POST(t, e, consentSettingsPath+"/require-again", `{}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assertConsentBlocked(t, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)))
}

// Редакцию штампует сервер: иначе клиент прислал бы заведомо большое число и
// освободился от всех будущих переподтверждений.
func TestPDConsentGate_ClientCannotForgeVersion(t *testing.T) {
	e, _, admin, user, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	rec := testutil.POST(t, e, acceptPath, `{"document_version":999,"version":999}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, http.StatusOK, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)).Code)

	// Подъём редакции обязан снова закрыть доступ - значит записана серверная 1, а не 999.
	require.Equal(t, http.StatusOK,
		testutil.POST(t, e, consentSettingsPath+"/require-again", `{}`, testutil.AuthHeader(admin)).Code)
	assertConsentBlocked(t, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)))
}

func TestPDConsentGate_OpenWhenDisabledOrTextEmpty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestAppWithConsentGate(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	user := testutil.RegisterAndLogin(t, e, "consent_off_user", testPassword, 1, td.OrgID, td.CompanyID)

	// Тумблер выключен - гейт молчит.
	assert.Equal(t, http.StatusOK, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)).Code)

	// Включён, но текст стёрли: показать нечего, запирать систему нельзя.
	enableConsentSettings(t, e, admin, "<p>Согласие</p>")
	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, consentSettingsPath+"/text", `{"text":""}`, testutil.AuthHeader(admin)).Code)
	assert.Equal(t, http.StatusOK, testutil.GET(t, e, guardedPath, testutil.AuthHeader(user)).Code)
}

// Забаненный не может дать согласие (проверка бана режет POST), поэтому он обязан
// видеть блокировку, а не требование согласия - иначе он заперт без выхода.
func TestPDConsentGate_BanTakesPrecedence(t *testing.T) {
	e, db, admin, user, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	banPath := fmt.Sprintf("/users/%d/ban", userIDByName(t, db, "consent_gate_user"))
	rec := testutil.POST(t, e, banPath, `{"reason":"тест"}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("X-PD-Consent-Required"),
		"забаненному отвечает проверка бана, а не гейт согласия")
	assert.Contains(t, rec.Body.String(), "заблокирована")
}

// Белый список сверяем с реальным роутером: переименованный роут молча выпал бы из
// исключений и запер сам механизм согласия - фронт получил бы 403 на ручку, которой
// снимается блокировка.
func TestPDConsentGate_WhitelistMatchesRegisteredRoutes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	_ = db

	registered := make(map[string]bool)
	for _, r := range e.Routes() {
		registered[r.Method+" "+r.Path] = true
	}
	for key := range mw.PDConsentWhitelist {
		assert.True(t, registered[key], "роут %q из белого списка не зарегистрирован", key)
	}
}

// Гейт закрывает именно ПРОИЗВОЛЬНЫЕ protected-роуты, а не только тот один, на
// котором написаны остальные проверки.
func TestPDConsentGate_ArbitraryProtectedRoutesClosed(t *testing.T) {
	e, _, _, user, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	for _, path := range []string{"/citizenships", "/organizations", "/applications/user", "/notifications", "/system-tables"} {
		rec := testutil.GET(t, e, path, testutil.AuthHeader(user))
		assert.Equal(t, http.StatusForbidden, rec.Code, "путь %s обязан быть закрыт до согласия", path)
		assert.Equal(t, "1", rec.Header().Get("X-PD-Consent-Required"), "путь %s", path)
	}
}

// Незарегистрированный путь под /api тоже проходит через гейт: echo вешает на
// группу catch-all со всей её цепочкой middleware, поэтому вместо 404 приходит
// отказ гейта. Так же ведут себя проверки блокировки и техработ, живущие рядом, и
// это скорее плюс - существование роутов не подсказывается тому, кого не пустили.
func TestPDConsentGate_UnknownPathAnsweredByGate(t *testing.T) {
	e, _, _, user, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	assertConsentBlocked(t, testutil.GET(t, e, "/no-such-route-here", testutil.AuthHeader(user)))
}

// Гость (без токена) до гейта не доходит - его отбивает JWTAuth, и ответ не должен
// притворяться требованием согласия.
func TestPDConsentGate_AnonymousUntouched(t *testing.T) {
	e, _, _, _, cleanup := setupGate(t, "<p>Согласие</p>")
	defer cleanup()

	rec := testutil.GET(t, e, guardedPath, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Header().Get("X-PD-Consent-Required"))
}

// Существующие тесты поднимают приложение без гейта, и он обязан оставаться nil:
// иначе три сотни интеграционных тестов начнут получать 403.
func TestPDConsentGate_DisabledByDefaultInTests(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsentSettings(t, e, admin, "<p>Согласие</p>")
	user := testutil.RegisterAndLogin(t, e, "consent_nogate_user", testPassword, 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, guardedPath, testutil.AuthHeader(user))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("X-PD-Consent-Required"))
}
