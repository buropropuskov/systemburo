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

// findHistByAction возвращает первую запись истории с заданным action_type.
func findHistByAction(items []map[string]interface{}, action string) map[string]interface{} {
	for _, it := range items {
		if it["action_type"] == action {
			return it
		}
	}
	return nil
}

// TestCars_History_UnionLegacyAndAudit проверяет переходную модель #870 (срез 1.12a):
// чтение истории машины объединяет замороженную cars_history (legacy create от заявки)
// и новые строки из audit_log[car]. Запись ещё НЕ перенесена (1.12c) - строки audit_log
// вставляются напрямую, имитируя пост-cutover события. Гарантирует, что после переноса
// записи история останется той же формы (плоские поля собираются из details).
func TestCars_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carunion1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	userID := getUserID(t, db, "carunion1")

	// Legacy 'create' уже в cars_history (его пишет submit заявки).
	var legacyCreate int64
	require.NoError(t, db.Model(&models.CarHistory{}).
		Where("car_id = ? AND action_type = 'create'", carID).Count(&legacyCreate).Error)
	require.Equal(t, int64(1), legacyCreate, "submit заявки пишет legacy create в cars_history")

	now := time.Now().UTC()
	// Новое событие въезда - напрямую в audit_log (как после среза 1.12c).
	entry := models.AuditLog{
		EntityType:  models.AuditEntityCar,
		EntityID:    &carID,
		Action:      "entry",
		ActorUserID: &userID,
		Details:     json.RawMessage(`{"comment":"Въезд через КПП"}`),
		CreatedAt:   now,
	}
	require.NoError(t, db.Create(&entry).Error)
	// Поячеечный diff - проверяем сборку плоских полей из details (field_name/old/new/metadata).
	changed := models.AuditLog{
		EntityType:  models.AuditEntityCar,
		EntityID:    &carID,
		Action:      "data_changed",
		ActorUserID: &userID,
		Details:     json.RawMessage(`{"field_name":"car_number","old_value":"A001AA","new_value":"B002BB","metadata":{"src":"manual"}}`),
		CreatedAt:   now.Add(time.Second),
	}
	require.NoError(t, db.Create(&changed).Error)

	rec := testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	hist := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(hist), 3, "union: legacy create + entry + data_changed")

	// Новейшее сверху (data_changed позже entry позже create).
	assert.Equal(t, "data_changed", hist[0]["action_type"], "новейшее сверху")

	entryRow := findHistByAction(hist, "entry")
	require.NotNil(t, entryRow, "событие entry из audit_log должно быть в истории")
	assert.Equal(t, "Въезд через КПП", entryRow["comment"])
	assert.NotEmpty(t, entryRow["user_name"], "актор из audit_log разрезолвлен")

	changedRow := findHistByAction(hist, "data_changed")
	require.NotNil(t, changedRow)
	assert.Equal(t, "car_number", changedRow["field_name"], "field_name собран из details")
	assert.Equal(t, "A001AA", changedRow["old_value"])
	assert.Equal(t, "B002BB", changedRow["new_value"])
	assert.Equal(t, map[string]interface{}{"src": "manual"}, changedRow["metadata"], "metadata собран из details->'metadata'")

	// Legacy create по-прежнему виден через union.
	assert.NotNil(t, findHistByAction(hist, "create"), "legacy create из cars_history виден")

	// /cars/history/all фильтрует action_type IN ('entry','exit') - audit-ветка union
	// тоже должна попадать под фильтр.
	recAll := testutil.GET(t, e, "/cars/history/all", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recAll.Code, recAll.Body.String())
	var foundEntry bool
	for _, it := range testutil.ParseSlice(t, recAll) {
		if it["car_id"] == float64(carID) && it["action_type"] == "entry" {
			foundEntry = true
			assert.Equal(t, "Въезд через КПП", it["comment"])
		}
	}
	assert.True(t, foundEntry, "событие entry из audit_log видно в /cars/history/all")
}

// TestCars_Trash_UnionAuditDeleteRow проверяет, что корзина скоупит удалённые машины
// и через audit_log: запись delete с table_id внутри details попадает в EXISTS-скоуп
// и в подзапрос deleted_by_name. Без этого после переноса записи (1.12c) удалённые
// машины пропали бы из корзины.
func TestCars_Trash_UnionAuditDeleteRow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "cartrashunion", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "cartrashunion")
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"last_name": "Петров", "first_name": "Пётр", "middle_name": "Петрович",
	}).Error)

	dn := "Корзина КПП union"
	tbl := models.SystemTable{Name: "trash_cars_union", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")

	// Имитируем пост-cutover мягкое удаление: статус/дата как ставит DeactivateCar,
	// а событие delete с table_id - напрямую в audit_log (как будет после 1.12c).
	now := time.Now().UTC()
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]any{
		"status": 0, "date_removed": now,
	}).Error)
	del := models.AuditLog{
		EntityType:  models.AuditEntityCar,
		EntityID:    &carID,
		Action:      "delete",
		ActorUserID: &userID,
		Details:     json.RawMessage(fmt.Sprintf(`{"table_id":%d,"comment":"Удалён"}`, tbl.ID)),
		CreatedAt:   now,
	}
	require.NoError(t, db.Create(&del).Error)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 1, "delete-строка из audit_log должна попасть в скоуп корзины")
	assert.Equal(t, float64(carID), items[0]["id"])
	assert.NotEmpty(t, items[0]["deleted_by_name"], "deleted_by_name собран через union из audit_log")

	// Очистка корзины (ClearCarsTrash) скоупит машины подзапросом по cars_history.
	// Проверяем, что union находит и машину, помеченную delete-строкой из audit_log.
	recClear := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recClear.Code, recClear.Body.String())
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	assert.True(t, car.IsPurged, "ClearCarsTrash через union находит машину по audit_log delete")
}
