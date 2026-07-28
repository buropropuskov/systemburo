package handlers_test

// Гвард против регрессии BFLA (OWASP API5): обычный пользователь без админских
// прав не должен ни выгружать карту доступов (группы прав, роли), ни писать в
// админ-справочники. Дыры закрыты навешиванием permission-middleware в роутере;
// этот тест ловит их повторное открытие (забытый middleware на новом роуте).
//
// Параллельно фиксируем инвариант «чтение справочника для формы заявки остаётся
// открытым» — чтобы фикс BFLA случайно не отрезал обычному юзеру дропдауны.

import (
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
}

// readBlockedForRegularUser — списки/карта доступов, которые нельзя выгружать вне контекста.
var readBlockedForRegularUser = []struct {
	name, path string
}{
	{"permissionGroups.list", "/permission-groups"},
	{"roles.list", "/roles"},
}

// readOpenForRegularUser — справочники, чтение которых нужно форме заявки и должно
// остаться доступным обычному юзеру (регресс-страховка от чрезмерного закрытия).
var readOpenForRegularUser = []string{"/marks", "/license-plate-formats", "/unload-places"}

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
