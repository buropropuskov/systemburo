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

// Снятие, возврат и восстановление элементов поста требуют права table.<name>.delete
// на пост элемента; запись в журнал машины - администратора (#2600).

type postGateFixture struct {
	carID, empID, unboundCarID int
	tableName, otherTableName  string
}

func seedPostGateFixture(t *testing.T, db *gorm.DB) postGateFixture {
	t.Helper()
	org := models.Organization{Name: "Организация поста #2600"}
	require.NoError(t, db.Create(&org).Error)
	senderID := seedDatesUser(t, db, "post_gate_sender", org.ID)
	inWork, approved := models.StatusInWork, models.ConfirmationApproved
	num := "POST-GATE-1"
	app := models.Application{ApplicationNumber: &num, Status: &inWork, Confirmation: &approved, OrganizationID: org.ID, SenderUserID: senderID}
	require.NoError(t, db.Create(&app).Error)

	st, from, to := 1, "2020-01-01", "2099-12-31"
	attC := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", Status: &st, EntryDateFrom: &from, EntryDateTo: &to}
	require.NoError(t, db.Create(&attC).Error)
	plate, plate2, ln := "Р200ОС77", "Р201ОС77", "Постовой"
	car := models.Car{AttachmentID: attC.ID, CarNumber: &plate, Status: &st}
	require.NoError(t, db.Create(&car).Error)
	unbound := models.Car{AttachmentID: attC.ID, CarNumber: &plate2, Status: &st}
	require.NoError(t, db.Create(&unbound).Error)
	attP := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", Status: &st, EntryDateFrom: &from, EntryDateTo: &to}
	require.NoError(t, db.Create(&attP).Error)
	emp := models.Employee{AttachmentID: &attP.ID, LastName: &ln, Status: &st}
	require.NoError(t, db.Create(&emp).Error)

	cars := models.SystemTable{Name: "post_gate_cars", TableType: "cars", IsActive: true}
	people := models.SystemTable{Name: "post_gate_people", TableType: "people", IsActive: true}
	other := models.SystemTable{Name: "post_gate_other", TableType: "cars", IsActive: true}
	for _, tbl := range []*models.SystemTable{&cars, &people, &other} {
		require.NoError(t, db.Create(tbl).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", car.ID, cars.ID).Error)
	require.NoError(t, db.Exec("INSERT INTO employee_target_tables (employee_id, table_id, order_index) VALUES (?, ?, 1)", emp.ID, people.ID).Error)
	return postGateFixture{carID: car.ID, empID: emp.ID, unboundCarID: unbound.ID, tableName: cars.Name, otherTableName: other.Name}
}

func elementStatus(t *testing.T, db *gorm.DB, table string, id int) int {
	t.Helper()
	var s int
	require.NoError(t, db.Raw(fmt.Sprintf("SELECT COALESCE(status, -1) FROM %s WHERE id = ?", table), id).Scan(&s).Error)
	return s
}

// Пользователь другой организации без прав на пост ничего не меняет у элементов поста.
func TestPostElementGate_OutsiderForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedPostGateFixture(t, db)
	token := testutil.RegisterAndLogin(t, e, "post_gate_outsider", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	cases := []struct {
		name, path, table string
		id, want          int
	}{
		{"машина: снятие", fmt.Sprintf("/cars/%d/deactivate", fx.carID), "cars", fx.carID, 1},
		{"сотрудник: снятие", fmt.Sprintf("/employees/%d/deactivate", fx.empID), "employees", fx.empID, 1},
	}
	for _, c := range cases {
		rec := testutil.PUT(t, e, c.path, `{}`, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", c.name, rec.Body.String())
		assert.Equal(t, c.want, elementStatus(t, db, c.table, c.id), "%s: статус не тронут", c.name)
	}

	// Снятое бюро не возвращается посторонним.
	require.NoError(t, db.Exec("UPDATE cars SET status = 0, date_removed = NOW() WHERE id = ?", fx.carID).Error)
	require.NoError(t, db.Exec("UPDATE employees SET status = 0 WHERE id = ?", fx.empID).Error)
	for _, c := range []struct {
		name, path, table string
		id                int
	}{
		{"машина: возврат", fmt.Sprintf("/cars/%d/activate", fx.carID), "cars", fx.carID},
		{"машина: восстановление", fmt.Sprintf("/cars/%d/restore", fx.carID), "cars", fx.carID},
		{"сотрудник: возврат", fmt.Sprintf("/employees/%d/activate", fx.empID), "employees", fx.empID},
		{"сотрудник: восстановление", fmt.Sprintf("/employees/%d/restore", fx.empID), "employees", fx.empID},
	} {
		rec := testutil.PUT(t, e, c.path, `{}`, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", c.name, rec.Body.String())
		assert.Equal(t, 0, elementStatus(t, db, c.table, c.id), "%s: элемент остался снятым", c.name)
	}

	rec := testutil.POST(t, e, fmt.Sprintf("/cars/%d/history", fx.carID), `{"action_type":"entry"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "журнал машины: %s", rec.Body.String())
	var entries int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'entry'",
		models.AuditEntityCar, fx.carID).Scan(&entries).Error)
	assert.Zero(t, entries, "в журнал машины ничего не записано")
}

// Охранник с правом удаления на пост элемента снимает его; то же право на ДРУГОЙ пост
// не помогает; элемент без постов доступен только администратору.
func TestPostElementGate_TableVerb(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedPostGateFixture(t, db)

	guardToken := testutil.RegisterAndLogin(t, e, "post_gate_guard", "pass123", 4, td.OrgID, td.CompanyID)
	testutil.GrantTableVerb(t, getUserID(t, db, "post_gate_guard"), fx.tableName, "delete")
	strangerToken := testutil.RegisterAndLogin(t, e, "post_gate_stranger", "pass123", 4, td.OrgID, td.CompanyID)
	testutil.GrantTableVerb(t, getUserID(t, db, "post_gate_stranger"), fx.otherTableName, "delete")

	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", fx.carID), `{}`, testutil.AuthHeader(strangerToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "право на чужой пост: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "table."+fx.tableName+".delete", "отказ называет нужное право")
	assert.Equal(t, 1, elementStatus(t, db, "cars", fx.carID))

	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", fx.carID), `{}`, testutil.AuthHeader(guardToken))
	require.Equal(t, http.StatusOK, rec.Code, "охранник своего поста: %s", rec.Body.String())
	assert.Equal(t, 0, elementStatus(t, db, "cars", fx.carID), "машина снята")

	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", fx.unboundCarID), `{}`, testutil.AuthHeader(guardToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "элемент без постов - не охраннику: %s", rec.Body.String())

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", fx.unboundCarID), `{}`, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code, "администратор: %s", rec.Body.String())
}

// allowPostElementActions выдаёт пользователю права на снятие, возврат и восстановление
// элементов поста после #2600: table.<name>.delete на каждый пост, существующий на момент
// вызова, и page.admin для элементов без постов. Нужен тестам, которые проверяют сами
// действия (история, корзина, уведомления), а не права на них.
func allowPostElementActions(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	allowPostElementActionsForID(t, db, getUserID(t, db, username))
}

func allowPostElementActionsForID(t *testing.T, db *gorm.DB, userID int) {
	t.Helper()
	var names []string
	require.NoError(t, db.Raw("SELECT name FROM system_tables").Scan(&names).Error)
	keys := []string{"page.admin"}
	for _, name := range names {
		keys = append(keys, "table."+name+".delete")
	}
	for _, key := range keys {
		var granted int64
		require.NoError(t, db.Model(&models.UserPermissionOverride{}).
			Where("user_id = ? AND permission_key = ?", userID, key).Count(&granted).Error)
		if granted == 0 {
			testutil.GrantPermission(t, userID, key)
		}
	}
}
