package handlers_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// buildEmployeesJSON строит n объектов employees с уникальными ФИО/паспортом для тела
// подачи. Паспорт непустой (patternSeries%04d) - нужен тесту шифрования, остальным тестам
// не мешает.
func buildEmployeesJSON(n int, citizenshipID, tableID int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"last_name":"Фамилия%d","first_name":"Имя%d","citizenship_id":%d,"position":"Рабочий","target_tables":[%d]}`,
			i, i, citizenshipID, tableID)
	}
	return sb.String()
}

func submitEmployeesBody(orgName string, uaID, n, citizenshipID, tableID int) string {
	return fmt.Sprintf(`{
		"message": "batch submit test",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people",
			"attachment_name": "people_tmpl",
			"attachment_display_name": "People Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"employees": [%s]}
		}]
	}`, orgName, uaID, buildEmployeesJSON(n, citizenshipID, tableID))
}

// submitTwoAttachmentsBody - заявка с ДВУМЯ людскими вложениями по n сотрудников в каждом:
// потолок считается по всей заявке, поэтому попарно-проходящие списки должны отлететь.
func submitTwoAttachmentsBody(orgName string, uaOne, uaTwo, n, citizenshipID, tableID int) string {
	attachment := func(uaID int, name string) string {
		return fmt.Sprintf(`{
			"attachment_type": "people",
			"attachment_name": "%s",
			"attachment_display_name": "People Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"employees": [%s]}
		}`, name, uaID, buildEmployeesJSON(n, citizenshipID, tableID))
	}
	return fmt.Sprintf(`{
		"message": "batch submit test",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [%s, %s]
	}`, orgName, attachment(uaOne, "people_tmpl_1"), attachment(uaTwo, "people_tmpl_2"))
}

// TestSubmitCompleteApplication_EmployeesCapBoundary проверяет границу MaxSubmitRowsPerList
// (blank-import, срез A2A3): BindAndValidate срезы не валидирует by design, поэтому потолок
// длины data.employees - явная проверка хендлера. N-1 (1999) должен пройти, N+1 (2001) -
// отлететь 400 с понятным текстом до захода в сервис (транзакция не открывается).
func TestSubmitCompleteApplication_EmployeesCapBoundary(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)

	t.Run("N+1 отлетает с понятным текстом", func(t *testing.T) {
		token := testutil.RegisterAndLogin(t, e, "cap_over_user", "pass123", 1, td.OrgID, td.CompanyID)
		uaID := seedUniqueAttachment(t, db, "people", "cap_over_tmpl", "People Template")
		body := submitEmployeesBody(orgName, uaID, handlers.MaxSubmitRowsPerList+1, citizenshipID, tableID)

		rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Слишком много")
		assert.Contains(t, rec.Body.String(), fmt.Sprintf("%d", handlers.MaxSubmitRowsPerList))

		// Считаются сотрудники ЭТОГО подтеста, а не вся таблица: база общая для
		// пакетов при -p 4, и чужая запись давала бы ложное падение (#2252).
		var count int64
		require.NoError(t, db.Model(&models.Employee{}).
			Where("last_name LIKE 'Фамилия%'").Count(&count).Error)
		assert.Zero(t, count, "заявка с превышением потолка не должна создавать ни одного сотрудника")
	})

	t.Run("два вложения суммарно выше потолка отлетают", func(t *testing.T) {
		token := testutil.RegisterAndLogin(t, e, "cap_sum_user", "pass123", 1, td.OrgID, td.CompanyID)
		uaOne := seedUniqueAttachment(t, db, "people", "cap_sum_tmpl_1", "People Template")
		uaTwo := seedUniqueAttachment(t, db, "people", "cap_sum_tmpl_2", "People Template")
		half := handlers.MaxSubmitRowsPerList/2 + 1
		body := submitTwoAttachmentsBody(orgName, uaOne, uaTwo, half, citizenshipID, tableID)

		rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Слишком много")

		var count int64
		require.NoError(t, db.Model(&models.Employee{}).
			Where("last_name LIKE 'Фамилия%'").Count(&count).Error)
		assert.Zero(t, count, "потолок считается по всей заявке, а не по каждому вложению отдельно")
	})

	t.Run("N-1 проходит", func(t *testing.T) {
		token := testutil.RegisterAndLogin(t, e, "cap_under_user", "pass123", 1, td.OrgID, td.CompanyID)
		uaID := seedUniqueAttachment(t, db, "people", "cap_under_tmpl", "People Template")
		body := submitEmployeesBody(orgName, uaID, handlers.MaxSubmitRowsPerList-1, citizenshipID, tableID)

		rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		var count int64
		require.NoError(t, db.Model(&models.Employee{}).
			Where("last_name LIKE 'Фамилия%'").Count(&count).Error)
		assert.EqualValues(t, handlers.MaxSubmitRowsPerList-1, count, "все сотрудники ниже потолка должны быть созданы")
	})
}

// TestSubmitCompleteApplication_BatchEmployeesEncryptedAndBound - регресс пакетной вставки
// сотрудников (CreateInBatches вместо построчного tx.Create, blank-import срез A2A3):
// паспорт/патент реально шифруются (BeforeSave отрабатывает под CreateInBatches так же,
// как под одиночным Create) и расшифровываются обратно при чтении, привязки к таблицам
// создаются для КАЖДОГО сотрудника, а не только для части батча.
func TestSubmitCompleteApplication_BatchEmployeesEncryptedAndBound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Тестовый стенд по умолчанию гоняет крипто-хуки в passthrough (testutil.go:
	// crypto.SetGlobalKey(nil)) - без реального ключа "шифротекст" был бы равен
	// исходной строке, и проверка ниже ничего бы не доказывала. Включаем настоящее
	// шифрование локально для этого теста (образец - application_files_test.go).
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	crypto.SetGlobalKey(key)
	defer crypto.SetGlobalKey(nil)

	const orgName = "Test Organization"
	const n = 30 // больше одного гипотетического мини-батча, но дёшево гонять
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "enc_tmpl", "People Template")
	token := testutil.RegisterAndLogin(t, e, "enc_batch_user", "pass123", 1, td.OrgID, td.CompanyID)

	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"last_name":"Шифр%d","first_name":"Тест%d","citizenship_id":%d,"position":"Рабочий","passport_series_number":"12%02d 654321","patent_number":"77%02d123456","target_tables":[%d]}`,
			i, i, citizenshipID, i, i, tableID)
	}
	body := fmt.Sprintf(`{
		"message": "encryption batch test",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people",
			"attachment_name": "people_tmpl",
			"attachment_display_name": "People Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"employees": [%s]}
		}]
	}`, orgName, uaID, sb.String())

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	// Дальше проверяется область СВОЕЙ заявки, а не таблица целиком. Пакеты идут с
	// -p 4 на общей базе, и чужой CleanDB между вставкой и проверкой обнулял счёт:
	// тест падал через раз, а перезапуск того же коммита проходил (#2252).
	appID := submittedApplicationID(t, rec)

	// Запись-помеха: та же фамилия, но вне этой заявки. Прежние проверки считали по
	// всей таблице через LIKE 'Шифр%' и на ней бы упали - именно так их ломал сосед
	// по общей базе. Держится здесь замком, чтобы выборка не вернулась к таблице.
	noise := "Шифр999"
	noisePassport := "9999 999999"
	require.NoError(t, db.Create(&models.Employee{LastName: &noise, PassportSeriesNumber: &noisePassport}).Error)
	ownEmployees := db.Where("attachment_id IN (?)",
		db.Model(&models.Attachment{}).Select("id").Where("application_id = ?", appID))

	var employees []models.Employee
	require.NoError(t, ownEmployees.Session(&gorm.Session{}).Order("id asc").Find(&employees).Error)
	require.Len(t, employees, n, "CreateInBatches должен создать всех сотрудников пачки")

	for i, emp := range employees {
		require.NotNil(t, emp.PassportSeriesNumber, "паспорт должен расшифроваться обратно (AfterFind)")
		require.NotNil(t, emp.PatentNumber, "патент должен расшифроваться обратно (AfterFind)")
		assert.Equal(t, fmt.Sprintf("12%02d 654321", i), *emp.PassportSeriesNumber, "паспорт сотрудника %d должен совпасть после шифрования/расшифровки", i)
		assert.Equal(t, fmt.Sprintf("77%02d123456", i), *emp.PatentNumber, "патент сотрудника %d должен совпасть после шифрования/расшифровки", i)
	}

	// В базе поле должно быть НЕ в открытом виде - иначе BeforeSave не отработал под
	// CreateInBatches, а это и есть риск, ради которого сотрудники остались на GORM.
	var plaintextCount int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM employees
		  WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)
		    AND passport_series_number LIKE '12%'`, appID,
	).Scan(&plaintextCount).Error)
	assert.Zero(t, plaintextCount, "паспорт в столбце БД должен быть шифротекстом, не читаемой строкой")

	var hmacCount int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM employees
		  WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)
		    AND passport_series_number_hmac IS NOT NULL AND patent_number_hmac IS NOT NULL`, appID,
	).Scan(&hmacCount).Error)
	assert.EqualValues(t, n, hmacCount, "у каждого сотрудника пачки должен быть проставлен HMAC (BeforeSave отработал построчно внутри батча)")

	var bindingCount int64
	require.NoError(t, db.Model(&models.EmployeeTargetTable{}).
		Where("employee_id IN (?)", ownEmployees.Session(&gorm.Session{}).Model(&models.Employee{}).Select("id")).
		Count(&bindingCount).Error)
	assert.EqualValues(t, n, bindingCount, "привязка к таблице проходной должна быть создана для КАЖДОГО сотрудника пачки")
}

