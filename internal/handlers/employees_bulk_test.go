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
	"gorm.io/gorm"
)

// Групповые операции над строками таблицы проходной сотрудников (#1194, зеркало cars).
// Сервисный уровень - лёгкие тесты без CleanDB/Seed (урок #ci_handlers_test_timeout):
// реюзают единожды засеянную БД, создают свои таблицы/сотрудников с уникальными именами
// и чистят их за собой.

// createTestEmployeeForBulk создаёт активного сотрудника без вложения/заявки - для
// групповых операций над employee_target_tables этого достаточно.
func createTestEmployeeForBulk(t *testing.T, db *gorm.DB, lastName string) int {
	t.Helper()
	firstName := "Иван"
	status := 1
	employee := models.Employee{LastName: &lastName, FirstName: &firstName, Status: &status}
	require.NoError(t, db.Create(&employee).Error)
	return employee.ID
}

// bindEmployeeToTableForBulk создаёт привязку сотрудника к таблице напрямую (минуя сервис).
func bindEmployeeToTableForBulk(t *testing.T, db *gorm.DB, employeeID, tableID int) {
	t.Helper()
	orderIdx := 1
	require.NoError(t, db.Create(&models.EmployeeTargetTable{EmployeeID: employeeID, TableID: tableID, OrderIndex: &orderIdx}).Error)
}

// countEmployeeTableLinks считает связи employee_target_tables сотрудника с таблицей.
func countEmployeeTableLinks(t *testing.T, db *gorm.DB, employeeID, tableID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.EmployeeTargetTable{}).
		Where("employee_id = ? AND table_id = ?", employeeID, tableID).Count(&count).Error)
	return count
}

// countAuditActions считает записи audit_log по сотруднику и действию.
func countAuditActions(t *testing.T, db *gorm.DB, employeeID int, action string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, employeeID, action).
		Count(&count).Error)
	return count
}

func TestEmployeeService_BulkMoveTable_MovesLinksAndAudits(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	fromTable := seedPeopleTable(t, db, uniq("bulk_emp_move_from"), "Move From")
	toTableA := seedPeopleTable(t, db, uniq("bulk_emp_move_to_a"), "Move To A")
	toTableB := seedPeopleTable(t, db, uniq("bulk_emp_move_to_b"), "Move To B")
	empID := createTestEmployeeForBulk(t, db, uniq("MoveEmp"))
	bindEmployeeToTableForBulk(t, db, empID, fromTable)
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id IN (?, ?, ?)", fromTable, toTableA, toTableB)
	}()

	res, err := svc.BulkMoveTable(ctx, services.EmployeeBulkMoveTableRequest{
		IDs:         []int{empID},
		FromTableID: fromTable,
		ToTableIDs:  []int{toTableA, toTableB},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)

	assert.EqualValues(t, 0, countEmployeeTableLinks(t, db, empID, fromTable), "связь с исходной таблицей снята")
	assert.EqualValues(t, 1, countEmployeeTableLinks(t, db, empID, toTableA), "привязан к первой целевой таблице")
	assert.EqualValues(t, 1, countEmployeeTableLinks(t, db, empID, toTableB), "привязан ко второй целевой таблице")

	assert.EqualValues(t, 1, countAuditActions(t, db, empID, models.AuditActionMovedBetweenTables), "одна сводная запись переноса")
	assert.EqualValues(t, 0, countAuditActions(t, db, empID, models.AuditActionAddedToTable), "перенос НЕ пишет added_to_table на каждую целевую (зеркало cars)")
}

// Тип-матч: попытка перенести в cars-таблицу (не people) отклоняется целиком, без
// частичного изменения связей.
func TestEmployeeService_BulkMoveTable_RejectsTypeMismatch(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	fromTable := seedPeopleTable(t, db, uniq("bulk_emp_mismatch_from"), "Mismatch From")
	carsTable := models.SystemTable{Name: uniq("bulk_emp_mismatch_cars"), TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&carsTable).Error)
	empID := createTestEmployeeForBulk(t, db, uniq("MismatchEmp"))
	bindEmployeeToTableForBulk(t, db, empID, fromTable)
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id IN (?, ?)", fromTable, carsTable.ID)
	}()

	_, err := svc.BulkMoveTable(ctx, services.EmployeeBulkMoveTableRequest{
		IDs:         []int{empID},
		FromTableID: fromTable,
		ToTableIDs:  []int{carsTable.ID},
	}, 1)
	require.Error(t, err)

	// Связь с исходной таблицей не тронута - валидация до цикла по сотрудникам.
	assert.EqualValues(t, 1, countEmployeeTableLinks(t, db, empID, fromTable))
}

