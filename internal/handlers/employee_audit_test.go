package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmployees_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.6):
// читатель истории сотрудника переведён на audit_log-only, а до-cutover строки
// замороженной employees_history разово поднимаются в audit_log через
// BackfillAuditFromLegacy. Плоские поля и metadata сворачиваются в details в форму
// carAuditDetails - то же, что пишет recorder, поэтому история байт-в-байт совпадает
// с до-switch union. Зеркало F.5 car.
func TestEmployees_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empunion1", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	userID := getUserID(t, db, "empunion1")

	// После cutover (1.13b) submit пишет 'create' уже в audit_log, не в employees_history.
	var auditCreate int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = 'create'", models.AuditEntityEmployee, empID).Count(&auditCreate).Error)
	require.Equal(t, int64(1), auditCreate, "submit пишет create в audit_log")

	now := time.Now().UTC()
	// Новое событие выхода - напрямую в audit_log (как пишет recorder после cutover).
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

	// Замороженная legacy-строка в employees_history (накоплена до cutover, ещё не перенесена).
	// metadata в плоской jsonb-колонке проверяет, что backfill переносит её через
	// details->'metadata', а не теряет.
	legacyComment := "Вход (legacy employees_history)"
	legacyMeta := `{"src":"legacy"}`
	require.NoError(t, db.Create(&models.EmployeeHistory{
		EmployeeID: empID, ActionType: "entry", Comment: &legacyComment,
		Metadata: &legacyMeta, CreatedAt: now.Add(-time.Hour),
	}).Error)

	// До backfill читатель видит только audit_log -> legacy entry ещё невидим.
	rec := testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Nil(t, findHistByAction(testutil.ParseSlice(t, rec), "entry"), "до backfill legacy entry невидим")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityEmployee).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Старая таблица не тронута backfill'ом - read-only бэкап.
	var legacyCount int64
	require.NoError(t, db.Model(&models.EmployeeHistory{}).Where("employee_id = ? AND action_type = 'entry'", empID).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "employees_history цела как бэкап")

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	hist := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(hist), 4, "create + exit + data_changed + перенесённый legacy entry")

	// Новейшее сверху (data_changed позже exit; legacy entry на час раньше).
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

	// Перенесённая legacy-строка видна, comment и metadata сохранены через backfill.
	entryRow := findHistByAction(hist, "entry")
	require.NotNil(t, entryRow, "legacy entry поднят в audit_log backfill'ом")
	assert.Equal(t, "Вход (legacy employees_history)", entryRow["comment"], "comment перенесён из employees_history")
	assert.Equal(t, `{"src": "legacy"}`, entryRow["metadata"], "metadata перенесён через details->'metadata' (как text)")

	// /employees/history/all фильтрует action_type IN ('entry','exit') - и audit-exit, и
	// перенесённый legacy-entry попадают под фильтр.
	recAll := testutil.GET(t, e, "/employees/history/all", h)
	require.Equal(t, http.StatusOK, recAll.Code, recAll.Body.String())
	var foundExit, foundEntry bool
	for _, it := range testutil.ParseSlice(t, recAll) {
		if it["employee_id"] != float64(empID) {
			continue
		}
		switch it["action_type"] {
		case "exit":
			foundExit = true
			assert.Equal(t, "Выход через КПП", it["comment"])
		case "entry":
			foundEntry = true
		}
	}
	assert.True(t, foundExit, "событие exit из audit_log видно в /employees/history/all")
	assert.True(t, foundEntry, "перенесённый legacy entry виден в /employees/history/all")

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", empID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var entryCount int
	for _, it := range testutil.ParseSlice(t, rec) {
		if it["action_type"] == "entry" {
			entryCount++
		}
	}
	assert.Equal(t, 1, entryCount, "повторный backfill не создаёт дублей legacy entry")
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

// TestEmployees_WriteFlip_AllActionsToAuditLog проверяет cutover записи (#870, срез
// 1.13b): каждое действие (entry/exit/delete/activate/restore через employee-endpoint,
// purge через trash-сервис) пишет строку в audit_log[employee], а замороженная
// employees_history больше НЕ растёт.
func TestEmployees_WriteFlip_AllActionsToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empflip1", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	// employees_history заморожена: submit уже не пишет в неё (create -> audit_log).
	var legacyBefore int64
	require.NoError(t, db.Model(&models.EmployeeHistory{}).Where("employee_id = ?", empID).Count(&legacyBefore).Error)

	// Прогоняем все основные действия через endpoint-ы. restore требует предварительной
	// деактивации, поэтому delete встречается дважды.
	steps := []struct{ path, body string }{
		{fmt.Sprintf("/employees/%d/territory-status", empID), `{"territory_status":1}`}, // entry
		{fmt.Sprintf("/employees/%d/territory-status", empID), `{"territory_status":2}`}, // exit
		{fmt.Sprintf("/employees/%d/deactivate", empID), `{"status":0}`},                 // delete
		{fmt.Sprintf("/employees/%d/restore", empID), `{}`},                              // restore
		{fmt.Sprintf("/employees/%d/deactivate", empID), `{"status":0}`},                 // delete (повторно)
		{fmt.Sprintf("/employees/%d/activate", empID), `{}`},                             // activate
	}
	for _, s := range steps {
		rec := testutil.PUT(t, e, s.path, s.body, h)
		require.Equal(t, http.StatusOK, rec.Code, "PUT %s: %s", s.path, rec.Body.String())
	}

	// purge идёт через trashService.PurgeEmployee (tx-critical Record), отдельный
	// от employee-endpoint путь - доводим сотрудника до status=0 и вычищаем из корзины.
	recDel := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/deactivate", empID), `{"status":0}`, h)
	require.Equal(t, http.StatusOK, recDel.Code, recDel.Body.String())
	trashSvc := services.NewTrashService(db, services.NewAuditRecorder(db))
	require.NoError(t, trashSvc.PurgeEmployee(context.Background(), seedSystemTable(t, db), empID, getUserID(t, db, "empflip1")))

	// Каждое действие пишет строку в audit_log[employee] (create - от submit заявки).
	for _, action := range []string{"create", "entry", "exit", "delete", "activate", "restore", "purge"} {
		var n int64
		require.NoError(t, db.Model(&models.AuditLog{}).
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, action).
			Count(&n).Error)
		assert.GreaterOrEqualf(t, n, int64(1), "действие %q должно писаться в audit_log", action)
	}

	// employees_history НЕ выросла - старый write-path убран.
	var legacyAfter int64
	require.NoError(t, db.Model(&models.EmployeeHistory{}).Where("employee_id = ?", empID).Count(&legacyAfter).Error)
	assert.Equal(t, legacyBefore, legacyAfter, "employees_history не должна расти после cutover")
}
