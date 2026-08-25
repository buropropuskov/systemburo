package handlers_test

// Гвард сквозного поиска. Поиск ходит сразу по нескольким разделам, поэтому забытое
// сужение выборки в одном провайдере открывает данные шире, чем листинг того же раздела
// -- ровно класс #1524/#1528/#1530/#1531, только с широкой поверхностью. Проверяем три
// слоя по отдельности: раздел без права не виден вовсе, строки внутри раздела сужены по
// владельцу, персональные данные не утекают в выдачу.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type searchResponseBody struct {
	Success bool `json:"success"`
	Data    struct {
		Query  string `json:"query"`
		Groups []struct {
			Type  string `json:"type"`
			Count int    `json:"count"`
			Items []struct {
				ID       int    `json:"id"`
				Title    string `json:"title"`
				Subtitle string `json:"subtitle"`
			} `json:"items"`
		} `json:"groups"`
		Total int `json:"total"`
	} `json:"data"`
}

func decodeSearch(t *testing.T, body string) searchResponseBody {
	t.Helper()
	var out searchResponseBody
	require.NoError(t, json.Unmarshal([]byte(body), &out), "тело ответа: %s", body)
	return out
}

// groupByType возвращает группу раздела и признак её присутствия. Отсутствие группы и
// пустая группа -- разные вещи: пустая сама по себе сообщает, что такой раздел в системе
// есть, поэтому провайдер без права не должен давать даже её.
func groupByType(resp searchResponseBody, typ string) (int, bool) {
	for _, g := range resp.Data.Groups {
		if g.Type == typ {
			return g.Count, true
		}
	}
	return 0, false
}

func searchStrPtr(s string) *string { return &s }

// assignBaseRole выдаёт пользователю базовую роль "Пользователь". Регистрация роль не
// проставляет (её назначает администратор в карточке, а старым учётным записям --
// BackfillBaseRole), поэтому без этого шага пользователь приходит в поиск вообще без
// прав и проверка видимости строк ничего не проверяет: разделов в ответе просто нет.
func assignBaseRole(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	var role models.Role
	require.NoError(t, db.Where("code = ?", "user").First(&role).Error)
	require.NoError(t, db.Table("users").Where("username = ?", username).Update("role_id", role.ID).Error)
}

// seedSearchEmployee кладёт сотрудника напрямую в реестр: через API пришлось бы тащить
// целую заявку, а проверяем мы видимость, а не создание.
// userIDByName живёт в pd_consent_gate_middleware_test.go -- пакет тестов общий.
func seedSearchEmployee(t *testing.T, db *gorm.DB, lastName string, userID, orgID int, passport string) int {
	t.Helper()
	emp := models.UniqueEmployee{
		LastName:             searchStrPtr(lastName),
		FirstName:            searchStrPtr("Иван"),
		MiddleName:           searchStrPtr("Петрович"),
		Position:             searchStrPtr("водитель"),
		UserID:               &userID,
		OrganizationID:       &orgID,
		PassportSeriesNumber: searchStrPtr(passport),
	}
	require.NoError(t, db.Create(&emp).Error)
	return emp.ID
}