func TestEmployeeService_BulkAddTable_DedupsExistingLink(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	tableA := seedPeopleTable(t, db, uniq("bulk_emp_add_a"), "Add A")
	tableB := seedPeopleTable(t, db, uniq("bulk_emp_add_b"), "Add B")
	empID := createTestEmployeeForBulk(t, db, uniq("AddEmp"))
	bindEmployeeToTableForBulk(t, db, empID, tableA)
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id IN (?, ?)", tableA, tableB)
	}()

	res, err := svc.BulkAddTable(ctx, services.EmployeeBulkAddTableRequest{
		IDs:      []int{empID},
		TableIDs: []int{tableA, tableB},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)

	assert.EqualValues(t, 1, countEmployeeTableLinks(t, db, empID, tableA), "дедуп - связь не задвоилась")
	assert.EqualValues(t, 1, countEmployeeTableLinks(t, db, empID, tableB), "новая связь создана")
	assert.EqualValues(t, 1, countAuditActions(t, db, empID, models.AuditActionAddedToTable), "аудит только за реально новую привязку")
}

func TestEmployeeService_BulkUnbindTable_LastLinkDeactivates(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	tableA := seedPeopleTable(t, db, uniq("bulk_emp_unbind_last"), "Unbind Last")
	empID := createTestEmployeeForBulk(t, db, uniq("UnbindLastEmp"))
	bindEmployeeToTableForBulk(t, db, empID, tableA)
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id = ?", tableA)
	}()

	res, err := svc.BulkUnbindTable(ctx, services.EmployeeBulkUnbindTableRequest{
		IDs:     []int{empID},
		TableID: tableA,
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)

	assert.EqualValues(t, 0, countEmployeeTableLinks(t, db, empID, tableA), "связь снята")
	var employee models.Employee
	require.NoError(t, db.First(&employee, empID).Error)
	require.NotNil(t, employee.Status)
	assert.Equal(t, 0, *employee.Status, "последняя привязка снята -> деактивация")
	assert.NotNil(t, employee.DateDeleted)

	assert.EqualValues(t, 1, countAuditActions(t, db, empID, models.AuditActionUnboundFromTable))
	assert.EqualValues(t, 1, countAuditActions(t, db, empID, "delete"), "деактивация записана как обычный delete")
}

func TestEmployeeService_BulkUnbindTable_KeepsRemainingLink(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	tableA := seedPeopleTable(t, db, uniq("bulk_emp_unbind_keep_a"), "Unbind Keep A")
	tableB := seedPeopleTable(t, db, uniq("bulk_emp_unbind_keep_b"), "Unbind Keep B")
	empID := createTestEmployeeForBulk(t, db, uniq("UnbindKeepEmp"))
	bindEmployeeToTableForBulk(t, db, empID, tableA)
	bindEmployeeToTableForBulk(t, db, empID, tableB)
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id IN (?, ?)", tableA, tableB)
	}()

	res, err := svc.BulkUnbindTable(ctx, services.EmployeeBulkUnbindTableRequest{
		IDs:     []int{empID},
		TableID: tableA,
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)

	assert.EqualValues(t, 0, countEmployeeTableLinks(t, db, empID, tableA))
	assert.EqualValues(t, 1, countEmployeeTableLinks(t, db, empID, tableB), "оставшаяся привязка не тронута")
	var employee models.Employee
	require.NoError(t, db.First(&employee, empID).Error)
	require.NotNil(t, employee.Status)
	assert.Equal(t, 1, *employee.Status, "есть ещё одна привязка -> без деактивации")
}

