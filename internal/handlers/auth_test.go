package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	assert.Equal(t, "/api", refreshCookie.Path, "cookie ходит только к API, не за картинками и не на pgAdmin")

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
	// Первая неудача: 5 - 1 = 4.
	assert.Equal(t, "4", rec.Header().Get("X-Auth-Attempts-Remaining"))
}

func TestLogin_NonexistentUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"username":"ghost","password":"whatever"}`
	rec := testutil.POST(t, e, "/login", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Неверный логин или пароль")
	// Несуществующий логин ТОЖЕ получает счётчик (per-IP guard) - тот же ответ, что
	// и для существующего, иначе по наличию счётчика можно перебирать имена.
	assert.Equal(t, "4", rec.Header().Get("X-Auth-Attempts-Remaining"))
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

// Маркер продления принимается только из cookie. Раньше при её отсутствии он
// читался из тела запроса - остаток тех времён, когда маркер отдавался клиенту в
// JSON. Тест сторожит, что путь закрыт: тело с ДЕЙСТВИТЕЛЬНЫМ маркером сеанс не
// продлевает.
func TestRefreshToken_BodyIgnored(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "snakeuser", "pass123", 1, td.OrgID, td.CompanyID)
	_, refreshToken := testutil.LoginUser(t, e, "snakeuser", "pass123")

	body := `{"refresh_token":"` + refreshToken + `"}`
	rec := testutil.POST(t, e, "/refresh-token", body, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "тело запроса не должно продлевать сеанс")

	// Тот же маркер в cookie работает - значит отказ выше про путь, а не про
	// негодный маркер.
	req := httptest.NewRequest(http.MethodPost, "/api/refresh-token", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
	viaCookie := httptest.NewRecorder()
	e.ServeHTTP(viaCookie, req)
	assert.Equal(t, http.StatusOK, viaCookie.Code, "cookie с тем же маркером должна продлевать сеанс")
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
	assert.Nil(t, user.LockedUntil, "до 5 попыток lock не ставится")
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

func TestLogin_BlocksAfter5FailedAttempts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "lockme", "correctpass", 1, td.OrgID, td.CompanyID)

	// Попытки 1..4: счётчик убывает 4..1, статус 401.
	for i := 1; i <= 4; i++ {
		rec := testutil.POST(t, e, "/login", `{"username":"lockme","password":"wrong"}`, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "попытка %d", i)
		assert.Equal(t, strconv.Itoa(5-i), rec.Header().Get("X-Auth-Attempts-Remaining"),
			"остаток на попытке %d", i)
	}

	// 5-я попытка исчерпывает лимит - сразу таймер блокировки (429), а не "осталось 0".
	rec := testutil.POST(t, e, "/login", `{"username":"lockme","password":"wrong"}`, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "на 5-й неудаче сразу таймер")
	assert.Empty(t, rec.Header().Get("X-Auth-Attempts-Remaining"), "при блокировке счётчик не отдаём")

	// Первая ступень лестницы - минута, ступень поднята для следующего круга.
	var user models.User
	require.NoError(t, db.Where("username = ?", "lockme").First(&user).Error)
	require.NotNil(t, user.LockedUntil)
	assert.InDelta(t, 60, time.Until(*user.LockedUntil).Seconds(), 5, "первая блокировка - минута")
	assert.Equal(t, 1, user.LockoutLevel)
	assert.Equal(t, 0, user.FailedLoginCount, "после блокировки счётчик обнулён")
}

// TestLogin_LockoutLadderEscalates - каждый следующий круг из 5 неудач держит
// учётку дольше предыдущего: 1 -> 5 -> 15 -> 30 -> 60 минут, дальше не растёт.
func TestLogin_LockoutLadderEscalates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "ladder", "correctpass", 1, td.OrgID, td.CompanyID)

	// Ожидаемые ступени в минутах; последняя повторяется - лестница упирается в час.
	wantMinutes := []float64{1, 5, 15, 30, 60, 60}
	for step, want := range wantMinutes {
		// Отматываем уже поставленную блокировку в прошлое: ждать её вживую нельзя,
		// а истёкший лок - ровно то состояние, в котором человек приходит за новым кругом.
		require.NoError(t, db.Model(&models.User{}).Where("username = ?", "ladder").
			Update("locked_until", time.Now().Add(-time.Second)).Error)

		// Каждый круг - со своего адреса: per-IP гвард живёт в памяти и отдал бы
		// свою плоскую минуту, не доходя до учётки (в жизни его минута к этому
		// моменту уже истекла, здесь ждать её нечем).
		round := http.Header{}
		round.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", step+1))

		for i := 1; i <= 5; i++ {
			rec := testutil.POST(t, e, "/login", `{"username":"ladder","password":"wrong"}`, round)
			// Пятая неудача круга даёт таймер, остальные - обычный отказ.
			if i == 5 {
				require.Equal(t, http.StatusTooManyRequests, rec.Code, "круг %d, попытка %d", step+1, i)
			} else {
				require.Equal(t, http.StatusUnauthorized, rec.Code, "круг %d, попытка %d", step+1, i)
			}
		}

		var user models.User
		require.NoError(t, db.Where("username = ?", "ladder").First(&user).Error)
		require.NotNil(t, user.LockedUntil, "круг %d", step+1)
		assert.InDelta(t, want, time.Until(*user.LockedUntil).Minutes(), 0.2,
			"круг %d: блокировка на %v минут", step+1, want)
	}
}

// TestLogin_SuccessResetsLockoutLadder - успешный вход опускает лестницу на первую
// ступень: следующая серия опечаток снова стоит минуту, а не полчаса.
func TestLogin_SuccessResetsLockoutLadder(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "climber", "correctpass", 1, td.OrgID, td.CompanyID)

	// Поднимаем лестницу на третью ступень и отпускаем блокировку.
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "climber").
		Updates(map[string]interface{}{
			"lockout_level":        3,
			"locked_until":         time.Now().Add(-time.Second),
			"last_failed_login_at": time.Now().Add(-time.Minute),
		}).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"climber","password":"correctpass"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var user models.User
	require.NoError(t, db.Where("username = ?", "climber").First(&user).Error)
	assert.Equal(t, 0, user.LockoutLevel, "успешный вход обнуляет ступень")
	assert.Nil(t, user.LastFailedLoginAt)
}

// TestLogin_StaleFailuresDoNotAccumulate - неудачи старше окна не копятся:
// четыре опечатки утром и одна вечером не должны запирать учётку.
func TestLogin_StaleFailuresDoNotAccumulate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "forgetful", "correctpass", 1, td.OrgID, td.CompanyID)

	// Состояние "четыре неудачи, но давно".
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "forgetful").
		Updates(map[string]interface{}{
			"failed_login_count":   4,
			"last_failed_login_at": time.Now().Add(-time.Hour),
		}).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"forgetful","password":"wrong"}`, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "давние неудачи не должны запирать")

	var user models.User
	require.NoError(t, db.Where("username = ?", "forgetful").First(&user).Error)
	assert.Equal(t, 1, user.FailedLoginCount, "счётчик начат заново")
	assert.Nil(t, user.LockedUntil)
}