// submittedApplicationID достаёт идентификатор созданной заявки из ответа подачи.
// Проверки, привязанные к нему, переживают чужой CleanDB на общей тестовой базе,
// тогда как выборка по фамилии считает всю таблицу и зависит от соседних пакетов.
func submittedApplicationID(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	var resp struct {
		Data struct {
			ApplicationID int `json:"application_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), "тело ответа: %s", rec.Body.String())
	require.NotZero(t, resp.Data.ApplicationID, "подача обязана вернуть идентификатор заявки: %s", rec.Body.String())
	return resp.Data.ApplicationID
}

// TestSubmitCompleteApplication_AuditHasSummaryAndPerEmployeeRecords - регресс сводной
// записи аудита (blank-import, срез A2A3): заявка получает ОДНУ запись
// "employees_bulk_added" в своей истории, а КАЖДЫЙ сотрудник продолжает получать
// собственную запись "create" (её читает /employees/:id/history) - сводная запись её не
// заменяет.
func TestSubmitCompleteApplication_AuditHasSummaryAndPerEmployeeRecords(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	const n = 5
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "audit_tmpl", "People Template")
	token := testutil.RegisterAndLogin(t, e, "audit_batch_user", "pass123", 1, td.OrgID, td.CompanyID)

	body := submitEmployeesBody(orgName, uaID, n, citizenshipID, tableID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var appID int
	require.NoError(t, db.Raw(
		`SELECT id FROM applications WHERE sender_user_id = (SELECT id FROM users WHERE username = 'audit_batch_user')`,
	).Scan(&appID).Error)
	require.NotZero(t, appID)

	var summary struct {
		Comment string
	}
	require.NoError(t, db.Raw(
		`SELECT details->>'comment' AS comment FROM audit_log
		 WHERE entity_type = ? AND entity_id = ? AND action = ?`,
		models.AuditEntityApplication, appID, models.AuditActionEmployeesBulkAdded,
	).Scan(&summary).Error)
	assert.Contains(t, summary.Comment, fmt.Sprintf("%d", n), "сводная запись должна называть число добавленных сотрудников")

	var perEmployeeCreateCount int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM audit_log a
		 JOIN employees e ON e.id = a.entity_id
		 WHERE a.entity_type = ? AND a.action = 'create' AND e.last_name LIKE 'Фамилия%'`,
		models.AuditEntityEmployee,
	).Scan(&perEmployeeCreateCount).Error)
	assert.EqualValues(t, n, perEmployeeCreateCount,
		"сводная запись не должна вытеснять собственную запись create у каждого сотрудника (иначе ломает /employees/:id/history)")
}

