package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// carTargetTableIDs возвращает набор id таблиц, к которым привязана машина.
func carTargetTableIDs(t *testing.T, db *gorm.DB, carID int) map[int]bool {
	t.Helper()
	var ids []int
	require.NoError(t, db.Table("car_target_tables").Where("car_id = ?", carID).Pluck("table_id", &ids).Error)
	set := make(map[int]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func carStatus(t *testing.T, db *gorm.DB, carID int) int {
	t.Helper()
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	if car.Status == nil {
		return -1
	}
	return *car.Status
}

func TestCarsBulk_MoveTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableA := seedCarsTable(t, db, "move_a", "Move A")
	tableB := seedCarsTable(t, db, "move_b", "Move B")
	tableC := seedCarsTable(t, db, "move_c", "Move C")

	createCarIn := func(number string, tableID int) int {
		body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": %q, "car_brand": "Test"}]}`,
			td.OrgID, tableID, number)
		rec := testutil.POST(t, e, "/cars/manual", body, h)
		require.Equal(t, http.StatusOK, rec.Code, "create car: %s", rec.Body.String())
		carIDs := testutil.ParseMap(t, rec)["car_ids"].([]interface{})
		require.Len(t, carIDs, 1)
		return int(carIDs[0].(float64))
	}

	carX := createCarIn("MOVE001", tableA)
	carY := createCarIn("MOVE002", tableA)

	// Успешный перенос обеих машин из A в {B, C}.
	rec := testutil.POST(t, e, "/cars/bulk/move-table",
		fmt.Sprintf(`{"ids":[%d,%d],"from_table_id":%d,"to_table_ids":[%d,%d]}`, carX, carY, tableA, tableB, tableC), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	res := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(2), res["success_count"])
	assert.Equal(t, float64(0), res["error_count"])

	for _, carID := range []int{carX, carY} {
		set := carTargetTableIDs(t, db, carID)
		assert.False(t, set[tableA], "исходная таблица снята")
		assert.True(t, set[tableB] && set[tableC], "машина привязана к обеим целевым")
		assert.Equal(t, 1, carStatus(t, db, carID), "машина остаётся активной (не последняя таблица)")
	}

	// Аудит: запись moved_between_tables.
	var histCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityCar, carX, "moved_between_tables").
		Count(&histCount).Error)
	assert.EqualValues(t, 1, histCount, "запись переноса в аудите")

	// Последняя привязка -> деактивация: машина в единственной таблице C, move из C в пустоту деактивирует.
	carZ := createCarIn("MOVE003", tableC)
	recLast := testutil.POST(t, e, "/cars/bulk/move-table",
		fmt.Sprintf(`{"ids":[%d],"from_table_id":%d,"to_table_ids":[]}`, carZ, tableC), h)
	require.Equal(t, http.StatusOK, recLast.Code, recLast.Body.String())
	assert.Equal(t, 0, carStatus(t, db, carZ), "машина деактивирована - таблиц не осталось")
	assert.Empty(t, carTargetTableIDs(t, db, carZ))

	// Партиционирование: одна валидная машина + одна несуществующая -> 207, 1 успех/1 ошибка.
	carW := createCarIn("MOVE004", tableA)
	recPartial := testutil.POST(t, e, "/cars/bulk/move-table",
		fmt.Sprintf(`{"ids":[%d,999999],"from_table_id":%d,"to_table_ids":[%d]}`, carW, tableA, tableB), h)
	require.Equal(t, http.StatusMultiStatus, recPartial.Code)
	pres := testutil.ParseMap(t, recPartial)
	assert.Equal(t, float64(1), pres["success_count"])
	assert.Equal(t, float64(1), pres["error_count"])

	// Дедуп: повторный id в ids не удваивает success_count.
	carDup := createCarIn("MOVE005", tableA)
	recDup := testutil.POST(t, e, "/cars/bulk/move-table",
		fmt.Sprintf(`{"ids":[%d,%d],"from_table_id":%d,"to_table_ids":[%d]}`, carDup, carDup, tableA, tableB), h)
	require.Equal(t, http.StatusOK, recDup.Code)
	assert.Equal(t, float64(1), testutil.ParseMap(t, recDup)["success_count"])

	// Тип-матч: целевая таблица типа people -> 400 на весь запрос.
	peopleTable := models.SystemTable{Name: "move_people", TableType: models.TableTypePeople, IsActive: true}
	require.NoError(t, db.Create(&peopleTable).Error)
	carP := createCarIn("MOVE006", tableA)
	recBadType := testutil.POST(t, e, "/cars/bulk/move-table",
		fmt.Sprintf(`{"ids":[%d],"from_table_id":%d,"to_table_ids":[%d]}`, carP, tableA, peopleTable.ID), h)
	assert.Equal(t, http.StatusBadRequest, recBadType.Code, "нельзя перенести машину в people-таблицу")

	// Машина не привязана к заявленной исходной таблице -> ошибка по этой строке (207).
	recNotBound := testutil.POST(t, e, "/cars/bulk/move-table",
		fmt.Sprintf(`{"ids":[%d],"from_table_id":%d,"to_table_ids":[%d]}`, carP, tableB, tableC), h)
	require.Equal(t, http.StatusMultiStatus, recNotBound.Code)
	assert.Equal(t, float64(1), testutil.ParseMap(t, recNotBound)["error_count"])
}

