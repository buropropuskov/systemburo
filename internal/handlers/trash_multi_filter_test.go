package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Мультивыбор организаций в корзине таблицы (#1398). Списки корзины строятся сырым
// SQL-конкатом, а слайс id уезжает в args ...any - расширение "IN ?" проверяется
// только исполнением запроса, go build видит там обычную строку.

// seedTrashedCar подаёт заявку с машиной, приписывает заявку организации orgID и
// удаляет машину в таблицу tableID (запись delete в истории = скоуп корзины).
func seedTrashedCar(t *testing.T, e *echo.Echo, db *gorm.DB, token string, userID, tableID, orgID int) int {
	t.Helper()
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
		Update("organization_id", orgID).Error)
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, userID, tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return carID
}

// seedTrashedEmployee - то же для сотрудника: половина правки живёт в
// ListEmployeesTrash, и без people-кейса она остаётся непокрытой.
func seedTrashedEmployee(t *testing.T, e *echo.Echo, db *gorm.DB, token string, userID, tableID, orgID int) int {
	t.Helper()
	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
		Update("organization_id", orgID).Error)
	rec := testutil.PUT(t, e, fmt.Sprintf("/employees/%d/deactivate", empID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, userID, tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return empID
}

func TestTrash_MultiOrganizationFilter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashmf1", "pass123", 1, td.OrgID, td.CompanyID)
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashmf1").First(&u).Error)

	orgB := models.Organization{Name: "Корзина Орг-Б"}
	require.NoError(t, db.Create(&orgB).Error)
	orgC := models.Organization{Name: "Корзина Орг-В"}
	require.NoError(t, db.Create(&orgC).Error)

	dnCars, dnPeople := "Корзина мультифильтр авто", "Корзина мультифильтр люди"
	carsTable := models.SystemTable{Name: "trash_mf_cars", DisplayName: &dnCars, TableType: "cars", IsActive: true}
	peopleTable := models.SystemTable{Name: "trash_mf_people", DisplayName: &dnPeople, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&carsTable).Error)
	require.NoError(t, db.Create(&peopleTable).Error)
	testutil.GrantTableVerb(t, u.ID, carsTable.Name, "trash")
	testutil.GrantTableVerb(t, u.ID, peopleTable.Name, "trash")

	carA := seedTrashedCar(t, e, db, token, u.ID, carsTable.ID, td.OrgID)
	carB := seedTrashedCar(t, e, db, token, u.ID, carsTable.ID, orgB.ID)
	carC := seedTrashedCar(t, e, db, token, u.ID, carsTable.ID, orgC.ID)

	empA := seedTrashedEmployee(t, e, db, token, u.ID, peopleTable.ID, td.OrgID)
	empB := seedTrashedEmployee(t, e, db, token, u.ID, peopleTable.ID, orgB.ID)

	idsFor := func(t *testing.T, tableID int, query string) map[int]bool {
		t.Helper()
		rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash%s", tableID, query), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		ids := map[int]bool{}
		for _, row := range testutil.ParseSlice(t, rec) {
			if id, ok := row["id"].(float64); ok {
				ids[int(id)] = true
			}
		}
		return ids
	}

	t.Run("organization_ids - обе организации в выборке", func(t *testing.T) {
		ids := idsFor(t, carsTable.ID, fmt.Sprintf("?organization_ids=%d,%d", td.OrgID, orgB.ID))
		assert.True(t, ids[carA], "машина первой организации должна быть")
		assert.True(t, ids[carB], "машина второй организации должна быть")
		assert.False(t, ids[carC], "машина третьей организации не должна попасть")
	})

	t.Run("organization_ids с одним значением сужает до него", func(t *testing.T) {
		ids := idsFor(t, carsTable.ID, fmt.Sprintf("?organization_ids=%d", orgC.ID))
		assert.True(t, ids[carC])
		assert.False(t, ids[carA], "чужая организация не должна попасть")
		assert.False(t, ids[carB], "чужая организация не должна попасть")
	})

	t.Run("мусор в параметре не роняет запрос и не фильтрует", func(t *testing.T) {
		// Опечатка в query не должна отдавать 500 и не должна выглядеть как "корзина
		// пуста": невалидные элементы отбрасываются, пустой список = фильтр не применён.
		ids := idsFor(t, carsTable.ID, "?organization_ids=abc")
		assert.True(t, ids[carA] && ids[carB] && ids[carC], "мусорный фильтр не должен сужать выборку: %v", ids)
	})

	t.Run("одиночный organization_id продолжает работать", func(t *testing.T) {
		ids := idsFor(t, carsTable.ID, fmt.Sprintf("?organization_id=%d", orgB.ID))
		assert.True(t, ids[carB])
		assert.False(t, ids[carA])
		assert.False(t, ids[carC])
	})

	t.Run("одиночный и множественный комбинируются по И", func(t *testing.T) {
		ids := idsFor(t, carsTable.ID, fmt.Sprintf("?organization_id=%d&organization_ids=%d", td.OrgID, orgB.ID))
		assert.Empty(t, ids, "непересекающиеся условия дают пустую выборку: %v", ids)
	})

	t.Run("сотрудники: мультивыбор и сужение", func(t *testing.T) {
		both := idsFor(t, peopleTable.ID, fmt.Sprintf("?organization_ids=%d,%d", td.OrgID, orgB.ID))
		assert.True(t, both[empA] && both[empB], "оба сотрудника должны быть: %v", both)

		one := idsFor(t, peopleTable.ID, fmt.Sprintf("?organization_ids=%d", orgB.ID))
		assert.True(t, one[empB])
		assert.False(t, one[empA], "сотрудник чужой организации не должен попасть")
	})
}