// TestSubmitCompleteApplication_BatchEmployeesPreserveOrder - регресс CreateInBatches
// (blank-import, срез A2A3): GORM обязан вернуть id каждой строки батча в ТОТ ЖЕ элемент
// employeeRecords, из которого строится дальнейший аудит/привязки/pending-флаги. Если бы
// порядок разошёлся, сотрудник i получил бы id и запись create ЧУЖОГО сотрудника - тест
// ловит это перекрёстной проверкой: аудит-комментарий по id сотрудника должен называть
// ЕГО собственное ФИО, а не чьё-то ещё (зеркало BatchVehiclesPreserveOrder для машин).
func TestSubmitCompleteApplication_BatchEmployeesPreserveOrder(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	const n = 40 // больше одного гипотетического мини-батча
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "order_emp_tmpl", "People Template")
	token := testutil.RegisterAndLogin(t, e, "order_emp_user", "pass123", 1, td.OrgID, td.CompanyID)

	body := submitEmployeesBody(orgName, uaID, n, citizenshipID, tableID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	type empRow struct {
		ID       int
		LastName string
	}
	var employees []empRow
	require.NoError(t, db.Raw(`SELECT id, last_name FROM employees WHERE last_name LIKE 'Фамилия%'`).Scan(&employees).Error)
	require.Len(t, employees, n)

	for _, emp := range employees {
		var comment string
		require.NoError(t, db.Raw(
			`SELECT details->>'comment' FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'create'`,
			models.AuditEntityEmployee, emp.ID,
		).Scan(&comment).Error)
		assert.Contains(t, comment, emp.LastName,
			"аудит сотрудника id=%d должен называть ЕГО фамилию (%s) - иначе CreateInBatches разошёлся по порядку с id", emp.ID, emp.LastName)
	}
}

