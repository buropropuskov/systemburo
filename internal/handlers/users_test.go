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

func TestUsers_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Register a second user
	testutil.RegisterUser(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)

	// Активность одного из юзеров: колонка «В сети» в админке рисуется по last_seen,
	// поэтому список обязан отдавать его как заполненным, так и явным null.
	seen := time.Now().UTC().Add(-2 * time.Minute)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "regularuser").
		Update("last_seen", seen).Error)

	rec := testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(list), 2) // admin + regularuser

	// Verify response structure
	for _, u := range list {
		assert.Contains(t, u, "id")
		assert.Contains(t, u, "username")
		assert.Contains(t, u, "type_id")
		assert.Contains(t, u, "user_type")
		assert.Contains(t, u, "last_seen")
	}

	byLogin := map[string]map[string]any{}
	for _, u := range list {
		if login, ok := u["username"].(string); ok {
			byLogin[login] = u
		}
	}
	require.Contains(t, byLogin, "regularuser")
	activeSeen, ok := byLogin["regularuser"]["last_seen"].(string)
	require.True(t, ok, "last_seen активного юзера приходит строкой-датой")
	parsed, err := time.Parse(time.RFC3339Nano, activeSeen)
	require.NoError(t, err)
	assert.WithinDuration(t, seen, parsed, time.Second)
}

func TestUsers_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/users/all", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsers_GetAll_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/users/all", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdateType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Get user types to find the "renter" type_id
	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	types := testutil.ParseSlice(t, rec)

	var renterTypeID int
	for _, ut := range types {
		if ut["code"] == "renter" {
			renterTypeID = int(ut["id"].(float64))
			break
		}
	}
	require.Greater(t, renterTypeID, 0)

	// Update type
	body := fmt.Sprintf(`{"type_id":%d}`, renterTypeID)
	rec = testutil.PUT(t, e, "/users/targetuser/type", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "User type updated successfully", resp)

	// Verify the change via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	users := testutil.ParseSlice(t, rec)

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, float64(renterTypeID), u["type_id"])
			break
		}
	}
}

func TestUsers_UpdateType_InvalidType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/users/targetuser/type", `{"type_id":99999}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUsers_UpdateType_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/users/regularuser/type", `{"type_id":2}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdatePassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "oldpassword", 1, td.OrgID, td.CompanyID)

	// Update password
	rec := testutil.PUT(t, e, "/users/targetuser/password", `{"password":"newpassword123"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Password updated successfully", resp)

	// Verify new password works by logging in
	rec = testutil.POST(t, e, "/login", `{"username":"targetuser","password":"newpassword123"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify old password no longer works
	rec = testutil.POST(t, e, "/login", `{"username":"targetuser","password":"oldpassword"}`, nil)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestUsers_UpdatePassword_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/users/regularuser/password", `{"password":"newpass"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdateInfo(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	body := `{
		"last_name": "Иванов",
		"first_name": "Иван",
		"middle_name": "Иванович",
		"position": "Инженер",
		"email": "ivanov@test.com",
		"phone": "+79001234567"
	}`
	rec := testutil.PUT(t, e, "/users/targetuser/info", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "User info updated successfully", resp)

	// Verify via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	users := testutil.ParseSlice(t, rec)

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, "Иванов", u["last_name"])
			assert.Equal(t, "Иван", u["first_name"])
			assert.Equal(t, "Иванович", u["middle_name"])
			assert.Equal(t, "Инженер", u["position"])
			assert.Equal(t, "ivanov@test.com", u["email"])
			assert.Equal(t, "+79001234567", u["phone"])
			break
		}
	}
}

