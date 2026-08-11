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

// Право таблицы, которой в справочнике нет вовсе, из каталога не приходит (#1881).
//
// Архивную таблицу от удалённой отличает то, что у второй имя брать неоткуда:
// каталог подставлял служебный слаг (`checkpoint-main`, `kpp-cargo`), и в списке
// прав администратора висели сорок строк, не относящихся ни к чему живому.
// Восстановить такую таблицу нельзя - значит и повод показывать её права
// пропадает вместе с ней.
//
// Живая таблица в тесте стережёт от противоположной ошибки: условие "таблица
// существует и активна" легко написать так, что оно вынесет права всех таблиц
// разом, и каталог останется без раздела вовсе.
func TestPermissions_Catalog_HidesOrphanTablePermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"kpp_orphan_live","display_name":"КПП живой","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	const liveKey = "table.kpp_orphan_live.view"

	// Сирота: entity_id указывает на таблицу, которой в справочнике нет.
	// Идентификатор берём заведомо за пределами занятых, чтобы он не совпал ни с
	// одной существующей строкой.
	var maxTableID int
	require.NoError(t, db.Table("system_tables").
		Select("COALESCE(MAX(id), 0)").Row().Scan(&maxTableID))
	ghostID := maxTableID + 1000

	const orphanKey = "table.kpp_ghost.view"
	require.NoError(t, db.Create(&models.Permission{
		Key:         orphanKey,
		Category:    "table",
		EntityID:    &ghostID,
		DisplayName: "kpp-ghost: Доступ к таблице",
	}).Error)

	// Право без ссылки на таблицу вообще. Автогенерация (AutoGenerateForTable,
	// ReconcileAllTablePermissions) entity_id заполняет всегда, но условие витрины
	// обязано отвечать на NULL явно, а не пропускать его молча в видимые.
	const nullEntityKey = "table.kpp_no_entity.view"
	require.NoError(t, db.Create(&models.Permission{
		Key:         nullEntityKey,
		Category:    "table",
		EntityID:    nil,
		DisplayName: "kpp-no-entity: Доступ к таблице",
	}).Error)

	keys := catalogKeys(t, e, h)
	assert.False(t, keys[orphanKey], "право несуществующей таблицы не должно приходить в каталоге")
	assert.False(t, keys[nullEntityKey], "право без ссылки на таблицу не должно приходить в каталоге")
	assert.True(t, keys[liveKey], "право живой активной таблицы обязано остаться")
	assert.True(t, keys["page.center"], "статический каталог не должен пострадать")

	// Скрытие - витрина, а не модель доступа: строки прав на месте, выдать их
	// по-прежнему можно (редактор шлёт то, что пришло в direct_grants).
	var orphanCount int64
	require.NoError(t, db.Model(&models.Permission{}).
		Where("key IN ?", []string{orphanKey, nullEntityKey}).Count(&orphanCount).Error)
	assert.EqualValues(t, 2, orphanCount, "скрытые права остаются строками в permissions")

	role := models.Role{Code: "role_orphan_perm", Name: "С правом сироты"}
	require.NoError(t, db.Create(&role).Error)
	rec = testutil.PUT(t, e, fmt.Sprintf("/roles/%d/permissions", role.ID),
		fmt.Sprintf(`{"keys":["%s","%s"]}`, orphanKey, liveKey), h)
	require.Equal(t, http.StatusOK, rec.Code, "сохранение роли со скрытым ключом не должно отбиваться")
	assert.True(t, roleHasGrant(t, db, role.ID, orphanKey), "грант скрытого права пережил сохранение роли")
}
