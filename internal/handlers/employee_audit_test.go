package handlers_test

import (
	"context"
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
)

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
	testutil.GrantTableVerb(t, userID, tbl.Name, "trash")

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
// purge через trash-сервис) пишет строку в audit_log[employee].
func TestEmployees_WriteFlip_AllActionsToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empflip1", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	_, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	passTbl := seedPassTableGrant(t, db, getUserID(t, db, "empflip1"), "people")

	// Прогоняем все основные действия через endpoint-ы. restore требует предварительной
	// деактивации, поэтому delete встречается дважды.
	steps := []struct{ path, body string }{
		{fmt.Sprintf("/employees/%d/territory-status", empID), fmt.Sprintf(`{"territory_status":1,"table_id":%d}`, passTbl)}, // entry
		{fmt.Sprintf("/employees/%d/territory-status", empID), fmt.Sprintf(`{"territory_status":2,"table_id":%d}`, passTbl)}, // exit
		{fmt.Sprintf("/employees/%d/deactivate", empID), `{"status":0}`},                                                     // delete
		{fmt.Sprintf("/employees/%d/restore", empID), `{}`},                                                                  // restore
		{fmt.Sprintf("/employees/%d/deactivate", empID), `{"status":0}`},                                                     // delete (повторно)
		{fmt.Sprintf("/employees/%d/activate", empID), `{}`},                                                                 // activate
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
}
