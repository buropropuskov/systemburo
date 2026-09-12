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
	"gorm.io/gorm"
)

// writeEmployeePassageAt пишет отметку прохода человека в общий журнал - в том же виде,
// какой читает employeesHistoryUnion.
func writeEmployeePassageAt(t *testing.T, db *gorm.DB, employeeID int, action string, at time.Time, actorID *int, tableID *int) int {
	t.Helper()
	details := map[string]any{"comment": "тестовый проход"}
	if tableID != nil {
		details["table_id"] = *tableID
	}
	raw, err := json.Marshal(details)
	require.NoError(t, err)
	entry := models.AuditLog{
		EntityType:  models.AuditEntityEmployee,
		EntityID:    &employeeID,
		Action:      action,
		ActorUserID: actorID,
		Details:     raw,
		CreatedAt:   at,
	}
	require.NoError(t, db.Create(&entry).Error)
	return entry.ID
}

// Журнал людей отдаётся страницами (#2469): раньше история таблицы выгружалась целиком.
// Проверяем и то, что скоуп таблицы уцелел вместе с фильтром: базовое условие места -
// это OR-ветка, и без скобок фильтр приклеился бы только к последней ветке, пустив в
// выдачу людей чужого поста.
func TestEmployeesHistory_TablePageKeepsScope(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	citizenshipID := seedCitizenship(t, db)
	adminID := userIDByName(t, db, "testadmin")

	tableOwn := seedPeopleTable(t, db, "people_page_own", "Проход свой")
	tableOther := seedPeopleTable(t, db, "people_page_other", "Проход чужой")

	addEmployee := func(tableID int, last, first string) int {
		body := fmt.Sprintf(`{
			"organization_id": %d,
			"company_id": %d,
			"table_id": %d,
			"entry_date_from": "2026-07-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"employees": [{
				"last_name": %q,
				"first_name": %q,
				"citizenship_id": %d,
				"position": "Loader",
				"passport_series_number": "1234 567890",
				"target_tables": []
			}]
		}`, td.OrgID, td.CompanyID, tableID, last, first, citizenshipID)
		rec := testutil.POST(t, e, "/employees/manual", body, h)
		require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
		resp := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec)
		require.Len(t, resp.EmployeeIDs, 1)
		return resp.EmployeeIDs[0]
	}

	own := addEmployee(tableOwn, "Petrov", "Petr")
	other := addEmployee(tableOther, "Petrov", "Pavel")

	now := time.Now()
	for i := 0; i < 3; i++ {
		writeEmployeePassageAt(t, db, own, "entry", now.Add(-time.Duration(i+1)*time.Minute), &adminID, &tableOwn)
	}
	writeEmployeePassageAt(t, db, other, "entry", now, &adminID, &tableOther)

	page := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/employees/history/table/%d?per_page=2", tableOwn), h).Body)
	assert.Len(t, page.Data, 2, "страница не больше запрошенной")
	// Кроме трёх проходов в истории места лежат два события появления человека -
	// create и added_to_table: историю таблицы метод не сужает до проходов (урок #1085).
	assert.EqualValues(t, 5, page.Meta.Total, "счётчик считает все события своей таблицы")

	byName := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/employees/history/table/%d?search=Petrov", tableOwn), h).Body)
	assert.EqualValues(t, 5, byName.Meta.Total, "поиск по фамилии внутри своей таблицы")
	for _, row := range byName.Data {
		assert.EqualValues(t, own, row["employee_id"], "человек чужого поста в выдачу не попадает")
	}

	byEmployee := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/employees/history/table/%d?employee_id=%d", tableOwn, other), h).Body)
	assert.EqualValues(t, 0, byEmployee.Meta.Total, "фильтр по человеку не обходит скоуп таблицы")
}

// История таблицы не сужается до проходов: событие создания человека обязано остаться в
// выдаче. Это урок #1085 - фильтр по action_type здесь однажды уже ломал историю места.
func TestEmployeesHistory_TableKeepsNonPassageEvents(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	citizenshipID := seedCitizenship(t, db)
	table := seedPeopleTable(t, db, "people_events", "Проход события")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"company_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"entry_time_from": "08:00",
		"entry_time_to": "18:00",
		"employees": [{
			"last_name": "Sidorov",
			"first_name": "Sidr",
			"citizenship_id": %d,
			"position": "Loader",
			"passport_series_number": "1234 567890",
			"target_tables": []
		}]
	}`, td.OrgID, td.CompanyID, table, citizenshipID)
	rec := testutil.POST(t, e, "/employees/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	page := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/employees/history/table/%d", table), h).Body)
	require.NotEmpty(t, page.Data)
	actions := make([]string, 0, len(page.Data))
	for _, row := range page.Data {
		actions = append(actions, row["action_type"].(string))
	}
	assert.Contains(t, actions, "create", "история места показывает не только проходы")

	// Общий журнал проходов, наоборот, берёт только входы и выходы.
	all := parsePassagePage(t, testutil.GET(t, e, "/employees/history/all", h).Body)
	assert.EqualValues(t, 0, all.Meta.Total, "в журнале проходов пока нет ни входов, ни выходов")
}

// Значения выпадающих списков журнала людей берутся из журнала: и отметившие, и те, кого
// отмечали. Собранные из загруженной страницы, они предлагали бы не тех, кто в журнале.
func TestEmployeesHistory_FilterOptions(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	citizenshipID := seedCitizenship(t, db)
	adminID := userIDByName(t, db, "testadmin")

	tableOwn := seedPeopleTable(t, db, "people_opt_own", "Проход свой")
	tableOther := seedPeopleTable(t, db, "people_opt_other", "Проход чужой")
	addEmployee := func(tableID int, last string) int {
		body := fmt.Sprintf(`{
			"organization_id": %d,
			"company_id": %d,
			"table_id": %d,
			"entry_date_from": "2026-07-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"employees": [{
				"last_name": %q,
				"first_name": "Ivan",
				"citizenship_id": %d,
				"position": "Loader",
				"passport_series_number": "1234 567890",
				"target_tables": []
			}]
		}`, td.OrgID, td.CompanyID, tableID, last, citizenshipID)
		rec := testutil.POST(t, e, "/employees/manual", body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return testutil.ParseResponse[services.ManualEmployeeResponse](t, rec).EmployeeIDs[0]
	}

	own := addEmployee(tableOwn, "Ovnov")
	other := addEmployee(tableOther, "Chuzhov")

	now := time.Now()
	writeEmployeePassageAt(t, db, own, "entry", now.Add(-time.Minute), &adminID, &tableOwn)
	writeEmployeePassageAt(t, db, other, "entry", now, nil, &tableOther)

	all := testutil.ParseMap(t, testutil.GET(t, e, "/employees/history/filter-options", h))
	users := all["users"].([]interface{})
	employees := all["employees"].([]interface{})
	require.Len(t, users, 1, "в списке только тот, кто отмечал проходы")
	assert.Equal(t, float64(adminID), users[0].(map[string]interface{})["id"])
	assert.Len(t, employees, 2, "оба человека с отметками")

	scoped := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/employees/history/filter-options?table_id=%d", tableOwn), h))
	scopedEmployees := scoped["employees"].([]interface{})
	require.Len(t, scopedEmployees, 1, "список сужается до своей таблицы")
	assert.Equal(t, float64(own), scopedEmployees[0].(map[string]interface{})["id"])

	bad := testutil.GET(t, e, "/employees/history/filter-options?table_id=-1", h)
	assert.Equal(t, http.StatusBadRequest, bad.Code, "мусор в table_id не должен молча отдавать весь журнал")
}