// TestLogin_LadderDecaysAfterQuietDay - сутки без неудач возвращают лестницу
// на первую ступень: старая блокировка не встречает человека сразу часом.
func TestLogin_LadderDecaysAfterQuietDay(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "longago", "correctpass", 1, td.OrgID, td.CompanyID)

	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "longago").
		Updates(map[string]interface{}{
			"lockout_level":        4,
			"failed_login_count":   0,
			"last_failed_login_at": time.Now().Add(-48 * time.Hour),
		}).Error)

	for i := 1; i <= 5; i++ {
		testutil.POST(t, e, "/login", `{"username":"longago","password":"wrong"}`, nil)
	}

	var user models.User
	require.NoError(t, db.Where("username = ?", "longago").First(&user).Error)
	require.NotNil(t, user.LockedUntil)
	assert.InDelta(t, 60, time.Until(*user.LockedUntil).Seconds(), 5,
		"после суток тишины блокировка снова минутная")
	assert.Equal(t, 1, user.LockoutLevel)
}

// TestLogin_LockTimerBeatsIPTimer - когда учётка заперта дольше, чем адрес,
// пользователю показывается больший остаток: меньший обещал бы вход раньше срока.
func TestLogin_LockTimerBeatsIPTimer(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "longlock", "correctpass", 1, td.OrgID, td.CompanyID)

	// Учётка уже на четвёртой ступени (следующая блокировка - полчаса). Отметка
	// последней неудачи обязательна: без неё ступень считается протухшей и падает.
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "longlock").
		Updates(map[string]interface{}{
			"lockout_level":        3,
			"last_failed_login_at": time.Now(),
		}).Error)

	var rec *httptest.ResponseRecorder
	for i := 1; i <= 5; i++ {
		rec = testutil.POST(t, e, "/login", `{"username":"longlock","password":"wrong"}`, nil)
	}
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.Greater(t, retryAfter, 60, "таймер учётки (30 минут), а не минута адреса")
}

