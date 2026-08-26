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

// auto-generate плодит ключи прав для таблицы. Раньше роут был открыт любому
// авторизованному (обычный юзер мог засорять каталог прав); теперь закрыт правом
// конструктора таблиц. Обычный юзер получает 403, ключи не создаются; админ - 200.
func TestPermissions_AutoGenerate_RequiresTablesConstructor(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	body := `{"table_id":100,"table_name":"edge_test_table"}`

	regularToken := testutil.RegisterAndLogin(t, e, "regauto1", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/permissions/auto-generate", body, testutil.AuthHeader(regularToken))
	require.Equal(t, http.StatusForbidden, rec.Code,
		"обычный юзер не должен генерировать права таблиц")

	var count int64
	db.Model(&models.Permission{}).Where("key LIKE ?", "table.edge_test_table.%").Count(&count)
	assert.Equal(t, int64(0), count, "при 403 ключи прав не создаются")

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.POST(t, e, "/permissions/auto-generate", body, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "админ генерирует права таблицы")

	db.Model(&models.Permission{}).Where("key LIKE ?", "table.edge_test_table.%").Count(&count)
	assert.Equal(t, int64(10), count)
}
