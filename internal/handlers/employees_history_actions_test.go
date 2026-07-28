package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedEmployeeViaCompleteApp создаёт заявку с одним сотрудником и возвращает (appID, attID, empID).
// Дата окончания (entry_date_to) задана 2099-12-31 чтобы заявка не истекала во время теста.
func seedEmployeeViaCompleteApp(t *testing.T, e *echo.Echo, db *gorm.DB, token string, orgName string) (int, int, int) {
	t.Helper()

	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", fmt.Sprintf("emp_tmpl_%s", t.Name()), "Employee Template")

	body := fmt.Sprintf(`{
		"message": "employee history test",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people",
			"attachment_name": "people_tmpl",
			"attachment_display_name": "People Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"employees": [{
					"last_name": "Ivanov",
					"first_name": "Ivan",
					"middle_name": "Ivanovich",
					"citizenship_id": %d,
					"position": "Worker",
					"passport_series_number": "1234 567890",
					"target_tables": [%d]
				}]
			}
		}]
	}`, orgName, uaID, citizenshipID, tableID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit complete app: %s", rec.Body.String())

	createResp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	appID := createResp.ApplicationID

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	atts := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, atts)
	attID := int(atts[0]["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	emps := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, emps)
	empID := int(emps[0]["id"].(float64))

	return appID, attID, empID
}

// TestSubmitApplication_CreatesEmployeeHistoryEntry проверяет, что при подаче
// заявки на сотрудника в audit_log[employee] создаётся запись action=create
// (после cutover #870, срез 1.13b). По аналогии с cars.
func TestSubmitApplication_CreatesEmployeeHistoryEntry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "emphist_create1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	var historyCount int64
	db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "create").
		Count(&historyCount)
	assert.Equal(t, int64(1), historyCount, "должна быть ровно одна запись create в audit_log сотрудника")

	var entry models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "create").First(&entry).Error)
	require.NotNil(t, entry.ActorUserID, "actor_user_id должен быть установлен (отправитель заявки)")
	assert.Contains(t, string(entry.Details), "Ivanov", "details.comment должен содержать ФИО сотрудника")
	assert.Contains(t, string(entry.Details), "создан")
}

// TestCheckExpiredAttachments_CreatesEmployeeDeactivateHistory проверяет, что при
// истечении срока заявки на сотрудника пишется запись action=deactivate в
// audit_log[employee] (после cutover #870, срез 1.13b; по аналогии с cars).
func TestCheckExpiredAttachments_CreatesEmployeeDeactivateHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	permSvc := services.NewPermissionService(db)
	notifSvc := services.NewNotificationService(db)
	blRecorder := services.NewAuditRecorder(db)
	vblSvc := services.NewVehicleBlacklistService(db, blRecorder)
	pblSvc := services.NewPersonBlacklistService(db, blRecorder)
	appSvc := services.NewApplicationService(db, permSvc, notifSvc, vblSvc, pblSvc, blRecorder)

	token := testutil.RegisterAndLogin(t, e, "emphist_expiry1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, attID, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = ? WHERE id = ?", yesterday, attID).Error)

	require.NoError(t, appSvc.CheckExpiredAttachments(context.Background()))

	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 0, *emp.Status, "сотрудник должен быть деактивирован")

	var historyCount int64
	db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "deactivate").
		Count(&historyCount)
	assert.Equal(t, int64(1), historyCount, "должна быть запись deactivate в audit_log при истечении срока")

	var entry models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "deactivate").First(&entry).Error)
	assert.Nil(t, entry.ActorUserID, "деактивация по сроку без актора (user NULL)")
	assert.Contains(t, string(entry.Details), "Ivanov", "details должен содержать ФИО")
	assert.Contains(t, string(entry.Details), "истёк", "details должен указывать на истечение срока")
}

// TestDeactivateEmployee_CreatesDeleteHistory проверяет PUT /employees/:id/deactivate.
// Должен записать action_type=delete и установить date_deleted.
func TestDeactivateEmployee_CreatesDeleteHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "emphist_deact1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	userID := getUserID(t, db, "emphist_deact1")
	body := fmt.Sprintf(`{"status": 0, "user_id": %d}`, userID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/deactivate", empID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "Employee deactivated successfully", testutil.ParseMessage(t, rec))

	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 0, *emp.Status, "статус должен стать 0 после деактивации")
	require.NotNil(t, emp.DateDeleted, "date_deleted должно быть установлено")

	var deleteCount int64
	db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "delete").
		Count(&deleteCount)
	assert.Equal(t, int64(1), deleteCount, "должна быть запись delete в audit_log сотрудника")

	var entry models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "delete").First(&entry).Error)
	assert.Contains(t, string(entry.Details), "Ivanov")
	require.NotNil(t, entry.ActorUserID)
	assert.Equal(t, userID, *entry.ActorUserID)
}