// TestLogin_NonexistentUserAlsoBlocks - несуществующий логин ведёт тот же per-IP
// счётчик и так же блокируется (единое поведение, не палит существование логина).
func TestLogin_NonexistentUserAlsoBlocks(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	for i := 1; i <= 4; i++ {
		rec := testutil.POST(t, e, "/login", `{"username":"ghost","password":"x"}`, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "попытка %d", i)
		assert.Equal(t, strconv.Itoa(5-i), rec.Header().Get("X-Auth-Attempts-Remaining"),
			"счётчик и для несуществующего логина, попытка %d", i)
	}
	rec := testutil.POST(t, e, "/login", `{"username":"ghost","password":"x"}`, nil)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "несуществующий логин тоже блокируется")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// TestLogin_ExpiredLockResetsCounter - после истечения блокировки счётчик неудач
// сбрасывается: новая неверная попытка даёт свежий цикл ("осталось 9"), а не
// мгновенный ре-лок с "осталось 0".
func TestLogin_ExpiredLockResetsCounter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "trapme", "correctpass", 1, td.OrgID, td.CompanyID)

	// Истёкшая блокировка + счётчик на пороге (состояние после отбытой минуты).
	pastLock := time.Now().Add(-1 * time.Second)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "trapme").
		Updates(map[string]interface{}{"locked_until": pastLock, "failed_login_count": 5}).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"trapme","password":"wrong"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "4", rec.Header().Get("X-Auth-Attempts-Remaining"),
		"свежий цикл: 5 - 1 = 4, а не мгновенный ре-лок")

	var user models.User
	require.NoError(t, db.Where("username = ?", "trapme").First(&user).Error)
	assert.Equal(t, 1, user.FailedLoginCount, "счётчик сброшен и инкрементнут на 1")
	assert.Nil(t, user.LockedUntil, "не залочен заново")
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
	assert.Contains(t, rec.Body.String(), "Вход заблокирован")
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
		Updates(map[string]interface{}{"locked_until": pastLock, "failed_login_count": 5}).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"unlocked","password":"correctpass"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// После успешного входа lock и счётчик сброшены.
	var user models.User
	require.NoError(t, db.Where("username = ?", "unlocked").First(&user).Error)
	assert.Equal(t, 0, user.FailedLoginCount)
	assert.Nil(t, user.LockedUntil)
}

// TestLogin_BlockedIPStillShowsLongerAccountTimer - когда заперты и адрес, и
// учётка, отдаётся больший срок. Меньший обещал бы вход раньше, чем он откроется:
// человек досидел бы минуту адреса и упёрся в ту же плашку.
func TestLogin_BlockedIPStillShowsLongerAccountTimer(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "bothlocked", "correctpass", 1, td.OrgID, td.CompanyID)

	from := http.Header{}
	from.Set("X-Forwarded-For", "192.0.2.55")
	// Пять неудач запирают адрес на минуту и учётку на первую ступень.
	for i := 1; i <= 5; i++ {
		testutil.POST(t, e, "/login", `{"username":"bothlocked","password":"wrong"}`, from)
	}
	// Учётку удлиняем до получаса - как если бы это был не первый её круг.
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "bothlocked").
		Update("locked_until", time.Now().Add(30*time.Minute)).Error)

	rec := testutil.POST(t, e, "/login", `{"username":"bothlocked","password":"correctpass"}`, from)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.Greater(t, retryAfter, 60, "срок учётки, а не минута адреса")
}

// TestLogin_LockoutMessageSameForUnknownUser - текст блокировки не отличает
// существующий логин от выдуманного, иначе по нему можно перебирать имена.
func TestLogin_LockoutMessageSameForUnknownUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "real", "correctpass", 1, td.OrgID, td.CompanyID)

	lockedBody := func(username, ip string) string {
		h := http.Header{}
		h.Set("X-Forwarded-For", ip)
		var rec *httptest.ResponseRecorder
		for i := 1; i <= 5; i++ {
			rec = testutil.POST(t, e, "/login",
				fmt.Sprintf(`{"username":%q,"password":"wrong"}`, username), h)
		}
		require.Equal(t, http.StatusTooManyRequests, rec.Code, "логин %s", username)
		return rec.Body.String()
	}

	assert.Equal(t, lockedBody("real", "192.0.2.71"), lockedBody("ghost", "192.0.2.72"),
		"ответ на исчерпание попыток одинаков для существующего и выдуманного логина")
}

