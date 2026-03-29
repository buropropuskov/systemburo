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

func TestPermissions_GetMy_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/permissions/my", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPermissions_GetMy_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "permuser1", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/permissions/my", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var perms []models.UserPermissionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perms))
	assert.Equal(t, 0, len(perms))
}

func TestPermissions_GetMy_WithPermissions(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "permuser2", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Manually grant a permission
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permuser2").Row().Scan(&userID))

	up := models.UserPermission{
		UserID:        userID,
		PermissionKey: "tab.cars.view",
		Value:         "allow",
	}
	require.NoError(t, db.Create(&up).Error)

	rec := testutil.GET(t, e, "/permissions/my", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var perms []models.UserPermissionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perms))
	assert.Equal(t, 1, len(perms))
	assert.Equal(t, "tab.cars.view", perms[0].Key)
	assert.Equal(t, "allow", perms[0].Value)
	assert.Equal(t, "tab", perms[0].Category)
}

func TestPermissions_GetUserPermissions_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Regular user should get 403
	userToken := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "regularuser").Row().Scan(&userID))

	rec := testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Admin should succeed
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPermissions_GetUserPermissions_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/permissions/user/99999", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPermissions_UpdateUserPermissions_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "permtarget", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permtarget").Row().Scan(&userID))

	body := fmt.Sprintf(`{"permissions":[{"key":"tab.cars.view","value":"allow"}]}`)

	// Regular user should get 403
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Admin should succeed
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify permission was granted
	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	var perms []models.UserPermissionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perms))
	assert.Equal(t, 1, len(perms))
	assert.Equal(t, "tab.cars.view", perms[0].Key)
	assert.Equal(t, "allow", perms[0].Value)
}

func TestPermissions_UpdateUserPermissions_InvalidValue(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "permtarget2", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permtarget2").Row().Scan(&userID))

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Invalid value (not in allow/deny/read/write)
	body := fmt.Sprintf(`{"permissions":[{"key":"tab.cars.view","value":"invalid"}]}`)
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPermissions_UpdateUserPermissions_NonexistentKey(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "permtarget3", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permtarget3").Row().Scan(&userID))

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{"permissions":[{"key":"nonexistent.key","value":"allow"}]}`)
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPermissions_UpdateUserPermissions_Upsert(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "permupsert", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permupsert").Row().Scan(&userID))

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// First set to allow
	body := `{"permissions":[{"key":"tab.cars.view","value":"allow"}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Then update to deny
	body = `{"permissions":[{"key":"tab.cars.view","value":"deny"}]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify it was updated
	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	var perms []models.UserPermissionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perms))
	assert.Equal(t, 1, len(perms))
	assert.Equal(t, "deny", perms[0].Value)
}

func TestPermissions_GetTree(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "treeuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/permissions/tree", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var tree []models.PermissionTreeNode
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tree))
	// Should have at least the seeded tab permissions
	assert.GreaterOrEqual(t, len(tree), 4)
}

func TestPermissions_AutoGenerate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "autogenuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"table_id":1,"table_name":"test_table"}`
	rec := testutil.POST(t, e, "/permissions/auto-generate", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify permissions were created
	var count int64
	db.Model(&models.Permission{}).Where("key LIKE ?", "table.test_table.%").Count(&count)
	assert.Equal(t, int64(2), count)

	// Verify specific keys
	var perm models.Permission
	require.NoError(t, db.Where("key = ?", "table.test_table.view").First(&perm).Error)
	assert.Equal(t, "table", perm.Category)
	assert.Contains(t, perm.DisplayName, "test_table")

	var editPerm models.Permission
	require.NoError(t, db.Where("key = ?", "table.test_table.edit").First(&editPerm).Error)
	assert.Equal(t, "table", editPerm.Category)
}

func TestPermissions_AutoGenerate_Idempotent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "idempuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"table_id":2,"table_name":"idem_table"}`

	// Call twice
	rec := testutil.POST(t, e, "/permissions/auto-generate", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.POST(t, e, "/permissions/auto-generate", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Should still only have 2 permissions
	var count int64
	db.Model(&models.Permission{}).Where("key LIKE ?", "table.idem_table.%").Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestPermissions_DefaultSeeded(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	_ = e

	// Verify default tab permissions were seeded
	var count int64
	db.Model(&models.Permission{}).Where("category = ?", "tab").Count(&count)
	assert.Equal(t, int64(4), count)

	expectedKeys := []string{"tab.cars.view", "tab.employees.view", "tab.overview.view", "tab.profile.view"}
	for _, key := range expectedKeys {
		var perm models.Permission
		err := db.Where("key = ?", key).First(&perm).Error
		assert.NoError(t, err, "expected permission %s to be seeded", key)
	}
}

func TestPermissions_SystemTableCreate_AutoGenerates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Create a system table
	body := `{"name":"autogen_test","display_name":"Auto Gen Test","table_type":"cars"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify permissions were auto-generated
	var count int64
	db.Model(&models.Permission{}).Where("key LIKE ?", "table.autogen_test.%").Count(&count)
	assert.Equal(t, int64(2), count)
}
