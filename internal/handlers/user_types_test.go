package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserTypes_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	// Seed creates 6 user types
	assert.GreaterOrEqual(t, len(list), 6)

	// Each entry should have id, name, code, users_count
	for _, item := range list {
		assert.Contains(t, item, "id")
		assert.Contains(t, item, "name")
		assert.Contains(t, item, "code")
		assert.Contains(t, item, "users_count")
	}
}

func TestUserTypes_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/user-types-management", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserTypes_GetAll_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/user-types-management", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUserTypes_CRUD_Cycle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// --- Create ---
	body := `{"name":"Тестовый тип","code":"test_type"}`
	rec := testutil.POST(t, e, "/user-types-management", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Тип пользователя успешно создан", createResp["message"])
	typeID := int(createResp["id"].(float64))
	assert.Greater(t, typeID, 0)

	// --- Read (verify created) ---
	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)

	var found bool
	for _, item := range list {
		if item["code"] == "test_type" {
			found = true
			assert.Equal(t, "Тестовый тип", item["name"])
			assert.Equal(t, float64(0), item["users_count"])
			break
		}
	}
	assert.True(t, found, "created user type not found in list")

	// --- Update ---
	rec = testutil.PUT(t, e, fmt.Sprintf("/user-types-management/%d", typeID),
		`{"name":"Обновлённый тип","code":"test_type_updated"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	updateResp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Тип пользователя успешно обновлен", updateResp)

	// --- Read (verify updated) ---
	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)

	found = false
	for _, item := range list {
		if int(item["id"].(float64)) == typeID {
			found = true
			assert.Equal(t, "Обновлённый тип", item["name"])
			break
		}
	}
	assert.True(t, found, "updated user type not found in list")

	// --- Delete ---
	rec = testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", typeID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	deleteResp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Тип пользователя успешно удален", deleteResp)

	// --- Read (verify gone) ---
	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)

	for _, item := range list {
		assert.NotEqual(t, float64(typeID), item["id"])
	}
}

func TestUserTypes_Create_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/user-types-management", `{"name":"Тест","code":"test"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUserTypes_Create_DuplicateCode(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// "user" code already exists from seed
	body := `{"name":"Дубликат","code":"user"}`
	rec := testutil.POST(t, e, "/user-types-management", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserTypes_Update_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/user-types-management/99999", `{"name":"Ghost","code":"ghost"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUserTypes_Delete_WithAssociatedUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// buropropuskov - системный тип (is_system), удаление блокируется (400).
	// FK-защита несистемных типов с пользователями проверяется отдельно
	// в TestUserTypes_Delete_NonSystemWithUsers.
	// Find the buropropuskov type ID by listing
	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)

	var buroID int
	for _, item := range list {
		if item["code"] == "buropropuskov" {
			buroID = int(item["id"].(float64))
			break
		}
	}
	require.Greater(t, buroID, 0)

	rec = testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", buroID), h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserTypes_Delete_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/user-types-management/1", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// findTypeIDByCode возвращает id типа по его code из ответа GetAll.
func findTypeIDByCode(t *testing.T, list []map[string]interface{}, code string) int {
	t.Helper()
	for _, item := range list {
		if item["code"] == code {
			return int(item["id"].(float64))
		}
	}
	t.Fatalf("user type with code %q not found", code)
	return 0
}

func TestUserTypes_Update_SystemTypeBlocked(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	// Системный тип "user" нельзя переименовать.
	userID := findTypeIDByCode(t, testutil.ParseSlice(t, rec), "user")

	rec = testutil.PUT(t, e, fmt.Sprintf("/user-types-management/%d", userID),
		`{"name":"Переименован","code":"user"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserTypes_Delete_SystemTypeBlocked(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	// "renter" - системный тип без связанных пользователей: блокируется именно
	// по is_system, а не по FK-проверке.
	renterID := findTypeIDByCode(t, testutil.ParseSlice(t, rec), "renter")

	rec = testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", renterID), h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserTypes_Delete_NonSystemWithUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Несистемный тип с привязанным пользователем должен блокироваться FK-проверкой.
	rec := testutil.POST(t, e, "/user-types-management", `{"name":"FK тип","code":"fk_type"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	fkTypeID := int(testutil.ParseMap(t, rec)["id"].(float64))

	testutil.RegisterAndLogin(t, e, "fkuser", "password123", fkTypeID, td.OrgID, td.CompanyID)

	rec = testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", fkTypeID), h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserTypes_GetAll_IncludesIsSystem(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Кастомный (несистемный) тип.
	rec := testutil.POST(t, e, "/user-types-management", `{"name":"Кастомный","code":"custom_type"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.ParseSlice(t, rec)

	var checkedSystem, checkedCustom bool
	for _, item := range list {
		assert.Contains(t, item, "is_system")
		switch item["code"] {
		case "buropropuskov":
			assert.Equal(t, true, item["is_system"], "системный тип должен быть is_system=true")
			checkedSystem = true
		case "custom_type":
			assert.Equal(t, false, item["is_system"], "кастомный тип должен быть is_system=false")
			checkedCustom = true
		}
	}
	assert.True(t, checkedSystem, "buropropuskov не найден")
	assert.True(t, checkedCustom, "custom_type не найден")
}

func TestUserTypes_GetAll_IncludesUsersCount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// admin user is registered with type_id=6 (buropropuskov)
	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)

	for _, item := range list {
		if item["code"] == "buropropuskov" {
			assert.GreaterOrEqual(t, item["users_count"].(float64), float64(1),
				"buropropuskov should have at least 1 user (the admin)")
		}
	}
}
