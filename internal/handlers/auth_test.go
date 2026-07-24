package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

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

	body := `{"username":"dupuser","password":"password123","type_id":1,"organization_id":` +
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
	assert.Contains(t, rec.Body.String(), "Неверный логин или пароль")
	// Существующий логин + неверный пароль -> остаток попыток до блокировки.
	// Первая неудача: 10 - 1 = 9.
	assert.Equal(t, "9", rec.Header().Get("X-Auth-Attempts-Remaining"))
}

func TestLogin_NonexistentUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"username":"ghost","password":"whatever"}`
	rec := testutil.POST(t, e, "/login", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	// Тот же текст, что и при неверном пароле (единый ответ), но БЕЗ заголовка
	// счётчика - иначе по нему можно было бы отличить существующий логин.
	assert.Contains(t, rec.Body.String(), "Неверный логин или пароль")
	assert.Empty(t, rec.Header().Get("X-Auth-Attempts-Remaining"))
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

// TestRefreshToken_GraceWindowDoesNotKillFamily проверяет регресс на issue #272.
// Если refresh пришёл с только что отозванным токеном (грейс-окно), family
// должна остаться активной - это race-condition двух табов, не reuse-атака.
func TestRefreshToken_GraceWindowDoesNotKillFamily(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "graceuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "graceuser", "pass123")

	h := http.Header{}
	h.Set("Cookie", "refresh_token="+refreshToken)
	rec := testutil.POST(t, e, "/refresh-token", "{}", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Сразу повторно с тем же токеном - попадаем в grace-window: 401 retry,
	// но family НЕ убита.
	rec = testutil.POST(t, e, "/refresh-token", "{}", h)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Token recently rotated")

	// Свежий refresh-токен из первого вызова должен ещё работать -
	// family жива. Берём cookie из последнего успешного ответа.
	// Для упрощения теста: проверяем что в БД есть активный токен в этой family.
	var activeCount int64
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?) AND is_revoked = false", "graceuser").
		Count(&activeCount)
	assert.Equal(t, int64(1), activeCount, "family должна остаться с одним активным токеном")
}

// TestRefreshToken_AfterGraceWindowKillsFamily проверяет что после истечения
// grace-window повторное использование revoked-токена убивает всю family.
func TestRefreshToken_AfterGraceWindowKillsFamily(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "killuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "killuser", "pass123")

	h := http.Header{}
	h.Set("Cookie", "refresh_token="+refreshToken)
	rec := testutil.POST(t, e, "/refresh-token", "{}", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Симулируем что прошло > grace-window: правим revoked_at в прошлое.
	pastTime := time.Now().Add(-1 * time.Hour)
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?) AND is_revoked = true", "killuser").
		Update("revoked_at", pastTime)

	// Повторный refresh старым токеном - reuse detection, family kill.
	rec = testutil.POST(t, e, "/refresh-token", "{}", h)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "reuse detected")

	// Все токены family должны быть revoked.
	var activeCount int64
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?) AND is_revoked = false", "killuser").
		Count(&activeCount)
	assert.Equal(t, int64(0), activeCount, "family должна быть полностью убита")
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

// --- Account lockout (P0.2) ---

func TestLogin_FailedLoginIncrementsCounter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "lockuser", "correctpass", 1, td.OrgID, td.CompanyID)

	// Неверный пароль -> counter увеличивается.
	body := `{"username":"lockuser","password":"wrongpass"}`
	rec := testutil.POST(t, e, "/login", body, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var user models.User
	require.NoError(t, db.Where("username = ?", "lockuser").First(&user).Error)
	assert.Equal(t, 1, user.FailedLoginCount)
	assert.Nil(t, user.LockedUntil, "до 10 попыток lock не ставится")
}

func TestLogin_SuccessResetsFailedCounter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "resetuser", "correctpass", 1, td.OrgID, td.CompanyID)

	// Две неудачные попытки.
	for i := 0; i < 2; i++ {
		rec := testutil.POST(t, e, "/login", `{"username":"resetuser","password":"wrong"}`, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	// Успешный вход - счётчик должен обнулиться.
	rec := testutil.POST(t, e, "/login", `{"username":"resetuser","password":"correctpass"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var user models.User
	require.NoError(t, db.Where("username = ?", "resetuser").First(&user).Error)
	assert.Equal(t, 0, user.FailedLoginCount, "успешный вход обнуляет счётчик")
	assert.Nil(t, user.LockedUntil)
}

