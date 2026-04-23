package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- POST /users (admin-only) ---

func TestCreateUser_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	body := `{"username":"newuser","password":"secret123","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "User created successfully", msg)
}

func TestCreateUser_WithCustomType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// type_id=5 — manager
	body := `{"username":"newmanager","password":"secret123","type_id":5,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCreateUser_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "regular", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(userToken)

	body := `{"username":"newuser","password":"secret123","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "dupuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"username":"dupuser","password":"pass456","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Username already exists")
}

func TestCreateUser_MissingFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	rec := testutil.POST(t, e, "/users", `{}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateUser_ShortUsername(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	body := `{"username":"ab","password":"secret123","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateUser_ShortPassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	body := `{"username":"validuser","password":"12345","type_id":1,"organization_id":` +
		itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateUser_WithOptionalFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	body := `{
		"username":"fulluser","password":"secret123","type_id":1,
		"organization_id":` + itoa(td.OrgID) + `,"company_id":` + itoa(td.CompanyID) + `,
		"last_name":"Ivanov","first_name":"Ivan","middle_name":"Ivanovich",
		"position":"Developer","email":"ivan@test.com","phone":"+79001234567"
	}`
	rec := testutil.POST(t, e, "/users", body, h)

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

	resp := testutil.ParseResponse[models.LoginResponse](t, rec)

	assert.NotEmpty(t, resp.Token)
	// refresh_token теперь в HttpOnly cookie, не в JSON body.
	assert.Empty(t, resp.RefreshToken, "refresh token должен быть пустым в JSON - он в cookie")

	// Проверяем что cookie выставлена с правильными флагами.
	var refreshCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie, "refresh_token cookie должна быть установлена")
	assert.NotEmpty(t, refreshCookie.Value)
	assert.True(t, refreshCookie.HttpOnly, "cookie должна быть HttpOnly")
	assert.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite)
	assert.Equal(t, "/", refreshCookie.Path)

	assert.Equal(t, td.OrgID, *resp.OrganizationID)
	assert.Equal(t, td.CompanyID, *resp.CompanyID)
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

	// Refresh token теперь в cookie, body оставлен для обратной совместимости.
	h := http.Header{}
	h.Set("Cookie", "refresh_token="+refreshToken)
	rec := testutil.POST(t, e, "/refresh-token", "{}", h)

	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[models.TokenPairResponse](t, rec)

	assert.NotEmpty(t, resp.Token)
	// Новый refresh уходит снова в cookie, не в body.
	assert.Empty(t, resp.RefreshToken)
	var newRefreshCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			newRefreshCookie = c
			break
		}
	}
	require.NotNil(t, newRefreshCookie, "должна быть новая refresh cookie (ротация)")
	assert.NotEmpty(t, newRefreshCookie.Value)
}

func TestRefreshToken_BodyFallback(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "snakeuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "snakeuser", "pass123")

	// Fallback: если cookie нет, читаем из body snake_case.
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
	h := http.Header{}
	h.Set("Cookie", "refresh_token="+refreshToken)
	rec := testutil.POST(t, e, "/refresh-token", "{}", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Try using the same refresh token again -- it has been revoked
	rec = testutil.POST(t, e, "/refresh-token", "{}", h)
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
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Logged out successfully", msg)
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

	resp := testutil.ParseResponse[models.UserDataResponse](t, rec)

	assert.Equal(t, "datauser", resp.Username)
	assert.Equal(t, "Test Organization", resp.Organization)
	assert.Equal(t, td.OrgID, *resp.OrganizationID)
	assert.Equal(t, "Test Company", resp.Company)
	assert.Equal(t, td.CompanyID, *resp.CompanyID)
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

	resp := testutil.ParseResponse[models.CurrentUserResponse](t, rec)

	assert.Equal(t, "meuser", resp.Username)
	assert.Equal(t, 2, resp.TypeID)
	assert.Equal(t, "Арендатор", resp.UserType)
	assert.Equal(t, "renter", resp.UserTypeCode)
	assert.Equal(t, "Test Organization", resp.Organization)
	assert.Equal(t, td.OrgID, *resp.OrganizationID)
	assert.Equal(t, "Test Company", resp.Company)
	assert.Equal(t, td.CompanyID, *resp.CompanyID)
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

	types := testutil.ParseResponse[[]models.UserType](t, rec)

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
