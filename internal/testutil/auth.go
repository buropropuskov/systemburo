package testutil

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

const adminPassword = "adminpass_long_enough_for_32chars!"

// intPtrOrZero возвращает указатель на v или nil если v <= 0.
// Дублирует services.intPtrOrNil чтобы избежать циклических импортов.
func intPtrOrZero(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// AuthHeader returns an Authorization header with the given Bearer token.
func AuthHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

// RegisterUser creates a user directly in DB.
// type_id=6 (buropropuskov) дополнительно получает is_super_admin=true,
// чтобы тесты-сценарии для супер-админа корректно работали с middleware
// которое после #231 проверяет is_super_admin вместо type_id=6.
func RegisterUser(t *testing.T, e *echo.Echo, username, password string, typeID, orgID, companyID int) {
	t.Helper()
	user := models.User{
		Username:       username,
		Password:       hashTestPassword(password),
		OrganizationID: intPtrOrZero(orgID),
		CompanyID:      intPtrOrZero(companyID),
		TypeID:         typeID,
		IsSuperAdmin:   typeID == 6,
	}
	err := cachedDB.Create(&user).Error
	require.NoError(t, err, "failed to seed user %s", username)
}

// LoginUser логинится и возвращает (accessToken, refreshToken).
// Access приходит в JSON body, refresh - в HttpOnly cookie (см. PR #113).
// Читаем cookie из response Set-Cookie.
func LoginUser(t *testing.T, e *echo.Echo, username, password string) (string, string) {
	t.Helper()
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	rec := POST(t, e, "/login", body, nil)
	require.Equal(t, http.StatusOK, rec.Code, "login failed: %s", rec.Body.String())

	resp := ParseMap(t, rec)
	access, _ := resp["token"].(string)

	refresh := ""
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			refresh = c.Value
			break
		}
	}
	return access, refresh
}

// RegisterAndLogin registers a user via API and returns the access token.
func RegisterAndLogin(t *testing.T, e *echo.Echo, username, password string, typeID, orgID, companyID int) string {
	t.Helper()
	RegisterUser(t, e, username, password, typeID, orgID, companyID)
	token, _ := LoginUser(t, e, username, password)
	return token
}

// RegisterAdmin creates an admin user (buropropuskov, type_id=6) directly in DB
// and returns the access token. Uses DB seed because the Register API
// hardcodes type_id=1 for security (prevents admin escalation via public API).
func RegisterAdmin(t *testing.T, e *echo.Echo, orgID, companyID int) string {
	t.Helper()
	return registerUserViaDB(t, e, "testadmin", 6, orgID, companyID)
}

// RegisterManager creates a manager user (type_id=5) directly in DB.
func RegisterManager(t *testing.T, e *echo.Echo, username string, orgID, companyID int) string {
	t.Helper()
	return registerUserViaDB(t, e, username, 5, orgID, companyID)
}

func registerUserViaDB(t *testing.T, e *echo.Echo, username string, typeID, orgID, companyID int) string {
	t.Helper()

	// Админство определяется флагами, а не type_id (Ф5): buropropuskov (6) ->
	// is_super_admin, manager (5) -> is_admin. Это зеркалит бэкфилл миграции
	// (migrate.go переносит manager-тип на is_admin), чтобы такие пользователи
	// не теряли доступ после снятия type-проверок.
	user := models.User{
		Username:       username,
		Password:       hashTestPassword(adminPassword),
		OrganizationID: intPtrOrZero(orgID),
		CompanyID:      intPtrOrZero(companyID),
		TypeID:         typeID,
		IsSuperAdmin:   typeID == 6,
		IsAdmin:        typeID == 5,
	}
	err := cachedDB.Create(&user).Error
	require.NoError(t, err, "failed to seed user %s", username)

	token, _ := LoginUser(t, e, username, adminPassword)
	return token
}

// GrantTableVerb выдаёт юзеру персональное право table.<name>.<verb> (override allow).
// Нужен тестам, где табличные операции (снимки версий, корзина) раньше были открыты
// всем, а теперь гейтятся per-table правом RequireTableVerb. Вызывать ДО первого
// защищённого запроса юзера - resolver закэширует права при первом резолве.
func GrantTableVerb(t *testing.T, userID int, tableName, verb string) {
	t.Helper()
	GrantPermission(t, userID, fmt.Sprintf("table.%s.%s", tableName, verb))
}

// GrantPermission выдаёт юзеру персональный override (allow) на произвольный ключ
// каталога прав - для тестов, где нужно право вне таблиц (например page.admin.feedback).
// Вызывать ДО первого защищённого запроса юзера - resolver закэширует права при
// первом резолве.
func GrantPermission(t *testing.T, userID int, key string) {
	t.Helper()
	err := cachedDB.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: key,
		Value:         "allow",
	}).Error
	require.NoError(t, err, "failed to grant %s to user %d", key, userID)
	// Сбрасываем кэш прав юзера - grant мог быть сделан после первого резолва.
	if cachedResolver != nil {
		cachedResolver.Invalidate(userID)
	}
}

// DenyPermission ставит юзеру персональный override (deny) на ключ каталога прав.
// Для администратора (is_admin) это единственный способ закрыть раздел: adminAll
// пропускает всё, кроме super-only и личных deny.
func DenyPermission(t *testing.T, userID int, key string) {
	t.Helper()
	err := cachedDB.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: key,
		Value:         "deny",
	}).Error
	require.NoError(t, err, "failed to deny %s for user %d", key, userID)
	if cachedResolver != nil {
		cachedResolver.Invalidate(userID)
	}
}
