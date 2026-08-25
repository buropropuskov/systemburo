package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniqueEmployees_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/unique-employees", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// active_application_id связывает строку реестра сотрудников с id активной заявки.
// Без него вкладка "Сотрудники" не может открыть заявку (кнопка "Открыть заявку").
// Реестр и заявочный сотрудник связаны по passport_series_number_hmac, поэтому
// реестровую запись активного сотрудника заводим с тем же паспортом, что и в seed-заявке.
func TestUniqueEmployees_ActiveApplicationIDForActiveApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Паспорт совпадает с сотрудником из seed-заявки ("1234 567890") - так подзапрос
	// активной заявки сматчится по passport_series_number_hmac.
	active := fmt.Sprintf(`{"pd_consent":true,"last_name":"RegActive","first_name":"A","passport_series_number":"1234 567890","organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
	testutil.POST(t, e, "/unique-employees", active, h)

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system", h)
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)

	var found map[string]interface{}
	for _, r := range rows {
		if n, _ := r["last_name"].(string); n == "RegActive" {
			found = r
			break
		}
	}
	require.NotNil(t, found, "реестровый сотрудник RegActive должен быть в списке")
	assert.Equal(t, true, found["status"], "у сотрудника с активной заявкой status=true")
	require.NotNil(t, found["active_employee_id"], "active_employee_id заполнен для активного сотрудника")
	assert.Equal(t, float64(empID), found["active_employee_id"], "active_employee_id = id активной заявочной строки")
	require.NotNil(t, found["active_application_id"], "active_application_id заполнен для активного сотрудника")
	assert.Equal(t, float64(appID), found["active_application_id"], "active_application_id = id активной заявки")

	// Сотрудник без активной заявки: active_* пустые (фронт прячет кнопку "Открыть заявку").
	idle := fmt.Sprintf(`{"pd_consent":true,"last_name":"RegIdle","first_name":"B","passport_series_number":"0000 000111","organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
	testutil.POST(t, e, "/unique-employees", idle, h)
	rec = testutil.GET(t, e, "/unique-employees?filter_type=all_system", h)
	require.Equal(t, http.StatusOK, rec.Code)
	rows = testutil.ParseSlice(t, rec)
	var idleRow map[string]interface{}
	for _, r := range rows {
		if n, _ := r["last_name"].(string); n == "RegIdle" {
			idleRow = r
			break
		}
	}
	require.NotNil(t, idleRow, "сотрудник без заявки должен быть в реестре")
	assert.Equal(t, false, idleRow["status"], "без активной заявки status=false")
	assert.Nil(t, idleRow["active_employee_id"], "без активной заявки active_employee_id пустой")
	assert.Nil(t, idleRow["active_application_id"], "без активной заявки active_application_id пустой")
}

func TestUniqueEmployees_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create
	body := fmt.Sprintf(`{
		"pd_consent":true,
		"last_name":"Ivanov",
		"first_name":"Ivan",
		"middle_name":"Ivanovich",
		"passport_series_number":"1234 567890",
		"position":"Engineer",
		"organization_id":%d,
		"company_id":%d
	}`, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.NotNil(t, createResp["id"])
	empID := int(createResp["id"].(float64))
	assert.Greater(t, empID, 0)
	assert.Equal(t, "Ivanov", createResp["last_name"])
	assert.Equal(t, "Ivan", createResp["first_name"])
	assert.Equal(t, false, createResp["status"])

	// Get all (default filter_type=user)
	rec = testutil.GET(t, e, "/unique-employees", h)
	require.Equal(t, http.StatusOK, rec.Code)

	listResp := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(listResp), 1)

	// Update
	updateBody := fmt.Sprintf(`{
		"last_name":"Petrov",
		"first_name":"Petr",
		"passport_series_number":"9999 111111",
		"organization_id":%d,
		"company_id":%d
	}`, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", empID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	updateResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Petrov", updateResp["last_name"])
	assert.Equal(t, "Petr", updateResp["first_name"])

	// Delete
	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-employees/%d", empID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, "Employee deleted successfully", testutil.ParseMessage(t, rec))
}