func TestUsers_UpdateInfo_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/users/regularuser/info", `{"last_name":"Test"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdateOrganization(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Create a second organization
	rec := testutil.POST(t, e, "/organizations", `{"name":"New Organization","type":"Организация"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	orgResp := testutil.ParseMap(t, rec)
	newOrgID := int(orgResp["id"].(float64))

	// Update user's organization
	body := fmt.Sprintf(`{"organization_id":%d}`, newOrgID)
	rec = testutil.PUT(t, e, "/users/targetuser/organization", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Organization updated successfully", resp)

	// Verify via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	users := testutil.ParseSlice(t, rec)

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, float64(newOrgID), u["organization_id"])
			break
		}
	}
}

func TestUsers_UpdateCompany(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Create a second company
	rec := testutil.POST(t, e, "/companies", `{"name":"New Company","type":"Организация"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	compResp := testutil.ParseMap(t, rec)
	newCompID := int(compResp["id"].(float64))

	// Update user's company
	body := fmt.Sprintf(`{"company_id":%d}`, newCompID)
	rec = testutil.PUT(t, e, "/users/targetuser/company", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Company updated successfully", resp)

	// Verify via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	users := testutil.ParseSlice(t, rec)

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, float64(newCompID), u["company_id"])
			break
		}
	}
}

func TestUsers_Delete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Verify user exists
	rec := testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	users := testutil.ParseSlice(t, rec)

	found := false
	for _, u := range users {
		if u["username"] == "targetuser" {
			found = true
			break
		}
	}
	require.True(t, found, "targetuser should exist before deletion")

	// Delete (soft-delete: архивация)
	rec = testutil.DELETE(t, e, "/users/targetuser", h)
	require.Equal(t, http.StatusOK, rec.Code)

	deleteResp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "User archived successfully", deleteResp)

	// Verify gone из активного списка
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	users = testutil.ParseSlice(t, rec)

	for _, u := range users {
		assert.NotEqual(t, "targetuser", u["username"])
	}
}

func TestUsers_SoftDeleteAndRestore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "softtarget", "password123", 1, td.OrgID, td.CompanyID)

	// Архивация
	rec := testutil.DELETE(t, e, "/users/softtarget", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Не виден в активном списке
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	for _, u := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "softtarget", u["username"])
	}

	// Виден с include_archived=true и is_active=false (строка не удалена физически)
	rec = testutil.GET(t, e, "/users/all?include_archived=true", h)
	require.Equal(t, http.StatusOK, rec.Code)
	foundArchived := false
	for _, u := range testutil.ParseSlice(t, rec) {
		if u["username"] == "softtarget" {
			foundArchived = true
			assert.Equal(t, false, u["is_active"])
		}
	}
	assert.True(t, foundArchived, "архивный пользователь должен возвращаться с include_archived=true")

	// Восстановление
	rec = testutil.POST(t, e, "/users/softtarget/restore", "", h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "User restored successfully", testutil.ParseMessage(t, rec))

	// Снова в активном списке, is_active=true
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	foundActive := false
	for _, u := range testutil.ParseSlice(t, rec) {
		if u["username"] == "softtarget" {
			foundActive = true
			assert.Equal(t, true, u["is_active"])
		}
	}
	assert.True(t, foundActive, "восстановленный пользователь должен быть в активном списке")
}

