package handlers_test

// Раздел справочников переехал с page.admin на page.admin.directories (#1982).
// Переезд шёл частями, и половина экранов раздела оставалась на старом ключе: роль
// «администратор справочников» открывала раздел, но списки приходили пустыми, а
// действия отвечали отказом. Тест держит три утверждения разом:
//
//   - админ справочников (page.admin.directories, без page.admin и без is_admin)
//     читает и пишет ВО ВСЕ справочники раздела;
//   - без обоих прав любой из этих маршрутов отвечает 403;
//   - действующий администратор (is_admin) доступа не теряет - он проходит не по
//     ключу, а по adminAll в резолвере, и переезд ключа его не касается.
//
// Последний подтест главный: он стережёт от того, что перевод отберёт раздел у тех,
// кто работает в нём сегодня.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// dirRoute - один маршрут раздела. Проверяем не «успех», а отсутствие 403: тело
// запроса намеренно минимальное, и валидация вправе ответить 400 - гейт при этом
// уже пройден, а именно он и есть предмет теста.
type dirRoute struct {
	name   string
	method string
	path   string
	body   string
}

// directoryRoutes - характерные маршруты справочников раздела. Предмет теста не
// отдельный маршрут, а утверждение «раздел закрыт своим правом», поэтому в списке
// обязан быть представлен каждый справочник: выпавший перестаёт стеречься, и переезд
// ключа отберёт его молча.
//
// Берём чтение там, где оно закрыто (типы пользователей, принимающие, документы,
// новости), и запись там, где чтение открыто форме заявки (гражданства, места
// разгрузки).
//
// У организаций и компаний маршрутов несколько, и это намеренно: их состав раньше
// расходился по гейтам. Участники и оба списка с работниками шли без права, а право
// на блокерах архивации обходилось вызовом соседа с тем же ответом (#2002). Дубль
// удалён, состав закрыт, и держим здесь весь набор, чтобы гейт снова не разъехался
// между близнецами. Закрытое чтение у организаций стерегут история и состав, так что
// удаление блокеров эту категорию не оголило.
func directoryRoutes(orgID, companyID, placeID int) []dirRoute {
	return []dirRoute{
		{"типы пользователей: список", http.MethodGet, "/user-types-management", ""},
		{"гражданства: создание", http.MethodPost, "/citizenships", `{"name":"Тестляндия"}`},
		{"организации: история", http.MethodGet, fmt.Sprintf("/organizations/%d/history", orgID), ""},
		{"организации: участники", http.MethodGet, fmt.Sprintf("/organizations/%d/members", orgID), ""},
		{"организации: список с работниками", http.MethodGet, "/organizations/with-users", ""},
		{"организации: расширенный список", http.MethodGet, "/organizations/with-users-extended", ""},
		{"организации: привязка таблиц", http.MethodPut, fmt.Sprintf("/organizations/%d/tables", orgID), `{"table_ids":[]}`},
		{"организации: привязка мест разгрузки", http.MethodPut, fmt.Sprintf("/organizations/%d/unload-places", orgID), `{"unload_place_ids":[]}`},
		{"организации: массовая архивация", http.MethodPost, "/organizations/bulk/archive", `{"ids":[]}`},
		{"компании: история", http.MethodGet, fmt.Sprintf("/companies/%d/history", companyID), ""},
		{"компании: участники", http.MethodGet, fmt.Sprintf("/companies/%d/members", companyID), ""},
		{"компании: список с работниками", http.MethodGet, "/companies/with-users", ""},
		{"компании: расширенный список", http.MethodGet, "/companies/with-users-extended", ""},
		{"компании: привязка таблиц", http.MethodPut, fmt.Sprintf("/companies/%d/tables", companyID), `{"table_ids":[]}`},
		{"компании: массовая архивация", http.MethodPost, "/companies/bulk/archive", `{"ids":[]}`},
		{"места разгрузки: отвязать всё", http.MethodPost, fmt.Sprintf("/unload-places/%d/detach-all", placeID), ""},
		{"принимающие: состав", http.MethodGet, "/application-approvers", ""},
		{"принимающие: кандидаты", http.MethodGet, "/application-approvers/available-users", ""},
		{"новости: полный список", http.MethodGet, "/news/all", ""},
		{"объявления: полный список", http.MethodGet, "/announcements/all", ""},
		{"документы: список", http.MethodGet, "/documents", ""},
		{"группы документов: список", http.MethodGet, "/document-groups", ""},
	}
}

// callDirRoute шлёт запрос выбранным методом и возвращает код ответа.
func callDirRoute(t *testing.T, e *echo.Echo, r dirRoute, token string) int {
	t.Helper()
	h := testutil.AuthHeader(token)
	switch r.method {
	case http.MethodGet:
		return testutil.GET(t, e, r.path, h).Code
	case http.MethodPost:
		return testutil.POST(t, e, r.path, r.body, h).Code
	case http.MethodPut:
		return testutil.PUT(t, e, r.path, r.body, h).Code
	case http.MethodDelete:
		return testutil.DELETE(t, e, r.path, h).Code
	default:
		t.Fatalf("неизвестный метод %s", r.method)
		return 0
	}
}

// seedDirectoryFixtures кладёт место разгрузки, на котором проверяется отвязка.
func seedDirectoryFixtures(t *testing.T, db *gorm.DB) int {
	t.Helper()
	place := models.UnloadPlace{Name: fmt.Sprintf("Площадка %d", time.Now().UnixNano()%1_000_000_000)}
	require.NoError(t, db.Create(&place).Error)
	return place.ID
}

func TestDirectories_MigratedToDirectoriesPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	placeID := seedDirectoryFixtures(t, db)
	routes := directoryRoutes(td.OrgID, td.CompanyID, placeID)

	testutil.RegisterUser(t, e, "dirmig_plain", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dirmig_plain")

	testutil.RegisterUser(t, e, "dirmig_dir", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dirmig_dir")
	grantPermission(t, db, "dirmig_dir", "page.admin.directories")

	testutil.RegisterUser(t, e, "dirmig_admin", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dirmig_admin")
	require.NoError(t, db.Table("users").Where("username = ?", "dirmig_admin").
		Update("is_admin", true).Error)

	t.Run("админ справочников проходит во все справочники раздела", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "dirmig_dir", "password123")
		for _, r := range routes {
			code := callDirRoute(t, e, r, token)
			assert.NotEqual(t, http.StatusForbidden, code,
				"%s: page.admin.directories должно открывать маршрут, получили 403", r.name)
		}
	})

	t.Run("без прав раздела везде отказ", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "dirmig_plain", "password123")
		for _, r := range routes {
			code := callDirRoute(t, e, r, token)
			assert.Equal(t, http.StatusForbidden, code,
				"%s: без права раздела ожидали 403", r.name)
		}
	})

	// Главный подтест переезда: действующий администратор ходит по флагу is_admin
	// (adminAll в резолвере), а не по ключу page.admin, поэтому смена ключа на
	// маршрутах его доступа не касается.
	t.Run("действующий администратор доступ не теряет", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "dirmig_admin", "password123")
		for _, r := range routes {
			code := callDirRoute(t, e, r, token)
			assert.NotEqual(t, http.StatusForbidden, code,
				"%s: администратор (is_admin) не должен терять доступ, получили 403", r.name)
		}
	})
}
