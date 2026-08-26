package handlers_test

// Гвард против регрессии BFLA (OWASP API5): обычный пользователь без админских
// прав не должен ни выгружать карту доступов (группы прав, роли), ни писать в
// админ-справочники. Дыры закрыты навешиванием permission-middleware в роутере;
// этот тест ловит их повторное открытие (забытый middleware на новом роуте).
//
// Параллельно фиксируем инвариант «чтение справочника для формы заявки остаётся
// открытым» — чтобы фикс BFLA случайно не отрезал обычному юзеру дропдауны.

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// writeBlockedForRegularUser — (метод, путь, тело) закрытых для обычного юзера ручек.
var writeBlockedForRegularUser = []struct {
	name, method, path, body string
}{
	{"marks.create", http.MethodPost, "/marks", `{"name":"authz_probe"}`},
	{"marks.bulkArchive", http.MethodPost, "/marks/bulk/archive", `{"ids":[1]}`},
	{"licensePlate.create", http.MethodPost, "/license-plate-formats", `{"name":"authz_probe","pattern":"A000AA00"}`},
	{"unloadPlaces.create", http.MethodPost, "/unload-places", `{"name":"authz_probe"}`},
	// Создание/изменение таблиц КПП — только конструктор (page.admin.tables_constructor).
	{"systemTables.create", http.MethodPost, "/system-tables", `{"name":"authz_probe","display_name":"x","table_type":"cars"}`},
	// Генерация ключей прав таблицы — не должна быть доступна обычному юзеру.
	{"permissions.autoGenerate", http.MethodPost, "/permissions/auto-generate", `{"table_id":1,"table_name":"authz_probe"}`},
	// Пересоздание бланков заявки переписывает файлы на диске сервера.
	{"fileArchive.reexport", http.MethodPost, "/file-archive/applications/1/reexport", `{}`},
	// Бэкфилл ставит в очередь массовую перезапись файлов диапазона заявок.
	{"fileArchive.backfill", http.MethodPost, "/file-archive/backfill", `{}`},
	// Оценка объёма и билет на потоковый ZIP архива (#1615, B3) - тот же раздел, что
	// настройки и пересоздание, обычному юзеру недоступен целиком.
	{"fileArchive.estimate", http.MethodPost, "/file-archive/estimate", `{"date_from":"2026-01-01","date_to":"2026-01-31"}`},
	{"fileArchive.downloadTicket", http.MethodPost, "/file-archive/download-ticket", `{"date_from":"2026-01-01","date_to":"2026-01-31"}`},
}

// readBlockedForRegularUser — списки/карта доступов, которые нельзя выгружать вне контекста.
var readBlockedForRegularUser = []struct {
	name, path string
}{
	{"permissionGroups.list", "/permission-groups"},
	{"roles.list", "/roles"},
	// Чёрные списки: выгрузка ФИО/номеров и причин (ПД) — только под правом.
	// Пометку реестра даёт сервер (is_blacklisted), список ЧС в браузер не идёт.
	{"personBlacklist.list", "/person-blacklist"},
	{"personBlacklist.history", "/person-blacklist/history"},
	{"vehicleBlacklist.list", "/vehicle-blacklist"},
	{"vehicleBlacklist.history", "/vehicle-blacklist/history"},
	// Настройки файлового архива: раскладка каталогов на диске и пороги места.
	{"fileArchive.settings", "/file-archive/settings"},
	{"fileArchive.stats", "/file-archive/stats"},
	// Реестр файлового архива (#1615, B3) - тем же ключом page.admin.file_archive.
	// GET /file-archive/files/:id сюда не входит - в отличие от прочих ручек этого
	// раздела, он зависит от конкретной строки реестра и на пустой базе отдаёт 404
	// даже носителю права, ломая инвариант "админ читает всё" ниже.
	{"fileArchive.items", "/file-archive/items"},
}

// readOpenForRegularUser — справочники, чтение которых нужно форме заявки и должно
// остаться доступным обычному юзеру (регресс-страховка от чрезмерного закрытия).
var readOpenForRegularUser = []string{"/marks", "/license-plate-formats", "/unload-places", "/system-tables"}

