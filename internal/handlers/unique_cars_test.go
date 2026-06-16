package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

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
