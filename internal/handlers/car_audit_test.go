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
	testutil.GrantTableVerb(t, userID, tbl.Name, "trash")

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

	// Очистка корзины (ClearCarsTrash) скоупит машины подзапросом по audit_log.
	// Проверяем, что скоуп находит и машину, помеченную delete-строкой из audit_log.
	recClear := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recClear.Code, recClear.Body.String())
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	assert.True(t, car.IsPurged, "ClearCarsTrash через union находит машину по audit_log delete")
}

// TestCars_WriteFlip_AllActionsToAuditLog проверяет cutover записи (#870, срез 1.12c):
// каждое действие через реальный endpoint пишет строку в audit_log[car]. Заодно
// проверяет round-trip плоских полей (data_changed -> details -> union собирает
// field_name/old/new обратно).
func TestCars_WriteFlip_AllActionsToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carflip1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	passTbl := seedPassTableGrant(t, db, getUserID(t, db, "carflip1"), "cars")

	// Прогоняем все основные действия через endpoint-ы.
	steps := []struct {
		method, path, body string
	}{
		{"PUT", fmt.Sprintf("/cars/%d/territory-status", carID), fmt.Sprintf(`{"territory_status":1,"table_id":%d}`, passTbl)},
		{"PUT", fmt.Sprintf("/cars/%d/territory-status", carID), fmt.Sprintf(`{"territory_status":2,"table_id":%d}`, passTbl)},
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

	// data_changed: плоские поля собираются из details обратно в форму истории.
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