// Обычный пользователь находит своих сотрудников и не находит чужих. Это и есть третий
// слой защиты: право entity.employees.read у базовой роли есть у всех, и если провайдер
// забудет сузить выборку, поиск начнёт отдавать весь реестр системы.
func TestSearch_RegularUser_SeesOnlyOwnRows(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	otherOrg := models.Organization{Name: "Чужая организация"}
	require.NoError(t, db.Create(&otherOrg).Error)

	testutil.RegisterUser(t, e, "search_owner", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "search_owner")
	testutil.RegisterUser(t, e, "search_stranger", "password123", 1, otherOrg.ID, td.CompanyID)
	assignBaseRole(t, db, "search_stranger")

	ownerID := userIDByName(t, db, "search_owner")
	strangerID := userIDByName(t, db, "search_stranger")

	seedSearchEmployee(t, db, "Роголев", ownerID, td.OrgID, "4510 111111")
	seedSearchEmployee(t, db, "Роголев", strangerID, otherOrg.ID, "4510 222222")

	token, _ := testutil.LoginUser(t, e, "search_owner", "password123")
	rec := testutil.GET(t, e, "/search?q=Роголев", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	resp := decodeSearch(t, rec.Body.String())
	count, found := groupByType(resp, "employees")
	require.True(t, found, "своего сотрудника поиск обязан находить: %s", rec.Body.String())
	assert.Equal(t, 1, count, "видно должно быть только своего сотрудника, чужой из другой организации в выдачу не идёт")
}

// Администратор видит системный срез -- поиск не шире и не уже листинга реестра.
func TestSearch_Admin_SeesOtherOrgRows(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	otherOrg := models.Organization{Name: "Чужая организация"}
	require.NoError(t, db.Create(&otherOrg).Error)

	testutil.RegisterUser(t, e, "search_admin", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "search_foreign", "password123", 1, otherOrg.ID, td.CompanyID)
	assignBaseRole(t, db, "search_foreign")
	require.NoError(t, db.Table("users").Where("username = ?", "search_admin").Update("is_admin", true).Error)

	foreignID := userIDByName(t, db, "search_foreign")
	seedSearchEmployee(t, db, "Роголев", foreignID, otherOrg.ID, "4510 333333")

	token, _ := testutil.LoginUser(t, e, "search_admin", "password123")
	rec := testutil.GET(t, e, "/search?q=Роголев", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	count, found := groupByType(decodeSearch(t, rec.Body.String()), "employees")
	require.True(t, found)
	assert.Equal(t, 1, count, "администратор видит записи чужой организации")
}

// Раздел, на который нет права, отсутствует в ответе целиком. Пустая группа тут была бы
// хуже: она подтверждает существование раздела тому, кому его не показывают.
func TestSearch_NoPermission_GroupAbsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "search_nogrant", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "search_nogrant")
	userID := userIDByName(t, db, "search_nogrant")
	seedSearchEmployee(t, db, "Роголев", userID, td.OrgID, "4510 444444")

	// Личный запрет перекрывает грант базовой роли -- это тот же путь, которым права
	// снимают у конкретного человека в интерфейсе.
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: "entity.employees.read",
		Value:         "deny",
	}).Error)

	token, _ := testutil.LoginUser(t, e, "search_nogrant", "password123")
	rec := testutil.GET(t, e, "/search?q=Роголев", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	_, found := groupByType(decodeSearch(t, rec.Body.String()), "employees")
	assert.False(t, found, "без права раздел не должен появляться даже пустым: %s", rec.Body.String())
}

// Забаненному отвечаем отказом, а не пустой выдачей: пустая читается как "ничего не
// нашлось" и скрывает блокировку.
func TestSearch_BannedUser_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "search_banned", "password123", 1, td.OrgID, td.CompanyID)
	token, _ := testutil.LoginUser(t, e, "search_banned", "password123")
	require.NoError(t, db.Table("users").Where("username = ?", "search_banned").Update("is_banned", true).Error)

	rec := testutil.GET(t, e, "/search?q=Роголев", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "тело: %s", rec.Body.String())
}

// Паспорт и патент не должны попадать в выдачу ни в каком виде. Проверяем грепом по
// сырому телу, а не по разобранной структуре: так тест поймает и поле, добавленное в
// ответ по недосмотру.
func TestSearch_NoPersonalDataLeak(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "search_pd", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "search_pd")
	userID := userIDByName(t, db, "search_pd")
	const passport = "4510987654"
	seedSearchEmployee(t, db, "Роголев", userID, td.OrgID, passport)

	token, _ := testutil.LoginUser(t, e, "search_pd", "password123")
	rec := testutil.GET(t, e, "/search?q=Роголев", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	require.Contains(t, body, "Роголев", "сотрудник должен находиться, иначе проверка ничего не стоит")
	assert.NotContains(t, body, passport, "паспорт не должен попадать в выдачу поиска")
	assert.False(t, strings.Contains(body, "passport"), "в ответе не должно быть паспортных полей: %s", body)
}

// Порог в три символа -- условие работоспособности, а не косметика: более короткий
// шаблон не покрывается индексом и уводит запрос в полный скан по всем разделам сразу.
func TestSearch_ShortQuery_BadRequest(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "search_short", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "search_short")
	token, _ := testutil.LoginUser(t, e, "search_short", "password123")
	h := testutil.AuthHeader(token)

	for _, q := range []string{"", "ро"} {
		rec := testutil.GET(t, e, "/search?q="+q, h)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "запрос %q должен отклоняться", q)
	}
}

// Опечатка в коде раздела -- ошибка запроса, а не молчаливо пустая выдача: иначе
// неверный параметр выглядит как "в этом разделе ничего нет".
func TestSearch_UnknownType_BadRequest(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "search_type", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "search_type")
	token, _ := testutil.LoginUser(t, e, "search_type", "password123")

	rec := testutil.GET(t, e, "/search?q=Роголев&types=employeez", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "тело: %s", rec.Body.String())
}

// Без токена поиск недоступен: он ходит по данным всех разделов сразу.
func TestSearch_Anonymous_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/search?q=Роголев", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
