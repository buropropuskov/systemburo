package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrash_CarFlowListScopingRestoreHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashcar1", "pass123", 1, td.OrgID, td.CompanyID)

	// Имя пользователю -> deleted_by_name не пустой.
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashcar1").First(&u).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"last_name": "Иванов", "first_name": "Иван", "middle_name": "Иванович",
	}).Error)

	// Две cars-таблицы для проверки скоупинга.
	dn1, dn2 := "Корзина КПП 1", "Корзина КПП 2"
	t1 := models.SystemTable{Name: "trash_cars_t1", DisplayName: &dn1, TableType: "cars", IsActive: true}
	t2 := models.SystemTable{Name: "trash_cars_t2", DisplayName: &dn2, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&t1).Error)
	require.NoError(t, db.Create(&t2).Error)
	testutil.GrantTableVerb(t, u.ID, t1.Name, "trash")

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Удаление из таблицы t1 с указанием table_id.
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, t1.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Корзина t1: ровно 1 элемент с заполненными полями.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", t1.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 1)
	assert.Equal(t, float64(carID), items[0]["id"])
	assert.Equal(t, "B002BB799", items[0]["car_number"])
	assert.Equal(t, "Test Organization", items[0]["organization"])
	assert.NotNil(t, items[0]["application_id"], "application_id нужен для кнопки 'Открыть заявку'")
	assert.Greater(t, items[0]["application_id"], float64(0))
	assert.NotEmpty(t, items[0]["deleted_by_name"], "deleted_by_name должен заполняться")
	assert.NotEmpty(t, items[0]["entry_date_to"], "Действует до должно заполняться")
	assert.NotEmpty(t, items[0]["deleted_at"])

	// Скоупинг: корзина t2 пуста.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", t2.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.ParseSlice(t, rec), "удалённое из t1 не должно показываться в t2")

	// Восстановление.
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", t1.ID),
		fmt.Sprintf(`{"ids":[%d]}`, carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	restoreResp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(1), restoreResp["restored"])

	// Машина снова активна и пропала из корзины.
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	require.NotNil(t, car.Status)
	assert.Equal(t, 1, *car.Status)
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", t1.ID), testutil.AuthHeader(token))
	assert.Empty(t, testutil.ParseSlice(t, rec))

	// История корзины: запись bulk_restored.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", t1.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, history)
	assert.Equal(t, "bulk_restored", history[0]["action_type"])
	assert.Equal(t, float64(1), history[0]["affected_count"])
}

func TestTrash_RestoreBlockedWithoutApprovedApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashcar2", "pass123", 1, td.OrgID, td.CompanyID)
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashcar2").First(&u).Error)

	dn := "Корзина КПП"
	tbl := models.SystemTable{Name: "trash_cars_blocked", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	testutil.GrantTableVerb(t, u.ID, tbl.Name, "trash")

	// Машина без согласованной заявки (не активируем).
	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// В корзине есть, но восстановление запрещено.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	require.Len(t, testutil.ParseSlice(t, rec), 1)

	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", tbl.ID),
		fmt.Sprintf(`{"ids":[%d]}`, carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(0), resp["restored"], "без согласованной заявки восстановление невозможно")
}

func TestTrash_PurgeOneRemovesFromTrash(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashcar3", "pass123", 1, td.OrgID, td.CompanyID)
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashcar3").First(&u).Error)

	dn := "Корзина КПП"
	tbl := models.SystemTable{Name: "trash_cars_purge", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	testutil.GrantTableVerb(t, u.ID, tbl.Name, "trash")

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Окончательное удаление.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/trash/%d", tbl.ID, carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Пропала из корзины, is_purged=true, есть history purge.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	assert.Empty(t, testutil.ParseSlice(t, rec))

	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	assert.True(t, car.IsPurged)

	var purgeCount int64
	db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = 'purge'", models.AuditEntityCar, carID).Count(&purgeCount)
	assert.Equal(t, int64(1), purgeCount)
}

func TestTrash_ClearAllCars_PurgesAndLogsDetails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashclear1", "pass123", 1, td.OrgID, td.CompanyID)
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashclear1").First(&u).Error)

	dn := "Корзина Очистка"
	tbl := models.SystemTable{Name: "trash_cars_clear", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	testutil.GrantTableVerb(t, u.ID, tbl.Name, "trash")

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	require.Len(t, testutil.ParseSlice(t, rec), 1)

	// Очистка корзины (ClearAll).
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Корзина пуста, машина окончательно удалена.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", tbl.ID), testutil.AuthHeader(token))
	assert.Empty(t, testutil.ParseSlice(t, rec))
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	assert.True(t, car.IsPurged)

	// История: cleared с деталями (id + label заполнены).
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, history)
	assert.Equal(t, "cleared", history[0]["action_type"])
	assert.Equal(t, float64(1), history[0]["affected_count"])
	details, ok := history[0]["details"].([]interface{})
	require.True(t, ok, "details должен быть массивом, got %v", history[0]["details"])
	require.Len(t, details, 1)
	d0 := details[0].(map[string]interface{})
	assert.Equal(t, float64(carID), d0["id"])
	assert.NotEmpty(t, d0["label"], "label детали должен заполняться (номер + марка)")
}

func TestTrash_EmployeeFlow_ScopingRestoreHistoryDetails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashemp1", "pass123", 1, td.OrgID, td.CompanyID)
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashemp1").First(&u).Error)

	dn1, dn2 := "Корзина Сотр 1", "Корзина Сотр 2"
	t1 := models.SystemTable{Name: "trash_emp_t1", DisplayName: &dn1, TableType: "people", IsActive: true}
	t2 := models.SystemTable{Name: "trash_emp_t2", DisplayName: &dn2, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&t1).Error)
	require.NoError(t, db.Create(&t2).Error)
	testutil.GrantTableVerb(t, u.ID, t1.Name, "trash")

	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td) // согласовать + в работу: нужно для restore

	// Деактивация с table_id -> сотрудник в корзину t1.
	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/deactivate", empID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, t1.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Корзина t1: 1 сотрудник; скоупинг - t2 пусто.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", t1.ID), testutil.AuthHeader(token))
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 1)
	assert.Equal(t, float64(empID), items[0]["id"])
	assert.Equal(t, "employee", items[0]["type"])
	assert.NotEmpty(t, items[0]["last_name"])

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", t2.ID), testutil.AuthHeader(token))
	assert.Empty(t, testutil.ParseSlice(t, rec), "удалённое из t1 не должно показываться в t2")

	// Восстановление.
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", t1.ID),
		fmt.Sprintf(`{"ids":[%d]}`, empID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, float64(1), testutil.ParseMap(t, rec)["restored"])

	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 1, *emp.Status)
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash", t1.ID), testutil.AuthHeader(token))
	assert.Empty(t, testutil.ParseSlice(t, rec))

	// История: bulk_restored с деталью (ФИО в label).
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", t1.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, history)
	assert.Equal(t, "bulk_restored", history[0]["action_type"])
	assert.Equal(t, float64(1), history[0]["affected_count"])
	details, ok := history[0]["details"].([]interface{})
	require.True(t, ok, "details должен быть массивом, got %v", history[0]["details"])
	require.Len(t, details, 1)
	assert.NotEmpty(t, details[0].(map[string]interface{})["label"], "label детали - ФИО сотрудника")
}