func TestUniqueEmployees_DuplicatePassport(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"pd_consent":true,"last_name":"Dup","first_name":"Test","passport_series_number":"DUP 123456"}`
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Same passport = duplicate
	rec = testutil.POST(t, e, "/unique-employees", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUniqueEmployees_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/unique-employees/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUniqueEmployees_GetOwnershipInfo(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/unique-employees/ownership-info", h)
	require.Equal(t, http.StatusOK, rec.Code)

	info := testutil.ParseMap(t, rec)
	assert.Contains(t, info, "has_organization")
	assert.Contains(t, info, "has_company")
	assert.Contains(t, info, "user_id")
	assert.Contains(t, info, "organization_id")
	assert.Contains(t, info, "company_id")
}

func TestUniqueEmployees_FilterTypes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create an employee
	body := fmt.Sprintf(`{"pd_consent":true,"last_name":"Filter","first_name":"Test","organization_id":%d}`, td.OrgID)
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	filters := []string{"user", "organization", "company", "all"}
	for _, f := range filters {
		rec = testutil.GET(t, e, "/unique-employees?filter_type="+f, h)
		assert.Equal(t, http.StatusOK, rec.Code, "filter_type=%s should return 200", f)
	}
}

func TestUniqueEmployees_CreateWithoutPassport(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Should work without passport (no uniqueness check triggered)
	body := `{"pd_consent":true,"last_name":"NoPassport","first_name":"Worker"}`
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUniqueEmployees_Update_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/unique-employees/99999", `{"pd_consent":true,"last_name":"X"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUniqueEmployees_Lookup(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := fmt.Sprintf(`{"pd_consent":true,"last_name":"Сидоров","first_name":"Семён","middle_name":"Семёнович","passport_series_number":"9999 888777","organization_id":%d}`, td.OrgID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", body, h).Code)

	t.Run("находит по ФИО без учёта регистра/пробелов", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-employees/lookup?last_name=%20сидоров%20&first_name=семён&middle_name=Семёнович", h)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		resp := testutil.ParseMap(t, rec)
		assert.Equal(t, "Сидоров", resp["last_name"])
		assert.Greater(t, int(resp["id"].(float64)), 0)
	})

	t.Run("404 при несовпадении отчества", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-employees/lookup?last_name=Сидоров&first_name=Семён&middle_name=Петрович", h)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("400 без имени", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-employees/lookup?last_name=Сидоров", h)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("400 без фамилии", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-employees/lookup?first_name=Семён", h)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestUniqueEmployees_Paginated проверяет серверную пагинацию реестра (#1158, срез 3):
// per_page переключает GetAll на GetAllPaginated, meta.total считает все совпадения,
// не размер страницы (secMetaEnvelope переиспользован из security_attachments_test.go,
// тот же пакет handlers_test).
func TestUniqueEmployees_Paginated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	for i, ln := range []string{"Pgn1", "Pgn2", "Pgn3"} {
		body := fmt.Sprintf(`{"pd_consent":true,"last_name":"%s","first_name":"F%d"}`, ln, i)
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", body, h).Code)
	}

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=1&page=1", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	require.Len(t, rows, 1, "страница ограничена per_page")

	var env secMetaEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), rec.Body.String())
	assert.GreaterOrEqual(t, env.Meta.Total, int64(3), "total считает все совпадения, не размер страницы")
	assert.Equal(t, 1, env.Meta.Page)
	assert.Equal(t, 1, env.Meta.PerPage)
}

// TestUniqueEmployees_SearchQuery_ExactMatch проверяет серверный поиск по фамилии:
// точное совпадение находит нужного сотрудника среди прочих (не просто 200 - #46).
func TestUniqueEmployees_SearchQuery_ExactMatch(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Срхтестовый","first_name":"Иван"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Другойчел","first_name":"Пётр"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=20&search_query=Срхтестовый", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	require.Len(t, rows, 1, "поиск должен вернуть только совпавшего сотрудника")
	require.NotNil(t, rows[0].LastName)
	assert.Equal(t, "Срхтестовый", *rows[0].LastName)
}