// --- Сброс блокировки входа из админки (#1600) ---

// TestResetLockout_UnlocksAccount - после сброса заблокированный входит сразу,
// не дожидаясь конца кулдауна, а лестница возвращается на первую ступень.
func TestResetLockout_UnlocksAccount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "stuck", "correctpass", 1, td.OrgID, td.CompanyID)

	// Человек залочен на полчаса и стоит на четвёртой ступени.
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "stuck").
		Updates(map[string]interface{}{
			"locked_until":         time.Now().Add(30 * time.Minute),
			"lockout_level":        3,
			"failed_login_count":   0,
			"last_failed_login_at": time.Now(),
		}).Error)

	// До сброса даже верный пароль не пускает.
	rec := testutil.POST(t, e, "/login", `{"username":"stuck","password":"correctpass"}`, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	rec = testutil.POST(t, e, "/users/stuck/reset-lockout", "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, true, testutil.ParseMap(t, rec)["reset"])

	var user models.User
	require.NoError(t, db.Where("username = ?", "stuck").First(&user).Error)
	assert.Nil(t, user.LockedUntil)
	assert.Equal(t, 0, user.LockoutLevel, "лестница опущена на первую ступень")
	assert.Nil(t, user.LastFailedLoginAt)

	// И вход открывается тем же паролем.
	rec = testutil.POST(t, e, "/login", `{"username":"stuck","password":"correctpass"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestResetLockout_ClearsIPGuard - сброс снимает и блокировку адреса, с которого
// человек ошибался: иначе лок с учётки снят, а войти всё равно нельзя.
func TestResetLockout_ClearsIPGuard(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "typist", "correctpass", 1, td.OrgID, td.CompanyID)

	from := http.Header{}
	from.Set("X-Forwarded-For", "198.51.100.7")
	for i := 1; i <= 5; i++ {
		testutil.POST(t, e, "/login", `{"username":"typist","password":"wrong"}`, from)
	}
	// Адрес заперт: верный пароль не проходит.
	rec := testutil.POST(t, e, "/login", `{"username":"typist","password":"correctpass"}`, from)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	rec = testutil.POST(t, e, "/users/typist/reset-lockout", "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.POST(t, e, "/login", `{"username":"typist","password":"correctpass"}`, from)
	assert.Equal(t, http.StatusOK, rec.Code, "с того же адреса вход открылся: %s", rec.Body.String())
}

// TestResetLockout_NothingToReset - на незаблокированном сброс отвечает reset=false
// и не пишет событие: администратор не должен видеть в журнале пустые срабатывания.
func TestResetLockout_NothingToReset(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "clean", "correctpass", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/users/clean/reset-lockout", "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, false, testutil.ParseMap(t, rec)["reset"])

	var events int64
	require.NoError(t, db.Model(&models.AuthEvent{}).
		Where("username = ? AND event_type = ?", "clean", models.AuthEventLockoutReset).
		Count(&events).Error)
	assert.Zero(t, events, "пустой сброс не пишет событие")
}

// TestResetLockout_RecordsAuthEvent - снятие видно в истории входов рядом с самой
// блокировкой, с указанием, кто снял.
func TestResetLockout_RecordsAuthEvent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "logged", "correctpass", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "logged").
		Update("locked_until", time.Now().Add(time.Hour)).Error)

	rec := testutil.POST(t, e, "/users/logged/reset-lockout", "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	var ev models.AuthEvent
	require.NoError(t, db.Where("username = ? AND event_type = ?", "logged", models.AuthEventLockoutReset).
		First(&ev).Error)
	assert.True(t, ev.Success)
	assert.Contains(t, ev.Detail, "testadmin", "в детали виден снявший блокировку")
}

// TestResetLockout_RequiresUsersPermission - обычный пользователь снять блокировку
// не может: эндпоинт под тем же гейтом, что и остальное управление учётками.
func TestResetLockout_RequiresUsersPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "plain", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "victim", "pass123", 1, td.OrgID, td.CompanyID)
	token, _ := testutil.LoginUser(t, e, "plain", "pass123")

	rec := testutil.POST(t, e, "/users/victim/reset-lockout", "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestResetLockout_UnknownUser - несуществующий логин даёт 404, а не молчаливый успех.
func TestResetLockout_UnknownUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/users/призрак/reset-lockout", "", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusNotFound, rec.Code)
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
