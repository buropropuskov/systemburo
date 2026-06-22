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

// Администратор (is_admin, не супер) может читать и менять права пользователей -
// раздел управления правами больше не super-only (page.admin.users/audit.manage).
func TestPermissions_Admin_CanReadAndUpdate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "managed_user", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "managed_user").Row().Scan(&userID))

	// is_admin, НЕ супер (RegisterManager -> type 5 -> is_admin).
	adminToken := testutil.RegisterManager(t, e, "plain_admin", td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d/effective", userID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "admin должен читать effective-права")

	body := `{"permissions":[{"key":"tab.cars.view","value":"allow"}]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "admin должен ставить override")

	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	perms := testutil.ParseResponse[[]models.UserPermissionResponse](t, rec)
	require.Equal(t, 1, len(perms))
	assert.Equal(t, "tab.cars.view", perms[0].Key)
	assert.Equal(t, "allow", perms[0].Value)
}

// Администратор НЕ может через override выдать super-only ключ (защита от эскалации),
// а супер-админ - может.
func TestPermissions_Admin_CannotGrantSuperOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "managed_user2", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "managed_user2").Row().Scan(&userID))

	adminToken := testutil.RegisterManager(t, e, "plain_admin2", td.OrgID, td.CompanyID)
	body := `{"permissions":[{"key":"action.grant.admin","value":"allow"}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "admin не может выдать super-only ключ")

	superToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, testutil.AuthHeader(superToken))
	assert.Equal(t, http.StatusOK, rec.Code, "супер-админ может управлять super-only ключами")
}

// Обычный пользователь без permission.audit.manage не имеет доступа к управлению правами.
func TestPermissions_Regular_ForbiddenToManage(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "plain_user", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "plain_user").Row().Scan(&userID))

	rec := testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d/effective", userID), testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