func TestAuthz_RegularUser_CannotWriteDirectoriesOrReadPrivileged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Обычный пользователь: type_id=1, без is_admin/is_super_admin и без грантов.
	userH := testutil.AuthHeader(
		testutil.RegisterAndLogin(t, e, "authz_regular", "password123", 1, td.OrgID, td.CompanyID),
	)

	for _, tc := range writeBlockedForRegularUser {
		t.Run("write_blocked/"+tc.name, func(t *testing.T) {
			rec := testutil.POST(t, e, tc.path, tc.body, userH)
			require.Equalf(t, http.StatusForbidden, rec.Code,
				"%s %s должен быть 403 для обычного юзера, получили %d", tc.method, tc.path, rec.Code)
		})
	}

	for _, tc := range readBlockedForRegularUser {
		t.Run("read_blocked/"+tc.name, func(t *testing.T) {
			rec := testutil.GET(t, e, tc.path, userH)
			require.Equalf(t, http.StatusForbidden, rec.Code,
				"GET %s должен быть 403 для обычного юзера, получили %d", tc.path, rec.Code)
		})
	}

	for _, path := range readOpenForRegularUser {
		t.Run("read_open/"+path, func(t *testing.T) {
			rec := testutil.GET(t, e, path, userH)
			require.Equalf(t, http.StatusOK, rec.Code,
				"GET %s нужен форме заявки и должен остаться открытым, получили %d", path, rec.Code)
		})
	}

	// Точечная проверка ЧС остаётся доступной обычному юзеру: форме нужно узнать,
	// не в ЧС ли конкретная машина/человек. Без параметров сервер отвечает 400
	// (валидация), но НЕ 403 — авторизация проходит, в отличие от выгрузки списка.
	for _, path := range []string{"/vehicle-blacklist/check", "/person-blacklist/check"} {
		t.Run("check_open/"+path, func(t *testing.T) {
			rec := testutil.GET(t, e, path, userH)
			require.NotEqualf(t, http.StatusForbidden, rec.Code,
				"GET %s (точечная проверка) должен быть доступен обычному юзеру, получили 403", path)
		})
	}
}

// Снимки версий и корзина таблицы гейтятся per-table правом table.<name>.versions/.trash
// (RequireTableVerb). Обычный юзер без такого права не должен снимать снимок или чистить
// корзину любой таблицы.
func TestAuthz_RegularUser_CannotSnapshotOrTrashTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tableID := seedSystemTable(t, db)
	userH := testutil.AuthHeader(
		testutil.RegisterAndLogin(t, e, "authz_table_regular", "password123", 1, td.OrgID, td.CompanyID),
	)

	snapshot := testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/snapshots", tableID), `{}`, userH)
	require.Equalf(t, http.StatusForbidden, snapshot.Code,
		"снимок версии без права table.*.versions должен быть 403, получили %d", snapshot.Code)

	restore := testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", tableID), `{}`, userH)
	require.Equalf(t, http.StatusForbidden, restore.Code,
		"восстановление из корзины без права table.*.trash должно быть 403, получили %d", restore.Code)

	clear := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/trash", tableID), userH)
	require.Equalf(t, http.StatusForbidden, clear.Code,
		"очистка корзины без права table.*.trash должна быть 403, получили %d", clear.Code)
}

func TestAuthz_Admin_CanWriteAndReadPrivileged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	// Админ читает карту доступов.
	for _, tc := range readBlockedForRegularUser {
		rec := testutil.GET(t, e, tc.path, adminH)
		require.Equalf(t, http.StatusOK, rec.Code, "админ должен читать %s, получили %d", tc.path, rec.Code)
	}

	// Админ создаёт запись справочника (имя уникально — тест-БД общая, дубль даст 409).
	rec := testutil.POST(t, e, "/marks", `{"name":"`+uniqMarkName("authz_admin")+`"}`, adminH)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, rec.Code,
		"админ должен создавать марку, получили %d", rec.Code)
}
