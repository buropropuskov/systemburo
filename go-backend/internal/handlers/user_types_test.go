package handlers_test

import (
	"encoding/json"
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

	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
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

	var createResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createResp))
	assert.Equal(t, "Тип пользователя успешно создан", createResp["message"])
	typeID := int(createResp["id"].(float64))
	assert.Greater(t, typeID, 0)

	// --- Read (verify created) ---
	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))

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

	var updateResp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updateResp))
	assert.Equal(t, "Тип пользователя успешно обновлен", updateResp)

	// --- Read (verify updated) ---
	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))

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

	var deleteResp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &deleteResp))
	assert.Equal(t, "Тип пользователя успешно удален", deleteResp)

	// --- Read (verify gone) ---
	rec = testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))

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

	// buropropuskov type (ID 6) has the admin user, so deletion should fail.
	// Find the buropropuskov type ID by listing
	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))

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

	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))

	for _, item := range list {
		if item["code"] == "buropropuskov" {
			assert.GreaterOrEqual(t, item["users_count"].(float64), float64(1),
				"buropropuskov should have at least 1 user (the admin)")
		}
	}
}