func TestUsers_ArchivedCannotLogin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "logintarget", "password123", 1, td.OrgID, td.CompanyID)

	// До архивации login работает
	rec := testutil.POST(t, e, "/login", `{"username":"logintarget","password":"password123"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// Архивируем
	rec = testutil.DELETE(t, e, "/users/logintarget", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Архивный не может войти даже с верным паролем - 403 "отключена" (пароль проверен)
	rec = testutil.POST(t, e, "/login", `{"username":"logintarget","password":"password123"}`, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Неверный пароль архивного - всё ещё 401 (не раскрываем статус)
	rec = testutil.POST(t, e, "/login", `{"username":"logintarget","password":"wrongpass"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// После восстановления login снова работает
	rec = testutil.POST(t, e, "/users/logintarget/restore", "", h)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = testutil.POST(t, e, "/login", `{"username":"logintarget","password":"password123"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUsers_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "histtarget", "password123", 1, td.OrgID, td.CompanyID)

	// renter type_id для смены типа
	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	var renterID int
	for _, ut := range testutil.ParseSlice(t, rec) {
		if ut["code"] == "renter" {
			renterID = int(ut["id"].(float64))
			break
		}
	}
	require.Greater(t, renterID, 0)

	// Действия, которые должны попасть в историю: смена типа + архивация.
	testutil.PUT(t, e, "/users/histtarget/type", fmt.Sprintf(`{"type_id":%d}`, renterID), h)
	testutil.DELETE(t, e, "/users/histtarget", h)

	rec = testutil.GET(t, e, "/users/histtarget/history", h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(hist), 2, "история должна содержать type_changed + archived")

	actions := map[string]bool{}
	for _, item := range hist {
		actions[item["action_type"].(string)] = true
		assert.NotEmpty(t, item["actor_name"], "actor_name должен быть заполнен именем админа")
	}
	assert.True(t, actions["type_changed"], "ожидалась запись type_changed")
	assert.True(t, actions["archived"], "ожидалась запись archived")
}

func TestUsers_Delete_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.DELETE(t, e, "/users/someuser", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsers_Delete_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/users/regularuser", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_Create_RequiresOrgOrCompany(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Без organization_id и company_id - 400
	body := `{"username":"lonely","password":"password123","type_id":1,"organization_id":0,"company_id":0}`
	rec := testutil.POST(t, e, "/users", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Только organization - OK
	body = fmt.Sprintf(`{"username":"orgonly","password":"password123","type_id":1,"organization_id":%d,"company_id":0}`, td.OrgID)
	rec = testutil.POST(t, e, "/users", body, h)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Только company - OK
	body = fmt.Sprintf(`{"username":"companyonly","password":"password123","type_id":1,"organization_id":0,"company_id":%d}`, td.CompanyID)
	rec = testutil.POST(t, e, "/users", body, h)
	assert.Equal(t, http.StatusOK, rec.Code)

	// И то, и другое - OK
	body = fmt.Sprintf(`{"username":"both","password":"password123","type_id":1,"organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
	rec = testutil.POST(t, e, "/users", body, h)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUsers_ManagerAlsoHasAdminAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// manager (type_id=5) перенесён миграцией на is_admin, поэтому сохраняет
	// доступ к управлению пользователями и после снятия type-проверок (Ф5).
	managerToken := testutil.RegisterManager(t, e, "manager1", td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(managerToken)

	rec := testutil.GET(t, e, "/users/all", h)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCreateUser_WeakPassword_Rejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Пароль без цифры при require_digit=true (дефолт) -> 400
	body := fmt.Sprintf(`{"username":"weakpw","password":"passwordonly","type_id":1,"organization_id":%d}`, td.OrgID)
	rec := testutil.POST(t, e, "/users", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Слишком короткий пароль -> 400
	body = fmt.Sprintf(`{"username":"weakpw2","password":"ab1","type_id":1,"organization_id":%d}`, td.OrgID)
	rec = testutil.POST(t, e, "/users", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Валидный пароль (буква + цифра, >= 8 символов) -> 200
	body = fmt.Sprintf(`{"username":"strongpw","password":"password123","type_id":1,"organization_id":%d}`, td.OrgID)
	rec = testutil.POST(t, e, "/users", body, h)
	assert.Contains(t, []int{http.StatusOK, http.StatusCreated}, rec.Code)
}

func TestUpdatePassword_WeakPassword_Rejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Создаём целевого пользователя через DB (как в TestUsers_UpdatePassword)
	testutil.RegisterUser(t, e, "pwpolicytarget", "password123", 1, td.OrgID, td.CompanyID)

	// Слабый пароль (нет цифры) -> 400
	rec := testutil.PUT(t, e, "/users/pwpolicytarget/password", `{"password":"weakpassword"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Слишком короткий -> 400
	rec = testutil.PUT(t, e, "/users/pwpolicytarget/password", `{"password":"short1"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Валидный пароль -> 200
	rec = testutil.PUT(t, e, "/users/pwpolicytarget/password", `{"password":"newpassword123"}`, h)
	assert.Equal(t, http.StatusOK, rec.Code)
}
