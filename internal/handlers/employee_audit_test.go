package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmployees_History_UnionLegacyAndAudit проверяет переходную модель #870 (срез
// 1.13a): чтение истории сотрудника объединяет employees_history (legacy-строки,
// записываются текущим write-path) и новые строки из audit_log[employee]. Запись
// ещё НЕ переведена на recorder (это 1.13b), поэтому audit_log-строки вставляются
// вручную - так проверяем, что union читает ОБЕ таблицы в прежнюю форму ответа.
func TestEmployees_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empunion1", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	userID := getUserID(t, db, "empunion1")

	// Legacy-путь ещё активен: entry пишется в employees_history (frozen после 1.13b).
	putBody := `{"territory_status":1,"user_id":null}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/territory-status", empID), putBody, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var legacyEntry int64
	require.NoError(t, db.Model(&models.EmployeeHistory{}).
		Where("employee_id = ? AND action_type = 'entry'", empID).Count(&legacyEntry).Error)
	require.Equal(t, int64(1), legacyEntry, "entry пишется в employees_history (до cutover)")

	now := time.Now().UTC()
	// Новое событие выхода - напрямую в audit_log (как будет после среза 1.13b).
	exit := models.AuditLog{
		EntityType:  models.AuditEntityEmployee,
		EntityID:    &empID,
		Action:      "exit",
		ActorUserID: &userID,
		Details:     json.RawMessage(`{"comment":"Выход через КПП"}`),
		CreatedAt:   now.Add(time.Second),
	}
	require.NoError(t, db.Create(&exit).Error)
	// Поячеечный diff - проверяем сборку плоских полей из details (field_name/old/new/metadata).
	changed := models.AuditLog{
		EntityType:  models.AuditEntityEmployee,
		EntityID:    &empID,
		Action:      "data_changed",
		ActorUserID: &userID,
		Details:     json.RawMessage(`{"field_name":"position","old_value":"Грузчик","new_value":"Водитель","metadata":{"src":"manual"}}`),
		CreatedAt:   now.Add(2 * time.Second),
	}
	require.NoError(t, db.Create(&changed).Error)

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	hist := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(hist), 3, "union: legacy entry/create + audit exit + audit data_changed")

	// Новейшее сверху (data_changed позже exit).
	assert.Equal(t, "data_changed", hist[0]["action_type"], "новейшее сверху")

	exitRow := findHistByAction(hist, "exit")
	require.NotNil(t, exitRow, "событие exit из audit_log должно быть в истории")
	assert.Equal(t, "Выход через КПП", exitRow["comment"])
	assert.NotEmpty(t, exitRow["user_name"], "актор из audit_log разрезолвлен")

	changedRow := findHistByAction(hist, "data_changed")
	require.NotNil(t, changedRow)
	assert.Equal(t, "position", changedRow["field_name"], "field_name собран из details")
	assert.Equal(t, "Грузчик", changedRow["old_value"])
	assert.Equal(t, "Водитель", changedRow["new_value"])
	// EmployeeHistoryItem.Metadata - *string (baseSelectSQL отдаёт metadata::text),
	// поэтому контракт employee возвращает metadata строкой jsonb, не объектом.
	assert.Equal(t, `{"src": "manual"}`, changedRow["metadata"], "metadata собран из details->'metadata' (как text)")

	// Замороженная строка из employees_history (entry) по-прежнему видна через union.
	entryRow := findHistByAction(hist, "entry")
	require.NotNil(t, entryRow, "legacy entry из employees_history виден через union")

	// /employees/history/all фильтрует action_type IN ('entry','exit') - audit-ветка
	// union (exit) тоже должна попадать под фильтр.
	recAll := testutil.GET(t, e, "/employees/history/all", h)
	require.Equal(t, http.StatusOK, recAll.Code, recAll.Body.String())
	var foundExit bool
	for _, it := range testutil.ParseSlice(t, recAll) {
		if it["employee_id"] == float64(empID) && it["action_type"] == "exit" {
			foundExit = true
			assert.Equal(t, "Выход через КПП", it["comment"])
		}
	}
	assert.True(t, foundExit, "событие exit из audit_log видно в /employees/history/all")
}

// TestEmployees_Trash_UnionAuditDeleteRow проверяет, что корзина скоупит удалённых
// сотрудников и через audit_log: запись delete с table_id внутри details попадает в
// EXISTS-скоуп и в подзапрос deleted_by_name. Без этого после переноса записи (1.13b)
// удалённые сотрудники пропали бы из корзины.
func TestEmployees_Trash_UnionAuditDeleteRow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "emptrashunion", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	userID := getUserID(t, db, "emptrashunion")
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"last_name": "Петров", "first_name": "Пётр", "middle_name": "Петрович",
	}).Error)

	dn := "Корзина Сотр union"
	tbl := models.SystemTable{Name: "trash_emp_union", DisplayName: &dn, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	// Имитируем пост-cutover мягкое удаление: статус/дата как ставит DeactivateEmployee,
	// а событие delete с table_id - напрямую в audit_log (как будет после 1.13b).
	now := time.Now().UTC()
	require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", empID).Updates(map[string]any{
		"status": 0, "date_deleted": now,
	}).Error)
	del := models.AuditLog{
		EntityType:  models.AuditEntityEmployee,
		EntityID:    &empID,
		Action:      "delete",
		ActorUserID: &userID,
		Details:     json.RawMessage(fmt.Sprintf(`{"table_id":%d,"comment":"Удалён"}`, tbl.ID)),
		CreatedAt:   now,
	}
	require.NoError(t, db.Create(&del).Error)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 1, "delete-строка из audit_log должна попасть в скоуп корзины")
	assert.Equal(t, float64(empID), items[0]["id"])
	assert.Equal(t, "employee", items[0]["type"])
	assert.NotEmpty(t, items[0]["deleted_by_name"], "deleted_by_name собран через union из audit_log")

	// Очистка корзины (ClearEmployeesTrash) скоупит сотрудников подзапросом по union.
	// Проверяем, что union находит и сотрудника, помеченного delete-строкой из audit_log.
	recClear := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), h)
	require.Equal(t, http.StatusOK, recClear.Code, recClear.Body.String())
	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	assert.True(t, emp.IsPurged, "ClearEmployeesTrash через union находит сотрудника по audit_log delete")
}
