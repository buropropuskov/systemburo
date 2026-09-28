package handlers_test

import (
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// roleFixture - объекты, по которым идут запросы role-класса. Заявка и запись реестра
// принадлежат самому пользователю без прав: роль принимающего или админа не выводится
// из авторства, и отказ должен прийти даже тому, кто видит объект.
type roleFixture struct {
	applicationID    int
	uniqueEmployeeID int
}

// roleProbe - запрос, который проходит разбор и валидацию тела, так что 403 означает
// отказ по роли, а не 400 на пустое тело. id подставляется во все параметры пути,
// ноль - id поста замка.
type roleProbe struct {
	id   int
	body string
}

// removedRoutes - методы, снятые как лишний вход в обход прав. Вернуть такой роут -
// решение заново, а не строка в реестре.
var removedRoutes = map[string]string{
	"POST /api/employees": "сотрудник с паспортом и постами в обход заявки; " +
		"ручное добавление идёт через /employees/manual",
	"POST /api/applications/:id/update-items-status": "автор активировал машины и сотрудников " +
		"несогласованной заявки; активацию делает принятие в работу",
	"POST /api/unique-cars/batch":    "массовая запись в реестр, фронт не вызывал",
	"PUT /api/unique-cars/by-number": "правка реестра по номеру без проверки записи, фронт не вызывал",
}

func seedRoleFixture(t *testing.T, db *gorm.DB, td testutil.TestData, ownerID int) roleFixture {
	t.Helper()
	num, status, confirmation := "ROUTE-ACCESS-ROLE", models.StatusProcessing, models.ConfirmationApproved
	app := models.Application{
		ApplicationNumber: &num,
		Status:            &status,
		Confirmation:      &confirmation,
		OrganizationID:    td.OrgID,
		SenderUserID:      ownerID,
	}
	require.NoError(t, db.Create(&app).Error)

	// Снятие возражения ищет запись раньше проверки роли: без записи ответ 404.
	lastName := "Замок"
	emp := models.UniqueEmployee{LastName: &lastName, OrganizationID: &td.OrgID, UserID: &ownerID}
	require.NoError(t, db.Create(&emp).Error)

	return roleFixture{applicationID: app.ID, uniqueEmployeeID: emp.ID}
}

// roleProbes - по запросу на каждый метод класса role. Сервисы заявки проверяют роль
// принимающего раньше чтения заявки и её элементов, поэтому id элементов в телах
// условные.
func roleProbes(fx roleFixture) map[string]roleProbe {
	app := func(body string) roleProbe { return roleProbe{id: fx.applicationID, body: body} }
	return map[string]roleProbe{
		"GET /api/admin/maintenance": {},
		"PUT /api/admin/maintenance": {body: `{"enabled":false}`},

		"PUT /api/applications/:id/bureau-note": app(`{"note":"замок реестра"}`),
		"PUT /api/applications/:id/dates": app(`{"entry_date_from":"2026-10-01","entry_date_to":"2026-10-02",` +
			`"entry_time_from":"08:00","entry_time_to":"20:00","reason":"замок реестра"}`),
		"DELETE /api/applications/:id/elements":                    app(`{"element_type":"cars","element_ids":[1],"reason":"замок реестра"}`),
		"PUT /api/applications/:id/elements/tables":                app(`{"element_type":"cars","element_ids":[1],"table_ids":[1],"mode":"add"}`),
		"PUT /api/applications/:id/elements/unload-places":         app(`{"car_ids":[1],"place_ids":[1],"mode":"add"}`),
		"POST /api/applications/:id/take-to-work":                  app(`{"user_id":1,"action":"accept"}`),
		"POST /api/applications/:id/revoke-from-work":              app(`{"user_id":1,"comment":"замок реестра"}`),
		"POST /api/applications/:id/restore-to-work":               app(`{"user_id":1,"comment":"замок реестра"}`),
		"POST /api/applications/:id/supplements/:sid/take-to-work": app(`{"action":"accept"}`),

		"GET /api/applications/available-attachments":                    {},
		"GET /api/applications/available-attachments/:id":                {},
		"POST /api/applications/available-attachments/:id/mark-executed": {},

		"GET /api/unique-cars/history":               {},
		"GET /api/unique-employees/history":          {},
		"DELETE /api/unique-employees/:id/objection": {id: fx.uniqueEmployeeID},
	}
}