func TestLogin_LocksAccountAfter10FailedAttempts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "lockme", "correctpass", 1, td.OrgID, td.CompanyID)

	// Напрямую через БД выставляем счётчик в 9 - следующая неудача залочит.
	// Это заодно обходит IP rate limiter (5 попыток/15м), не связанный с этим кейсом.
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "lockme").
		Update("failed_login_count", 9).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"lockme","password":"wrong"}`, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var user models.User
	require.NoError(t, db.Where("username = ?", "lockme").First(&user).Error)
	assert.Equal(t, 10, user.FailedLoginCount)
	require.NotNil(t, user.LockedUntil, "после 10 неудач учётка залочена")
	assert.True(t, user.LockedUntil.After(time.Now().Add(29*time.Minute)),
		"lock примерно на 30 минут")
}

func TestLogin_LockedAccountRejectsEvenCorrectPassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "locked", "correctpass", 1, td.OrgID, td.CompanyID)

	// Прямо в БД выставляем lock на 5 минут вперёд.
	lockUntil := time.Now().Add(5 * time.Minute)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "locked").
		Update("locked_until", lockUntil).Error)

	// Даже правильный пароль возвращает 429 пока lock не истечёт.
	rec := testutil.POST(t, e, "/login", `{"username":"locked","password":"correctpass"}`, nil)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "заблокирована")
	// Retry-After даёт фронту остаток для таймера обратного отсчёта.
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "блокировка учётки должна отдавать Retry-After")
}

func TestLogin_ExpiredLockAllowsLogin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "unlocked", "correctpass", 1, td.OrgID, td.CompanyID)

	// Lock в прошлом - не должен препятствовать входу.
	pastLock := time.Now().Add(-1 * time.Minute)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "unlocked").
		Updates(map[string]interface{}{"locked_until": pastLock, "failed_login_count": 10}).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"unlocked","password":"correctpass"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// После успешного входа lock и счётчик сброшены.
	var user models.User
	require.NoError(t, db.Where("username = ?", "unlocked").First(&user).Error)
	assert.Equal(t, 0, user.FailedLoginCount)
	assert.Nil(t, user.LockedUntil)
}

// --- Refresh token family invalidation (P0.1) ---

func TestRefresh_ReuseDetection_InvalidatesFamily(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "replayuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refresh1 := testutil.LoginUser(t, e, "replayuser", "pass123")

	// Легитимная ротация: refresh1 -> refresh2.
	h1 := http.Header{}
	h1.Set("Cookie", "refresh_token="+refresh1)
	rec := testutil.POST(t, e, "/refresh-token", "{}", h1)
	require.Equal(t, http.StatusOK, rec.Code)

	// Достаём refresh2 из cookie в ответе.
	var refresh2 string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			refresh2 = c.Value
			break
		}
	}
	require.NotEmpty(t, refresh2)

	// Симулируем что между ротацией и reuse прошло > grace-window (10s).
	// Иначе попадём в grace-режим и family не убьём (см. #272).
	pastTime := time.Now().Add(-1 * time.Hour)
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?) AND is_revoked = true", "replayuser").
		Update("revoked_at", pastTime)

	// Attacker пробует заюзать revoked refresh1 - должно триггернуть reuse detection.
	rec = testutil.POST(t, e, "/refresh-token", "{}", h1)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "reuse detected")

	// refresh2 (активный до reuse) теперь тоже revoked - вся family мертва.
	h2 := http.Header{}
	h2.Set("Cookie", "refresh_token="+refresh2)
	rec = testutil.POST(t, e, "/refresh-token", "{}", h2)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"family invalidation: текущий активный токен тоже отозван")

	// В БД все токены family должны быть is_revoked=true.
	var activeCount int64
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?) AND is_revoked = false", "replayuser").
		Count(&activeCount)
	assert.Equal(t, int64(0), activeCount, "все токены family должны быть отозваны")
}

func TestRefresh_NormalRotation_PreservesFamily(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "rotateuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refresh1 := testutil.LoginUser(t, e, "rotateuser", "pass123")

	// Делаем 3 последовательных легитимных ротации.
	current := refresh1
	var familyIDs []string
	for i := 0; i < 3; i++ {
		h := http.Header{}
		h.Set("Cookie", "refresh_token="+current)
		rec := testutil.POST(t, e, "/refresh-token", "{}", h)
		require.Equal(t, http.StatusOK, rec.Code, "итерация %d", i)

		for _, c := range rec.Result().Cookies() {
			if c.Name == "refresh_token" {
				current = c.Value
				break
			}
		}
	}

	// Все 4 записи (login + 3 refresh) должны иметь один family_id.
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?)", "rotateuser").
		Distinct().Pluck("family_id", &familyIDs)
	assert.Len(t, familyIDs, 1, "все токены одной сессии должны быть в одной family")
	assert.NotEmpty(t, familyIDs[0])
}

func TestLogin_GeneratesNewFamily(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "multi", "pass123", 1, td.OrgID, td.CompanyID)

	// Два независимых login - каждый со своей family.
	testutil.LoginUser(t, e, "multi", "pass123")
	testutil.LoginUser(t, e, "multi", "pass123")

	var familyIDs []string
	db.Model(&models.RefreshToken{}).
		Where("user_id = (SELECT id FROM users WHERE username = ?)", "multi").
		Distinct().Pluck("family_id", &familyIDs)
	assert.Len(t, familyIDs, 2, "разные login-ы -> разные family")
}
