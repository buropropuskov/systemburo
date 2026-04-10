package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissions_GrantDefaultPermissions_VisibleViaMy(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "defperm1", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Before granting defaults, user has no permissions
	rec := testutil.GET(t, e, "/permissions/my", h)
	require.Equal(t, http.StatusOK, rec.Code)
	permsBefore := testutil.ParseResponse[[]models.UserPermissionResponse](t, rec)
	assert.Equal(t, 0, len(permsBefore), "new user should have no permissions initially")

	// Grant default permissions via service (simulating what registration should do)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "defperm1").Row().Scan(&userID))

	permService := services.NewPermissionService(db)
	err := permService.GrantDefaultPermissions(t.Context(), userID)
	require.NoError(t, err)

	// Verify via GET /permissions/my
	rec = testutil.GET(t, e, "/permissions/my", h)
	require.Equal(t, http.StatusOK, rec.Code)
	permsAfter := testutil.ParseResponse[[]models.UserPermissionResponse](t, rec)

	// GrantDefaultPermissions creates 5 user_permissions, but GetMyPermissions JOINs
	// with the permissions table which only has 4 seeded keys (tab.applications.view
	// is in the defaults list but not in the permissions table seed).
	assert.Equal(t, 4, len(permsAfter),
		"should see 4 default permissions (only those with matching permissions table entries)")

	// Verify specific keys
	keys := make(map[string]string)
	for _, p := range permsAfter {
		keys[p.Key] = p.Value
	}
	expectedKeys := []string{
		"tab.cars.view",
		"tab.employees.view",
		"tab.overview.view",
		"tab.profile.view",
	}
	for _, key := range expectedKeys {
		val, ok := keys[key]
		assert.True(t, ok, "expected default permission %s", key)
		assert.Equal(t, "allow", val, "default permission %s should have value 'allow'", key)
	}
}

func TestPermissions_GrantDefaultPermissions_Idempotent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "defperm2", "password123", 1, td.OrgID, td.CompanyID)

	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "defperm2").Row().Scan(&userID))

	permService := services.NewPermissionService(db)

	// Call twice
	require.NoError(t, permService.GrantDefaultPermissions(t.Context(), userID))
	require.NoError(t, permService.GrantDefaultPermissions(t.Context(), userID))

	// Should have exactly 5 user_permissions (no duplicates)
	var count int64
	db.Model(&models.UserPermission{}).Where("user_id = ?", userID).Count(&count)
	assert.Equal(t, int64(5), count, "double GrantDefaultPermissions should not create duplicates")
}

func TestPermissions_AutoGenerate_AnyAuthenticatedUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Regular user (type_id=1) can call auto-generate.
	// NOTE: The endpoint has no admin restriction -- any authenticated user succeeds.
	regularToken := testutil.RegisterAndLogin(t, e, "regauto1", "password123", 1, td.OrgID, td.CompanyID)

	body := `{"table_id":100,"table_name":"edge_test_table"}`
	rec := testutil.POST(t, e, "/permissions/auto-generate", body, testutil.AuthHeader(regularToken))
	assert.Equal(t, http.StatusOK, rec.Code,
		"auto-generate has no admin check -- any authenticated user can call it")

	// Verify permissions were created
	var count int64
	db.Model(&models.Permission{}).Where("key LIKE ?", "table.edge_test_table.%").Count(&count)
	assert.Equal(t, int64(2), count)
}
