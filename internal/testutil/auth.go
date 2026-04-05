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

// AuthHeader returns an Authorization header with the given Bearer token.
func AuthHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

// RegisterUser registers a user via API and asserts success.
// Note: Register endpoint always sets type_id=1 (security by design).
func RegisterUser(t *testing.T, e *echo.Echo, username, password string, typeID, orgID, companyID int) {
	t.Helper()
	body := fmt.Sprintf(`{"username":"%s","password":"%s","type_id":%d,"organization_id":%d,"company_id":%d}`,
		username, password, typeID, orgID, companyID)
	rec := POST(t, e, "/register", body, nil)
	require.Equal(t, http.StatusOK, rec.Code, "register failed: %s", rec.Body.String())
}

// LoginUser logs in and returns (accessToken, refreshToken).
func LoginUser(t *testing.T, e *echo.Echo, username, password string) (string, string) {
	t.Helper()
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	rec := POST(t, e, "/login", body, nil)
	require.Equal(t, http.StatusOK, rec.Code, "login failed: %s", rec.Body.String())

	resp := ParseMap(t, rec)
	return resp["token"].(string), resp["refreshToken"].(string)
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

	user := models.User{
		Username:       "testadmin",
		Password:       hashTestPassword(adminPassword),
		OrganizationID: orgID,
		CompanyID:      companyID,
		TypeID:         6,
	}
	err := cachedDB.Create(&user).Error
	require.NoError(t, err, "failed to seed admin user")

	token, _ := LoginUser(t, e, "testadmin", adminPassword)
	return token
}
