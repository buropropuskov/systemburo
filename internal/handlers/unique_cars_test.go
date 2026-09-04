package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniqueCars_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/unique-cars", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUniqueCars_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create
	body := fmt.Sprintf(`{"number":"A123BC","mark":"Toyota","organization_id":%d,"company_id":%d}`,
		td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/unique-cars", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.NotNil(t, createResp["id"])
	carID := int(createResp["id"].(float64))
	assert.Greater(t, carID, 0)
	assert.Equal(t, "A123BC", createResp["number"])
	assert.Equal(t, "Toyota", createResp["mark"])
	assert.Equal(t, false, createResp["status"])

	// Get all (default filter_type=user)
	rec = testutil.GET(t, e, "/unique-cars", h)
	require.Equal(t, http.StatusOK, rec.Code)

	listResp := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(listResp), 1)

	// Get all with filter_type=all_system
	rec = testutil.GET(t, e, "/unique-cars?filter_type=all_system", h)
	require.Equal(t, http.StatusOK, rec.Code)
	listResp = testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(listResp), 1)

	// Update by ID
	updateBody := fmt.Sprintf(`{"number":"B456DE","mark":"Honda","organization_id":%d,"company_id":%d}`,
		td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-cars/%d", carID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	updateResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "B456DE", updateResp["number"])
	assert.Equal(t, "Honda", updateResp["mark"])

	// Delete
	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-cars/%d", carID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, "Car deleted successfully", testutil.ParseMessage(t, rec))
}

// active_car_id связывает строку реестра с id активной заявочной машины (cars.id).
// Без него фронт вкладки "Автомобили" не может подтянуть статус территории и места
// разгрузки - они ключуются по cars.id, а реестр оперирует unique_cars.id.
func TestUniqueCars_ActiveCarIDForActiveApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Реестр должен содержать машину с тем же номером, чтобы подзапрос активной заявки сматчился.
	testutil.POST(t, e, "/unique-cars", `{"number":"B002BB799","mark":"Kamaz"}`, h)

	rec := testutil.GET(t, e, "/unique-cars?filter_type=all_system", h)
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)

	var found map[string]interface{}
	for _, r := range rows {
		if n, _ := r["number"].(string); n == "B002BB799" {
			found = r
			break
		}
	}
	require.NotNil(t, found, "уникальная машина B002BB799 должна быть в списке реестра")
	assert.Equal(t, true, found["status"], "у машины с активной заявкой status=true")
	require.NotNil(t, found["active_car_id"], "active_car_id должен быть заполнен для активной машины")
	assert.Equal(t, float64(carID), found["active_car_id"], "active_car_id = id активной заявочной машины")
	// active_application_id - id самой заявки той же активной строки; по нему вкладка
	// "Автомобили" открывает заявку (кнопка "Открыть заявку").
	require.NotNil(t, found["active_application_id"], "active_application_id заполнен для активной машины")
	assert.Equal(t, float64(appID), found["active_application_id"], "active_application_id = id активной заявки")

	// Машина в реестре без активной заявки: active_car_id пустой (фронт полагается на это,
	// чтобы не дёргать статус/места по чужому id).
	testutil.POST(t, e, "/unique-cars", `{"number":"NOAPP777","mark":"Lada"}`, h)
	rec = testutil.GET(t, e, "/unique-cars?filter_type=all_system", h)
	require.Equal(t, http.StatusOK, rec.Code)
	rows = testutil.ParseSlice(t, rec)
	var idle map[string]interface{}
	for _, r := range rows {
		if n, _ := r["number"].(string); n == "NOAPP777" {
			idle = r
			break
		}
	}
	require.NotNil(t, idle, "машина без заявки должна быть в реестре")
	assert.Equal(t, false, idle["status"], "без активной заявки status=false")
	assert.Nil(t, idle["active_car_id"], "без активной заявки active_car_id пустой")
	assert.Nil(t, idle["active_application_id"], "без активной заявки active_application_id пустой")
}