func TestCarsBulk_AddTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableA := seedCarsTable(t, db, "add_a", "Add A")
	tableB := seedCarsTable(t, db, "add_b", "Add B")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "ADD001", "car_brand": "Test"}]}`, td.OrgID, tableA)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	carIDs := testutil.ParseMap(t, rec)["car_ids"].([]interface{})
	carID := int(carIDs[0].(float64))

	// Добавление в B: A остаётся, B добавляется (union, не replace).
	recAdd := testutil.POST(t, e, "/cars/bulk/add-table", fmt.Sprintf(`{"ids":[%d],"table_ids":[%d]}`, carID, tableB), h)
	require.Equal(t, http.StatusOK, recAdd.Code, recAdd.Body.String())
	assert.Equal(t, float64(1), testutil.ParseMap(t, recAdd)["success_count"])
	set := carTargetTableIDs(t, db, carID)
	assert.True(t, set[tableA] && set[tableB], "старая таблица сохранена, новая добавлена")
	assert.Equal(t, 1, carStatus(t, db, carID), "add-table никогда не деактивирует")

	// Повторное добавление той же таблицы - идемпотентно, без дублей в car_target_tables.
	recAgain := testutil.POST(t, e, "/cars/bulk/add-table", fmt.Sprintf(`{"ids":[%d],"table_ids":[%d]}`, carID, tableB), h)
	require.Equal(t, http.StatusOK, recAgain.Code)
	var count int64
	require.NoError(t, db.Table("car_target_tables").Where("car_id = ? AND table_id = ?", carID, tableB).Count(&count).Error)
	assert.EqualValues(t, 1, count, "нет дубля привязки")

	// Аудит: added_to_table записан на новую привязку.
	var histCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityCar, carID, "added_to_table").
		Count(&histCount).Error)
	assert.GreaterOrEqual(t, histCount, int64(1))

	// Пустой список таблиц -> 400.
	assert.Equal(t, http.StatusBadRequest,
		testutil.POST(t, e, "/cars/bulk/add-table", fmt.Sprintf(`{"ids":[%d],"table_ids":[]}`, carID), h).Code)
}

func TestCarsBulk_UnbindTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableA := seedCarsTable(t, db, "unbind_a", "Unbind A")
	tableB := seedCarsTable(t, db, "unbind_b", "Unbind B")

	// createIn создаёт машину в tableIDs[0] через /cars/manual, затем привязывает
	// её напрямую (через db) к остальным tableIDs - для случая "машина в нескольких
	// таблицах" без раздувания запроса на manual (тот принимает только одну целевую
	// таблицу страницы + опциональный target_tables у машины).
	createIn := func(number string, tableIDs ...int) int {
		body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": %q, "car_brand": "Test"}]}`,
			td.OrgID, tableIDs[0], number)
		r := testutil.POST(t, e, "/cars/manual", body, h)
		require.Equal(t, http.StatusOK, r.Code, r.Body.String())
		ids := testutil.ParseMap(t, r)["car_ids"].([]interface{})
		carID := int(ids[0].(float64))
		for _, extra := range tableIDs[1:] {
			require.NoError(t, db.Create(&models.CarTargetTable{CarID: carID, TableID: extra}).Error)
		}
		return carID
	}

	// Машина в двух таблицах: снятие одной оставляет активной во второй.
	carMulti := createIn("UNB001", tableA, tableB)
	rec := testutil.POST(t, e, "/cars/bulk/unbind-table", fmt.Sprintf(`{"ids":[%d],"table_id":%d}`, carMulti, tableA), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, float64(1), testutil.ParseMap(t, rec)["success_count"])
	set := carTargetTableIDs(t, db, carMulti)
	assert.False(t, set[tableA])
	assert.True(t, set[tableB])
	assert.Equal(t, 1, carStatus(t, db, carMulti), "остаётся активной - есть другая таблица")

	// Аудит unbound_from_table.
	var histCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityCar, carMulti, "unbound_from_table").
		Count(&histCount).Error)
	assert.EqualValues(t, 1, histCount)

	// Машина в единственной таблице: снятие -> деактивация.
	carSingle := createIn("UNB002", tableB)
	recLast := testutil.POST(t, e, "/cars/bulk/unbind-table", fmt.Sprintf(`{"ids":[%d],"table_id":%d}`, carSingle, tableB), h)
	require.Equal(t, http.StatusOK, recLast.Code)
	assert.Equal(t, 0, carStatus(t, db, carSingle), "последняя таблица -> деактивация")
	assert.Empty(t, carTargetTableIDs(t, db, carSingle))

	// Снятие с таблицы, к которой машина не привязана - ошибка по строке (207).
	carOther := createIn("UNB003", tableB)
	recNotBound := testutil.POST(t, e, "/cars/bulk/unbind-table", fmt.Sprintf(`{"ids":[%d],"table_id":%d}`, carOther, tableA), h)
	require.Equal(t, http.StatusMultiStatus, recNotBound.Code)
	assert.Equal(t, float64(1), testutil.ParseMap(t, recNotBound)["error_count"])

	// Пустой список машин -> 400.
	assert.Equal(t, http.StatusBadRequest,
		testutil.POST(t, e, "/cars/bulk/unbind-table", fmt.Sprintf(`{"ids":[],"table_id":%d}`, tableA), h).Code)
}