// Частичный успех: несуществующий id даёт ошибку по элементу, не откатывая остальные;
// дубли в IDs схлопываются в один успех (дедуп).
func TestEmployeeService_BulkAddTable_PartialSuccessAndDedup(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	tableA := seedPeopleTable(t, db, uniq("bulk_emp_partial"), "Partial A")
	empID := createTestEmployeeForBulk(t, db, uniq("PartialEmp"))
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id = ?", tableA)
	}()

	res, err := svc.BulkAddTable(ctx, services.EmployeeBulkAddTableRequest{
		IDs:      []int{empID, empID, 999999999},
		TableIDs: []int{tableA},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount, "дубль id схлопнулся в один успех")
	assert.Equal(t, 1, res.ErrorCount)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, 999999999, res.Errors[0].ID)
	assert.Equal(t, http.StatusMultiStatus, res.HTTPStatus())
}

// HTTP-проводка (#1194): маршрут реально забинден, requireAdmin гейтит не-админа 403,
// админ получает 200 при успешной привязке.
func TestEmployeesBulkAddTable_HTTPWiringAndAdminGate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tableID := seedPeopleTable(t, db, "bulk_emp_http_table", "HTTP Table")
	empID := createTestEmployeeForBulk(t, db, "HttpEmp")

	plainToken := testutil.RegisterAndLogin(t, e, "plainuser_emp_bulk", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/employees/bulk/add-table",
		fmt.Sprintf(`{"ids":[%d],"table_ids":[%d]}`, empID, tableID),
		testutil.AuthHeader(plainToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "requireAdmin блокирует обычного пользователя")

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.POST(t, e, "/employees/bulk/add-table",
		fmt.Sprintf(`{"ids":[%d],"table_ids":[%d]}`, empID, tableID),
		testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "admin: %s", rec.Body.String())
	resp := testutil.ParseResponse[services.BulkOpResult](t, rec)
	assert.Equal(t, 1, resp.SuccessCount)
}

// Перенос сотрудника, НЕ привязанного к исходной таблице, не должен молча "переносить"
// и писать ложную запись истории (ревью S2, зеркало car-стороны del.RowsAffected==0->400).
func TestEmployeeService_BulkMoveTable_RejectsNotBoundToSource(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	fromTable := seedPeopleTable(t, db, uniq("bulk_emp_nb_from"), "NB From")
	toTable := seedPeopleTable(t, db, uniq("bulk_emp_nb_to"), "NB To")
	empID := createTestEmployeeForBulk(t, db, uniq("NotBoundEmp"))
	// намеренно НЕ привязываем к fromTable
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id IN (?, ?)", fromTable, toTable)
	}()

	res, err := svc.BulkMoveTable(ctx, services.EmployeeBulkMoveTableRequest{
		IDs:         []int{empID},
		FromTableID: fromTable,
		ToTableIDs:  []int{toTable},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 1, res.ErrorCount, "не привязан к исходной -> ошибка, а не молчаливый перенос")
	assert.EqualValues(t, 0, countEmployeeTableLinks(t, db, empID, toTable), "в целевую не добавлен")
	assert.EqualValues(t, 0, countAuditActions(t, db, empID, models.AuditActionMovedBetweenTables), "ложной записи переноса нет")
}

// Деактивированного сотрудника (status=0) групповая операция не трогает: привязки могли
// остаться от прежней жизни, повторная деактивация/аудит недопустимы (ревью S2, зеркало
// loadActiveCarForBulk).
func TestEmployeeService_BulkAddTable_SkipsInactiveEmployee(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	ctx := context.Background()

	toTable := seedPeopleTable(t, db, uniq("bulk_emp_inactive_to"), "Inactive To")
	lastName := uniq("InactiveEmp")
	firstName := "Иван"
	status := 0
	employee := models.Employee{LastName: &lastName, FirstName: &firstName, Status: &status}
	require.NoError(t, db.Create(&employee).Error)
	empID := employee.ID
	defer func() {
		db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", empID)
		db.Exec("DELETE FROM audit_log WHERE entity_type = ? AND entity_id = ?", models.AuditEntityEmployee, empID)
		db.Exec("DELETE FROM employees WHERE id = ?", empID)
		db.Exec("DELETE FROM system_tables WHERE id = ?", toTable)
	}()

	res, err := svc.BulkAddTable(ctx, services.EmployeeBulkAddTableRequest{
		IDs:      []int{empID},
		TableIDs: []int{toTable},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 1, res.ErrorCount, "неактивный сотрудник пропущен с ошибкой")
	assert.EqualValues(t, 0, countEmployeeTableLinks(t, db, empID, toTable), "привязка не создана")
}