// Реестр гасит признак действующей заявки, когда прошло крайнее время пребывания
// последнего дня, и считает этот момент по Москве (#2327).
//
// Признак читают на вкладке «Автомобили», по нему же берут active_car_id для
// статуса и мест. Сравнение только по дате держало машину «действующей» до конца
// суток, а из-за UTC-даты сессии - ещё три часа сверху.
func TestUniqueCars_StatusOffAfterPassHoursEnded(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	appID, attID, _ := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	testutil.POST(t, e, "/unique-cars", `{"number":"B002BB799","mark":"Kamaz"}`, h)

	msk := time.Now().In(time.FixedZone("MSK", 3*60*60)).Add(-time.Minute)
	require.NoError(t, db.Exec(
		`UPDATE attachments SET entry_date_to = ?, entry_time_to = ? WHERE id = ?`,
		msk.Format("2006-01-02"), msk.Format("15:04:05"), attID,
	).Error)

	rec := testutil.GET(t, e, "/unique-cars?filter_type=all_system", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var found map[string]interface{}
	for _, r := range testutil.ParseSlice(t, rec) {
		if n, _ := r["number"].(string); n == "B002BB799" {
			found = r
			break
		}
	}
	require.NotNil(t, found, "машина должна остаться в реестре")
	assert.Equal(t, false, found["status"],
		"время пребывания кончилось минуту назад по Москве, но заявка всё ещё числится действующей")
	assert.Nil(t, found["active_car_id"], "у недействующей заявки active_car_id не заполняется")
}

func TestUniqueCars_DuplicateCreate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"number":"DUP111","mark":"BMW"}`
	rec := testutil.POST(t, e, "/unique-cars", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Same number + mark = duplicate
	rec = testutil.POST(t, e, "/unique-cars", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUniqueCars_BatchCreate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `[
		{"number":"BATCH1","mark":"Audi"},
		{"number":"BATCH2","mark":"Mercedes"}
	]`
	rec := testutil.POST(t, e, "/unique-cars/batch", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	batchResp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(2), batchResp["success_count"])
	assert.Equal(t, float64(0), batchResp["error_count"])

	createdCars := batchResp["created_cars"].([]interface{})
	assert.Len(t, createdCars, 2)
}

func TestUniqueCars_BatchCreate_PartialDuplicate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create one car first
	rec := testutil.POST(t, e, "/unique-cars", `{"number":"EXISTS","mark":"Ford"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Batch with one existing and one new
	body := `[
		{"number":"EXISTS","mark":"Ford"},
		{"number":"NEW123","mark":"Kia"}
	]`
	rec = testutil.POST(t, e, "/unique-cars/batch", body, h)
	assert.Equal(t, http.StatusMultiStatus, rec.Code)

	batchResp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(1), batchResp["success_count"])
	assert.Equal(t, float64(1), batchResp["error_count"])
}

