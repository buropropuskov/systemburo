package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// AuthHeader returns an Authorization header with the given Bearer token.
func AuthHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

// RegisterUser registers a user and asserts success.
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

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	return resp["token"].(string), resp["refreshToken"].(string)
}

// RegisterAndLogin registers a user and returns the access token.
func RegisterAndLogin(t *testing.T, e *echo.Echo, username, password string, typeID, orgID, companyID int) string {
	t.Helper()
	RegisterUser(t, e, username, password, typeID, orgID, companyID)
	token, _ := LoginUser(t, e, username, password)
	return token
}

// RegisterAdmin registers an admin user (buropropuskov, type_id=6) and returns the access token.
func RegisterAdmin(t *testing.T, e *echo.Echo, orgID, companyID int) string {
	t.Helper()
	return RegisterAndLogin(t, e, "testadmin", "adminpass", 6, orgID, companyID)
}
