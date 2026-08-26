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

// TestRoles_Delete_SystemRole_Conflict: системную роль («Пользователь») удалить
// нельзя - 409 с понятной причиной, а не 500.
func TestRoles_Delete_SystemRole_Conflict(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	var sysRole models.Role
	require.NoError(t, db.Where("is_system = ?", true).First(&sysRole).Error)

	rec := testutil.DELETE(t, e, fmt.Sprintf("/roles/%d", sysRole.ID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "системную")
}

// TestRoles_Delete_WithAssignedUsers_Conflict: роль с привязанными юзерами
// удалить нельзя - 409 с количеством, а не 500.
func TestRoles_Delete_WithAssignedUsers_Conflict(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_with_users", Name: "С юзерами"}
	require.NoError(t, db.Create(&role).Error)
	user := models.User{Username: "role_holder", TypeID: 1, RoleID: &role.ID}
	require.NoError(t, db.Create(&user).Error)

	rec := testutil.DELETE(t, e, fmt.Sprintf("/roles/%d", role.ID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "назначена")
}

// TestRoles_Delete_NotFound: несуществующая роль - 404, а не 500.
func TestRoles_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.DELETE(t, e, "/roles/999999", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestRoles_Delete_EmptyCustomRole_OK: пустую кастомную роль без юзеров можно удалить.
func TestRoles_Delete_EmptyCustomRole_OK(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_empty", Name: "Пустая"}
	require.NoError(t, db.Create(&role).Error)

	rec := testutil.DELETE(t, e, fmt.Sprintf("/roles/%d", role.ID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRoles_SetPermissions_OK: PUT /roles/:id/permissions пишет прямые гранты,
// и List отдаёт их в direct_grants.
func TestRoles_SetPermissions_OK(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_grants", Name: "С грантами"}
	require.NoError(t, db.Create(&role).Error)

	body := `{"keys":["page.tables","header.report_problem"]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var grants []models.RolePermissionGrant
	require.NoError(t, db.Where("role_id = ?", role.ID).Find(&grants).Error)
	assert.Len(t, grants, 2)
	for _, g := range grants {
		assert.Equal(t, "allow", g.Value)
	}

	listRec := testutil.GET(t, e, "/roles", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, listRec.Code)
	roles := testutil.ParseResponse[[]models.RoleResponse](t, listRec)
	var found *models.RoleResponse
	for i := range roles {
		if roles[i].ID == role.ID {
			found = &roles[i]
		}
	}
	require.NotNil(t, found)
	assert.ElementsMatch(t, []string{"page.tables", "header.report_problem"}, found.DirectGrants)
}

// TestRoles_SetPermissions_InvalidKey: неизвестный ключ -> 400, гранты не пишутся.
func TestRoles_SetPermissions_InvalidKey(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_badkey", Name: "Плохой ключ"}
	require.NoError(t, db.Create(&role).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID), `{"keys":["bogus.key"]}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var count int64
	require.NoError(t, db.Model(&models.RolePermissionGrant{}).Where("role_id = ?", role.ID).Count(&count).Error)
	assert.Zero(t, count)
}

// TestRoles_SetPermissions_SuperOnlyRejected: super-only ключ нельзя выдать через роль -> 403.
func TestRoles_SetPermissions_SuperOnlyRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_super", Name: "Супер ключ"}
	require.NoError(t, db.Create(&role).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID), `{"keys":["action.grant.admin"]}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRoles_SetPermissions_NotFound: несуществующая роль -> 404.
func TestRoles_SetPermissions_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/roles/999999/permissions", `{"keys":["page.tables"]}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestRoles_SetPermissions_EmptyClears: пустой список очищает все прямые гранты роли.
func TestRoles_SetPermissions_EmptyClears(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_clear", Name: "Очистка"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&models.RolePermissionGrant{RoleID: role.ID, PermissionKey: "page.tables", Value: "allow"}).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID), `{"keys":[]}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.RolePermissionGrant{}).Where("role_id = ?", role.ID).Count(&count).Error)
	assert.Zero(t, count)
}

// TestRoles_SetPermissions_AdminAllowed: обычный администратор (не супер) тоже
// управляет точечными правами роли -- гейт auditManage пускает is_admin.
func TestRoles_SetPermissions_AdminAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterManager(t, e, "rolemgr", td.OrgID, td.CompanyID)

	role := models.Role{Code: "role_byadmin", Name: "Админ ставит"}
	require.NoError(t, db.Create(&role).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID), `{"keys":["page.cars"]}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
