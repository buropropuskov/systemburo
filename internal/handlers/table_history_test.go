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

// Тесты события истории «Добавлен в таблицу проходной» (#1085, action=added_to_table). При каждом
// попадании машины/сотрудника в таблицу проходной (car_target_tables/employee_target_tables) пишется
// ОДНА запись audit_log на таблицу с details.table_id -> reader резолвит table_name. Покрыты все
// пути попадания: ручное добавление (#1049), прямое создание сотрудника, подача заявки (обе ветки).

// historyAddedRows возвращает записи added_to_table из ответа GET .../history.
func historyAddedRows(items []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, it := range items {
		if it["action_type"] == models.AuditActionAddedToTable {
			out = append(out, it)
		}
	}
	return out
}

// TestCreateManualCars_WritesAddedToTableHistory — ручное добавление машины (#1049) пишет
// added_to_table на каждую уникальную целевую таблицу (таблица-страница + «Проезд»).
func TestCreateManualCars_WritesAddedToTableHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	pageTable := seedCarsTable(t, db, "th_cars_page", "Проезд P")
	passTable := seedCarsTable(t, db, "th_cars_pass", "Проезд A")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"table_id": %d,
		"vehicles": [{"car_number": "H101HH177", "car_brand": "Ford", "target_tables": [%d]}]
	}`, td.OrgID, pageTable, passTable)
	rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	carID := testutil.ParseResponse[services.ManualCarResponse](t, rec).CarIDs[0]

	// Две уникальные таблицы (страница + «Проезд») -> две записи added_to_table.
	var cnt int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ? AND entity_id = ?", models.AuditEntityCar, models.AuditActionAddedToTable, carID).
		Count(&cnt).Error)
	assert.EqualValues(t, 2, cnt, "по одной записи на уникальную таблицу")

	// В истории обе записи с непустым table_name и верными table_id.
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	added := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, added, 2)
	seen := map[int]bool{}
	for _, r := range added {
		assert.NotEmpty(t, r["table_name"], "table_name резолвится для added_to_table")
		seen[int(r["table_id"].(float64))] = true
	}
	assert.True(t, seen[pageTable] && seen[passTable], "обе таблицы в истории")
}

// TestCreateManualEmployees_WritesAddedToTableHistory — зеркало для ручного сотрудника (#1049).
func TestCreateManualEmployees_WritesAddedToTableHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	pageTable := seedPeopleTable(t, db, "th_ppl_page", "Проход P")
	passTable := seedPeopleTable(t, db, "th_ppl_pass", "Проход A")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"table_id": %d,
		"employees": [{
			"last_name": "Ivanov", "first_name": "Ivan",
			"citizenship_id": %d, "position": "Loader",
			"passport_series_number": "1234 567890",
			"target_tables": [%d]
		}]
	}`, td.OrgID, pageTable, citizenshipID, passTable)
	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	empID := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec).EmployeeIDs[0]

	var cnt int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ? AND entity_id = ?", models.AuditEntityEmployee, models.AuditActionAddedToTable, empID).
		Count(&cnt).Error)
	assert.EqualValues(t, 2, cnt, "по одной записи на уникальную таблицу")

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	added := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, added, 2)
	for _, r := range added {
		assert.NotEmpty(t, r["table_name"], "table_name резолвится")
	}
}

