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
func RegisterUser(t *testing.T, e *echo.Echo, username, password string, typeID, orgID, companyID int) {
	t.Helper()
	user := models.User{
		Username:       username,
		Password:       hashTestPassword(password),
		OrganizationID: intPtrOrZero(orgID),
		CompanyID:      intPtrOrZero(companyID),
		TypeID:         typeID,
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

	user := models.User{
		Username:       username,
		Password:       hashTestPassword(adminPassword),
		OrganizationID: intPtrOrZero(orgID),
		CompanyID:      intPtrOrZero(companyID),
		TypeID:         typeID,
	}
	err := cachedDB.Create(&user).Error
	require.NoError(t, err, "failed to seed user %s", username)

	token, _ := LoginUser(t, e, username, adminPassword)
	return token
}