// TestUniqueEmployees_SearchQuery_TypoVariant проверяет нечёткий поиск ФИО через
// strict_word_similarity (тот же приём, что использует Центр заявок для поиска
// сотрудников по опечаткам в фамилии) - опечатка в одну букву всё равно находит запись.
func TestUniqueEmployees_SearchQuery_TypoVariant(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Карбышев","first_name":"Дмитрий"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=20&search_query=Карбышоф", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	require.Len(t, rows, 1, "опечатка в фамилии должна находить сотрудника через strict_word_similarity")
	require.NotNil(t, rows[0].LastName)
	assert.Equal(t, "Карбышев", *rows[0].LastName)
}

// TestUniqueEmployees_SearchQuery_NoMatch проверяет, что несуществующий запрос честно
// отдаёт пустой список, а не 500 (ловит несуществующие колонки/синтаксис - #46).
func TestUniqueEmployees_SearchQuery_NoMatch(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Уникум","first_name":"Иван"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=20&search_query=совершенно-другой-запрос-zzz", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	assert.Empty(t, rows)
}

// TestUniqueEmployees_SearchQuery_PassportNotSearchable документирует известное
// ограничение (#1158, срез 3): паспорт/патент зашифрованы (HMAC), ILIKE по ним не
// работает - поиск по номеру паспорта не находит сотрудника ни по какому полю, кроме
// точного совпадения полей, входящих в поиск (ФИО/должность/организация/компания/
// гражданство). Тест фиксирует текущее поведение, а не 500.
func TestUniqueEmployees_SearchQuery_PassportNotSearchable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Паспортов","first_name":"Олег","passport_series_number":"7777 654321"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=20&search_query=7777654321", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	assert.Empty(t, rows, "поиск по номеру паспорта не находит сотрудника - паспорт зашифрован")
}

// TestUniqueEmployees_SearchQuery_NoCrossOwnerLeak - регресс-замок против будущего
// рефакторинга поиска (#1158, срез 3). Изоляция видимости при поиске держится
// ИСКЛЮЧИТЕЛЬНО на том, что owner-фильтр (ue.user_id = ...) и поисковый OR-блок
// (ILIKE ... OR strict_word_similarity ...) - два ОТДЕЛЬНЫХ .Where() в
// buildEmployeesQuery, а GORM оборачивает каждый в скобки: (owner) AND (search).
// Если кто-то сольёт их в одну строку `.Where(owner+" AND "+search)`, приоритет
// AND над OR даст `(owner AND ilike) OR strict_sim` - и strict_sim-ветка перестанет
// быть ограничена владельцем -> утечка чужих записей через поиск, тихо и без падения.
// Тест: владелец A под filter_type=user (видит только своих) ищет фамилию сотрудника
// владельца B -> ожидаем 0 (не находит чужого). Контроль: свою фамилию находит.
func TestUniqueEmployees_SearchQuery_NoCrossOwnerLeak(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Владелец A (админ, организация из seed).
	tokenA := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	hA := testutil.AuthHeader(tokenA)

	// Владелец B - отдельный пользователь ДРУГОЙ организации (иной user_id).
	orgB := models.Organization{Name: "Isolation Org B"}
	require.NoError(t, db.Create(&orgB).Error, "seed org B")
	tokenB := testutil.RegisterManager(t, e, "ownerb_iso", orgB.ID, 0)
	hB := testutil.AuthHeader(tokenB)

	// Каждый заводит своего сотрудника с УНИКАЛЬНОЙ фамилией.
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Иванцевич","first_name":"А"}`, hA).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Богуславский","first_name":"Б"}`, hB).Code)

	// A под filter_type=user ищет фамилию B -> НЕ находит (owner-scope не течёт через OR).
	rec := testutil.GET(t, e, "/unique-employees?filter_type=user&per_page=50&search_query=Богуславский", hA)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	assert.Empty(t, rows, "владелец A не должен находить сотрудника владельца B через поиск")

	// Контроль: A ищет СВОЮ фамилию -> находит свою запись (поиск работает, режется только чужое).
	rec = testutil.GET(t, e, "/unique-employees?filter_type=user&per_page=50&search_query=Иванцевич", hA)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	require.Len(t, rows, 1, "владелец A находит свою запись по своей фамилии")
	require.NotNil(t, rows[0].LastName)
	assert.Equal(t, "Иванцевич", *rows[0].LastName)
}