// TestCreateEmployee_WritesAddedToTableHistory — standalone POST /employees пишет added_to_table
// на каждую таблицу и проставляет автора (проброшен из контекста, не «Система»).
func TestCreateEmployee_WritesAddedToTableHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	citizenshipID := seedCitizenship(t, db)
	tableA := seedPeopleTable(t, db, "th_emp_a", "Проход A")
	tableB := seedPeopleTable(t, db, "th_emp_b", "Проход B")

	token := testutil.RegisterAndLogin(t, e, "th_empauthor", "pass123", 1, td.OrgID, td.CompanyID)
	var actorID int
	require.NoError(t, db.Raw("SELECT id FROM users WHERE username = ?", "th_empauthor").Scan(&actorID).Error)
	require.NotZero(t, actorID)

	body := fmt.Sprintf(`{
		"last_name": "Petrov", "first_name": "Petr",
		"citizenship_id": %d, "position": "Driver",
		"passport_series_number": "4567 890123",
		"target_tables": [%d, %d]
	}`, citizenshipID, tableA, tableB)
	rec := testutil.POST(t, e, "/employees", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "create employee: %s", rec.Body.String())
	empID := int(testutil.ParseMap(t, rec)["employee_id"].(float64))

	var rows []models.AuditLog
	require.NoError(t, db.
		Where("entity_type = ? AND action = ? AND entity_id = ?", models.AuditEntityEmployee, models.AuditActionAddedToTable, empID).
		Find(&rows).Error)
	require.Len(t, rows, 2, "по одной записи на таблицу")
	for _, r := range rows {
		require.NotNil(t, r.ActorUserID, "автор проброшен, не «Система»")
		assert.Equal(t, actorID, *r.ActorUserID)
	}
}

// TestSubmitComplete_WritesAddedToTableHistory — подача заявки пишет added_to_table для ОБЕИХ веток:
// машины (passage_tables) и сотрудники (target_tables). Две независимые точки в SubmitCompleteApplication.
func TestSubmitComplete_WritesAddedToTableHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "th_submit", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	carsTable := seedCarsTable(t, db, "th_sub_cars", "Проезд S")
	peopleTable := seedPeopleTable(t, db, "th_sub_ppl", "Проход S")
	carUA := seedUniqueAttachment(t, db, "cars", "th_sub_cars_tmpl", "Cars Tmpl")
	pplUA := seedUniqueAttachment(t, db, "people", "th_sub_ppl_tmpl", "People Tmpl")

	body := fmt.Sprintf(`{
		"message": "table history submit", "organization": "Test Organization",
		"responsible_person": "Test", "contact_phone": "+79001234567", "data_approval": true,
		"attachments": [
			{"attachment_type": "cars", "attachment_name": "cars_tmpl", "attachment_display_name": "Cars",
				"unique_attachment_id": %d, "entry_date_from": "2026-04-01", "entry_date_to": "2099-12-31",
				"data": {"vehicles": [{"car_number": "S777SS177", "car_brand": "Kia", "passage_tables": [%d]}]}},
			{"attachment_type": "people", "attachment_name": "ppl_tmpl", "attachment_display_name": "People",
				"unique_attachment_id": %d, "entry_date_from": "2026-04-01", "entry_date_to": "2099-12-31",
				"data": {"employees": [{"last_name": "Sidorov", "first_name": "Sidor",
					"citizenship_id": %d, "position": "Worker", "passport_series_number": "1234 567890",
					"target_tables": [%d]}]}}
		]
	}`, carUA, carsTable, pplUA, citizenshipID, peopleTable)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())

	// Ветка cars (#1): история машины содержит added_to_table по carsTable.
	var carID int
	require.NoError(t, db.Raw("SELECT id FROM cars WHERE car_number = ?", "S777SS177").Scan(&carID).Error)
	require.NotZero(t, carID)
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	carAdded := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, carAdded, 1, "одна таблица «Проезд» -> одна запись")
	assert.EqualValues(t, carsTable, int(carAdded[0]["table_id"].(float64)))
	assert.NotEmpty(t, carAdded[0]["table_name"])

	// Ветка people (#2): история сотрудника содержит added_to_table по peopleTable.
	var empID int
	require.NoError(t, db.Raw("SELECT id FROM employees WHERE last_name = ?", "Sidorov").Scan(&empID).Error)
	require.NotZero(t, empID)
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	empAdded := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, empAdded, 1, "одна таблица прохода -> одна запись")
	assert.EqualValues(t, peopleTable, int(empAdded[0]["table_id"].(float64)))
	assert.NotEmpty(t, empAdded[0]["table_name"])
}