// TestSubmitCompleteApplication_BatchVehiclesPreserveOrder - регресс пакетной вставки
// машин (multi-values INSERT ... RETURNING id вместо построчного, blank-import срез
// A2A3): RETURNING для одного multi-row INSERT обязан вернуть id в том же порядке, что и
// VALUES, иначе i-я машина входного среза свяжется с ЧУЖИМ id и получит чужой аудит/
// привязки. Проверяем это перекрёстно: аудит-комментарий каждой машины (собран из id,
// зафиксированного в момент вставки) должен называть ТОТ ЖЕ номер, что реально лежит в
// строке cars с этим id.
func TestSubmitCompleteApplication_BatchVehiclesPreserveOrder(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	const n = 60 // больше одного гипотетического мини-батча
	mark := seedMark(t, db, "OrderCheckMark")
	uaID := seedUniqueAttachment(t, db, "cars", "order_tmpl", "Car Template")
	token := testutil.RegisterAndLogin(t, e, "order_check_user", "pass123", 1, td.OrgID, td.CompanyID)

	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"car_number":"O%03dOO799","car_brand":"%s","mark_id":%d}`, i, mark.Name, mark.ID)
	}
	body := fmt.Sprintf(`{
		"message": "vehicle order test",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Car Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [%s]}
		}]
	}`, orgName, uaID, sb.String())

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	type carRow struct {
		ID        int
		CarNumber string
	}
	var cars []carRow
	require.NoError(t, db.Raw(`SELECT id, car_number FROM cars WHERE car_number LIKE 'O%OO799'`).Scan(&cars).Error)
	require.Len(t, cars, n)

	for _, c := range cars {
		var comment string
		require.NoError(t, db.Raw(
			`SELECT details->>'comment' FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'create'`,
			models.AuditEntityCar, c.ID,
		).Scan(&comment).Error)
		assert.Contains(t, comment, c.CarNumber,
			"аудит машины id=%d должен называть ЕЁ номер (%s) - иначе RETURNING разошёлся с порядком VALUES", c.ID, c.CarNumber)
	}
}
