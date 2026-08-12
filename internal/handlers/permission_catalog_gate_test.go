package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Каталог прав отдаёт перечень всех ключей системы с человеческими названиями -
// страницы, действия, разделы администрирования и права каждой системной таблицы.
// Это карта устройства доступа, а не справочник для формы, поэтому он закрыт тем
// же правом, что и соседи по группе (#1967). Открытым остаётся только /permissions/my.
func TestPermissions_Catalog_RequiresAuditManage(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	type catNode struct {
		Key string `json:"key"`
	}

	userToken := testutil.RegisterAndLogin(t, e, "catalog_plain", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/permissions/catalog", testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "без permission.audit.manage каталог прав не отдаётся")

	// Гейт смотрит на право, а не на админство: тот же обычный пользователь с
	// грантом каталог получает.
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "catalog_plain").Row().Scan(&userID))
	testutil.GrantPermission(t, userID, services.KeyAuditManage)

	rec = testutil.GET(t, e, "/permissions/catalog", testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code, "с permission.audit.manage каталог должен приходить")
	nodes := testutil.ParseResponse[[]catNode](t, rec)
	keys := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		keys[n.Key] = true
	}
	assert.True(t, keys["page.center"], "каталог обязан прийти наполненным, а не пустым списком")

	// Экран прав открывает администратор (модалка в разделе пользователей), поэтому
	// он проходит гейт без отдельного гранта - audit.manage не super-only.
	adminToken := testutil.RegisterManager(t, e, "catalog_admin", td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/permissions/catalog", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code, "администратор читает каталог прав")
}
