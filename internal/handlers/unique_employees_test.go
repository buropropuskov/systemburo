package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

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
	active := fmt.Sprintf(`{"last_name":"RegActive","first_name":"A","passport_series_number":"1234 567890","organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
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
	idle := fmt.Sprintf(`{"last_name":"RegIdle","first_name":"B","passport_series_number":"0000 000111","organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
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

	body := `{"last_name":"Dup","first_name":"Test","passport_series_number":"DUP 123456"}`
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
	body := fmt.Sprintf(`{"last_name":"Filter","first_name":"Test","organization_id":%d}`, td.OrgID)
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
	body := `{"last_name":"NoPassport","first_name":"Worker"}`
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

	rec := testutil.PUT(t, e, "/unique-employees/99999", `{"last_name":"X"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUniqueEmployees_Lookup(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := fmt.Sprintf(`{"last_name":"Сидоров","first_name":"Семён","middle_name":"Семёнович","passport_series_number":"9999 888777","organization_id":%d}`, td.OrgID)
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
