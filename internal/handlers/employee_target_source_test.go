package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Источник привязки employee_target_tables.source (#1227, зеркало cars P1): submit
// заявки проставляет 'application' через дефолт колонки (сырой INSERT без source),
// ручное/групповое добавление проставляет 'manual' явно в струк-Create.

// Подача заявки с сотрудником и целевой таблицей -> строка employee_target_tables
// получает source='application' от дефолта колонки (submit не указывает source явно).
func TestEmployeeTargetTables_SubmitApplication_SourceIsApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, uniq("emp_src_app_table"), "Source App Table")
	uaID := seedUniqueAttachment(t, db, "people", uniq("emp_src_app_tmpl"), "People Template")
	lastName := uniq("SourceAppEmp")

	token := testutil.RegisterAndLogin(t, e, uniq("emp_src_app_user"), "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{
		"message": "target table source test",
		"organization": "Test Organization",
		"responsible_person": "Source Test",
		"contact_phone": "+79001112233",
		"data_approval": true,
		"attachments": [
			{
				"attachment_type": "people",
				"attachment_name": "%s",
				"attachment_display_name": "People Template",
				"unique_attachment_id": %d,
				"entry_date_from": "2026-04-01",
				"entry_date_to": "2099-12-31",
				"data": {
					"employees": [{
						"last_name": "%s",
						"first_name": "Ivan",
						"citizenship_id": %d,
						"position": "Engineer",
						"passport_series_number": "1234 567890",
						"target_tables": [%d]
					}]
				}
			}
		]
	}`, uniq("emp_src_app_tmpl"), uaID, lastName, citizenshipID, tableID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	createResp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)

	var empID int
	require.NoError(t, db.Raw(`
		SELECT e.id FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		WHERE a.application_id = ? AND e.last_name = ?
	`, createResp.ApplicationID, lastName).Scan(&empID).Error)
	require.NotZero(t, empID, "заявочный сотрудник найден")

	var link models.EmployeeTargetTable
	require.NoError(t, db.Where("employee_id = ? AND table_id = ?", empID, tableID).First(&link).Error)
	assert.Equal(t, "application", link.Source, "submit проставляет source через дефолт колонки")
}

// Групповое добавление (BulkAddTable, #1194) проставляет source='manual' явно.
func TestEmployeeTargetTables_BulkAddTable_SourceIsManual(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	tableID := seedPeopleTable(t, db, uniq("emp_src_manual_table"), "Source Manual Table")
	empID := createTestEmployeeForBulk(t, db, uniq("SourceManualEmp"))
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id = ?", tableID)
	}()

	res, err := svc.BulkAddTable(ctx, services.EmployeeBulkAddTableRequest{
		IDs:      []int{empID},
		TableIDs: []int{tableID},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)

	var link models.EmployeeTargetTable
	require.NoError(t, db.Where("employee_id = ? AND table_id = ?", empID, tableID).First(&link).Error)
	assert.Equal(t, "manual", link.Source, "bulk add проставляет source='manual' явно")
}

// GET /employees/active-for-table/:table_id отдаёт список привязок target_tables[]
// с id/name/source, а не только счётчик (#1227, детальная карточка «Проезд»). Заводим
// сотрудника через CreateManualEmployees (нужно вложение для JOIN attachments в
// GetActiveEmployeesForTable - createTestEmployeeForBulk его не создаёт, годится только
// для сервис-уровневых bulk-тестов) с привязкой сразу к двум таблицам.
func TestEmployeeTargetTables_ActiveForTable_ReturnsTargetTablesList(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, uniq("emp_src_list_a"), "List Table A")
	otherTableID := seedPeopleTable(t, db, uniq("emp_src_list_b"), "List Table B")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"company_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"entry_time_from": "08:00",
		"entry_time_to": "18:00",
		"employees": [{
			"last_name": "%s",
			"first_name": "Ivan",
			"citizenship_id": %d,
			"position": "Loader",
			"passport_series_number": "1234 567890",
			"target_tables": [%d]
		}]
	}`, td.OrgID, td.CompanyID, tableID, uniq("ListEmp"), citizenshipID, otherTableID)

	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	manualResp := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec)
	require.Len(t, manualResp.EmployeeIDs, 1)

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseResponse[[]services.TableEmployeeResponse](t, rec)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].TargetTablesCount, "счётчик по-прежнему считает обе привязки")
	require.Len(t, rows[0].TargetTables, 2, "список привязок наполнен (не пустой, как было раньше)")

	byID := make(map[int]services.EmployeePassageTableRef, 2)
	for _, ref := range rows[0].TargetTables {
		byID[ref.ID] = ref
	}
	require.Contains(t, byID, tableID)
	assert.NotEmpty(t, byID[tableID].Name)
	// CreateManualEmployees проставляет source='manual' явно на обеих привязках (#1049).
	assert.Equal(t, "manual", byID[tableID].Source)
	require.Contains(t, byID, otherTableID)
	assert.Equal(t, "manual", byID[otherTableID].Source)
}
