package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Гейт обязательной смены пароля (#1911): пока у пользователя поднят
// users.must_change_password, protected-API отвечает 403 с кодом
// PASSWORD_CHANGE_REQUIRED везде, кроме белого списка. Навешивается только через
// SetupTestAppWithPasswordGate - в остальных тестах он nil.
//
// Тесты лежат в handlers, а не рядом с middleware: любой DB-backed Go-тест обязан
// быть в этом пакете, иначе второй бинарь с базой дерётся с этим за общую тест-БД.

const (
	mcpOldPassword = "letterpassword12345"
	mcpNewPassword = "ownpassword67890"
	// Protected-ручка вне белого списка, читающая и безобидная: отказ на ней
	// означает именно этот гейт, а не нехватку прав.
	mcpGuardedPath = "/citizenships"
)

func setPasswordFlag(t *testing.T, db *gorm.DB, username string, required bool) {
	t.Helper()
	require.NoError(t, db.Exec("UPDATE users SET must_change_password = ? WHERE username = ?", required, username).Error)
}

func assertPasswordChangeBlocked(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Equal(t, "1", rec.Header().Get("X-Password-Change-Required"),
		"фронт отличает требование сменить пароль от нехватки прав по маркеру ответа")
	assert.Contains(t, rec.Body.String(), `"code":"PASSWORD_CHANGE_REQUIRED"`)
}

// setupPasswordGate поднимает приложение с гейтом и возвращает вошедшего работника
// с поднятым флагом.
func setupPasswordGate(t *testing.T, username string) (*echo.Echo, *gorm.DB, string, func()) {
	t.Helper()
	e, db, cleanup := testutil.SetupTestAppWithPasswordGate(t)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, username, mcpOldPassword, 1, td.OrgID, td.CompanyID)
	setPasswordFlag(t, db, username, true)
	return e, db, token, cleanup
}

// Белый список сверяется с реальным роутером: переименованный или снятый роут молча
// выпал бы из исключений и запер сам механизм смены пароля - человек с паролем из
// письма остался бы без единственного выхода.
func TestMustChangePassword_WhitelistMatchesRouter(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	registered := make(map[string]bool)
	for _, r := range e.Routes() {
		registered[r.Method+" "+r.Path] = true
	}
	for key := range mw.MustChangePasswordWhitelist {
		assert.True(t, registered[key], "роут %q из белого списка не зарегистрирован в роутере", key)
	}
}

func TestMustChangePassword_BlocksUntilChanged(t *testing.T) {
	e, _, token, cleanup := setupPasswordGate(t, "mcp_blocked_user")
	defer cleanup()

	assertPasswordChangeBlocked(t, testutil.GET(t, e, mcpGuardedPath, testutil.AuthHeader(token)))
	assertPasswordChangeBlocked(t, testutil.GET(t, e, "/applications", testutil.AuthHeader(token)))
	// Мутации закрыты так же, как чтение: read-only режим тут не нужен, работать в
	// системе до смены пароля нельзя вовсе.
	assertPasswordChangeBlocked(t, testutil.POST(t, e, "/citizenships", `{"name":"Тест"}`, testutil.AuthHeader(token)))
}

// Белый список обязан держать окно смены пароля работоспособным: форма, её чеклист
// требований, профиль, права, тема и выход.
func TestMustChangePassword_WhitelistStaysOpen(t *testing.T) {
	e, _, token, cleanup := setupPasswordGate(t, "mcp_whitelist_user")
	defer cleanup()

	for _, path := range []string{"/users/me", "/permissions/my", "/settings/password-policy", "/users/me/theme"} {
		rec := testutil.GET(t, e, path, testutil.AuthHeader(token))
		assert.Equal(t, http.StatusOK, rec.Code, "%s обязан отвечать с поднятым флагом: %s", path, rec.Body.String())
	}
}

// Главный сценарий: человек с паролем из письма задаёт свой и получает систему
// целиком, без ожидания протухания маркера доступа.
func TestMustChangePassword_OpensAfterChange(t *testing.T) {
	e, _, token, cleanup := setupPasswordGate(t, "mcp_change_user")
	defer cleanup()

	assertPasswordChangeBlocked(t, testutil.GET(t, e, mcpGuardedPath, testutil.AuthHeader(token)))

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+mcpOldPassword+`","new_password":"`+mcpNewPassword+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Тот же маркер доступа: смена пароля отзывает сессии продления, но выданный
	// маркер живёт до своего срока - и теперь обязан открывать систему.
	after := testutil.GET(t, e, mcpGuardedPath, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, after.Code, after.Body.String())
	assert.Empty(t, after.Header().Get("X-Password-Change-Required"))
}

// Пароль из письма не годится в качестве нового: иначе смена превращается в
// формальность и письмо остаётся ключом к учётной записи.
func TestMustChangePassword_RejectsSamePassword(t *testing.T) {
	e, db, token, cleanup := setupPasswordGate(t, "mcp_same_user")
	defer cleanup()

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+mcpOldPassword+`","new_password":"`+mcpOldPassword+`"}`,
		testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var stillRequired bool
	require.NoError(t, db.Raw("SELECT must_change_password FROM users WHERE username = ?", "mcp_same_user").Scan(&stillRequired).Error)
	assert.True(t, stillRequired, "отклонённая смена не должна снимать требование")
	assertPasswordChangeBlocked(t, testutil.GET(t, e, mcpGuardedPath, testutil.AuthHeader(token)))
}

// Со снятым флагом гейт не меняет ничего: это большая часть пользователей, и любой
// лишний отказ здесь виден всей системе.
func TestMustChangePassword_NoFlagNoChange(t *testing.T) {
	e, db, cleanup := testutil.SetupTestAppWithPasswordGate(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "mcp_free_user", mcpOldPassword, 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, mcpGuardedPath, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Header().Get("X-Password-Change-Required"))
	assert.False(t, strings.Contains(rec.Body.String(), mw.PasswordChangeRequiredCode))
}

// Супер-администратор проходит гейт согласия как аварийную дверь, но здесь
// исключения нет: пароль из письма опасен независимо от прав, а выход открыт всегда.
func TestMustChangePassword_AppliesToSuperAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestAppWithPasswordGate(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	var isSuper bool
	require.NoError(t, db.Raw("SELECT is_super_admin FROM users WHERE username = ?", "testadmin").Scan(&isSuper).Error)
	require.True(t, isSuper, "хелпер обязан заводить именно супер-администратора")
	setPasswordFlag(t, db, "testadmin", true)

	assertPasswordChangeBlocked(t, testutil.GET(t, e, mcpGuardedPath, testutil.AuthHeader(admin)))
}