// TableCarResponse.target_tables_count (#1194) - FE решает по нему, показывать ли
// per-row подменю «Убрать из этой/из всех» (>1) или сразу деактивировать (1).
func TestCarsBulk_TargetTablesCount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableA := seedCarsTable(t, db, "count_a", "Count A")
	tableB := seedCarsTable(t, db, "count_b", "Count B")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "CNT001", "car_brand": "Test"}]}`, td.OrgID, tableA)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rows := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tableA), h))
	require.Len(t, rows, 1)
	assert.Equal(t, float64(1), rows[0]["target_tables_count"], "машина ровно в одной таблице")

	carID := int(rows[0]["id"].(float64))
	recAdd := testutil.POST(t, e, "/cars/bulk/add-table", fmt.Sprintf(`{"ids":[%d],"table_ids":[%d]}`, carID, tableB), h)
	require.Equal(t, http.StatusOK, recAdd.Code)

	rows2 := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tableA), h))
	require.Len(t, rows2, 1)
	assert.Equal(t, float64(2), rows2[0]["target_tables_count"], "после add-table машина в двух таблицах")
}

// Гейт requireAdmin: обычный пользователь без прав admin получает 403 на всех трёх bulk-эндпоинтах.
func TestCarsBulk_ForbiddenWithoutAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "carsbulknoperm", "pass123", 1, td.OrgID, td.CompanyID))
	tableA := seedCarsTable(t, db, "forbid_a", "Forbid A")

	cases := map[string]string{
		"/cars/bulk/move-table":   fmt.Sprintf(`{"ids":[1],"from_table_id":%d,"to_table_ids":[]}`, tableA),
		"/cars/bulk/add-table":    fmt.Sprintf(`{"ids":[1],"table_ids":[%d]}`, tableA),
		"/cars/bulk/unbind-table": fmt.Sprintf(`{"ids":[1],"table_id":%d}`, tableA),
	}
	for path, body := range cases {
		rec := testutil.POST(t, e, path, body, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s должен требовать admin", path)
	}
}
