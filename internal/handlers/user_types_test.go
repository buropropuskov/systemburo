package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createUserType создаёт несистемный тип через админский endpoint и возвращает его ID.
func createUserType(t *testing.T, e *echo.Echo, h http.Header, name, code string) int {
	t.Helper()
	rec := testutil.POST(t, e, "/user-types-management", fmt.Sprintf(`{"name":%q,"code":%q}`, name, code), h)
	require.Equal(t, http.StatusOK, rec.Code, "create user type: %s", rec.Body.String())
	return int(testutil.ParseMap(t, rec)["id"].(float64))
}

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

func TestUserTypes_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// create -> renamed -> deleted, каждое действие пишется в историю.
	rec := testutil.POST(t, e, "/user-types-management", `{"name":"Ист тип","code":"hist_type"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/user-types-management/%d", id),
		`{"name":"Ист тип 2","code":"hist_type"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// История после create+rename: 2 записи, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2)
	assert.Equal(t, "renamed", hist[0]["action_type"])
	assert.Equal(t, "created", hist[1]["action_type"])
	// actor проставлен (имя или username админа, не пусто).
	assert.NotEmpty(t, hist[0]["actor_name"])

	// Удаление логируется; история переживает удаление типа.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", id), h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist = testutil.ParseSlice(t, rec)
	require.Len(t, hist, 3)
	assert.Equal(t, "deleted", hist[0]["action_type"])
}

func TestUserTypes_History_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/user-types-management/1/history", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
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

// TestUserTypes_BlockingUsersAndReassign проверяет полный флоу: список блокеров
// (включая архивных - Delete считает все type_id), перенос всех в другой тип
// освобождает исходный (его можно удалить), повторный перенос идемпотентен,
// аудит смены типа пишется на каждого.
func TestUserTypes_BlockingUsersAndReassign(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	srcID := createUserType(t, e, h, "Источник", "src_type")
	tgtID := createUserType(t, e, h, "Цель", "tgt_type")

	// Активный пользователь исходного типа.
	testutil.RegisterAndLogin(t, e, "typeuser1", "password123", srcID, td.OrgID, td.CompanyID)
	// Архивный пользователь того же типа - ТОЖЕ блокирует удаление (Delete считает
	// все type_id, независимо от is_active), в отличие от org/company (active-only).
	archived := models.User{Username: "typeuserarchived", Password: "x", TypeID: srcID}
	require.NoError(t, db.Create(&archived).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", archived.ID).Update("is_active", false).Error)

	// Список блокеров = все пользователи типа, активный и архивный.
	blockers := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/blocking-users", srcID), h))
	active := map[string]bool{}
	for _, b := range blockers {
		active[b["username"].(string)] = b["is_active"].(bool)
	}
	assert.Len(t, blockers, 2)
	require.Contains(t, active, "typeuser1")
	require.Contains(t, active, "typeuserarchived")
	assert.True(t, active["typeuser1"], "активный помечен is_active")
	assert.False(t, active["typeuserarchived"], "архивный помечен !is_active")

	// Пока есть пользователи - удаление типа запрещено.
	assert.Equal(t, http.StatusBadRequest, testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", srcID), h).Code)

	// Перенос всех пользователей в целевой тип.
	rec := testutil.POST(t, e, fmt.Sprintf("/user-types-management/%d/reassign-users", srcID), fmt.Sprintf(`{"target_type_id":%d}`, tgtID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["reassigned"])

	// Исходный тип свободен, оба (включая архивного) теперь в целевом.
	assert.Empty(t, testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/blocking-users", srcID), h)), "исходный тип без блокеров")
	assert.Len(t, testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/blocking-users", tgtID), h)), 2)

	// Аудит смены типа записан на каждого перенесённого.
	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ?", models.AuditEntityUser, models.UserActionTypeChanged).
		Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount, "type_changed аудит на каждого перенесённого")

	// Идемпотентность: повторный перенос без пользователей - 200, reassigned:0
	// (тип ещё существует, поэтому не 404).
	again := testutil.POST(t, e, fmt.Sprintf("/user-types-management/%d/reassign-users", srcID), fmt.Sprintf(`{"target_type_id":%d}`, tgtID), h)
	require.Equal(t, http.StatusOK, again.Code, again.Body.String())
	assert.Equal(t, float64(0), testutil.ParseMap(t, again)["reassigned"])

	// Теперь исходный тип можно удалить.
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/user-types-management/%d", srcID), h).Code)
}

// TestUserTypes_ReassignUsers_Validation проверяет гейт и валидацию цели/источника.
func TestUserTypes_ReassignUsers_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	srcID := createUserType(t, e, h, "Источник-В", "src_val")
	tgtID := createUserType(t, e, h, "Цель-В", "tgt_val")
	systemID := findTypeIDByCode(t, testutil.ParseSlice(t, testutil.GET(t, e, "/user-types-management", h)), "renter")

	reassign := func(id int, body string) int {
		return testutil.POST(t, e, fmt.Sprintf("/user-types-management/%d/reassign-users", id), body, h).Code
	}

	// Не указан / нулевой целевой тип.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, `{}`))
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, `{"target_type_id":0}`))
	// Цель = источнику.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, fmt.Sprintf(`{"target_type_id":%d}`, srcID)))
	// Несуществующая цель.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, `{"target_type_id":999999}`))
	// Несуществующий источник.
	assert.Equal(t, http.StatusNotFound, reassign(999999, fmt.Sprintf(`{"target_type_id":%d}`, tgtID)))
	// Системный тип нельзя освободить как источник.
	assert.Equal(t, http.StatusBadRequest, reassign(systemID, fmt.Sprintf(`{"target_type_id":%d}`, tgtID)))

	// Перенос В системный тип ДОПУСТИМ (в дефолтный тип переносить можно).
	testutil.RegisterAndLogin(t, e, "valuser", "password123", srcID, td.OrgID, td.CompanyID)
	okRec := testutil.POST(t, e, fmt.Sprintf("/user-types-management/%d/reassign-users", srcID), fmt.Sprintf(`{"target_type_id":%d}`, systemID), h)
	require.Equal(t, http.StatusOK, okRec.Code, okRec.Body.String())
	assert.Equal(t, float64(1), testutil.ParseMap(t, okRec)["reassigned"])

	// Гейт: не-админ отбивается на обоих endpoint-ах.
	userToken := testutil.RegisterAndLogin(t, e, "plainuser2", "password123", 1, td.OrgID, td.CompanyID)
	assert.Equal(t, http.StatusForbidden, testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/blocking-users", srcID), testutil.AuthHeader(userToken)).Code)
	assert.Equal(t, http.StatusForbidden, testutil.POST(t, e, fmt.Sprintf("/user-types-management/%d/reassign-users", srcID), fmt.Sprintf(`{"target_type_id":%d}`, tgtID), testutil.AuthHeader(userToken)).Code)
}