// TestDeactivateEmployee_InputValidation покрывает edge-кейсы PUT /employees/:id/deactivate.
func TestDeactivateEmployee_InputValidation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "emphist_deact2", "pass123", 1, td.OrgID, td.CompanyID)

	tests := []struct {
		name           string
		path           string
		body           string
		expectedStatus int
	}{
		{
			name:           "invalid_id_returns_400",
			path:           "/employees/abc/deactivate",
			body:           `{"status": 0}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing_id_returns_404",
			path:           "/employees/999999/deactivate",
			body:           `{"status": 0}`,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testutil.PUT(t, e, tt.path, tt.body, testutil.AuthHeader(token))
			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestActivateEmployee_AfterDeactivateCreatesActivateHistory покрывает полный цикл:
// deactivate -> activate. Должны появиться обе записи в истории.
func TestActivateEmployee_AfterDeactivateCreatesActivateHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "emphist_act1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/deactivate", empID),
		`{"status": 0}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.PUT(t, e, fmt.Sprintf("/employees/%d/activate", empID),
		`{}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "Employee activated successfully", testutil.ParseMessage(t, rec))

	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 1, *emp.Status, "статус должен стать 1 после активации")
	assert.Nil(t, emp.DateDeleted, "date_deleted должно быть очищено")

	var deleteCount, activateCount int64
	db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "delete").Count(&deleteCount)
	db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "activate").Count(&activateCount)
	assert.Equal(t, int64(1), deleteCount, "должна быть одна запись delete")
	assert.Equal(t, int64(1), activateCount, "должна быть одна запись activate")
}

// TestRestoreEmployee_CreatesRestoreHistory покрывает PUT /employees/:id/restore.
func TestRestoreEmployee_CreatesRestoreHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "emphist_rest1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/deactivate", empID),
		`{"status": 0}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.PUT(t, e, fmt.Sprintf("/employees/%d/restore", empID),
		`{}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "Employee restored successfully", testutil.ParseMessage(t, rec))

	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 1, *emp.Status, "статус должен стать 1 после восстановления")
	assert.Nil(t, emp.DateDeleted, "date_deleted должно быть очищено")

	var restoreCount int64
	db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "restore").
		Count(&restoreCount)
	assert.Equal(t, int64(1), restoreCount, "должна быть одна запись restore")
}

// TestEmployeesHistory_Unauthorized гарантирует, что новые endpoint-ы требуют auth.
func TestEmployeesHistoryActions_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	endpoints := []string{
		"/employees/1/deactivate",
		"/employees/1/activate",
		"/employees/1/restore",
	}

	for _, path := range endpoints {
		t.Run(path, func(t *testing.T) {
			rec := testutil.PUT(t, e, path, "{}", nil)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// TestEmployeeTerritoryStatus_RecordsTableInHistory фиксирует регрессию: вход/выход
// сотрудника должны сохранять table_id таблицы (КПП) и история должна отдавать table_name.
// Фронт уже слал table_id, но UpdateTerritoryStatusRequest не имел поля и запись его теряла.
func TestEmployeeTerritoryStatus_RecordsTableInHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empentrytbl1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	// seedEmployeeViaCompleteApp уже создал system_table "test_table" (display_name "Test Table").
	var st models.SystemTable
	require.NoError(t, db.Where("name = ?", "test_table").First(&st).Error)
	testutil.GrantTableVerb(t, getUserID(t, db, "empentrytbl1"), "test_table", "entry")

	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/territory-status", empID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, st.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)

	var entry map[string]interface{}
	for _, h := range history {
		if h["action_type"] == "entry" {
			entry = h
			break
		}
	}
	require.NotNil(t, entry, "history should contain entry record")
	require.NotNil(t, entry["table_id"], "entry record should carry table_id")
	assert.Equal(t, float64(st.ID), entry["table_id"])
	assert.Equal(t, "Test Table", entry["table_name"], "entry record should resolve table_name from system_tables")
}

// TestRecentPassages_ResolvesPostFromTableID фиксирует регрессию ленты «Проход людей»
// на дашборде: после реальной отметки входа через /table (territory-status с table_id)
// лента должна показывать пост (system_tables.display_name), а не пусто (фронт рисует
// «не указан»). Источник поста - только history.table_id -> system_tables; пустой пост
// в ленте = у записи нет table_id (как у проходов, отмеченных до фикса #703).
func TestRecentPassages_ResolvesPostFromTableID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "recentpasspost1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	var st models.SystemTable
	require.NoError(t, db.Where("name = ?", "test_table").First(&st).Error)
	testutil.GrantTableVerb(t, getUserID(t, db, "recentpasspost1"), "test_table", "entry")

	// Реальная отметка входа через тот же endpoint, что и страница /table.
	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/territory-status", empID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, st.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.GetRecentPassages(context.Background(), 15)
	require.NoError(t, err)
	require.NotEmpty(t, res.People, "лента проходов людей не должна быть пустой после отметки")

	var entry *models.RecentPassage
	for i := range res.People {
		if res.People[i].ActionType == "entry" {
			entry = &res.People[i]
			break
		}
	}
	require.NotNil(t, entry, "в ленте должна быть запись entry")
	assert.Equal(t, "Test Table", entry.Place, "пост в ленте должен резолвиться из system_tables.display_name по table_id")
}
