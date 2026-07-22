package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Строка сотрудника в таблице прохода несёт territory_status - тот же источник
// (employees.territory_status), что отдаёт /employees/history/current-status.
// Без него страница считала всех «не отмеченными» до ответа второго запроса, и
// счётчик зашедших на каждое обновление проваливался в 0.
func TestGetActiveEmployeesForTable_CarriesTerritoryStatus(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "people_territory_a", "Проход Т")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"company_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"entry_time_from": "08:00",
		"entry_time_to": "18:00",
		"employees": [{
			"last_name": "Territoriev",
			"first_name": "Ivan",
			"citizenship_id": %d,
			"position": "Loader",
			"passport_series_number": "4321 098765",
			"target_tables": []
		}]
	}`, td.OrgID, td.CompanyID, tableID, citizenshipID)

	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	resp := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec)
	require.Len(t, resp.EmployeeIDs, 1)
	employeeID := resp.EmployeeIDs[0]

	// До отметки входа статус пустой - строка отдаётся, поле не выдумывается.
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0]["territory_status"], "без отметки входа статуса нет")

	// Отмечаем вход - строка обязана принести статус вместе с данными.
	require.NoError(t, db.Model(&models.Employee{}).
		Where("id = ?", employeeID).
		Update("territory_status", 1).Error)

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows = testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1)
	assert.EqualValues(t, 1, rows[0]["territory_status"], "вошедший сотрудник приходит со статусом 1")

	// И тот же источник, что у current-status: значения не расходятся.
	rec = testutil.GET(t, e, "/employees/history/current-status", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	statuses := testutil.ParseSlice(t, rec)
	var found bool
	for _, s := range statuses {
		if int(s["employee_id"].(float64)) == employeeID {
			found = true
			assert.EqualValues(t, 1, s["territory_status"])
		}
	}
	assert.True(t, found, "сотрудник присутствует в current-status")
}
