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
