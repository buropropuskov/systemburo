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
	"gorm.io/gorm"
)

type archCatNode struct {
	Key string `json:"key"`
}

// catalogKeys собирает ключи каталога прав, который читают редакторы доступа.
func catalogKeys(t *testing.T, e *echo.Echo, h http.Header) map[string]bool {
	t.Helper()
	rec := testutil.GET(t, e, "/permissions/catalog", h)
	require.Equal(t, http.StatusOK, rec.Code)
	keys := make(map[string]bool)
	for _, n := range testutil.ParseResponse[[]archCatNode](t, rec) {
		keys[n.Key] = true
	}
	return keys
}

// Права таблицы, ушедшей в архив, пропадают из каталога, но продолжают жить в БД,
// действовать и возвращаться при восстановлении таблицы (#1881).
//
// Сквозной сценарий, ради которого тест и написан: право выдано роли -> таблицу
// отправили в архив -> роль открыли и сохранили -> право осталось. Именно на этом
// шаге ломается наивная реализация: каталог ключ больше не отдаёт, и редактор,
// собирающий сохраняемый набор из каталога, молча снимает грант навсегда.
func TestPermissions_Catalog_HidesArchivedTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Две таблицы: одну заархивируем, вторая остаётся активной и стережёт от
	// фильтра "снесло все права таблиц разом".
	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"kpp_arch","display_name":"КПП архивный","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	archID := createdTableID(t, db, "kpp_arch")

	rec = testutil.POST(t, e, "/system-tables",
		`{"name":"kpp_live","display_name":"КПП живой","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	const archKey = "table.kpp_arch.view"
	const liveKey = "table.kpp_live.view"

	// Роль с правом архивируемой таблицы + пользователь на этой роли: через него
	// проверяем, что скрытие каталога не трогает проверку доступа.
	role := models.Role{Code: "role_arch_perm", Name: "С правом таблицы"}
	require.NoError(t, db.Create(&role).Error)
	testutil.RegisterAndLogin(t, e, "archroleuser", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "archroleuser").Row().Scan(&userID))
	require.NoError(t, db.Table("users").Where("id = ?", userID).Update("role_id", role.ID).Error)

	rec = testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID),
		fmt.Sprintf(`{"keys":["%s","%s"]}`, archKey, liveKey), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// До архивации оба права в каталоге.
	keys := catalogKeys(t, e, h)
	require.True(t, keys[archKey], "право живой таблицы должно быть в каталоге до архивации")
	require.True(t, keys[liveKey])

	// Архивируем (мягкое удаление, is_active=false).
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", archID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	keys = catalogKeys(t, e, h)
	assert.False(t, keys[archKey], "право архивной таблицы не должно приходить в каталоге")
	assert.True(t, keys[liveKey], "право активной таблицы обязано остаться")
	assert.True(t, keys["page.center"], "статический каталог не должен пострадать")

	// Само право из БД не удалялось - иначе восстанавливать было бы нечего.
	var permCount int64
	require.NoError(t, db.Model(&models.Permission{}).Where("key = ?", archKey).Count(&permCount).Error)
	assert.EqualValues(t, 1, permCount, "право архивной таблицы остаётся строкой в permissions")

	// Право продолжает ДЕЙСТВОВАТЬ: резолвер каталог не читает, скрытие на
	// проверку доступа не влияет.
	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d/effective", userID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	eff := testutil.ParseResponse[models.MyPermissionsResponse](t, rec)
	assert.True(t, hasEffectiveKey(eff, archKey),
		"право архивной таблицы обязано остаться действующим у пользователя")

	// Открыли роль и сохранили. Редактор шлёт то, что у него есть: видимый ключ
	// плюс скрытый, пришедший в direct_grants. Бэкенд обязан его принять.
	rec = testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID),
		fmt.Sprintf(`{"keys":["%s","%s"]}`, archKey, liveKey), h)
	require.Equal(t, http.StatusOK, rec.Code, "сохранение роли со скрытым ключом не должно отбиваться")
	assert.True(t, roleHasGrant(t, db, role.ID, archKey), "грант архивной таблицы пережил сохранение роли")

	// Вернули таблицу из архива - право снова в каталоге и по-прежнему выдано.
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/restore", archID), "", h)
	require.Equal(t, http.StatusOK, rec.Code)

	keys = catalogKeys(t, e, h)
	assert.True(t, keys[archKey], "восстановление таблицы возвращает её права в каталог")
	assert.True(t, roleHasGrant(t, db, role.ID, archKey), "грант никуда не делся за время архива")
}

func createdTableID(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Table("system_tables").Select("id").Where("name = ?", name).Row().Scan(&id))
	return id
}

func roleHasGrant(t *testing.T, db *gorm.DB, roleID int, key string) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.RolePermissionGrant{}).
		Where("role_id = ? AND permission_key = ?", roleID, key).Count(&count).Error)
	return count > 0
}

func hasEffectiveKey(resp models.MyPermissionsResponse, key string) bool {
	for _, p := range resp.Permissions {
		if p.Key == key {
			return true
		}
	}
	return false
}
