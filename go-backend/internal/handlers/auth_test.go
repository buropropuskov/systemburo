package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- POST /register ---

func TestRegister_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	body := `{"username":"newuser","password":"secret123","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/register", body, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "User registered successfully")
}

func TestRegister_DuplicateUsername(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "dupuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"username":"dupuser","password":"pass456","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/register", body, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Username already exists")
}

func TestRegister_MissingFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	// Empty body -- username and password are empty strings, org/company IDs are 0
	// This should fail because organization_id=0 and company_id=0 violate FK constraints
	rec := testutil.POST(t, e, "/register", `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegister_WithOptionalFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	body := `{
		"username":"fulluser","password":"secret123","type_id":1,
		"organization_id":` + itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `,
		"last_name":"Ivanov","first_name":"Ivan","middle_name":"Ivanovich",
		"position":"Developer","email":"ivan@test.com","phone":"+79001234567"
	}`
	rec := testutil.POST(t, e, "/register", body, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- POST /login ---

func TestLogin_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "loginuser", "mypassword", 1, td.OrgID, td.CompanyID)

	body := `{"username":"loginuser","password":"mypassword"}`
	rec := testutil.POST(t, e, "/login", body, nil)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.LoginResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, td.OrgID, resp.OrganizationID)
	assert.Equal(t, td.CompanyID, resp.CompanyID)
	assert.Equal(t, 1, resp.TypeID)
	assert.Equal(t, "Test Organization", resp.Organization)
	assert.Equal(t, "Test Company", resp.Company)
	assert.Equal(t, "Пользователь", resp.UserType)
}

func TestLogin_WrongPassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "loginuser", "correctpass", 1, td.OrgID, td.CompanyID)

	body := `{"username":"loginuser","password":"wrongpass"}`
	rec := testutil.POST(t, e, "/login", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid credentials")
}

func TestLogin_NonexistentUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"username":"ghost","password":"whatever"}`
	rec := testutil.POST(t, e, "/login", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid credentials")
}

// --- POST /refresh-token ---

func TestRefreshToken_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "refreshuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "refreshuser", "pass123")

	body := `{"refreshToken":"` + refreshToken + `"}`
	rec := testutil.POST(t, e, "/refresh-token", body, nil)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.TokenPairResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.RefreshToken)
}

func TestRefreshToken_SnakeCaseField(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "snakeuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "snakeuser", "pass123")

	body := `{"refresh_token":"` + refreshToken + `"}`
	rec := testutil.POST(t, e, "/refresh-token", body, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"refreshToken":"invalid.jwt.token"}`
	rec := testutil.POST(t, e, "/refresh-token", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid refresh token")
}

func TestRefreshToken_RevokedToken(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "revokeuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "revokeuser", "pass123")

	// Use the refresh token once (revokes it)
	body := `{"refreshToken":"` + refreshToken + `"}`
	rec := testutil.POST(t, e, "/refresh-token", body, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// Try using the same refresh token again -- it has been revoked
	rec = testutil.POST(t, e, "/refresh-token", body, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- POST /logout ---

func TestLogout_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "logoutuser", "pass123", 1, td.OrgID, td.CompanyID)
	accessToken, refreshToken := testutil.LoginUser(t, e, "logoutuser", "pass123")

	body := `{"refreshToken":"` + refreshToken + `"}`
	rec := testutil.POST(t, e, "/logout", body, testutil.AuthHeader(accessToken))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Logged out successfully")
}

func TestLogout_NoAuth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"refreshToken":"sometoken"}`
	rec := testutil.POST(t, e, "/logout", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- GET /user-data ---

func TestGetUserData_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "datauser", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/user-data", testutil.AuthHeader(token))

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.UserDataResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "datauser", resp.Username)
	assert.Equal(t, "Test Organization", resp.Organization)
	assert.Equal(t, td.OrgID, resp.OrganizationID)
	assert.Equal(t, "Test Company", resp.Company)
	assert.Equal(t, td.CompanyID, resp.CompanyID)
}

func TestGetUserData_NoAuth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/user-data", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- GET /users/me ---

func TestGetCurrentUser_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "meuser", "pass123", 2, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/users/me", testutil.AuthHeader(token))

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.CurrentUserResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "meuser", resp.Username)
	assert.Equal(t, 2, resp.TypeID)
	assert.Equal(t, "Арендатор", resp.UserType)
	assert.Equal(t, "renter", resp.UserTypeCode)
	assert.Equal(t, "Test Organization", resp.Organization)
	assert.Equal(t, td.OrgID, resp.OrganizationID)
	assert.Equal(t, "Test Company", resp.Company)
	assert.Equal(t, td.CompanyID, resp.CompanyID)
	assert.Greater(t, resp.ID, 0)
}

func TestGetCurrentUser_NoAuth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/users/me", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- GET /user-types ---

func TestGetUserTypes_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/user-types", nil)

	require.Equal(t, http.StatusOK, rec.Code)

	var types []models.UserType
	err := json.Unmarshal(rec.Body.Bytes(), &types)
	require.NoError(t, err)

	assert.Len(t, types, 6)

	// Verify order and content of seeded types
	expected := []struct {
		Name string
		Code string
	}{
		{"Пользователь", "user"},
		{"Арендатор", "renter"},
		{"Подрядчик", "contractor"},
		{"Охранник", "security"},
		{"Руководитель", "manager"},
		{"Бюро пропусков", "buropropuskov"},
	}
	for i, exp := range expected {
		assert.Equal(t, exp.Name, types[i].Name, "type[%d].Name", i)
		assert.Equal(t, exp.Code, types[i].Code, "type[%d].Code", i)
		assert.Greater(t, types[i].ID, 0, "type[%d].ID", i)
	}
}

func TestGetUserTypes_PublicEndpoint(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Accessible without any auth header
	rec := testutil.GET(t, e, "/user-types", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
