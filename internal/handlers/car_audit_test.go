package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// TestCars_History_UnionLegacyAndAudit проверяет переходную модель #870: чтение
// истории машины объединяет замороженную cars_history (legacy-строки до cutover) и
// новые строки из audit_log[car]. После среза 1.12c запись переведена на recorder -
// submit пишет 'create' уже в audit_log; legacy-строка вставляется вручную, чтобы
// проверить, что union по-прежнему читает frozen-таблицу (плоские поля из details).
func TestCars_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carunion1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	userID := getUserID(t, db, "carunion1")

	// После cutover (1.12c) submit пишет 'create' уже в audit_log, не в cars_history.
	var auditCreate int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = 'create'", models.AuditEntityCar, carID).Count(&auditCreate).Error)
	require.Equal(t, int64(1), auditCreate, "submit пишет create в audit_log")

	now := time.Now().UTC()
	// Замороженная legacy-строка в cars_history (осталась с до-cutover) - union обязан её читать.
	legacyComment := "Выезд (legacy cars_history)"
	require.NoError(t, db.Create(&models.CarHistory{
		CarID: carID, ActionType: "exit", Comment: &legacyComment, CreatedAt: now.Add(-time.Hour),
	}).Error)
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
	require.GreaterOrEqual(t, len(hist), 4, "union: audit create + legacy exit + entry + data_changed")

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

	// Замороженная строка из cars_history по-прежнему видна через union.
	legacyRow := findHistByAction(hist, "exit")
	require.NotNil(t, legacyRow, "legacy exit из cars_history виден через union")
	assert.Equal(t, "Выезд (legacy cars_history)", legacyRow["comment"], "comment из замороженной cars_history")

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

// TestCars_WriteFlip_AllActionsToAuditLog проверяет cutover записи (#870, срез 1.12c):
// каждое действие через реальный endpoint пишет строку в audit_log[car], а замороженная
// cars_history больше НЕ растёт. Заодно проверяет round-trip плоских полей (data_changed
// -> details -> union собирает field_name/old/new обратно).
func TestCars_WriteFlip_AllActionsToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carflip1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// cars_history заморожена: submit уже не пишет в неё. Фиксируем счётчик до действий.
	var legacyBefore int64
	require.NoError(t, db.Model(&models.CarHistory{}).Where("car_id = ?", carID).Count(&legacyBefore).Error)

	// Прогоняем все основные действия через endpoint-ы.
	steps := []struct {
		method, path, body string
	}{
		{"PUT", fmt.Sprintf("/cars/%d/territory-status", carID), `{"territory_status":1}`},
		{"PUT", fmt.Sprintf("/cars/%d/territory-status", carID), `{"territory_status":2}`},
		{"POST", fmt.Sprintf("/cars/%d/history", carID), `{"action_type":"data_changed","field_name":"car_number","old_value":"A001AA","new_value":"B002BB"}`},
		{"PUT", fmt.Sprintf("/cars/%d/deactivate", carID), `{"status":2}`},
		{"PUT", fmt.Sprintf("/cars/%d/activate", carID), `{}`},
	}
	for _, s := range steps {
		var rec *httptest.ResponseRecorder
		switch s.method {
		case "POST":
			rec = testutil.POST(t, e, s.path, s.body, testutil.AuthHeader(token))
		default:
			rec = testutil.PUT(t, e, s.path, s.body, testutil.AuthHeader(token))
		}
		require.Equal(t, http.StatusOK, rec.Code, "%s %s: %s", s.method, s.path, rec.Body.String())
	}

	// Каждое действие пишет строку в audit_log[car] (create - от submit).
	for _, action := range []string{"create", "entry", "exit", "data_changed", "delete", "activate"} {
		var n int64
		require.NoError(t, db.Model(&models.AuditLog{}).
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityCar, carID, action).
			Count(&n).Error)
		assert.GreaterOrEqualf(t, n, int64(1), "действие %q должно писаться в audit_log", action)
	}

	// cars_history НЕ выросла - старый write-path убран.
	var legacyAfter int64
	require.NoError(t, db.Model(&models.CarHistory{}).Where("car_id = ?", carID).Count(&legacyAfter).Error)
	assert.Equal(t, legacyBefore, legacyAfter, "cars_history не должна расти после cutover")

	// data_changed: плоские поля собираются из details обратно в форму cars_history.
	rec := testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	changed := findHistByAction(testutil.ParseSlice(t, rec), "data_changed")
	require.NotNil(t, changed, "data_changed виден в истории через union")
	assert.Equal(t, "car_number", changed["field_name"])
	assert.Equal(t, "A001AA", changed["old_value"])
	assert.Equal(t, "B002BB", changed["new_value"])
}

// TestCars_WriteFlip_RestoreToAuditLog проверяет что PUT /cars/:id/restore пишет
// action='restore' в audit_log (#870, срез 1.12c). TestRestoreCar_Success ассертит
// только HTTP 200; этот тест закрывает непокрытую проверку записи в audit_log.
func TestCars_WriteFlip_RestoreToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carrestore_audit1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	h := testutil.AuthHeader(token)

	// Деактивируем машину чтобы restore был валиден (предусловие из TestRestoreCar_Success).
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID), `{"status": 2}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Восстанавливаем через endpoint.
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/restore", carID), `{}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Проверяем что action='restore' попал в audit_log[car].
	var n int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityCar, carID, "restore").
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "PUT /cars/:id/restore должен писать action=restore в audit_log")
}
