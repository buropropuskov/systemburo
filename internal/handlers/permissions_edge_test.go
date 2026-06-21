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
	// Эндпоинт /permissions/my мигрирован на PermissionResolver (#187):
	// теперь возвращает ключи из roles+permission_groups, а не из старой
	// таблицы user_permissions через GrantDefaultPermissions. Старое
	// поведение здесь больше не проверяется.
	// GrantDefaultPermissions покрыт TestPermissions_GrantDefaultPermissions_Idempotent
	// (он работает напрямую с таблицей, не через /permissions/my).
	t.Skip("legacy user_permissions table no longer wired to /permissions/my")
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

	// Verify permissions were created (по одному на глагол, всего 8)
	var count int64
	db.Model(&models.Permission{}).Where("key LIKE ?", "table.edge_test_table.%").Count(&count)
	assert.Equal(t, int64(8), count)
}
