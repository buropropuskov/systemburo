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
	"gorm.io/gorm"
)

// Тесты события истории «Добавлен в таблицу проходной» (#1085, action=added_to_table). Запись = момент
// ПОПАДАНИЯ машины/сотрудника в таблицу проходной, т.е. когда сущность стала активной (status=1):
// принятие заявки в работу или ручное добавление (#1049). При подаче заявки (status=0) история НЕ
// пишется. По одной записи audit_log на таблицу с details.table_id -> reader резолвит table_name.

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

// addedToTableCount считает записи added_to_table по сущности прямо в БД.
func addedToTableCount(t *testing.T, db *gorm.DB, entityType string, entityID int) int64 {
	var c int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ? AND entity_id = ?", entityType, models.AuditActionAddedToTable, entityID).
		Count(&c).Error)
	return c
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

	// По одной записи на уникальную таблицу, автор проброшен (не «Система»).
	var rows []models.AuditLog
	require.NoError(t, db.
		Where("entity_type = ? AND action = ? AND entity_id = ?", models.AuditEntityEmployee, models.AuditActionAddedToTable, empID).
		Find(&rows).Error)
	require.Len(t, rows, 2, "по одной записи на уникальную таблицу")
	for _, r := range rows {
		require.NotNil(t, r.ActorUserID, "автор попадания проброшен (manual-путь)")
	}

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	added := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, added, 2)
	for _, r := range added {
		assert.NotEmpty(t, r["table_name"], "table_name резолвится")
	}
}

// TestCreateEmployee_NoAddedToTableHistory — standalone POST /employees создаёт НЕактивного
// сотрудника (status=0), которого в таблице проходной не видно, поэтому истории попадания НЕ пишет
// (она пишется только при активации, status->1). Само создание фиксируется записью create (#1085).
func TestCreateEmployee_NoAddedToTableHistory(t *testing.T) {
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

	// Неактивный сотрудник (status=0) в таблице проходной не виден -> истории попадания нет.
	assert.Zero(t, addedToTableCount(t, db, models.AuditEntityEmployee, empID), "status=0 -> added_to_table не пишется")

	// Создание зафиксировано записью create с автором.
	var createRows []models.AuditLog
	require.NoError(t, db.
		Where("entity_type = ? AND action = ? AND entity_id = ?", models.AuditEntityEmployee, "create", empID).
		Find(&createRows).Error)
	require.Len(t, createRows, 1, "создание сотрудника пишет одну запись create")
	require.NotNil(t, createRows[0].ActorUserID, "автор проброшен, не «Система»")
	assert.Equal(t, actorID, *createRows[0].ActorUserID)
}

