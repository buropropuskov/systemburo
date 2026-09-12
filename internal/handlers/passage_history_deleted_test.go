package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// subjectOfLastPassage - снимок «о ком отметка» из последней записи прохода в журнале.
func subjectOfLastPassage(t *testing.T, db *gorm.DB, entityType string, entityID int) string {
	t.Helper()
	var subject string
	require.NoError(t, db.Raw(`
		SELECT COALESCE(details->>'subject', '')
		FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action IN ('entry', 'exit')
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, entityType, entityID).Scan(&subject).Error)
	return subject
}

// Отметка прохода машины несёт снимок номера с маркой, и проход остаётся в журнале даже
// после безвозвратного удаления машины (#2485).
//
// До этого журнал соединялся с cars через INNER JOIN: удалили строку - и проход исчез из
// выдачи молча, хотя в audit_log он лежит. На стенде так пропадали 156 отметок из 256, и
// это противоречит правилу, по которому журнал проходов не обезличивают: он доказывает,
// кто был на объекте.
func TestCarsHistory_KeepsPassageOfDeletedCar(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	table := seedCarsTable(t, db, "del_cars", "Таблица удалений")
	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "D001AA777", "car_brand": "Kamaz"}]}`, td.OrgID, table)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	carID := int(testutil.ParseMap(t, rec)["car_ids"].([]interface{})[0].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, table), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, "D001AA777 Kamaz", subjectOfLastPassage(t, db, "car", carID),
		"отметка обязана нести снимок номера с маркой - после удаления машины взять их больше негде")

	before := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all", h).Body)
	require.EqualValues(t, 1, before.Meta.Total)

	// Безвозвратное удаление: строки в справочнике больше нет, отметка в журнале осталась.
	require.NoError(t, db.Exec("DELETE FROM car_target_tables WHERE car_id = ?", carID).Error)
	require.NoError(t, db.Exec("DELETE FROM cars WHERE id = ?", carID).Error)

	after := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all", h).Body)
	require.EqualValues(t, 1, after.Meta.Total, "проход удалённой машины остаётся в журнале")
	row := after.Data[0]
	assert.Equal(t, true, row["entity_deleted"], "строка помечена как запись удалённой машины")
	assert.Equal(t, "D001AA777 Kamaz", row["subject"], "опознать машину даёт снимок")
	assert.Nil(t, row["car_number"], "номер из справочника взять уже негде")

	// Поиск по номеру находит её по снимку, иначе проход нельзя отыскать вовсе.
	found := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?search=D001AA", h).Body)
	assert.EqualValues(t, 1, found.Meta.Total, "поиск идёт и по снимку")

	// История таблицы тоже показывает такую запись: привязки у удалённой машины нет, но
	// у самой отметки проставлен table_id.
	scoped := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/cars/history/table/%d", table), h).Body)
	assert.EqualValues(t, 1, scoped.Meta.Total, "история поста не теряет проход удалённой машины")
}

// То же для людей: снимок ФИО в отметке и проход, который остаётся после удаления
// сотрудника. У людей потери были ещё заметнее - 172 отметки из 193 на стенде.
func TestEmployeesHistory_KeepsPassageOfDeletedEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	citizenshipID := seedCitizenship(t, db)
	table := seedPeopleTable(t, db, "del_people", "Проход удалений")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"company_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"entry_time_from": "08:00",
		"entry_time_to": "18:00",
		"employees": [{
			"last_name": "Deletov",
			"first_name": "Denis",
			"middle_name": "Dmitrievich",
			"citizenship_id": %d,
			"position": "Loader",
			"passport_series_number": "1234 567890",
			"target_tables": []
		}]
	}`, td.OrgID, td.CompanyID, table, citizenshipID)
	rec := testutil.POST(t, e, "/employees/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ids := testutil.ParseMap(t, rec)["employee_ids"].([]interface{})
	require.Len(t, ids, 1)
	employeeID := int(ids[0].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/employees/%d/territory-status", employeeID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, table), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, "Deletov Denis Dmitrievich", subjectOfLastPassage(t, db, "employee", employeeID),
		"отметка обязана нести снимок ФИО")

	require.NoError(t, db.Exec("DELETE FROM employee_target_tables WHERE employee_id = ?", employeeID).Error)
	require.NoError(t, db.Exec("DELETE FROM employees WHERE id = ?", employeeID).Error)

	page := parsePassagePage(t, testutil.GET(t, e, "/employees/history/all", h).Body)
	require.EqualValues(t, 1, page.Meta.Total, "проход удалённого сотрудника остаётся в журнале")
	row := page.Data[0]
	assert.Equal(t, true, row["entity_deleted"])
	assert.Equal(t, "Deletov Denis Dmitrievich", row["subject"])
	assert.Nil(t, row["employee_last_name"], "ФИО из справочника взять уже негде")

	found := parsePassagePage(t, testutil.GET(t, e, "/employees/history/all?search=Deletov", h).Body)
	assert.EqualValues(t, 1, found.Meta.Total, "поиск по фамилии идёт и по снимку")
}