func TestUniqueCars_UpdateByNumber(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create car
	rec := testutil.POST(t, e, "/unique-cars", `{"number":"UBN001","mark":"Volvo"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Update by number
	body := `{"number":"UBN001","mark":"Volvo","update_data":{"number":"UBN002","mark":"Saab"}}`
	rec = testutil.PUT(t, e, "/unique-cars/by-number", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, "UBN002", resp["number"])
	assert.Equal(t, "Saab", resp["mark"])
}

func TestUniqueCars_UpdateByNumber_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"number":"NOCAR","mark":"None","update_data":{"number":"X","mark":"Y"}}`
	rec := testutil.PUT(t, e, "/unique-cars/by-number", body, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUniqueCars_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/unique-cars/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUniqueCars_GetOwnershipInfo(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/unique-cars/ownership-info", h)
	require.Equal(t, http.StatusOK, rec.Code)

	info := testutil.ParseMap(t, rec)
	assert.Contains(t, info, "has_organization")
	assert.Contains(t, info, "has_company")
	assert.Contains(t, info, "user_id")
	assert.Contains(t, info, "organization_id")
	assert.Contains(t, info, "company_id")
}

func TestUniqueCars_FilterTypes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create a car
	body := fmt.Sprintf(`{"number":"FLT001","mark":"Mazda","organization_id":%d}`, td.OrgID)
	rec := testutil.POST(t, e, "/unique-cars", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	filters := []string{"user", "organization", "company", "all", "all_system"}
	for _, f := range filters {
		rec = testutil.GET(t, e, "/unique-cars?filter_type="+f, h)
		assert.Equal(t, http.StatusOK, rec.Code, "filter_type=%s should return 200", f)
	}
}

// TestUniqueCars_Paginated проверяет серверную пагинацию реестра (#1158, срез 2):
// per_page переключает GetAll на GetAllPaginated, meta.total считает все совпадения,
// не размер страницы (secMetaEnvelope переиспользован из security_attachments_test.go,
// тот же пакет handlers_test).
func TestUniqueCars_Paginated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	for i, num := range []string{"PGN001", "PGN002", "PGN003"} {
		body := fmt.Sprintf(`{"number":"%s","mark":"PgMark%d"}`, num, i)
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", body, h).Code)
	}

	rec := testutil.GET(t, e, "/unique-cars?filter_type=all_system&per_page=1&page=1", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	require.Len(t, rows, 1, "страница ограничена per_page")

	var env secMetaEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), rec.Body.String())
	assert.GreaterOrEqual(t, env.Meta.Total, int64(3), "total считает все совпадения, не размер страницы")
	assert.Equal(t, 1, env.Meta.Page)
	assert.Equal(t, 1, env.Meta.PerPage)
}

// TestUniqueCars_SearchQuery_ExactMatch проверяет серверный поиск по номеру: точное
// совпадение находит нужную машину среди прочих (не просто 200, реально фильтрует - #46).
func TestUniqueCars_SearchQuery_ExactMatch(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"SRCH777AA","mark":"Kamaz"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"OTHER888BB","mark":"Volvo"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-cars?filter_type=all_system&per_page=20&search_query=SRCH777AA", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	require.Len(t, rows, 1, "поиск должен вернуть только совпавшую машину")
	require.NotNil(t, rows[0].Number)
	assert.Equal(t, "SRCH777AA", *rows[0].Number)
}

// TestUniqueCars_SearchQuery_SpaceVariant проверяет вариант поиска номера без пробелов
// (REPLACE убирает пробелы из номера перед ILIKE, тот же приём, что применяется в
// поиске заявок application_helpers.go) - номер хранится с пробелом, ищем слитно.
func TestUniqueCars_SearchQuery_SpaceVariant(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"У 777 УУ 799","mark":"Lada"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-cars?filter_type=all_system&per_page=20&search_query=У777УУ799", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	require.Len(t, rows, 1, "поиск слитным номером должен находить машину с пробелами в номере")
	require.NotNil(t, rows[0].Number)
	assert.Equal(t, "У 777 УУ 799", *rows[0].Number)
}

// TestUniqueCars_SearchQuery_NoMatch проверяет, что несуществующий запрос честно
// отдаёт пустой список, а не 500 (ловит несуществующие колонки/синтаксис - #46).
func TestUniqueCars_SearchQuery_NoMatch(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"NOMATCH001","mark":"Kia"}`, h).Code)

	rec := testutil.GET(t, e, "/unique-cars?filter_type=all_system&per_page=20&search_query=совершенно-другой-запрос-zzz", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	assert.Empty(t, rows)
}

func TestUniqueCars_Lookup(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := fmt.Sprintf(`{"number":"Х999ХХ77","mark":"Lada","organization_id":%d}`, td.OrgID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", body, h).Code)

	t.Run("находит по номеру и марке без учёта регистра/пробелов", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-cars/lookup?number=%20х999хх77%20&mark=lada", h)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		resp := testutil.ParseMap(t, rec)
		assert.Equal(t, "Х999ХХ77", resp["number"])
		assert.Equal(t, "Lada", resp["mark"])
		assert.Greater(t, int(resp["id"].(float64)), 0)
	})

	t.Run("404 если нет совпадения по марке", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-cars/lookup?number=Х999ХХ77&mark=BMW", h)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("400 без номера", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-cars/lookup?mark=Lada", h)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestUniqueCars_SearchQuery_NoCrossOwnerLeak - регресс-замок против будущего
// рефакторинга поиска (#1158), зеркало TestUniqueEmployees_SearchQuery_NoCrossOwnerLeak.
// Изоляция видимости при поиске держится ИСКЛЮЧИТЕЛЬНО на том, что owner-фильтр
// (uc.user_id = ...) и поисковый OR-блок (ILIKE по uc.number/uc.mark/lpf.name/o.name/c.name
// + REPLACE(...)ILIKE для номера без пробелов) - два ОТДЕЛЬНЫХ .Where() в buildCarsQuery,
// а GORM оборачивает каждый в скобки: (owner) AND (search). Если кто-то сольёт их в одну
// строку `.Where(owner+" AND "+search)`, приоритет AND над OR даст
// `(owner AND number_ilike) OR mark_ilike OR ...` - и все ветки поиска кроме первой
// перестанут быть ограничены владельцем -> утечка чужих машин через поиск, тихо и без падения.
// Тест: владелец A под filter_type=user (видит только своих) ищет номер/марку машины
// владельца B -> ожидаем 0 (не находит чужую). Контроль: свою находит.
func TestUniqueCars_SearchQuery_NoCrossOwnerLeak(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Владелец A (админ, организация из seed).
	tokenA := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	hA := testutil.AuthHeader(tokenA)

	// Владелец B - отдельный пользователь ДРУГОЙ организации (иной user_id).
	orgB := models.Organization{Name: "Isolation Org B Cars"}
	require.NoError(t, db.Create(&orgB).Error, "seed org B")
	tokenB := testutil.RegisterManager(t, e, "ownerb_iso_cars", orgB.ID, 0)
	hB := testutil.AuthHeader(tokenB)

	// Каждый заводит свою машину с УНИКАЛЬНЫМ номером/маркой.
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"А111АА11","mark":"IsolationMarkA"}`, hA).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"В222ВВ22","mark":"IsolationMarkB"}`, hB).Code)

	// A под filter_type=user ищет номер B -> НЕ находит (owner-scope не течёт через OR).
	rec := testutil.GET(t, e, "/unique-cars?filter_type=user&per_page=50&search_query=В222ВВ22", hA)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	assert.Empty(t, rows, "владелец A не должен находить машину владельца B через поиск по номеру")

	// A под filter_type=user ищет марку B -> тоже НЕ находит.
	rec = testutil.GET(t, e, "/unique-cars?filter_type=user&per_page=50&search_query=IsolationMarkB", hA)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	assert.Empty(t, rows, "владелец A не должен находить машину владельца B через поиск по марке")

	// Контроль: A ищет СВОЙ номер -> находит свою запись (поиск работает, режется только чужое).
	rec = testutil.GET(t, e, "/unique-cars?filter_type=user&per_page=50&search_query=А111АА11", hA)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	require.Len(t, rows, 1, "владелец A находит свою запись по своему номеру")
	require.NotNil(t, rows[0].Number)
	assert.Equal(t, "А111АА11", *rows[0].Number)
}