// TestTakeToWork_WritesAddedToTableHistory — момент попадания в таблицу = ПРИНЯТИЕ заявки в работу
// (status 0->1), а не подача. Проверяет обе ветки: после submit истории added_to_table НЕТ (машина
// и сотрудник ещё неактивны), после take-to-work(accept) она появляется по целевым таблицам с
// автором-принимающим (#1085).
func TestTakeToWork_WritesAddedToTableHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "th_ttw_sndr", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	carsTable := seedCarsTable(t, db, "th_ttw_cars", "Проезд S")
	peopleTable := seedPeopleTable(t, db, "th_ttw_ppl", "Проход S")
	carUA := seedUniqueAttachment(t, db, "cars", "th_ttw_cars_tmpl", "Cars Tmpl")
	pplUA := seedUniqueAttachment(t, db, "people", "th_ttw_ppl_tmpl", "People Tmpl")

	// Принимающий (тип 6 + запись в application_approvers).
	testutil.RegisterUser(t, e, "th_ttw_appr", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "th_ttw_appr")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "th_ttw_appr", "pass123")

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
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID
	require.NotZero(t, appID)

	var carID, empID int
	require.NoError(t, db.Raw("SELECT id FROM cars WHERE car_number = ?", "S777SS177").Scan(&carID).Error)
	require.NoError(t, db.Raw("SELECT id FROM employees WHERE last_name = ?", "Sidorov").Scan(&empID).Error)
	require.NotZero(t, carID)
	require.NotZero(t, empID)

	// После подачи истории попадания НЕТ - сущности ещё неактивны (status=0).
	assert.Zero(t, addedToTableCount(t, db, models.AuditEntityCar, carID), "после submit машина не активна - истории нет")
	assert.Zero(t, addedToTableCount(t, db, models.AuditEntityEmployee, empID), "после submit сотрудник не активен - истории нет")

	// Принятие в работу активирует машины/сотрудников -> пишется added_to_table.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
		fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "take-to-work: %s", rec.Body.String())

	// Ветка cars: одна запись по carsTable, table_name резолвится, автор-принимающий.
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code)
	carAdded := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, carAdded, 1, "одна таблица «Проезд» -> одна запись")
	assert.EqualValues(t, carsTable, int(carAdded[0]["table_id"].(float64)))
	assert.NotEmpty(t, carAdded[0]["table_name"])

	// Ветка people: одна запись по peopleTable.
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code)
	empAdded := historyAddedRows(testutil.ParseSlice(t, rec))
	require.Len(t, empAdded, 1, "одна таблица прохода -> одна запись")
	assert.EqualValues(t, peopleTable, int(empAdded[0]["table_id"].(float64)))
	assert.NotEmpty(t, empAdded[0]["table_name"])

	// Автор истории попадания - принимающий (актор проброшен, не «Система»).
	var rows []models.AuditLog
	require.NoError(t, db.
		Where("action = ? AND actor_user_id = ? AND entity_id IN ?", models.AuditActionAddedToTable, approverID, []int{carID, empID}).
		Find(&rows).Error)
	assert.Len(t, rows, 2, "обе записи с автором-принимающим")
}

// TestTakeToWork_NoDuplicateAddedToTableOnReAccept — повторная активация уже активной строки не
// плодит историю; после деактивации (revoke) и повторного принятия пишется НОВОЕ попадание (#1085).
func TestTakeToWork_NoDuplicateAddedToTableOnReAccept(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "th_re_sndr", "pass123", 1, td.OrgID, td.CompanyID)
	carsTable := seedCarsTable(t, db, "th_re_cars", "Проезд R")
	carUA := seedUniqueAttachment(t, db, "cars", "th_re_cars_tmpl", "Cars Tmpl")

	testutil.RegisterUser(t, e, "th_re_appr", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "th_re_appr")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "th_re_appr", "pass123")

	body := fmt.Sprintf(`{
		"message": "reaccept", "organization": "Test Organization",
		"responsible_person": "Test", "contact_phone": "+79001234567", "data_approval": true,
		"attachments": [{"attachment_type": "cars", "attachment_name": "cars_tmpl", "attachment_display_name": "Cars",
			"unique_attachment_id": %d, "entry_date_from": "2026-04-01", "entry_date_to": "2099-12-31",
			"data": {"vehicles": [{"car_number": "R555RR177", "car_brand": "Kia", "passage_tables": [%d]}]}}]
	}`, carUA, carsTable)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var carID int
	require.NoError(t, db.Raw("SELECT id FROM cars WHERE car_number = ?", "R555RR177").Scan(&carID).Error)

	accept := func() {
		r := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
			fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID), testutil.AuthHeader(approverToken))
		require.Equal(t, http.StatusOK, r.Code, "take-to-work: %s", r.Body.String())
	}

	// Первое принятие -> одна запись.
	accept()
	assert.EqualValues(t, 1, addedToTableCount(t, db, models.AuditEntityCar, carID), "первое принятие -> одна запись")

	// Отзыв из работы деактивирует (status->0), запись не добавляется.
	rRevoke := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-from-work", appID),
		fmt.Sprintf(`{"user_id":%d,"comment":"revoke"}`, approverID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rRevoke.Code, "revoke: %s", rRevoke.Body.String())
	assert.EqualValues(t, 1, addedToTableCount(t, db, models.AuditEntityCar, carID), "деактивация истории не пишет")

	// Повторное принятие -> новое попадание (вторая запись).
	accept()
	assert.EqualValues(t, 2, addedToTableCount(t, db, models.AuditEntityCar, carID), "повторное принятие -> новая запись")
}
