package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedPeopleTable создаёт активную people-таблицу прохода и возвращает её ID.
func seedPeopleTable(t *testing.T, db *gorm.DB, name, display string) int {
	t.Helper()
	tbl := models.SystemTable{Name: name, DisplayName: &display, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	return tbl.ID
}

// POST /employees/manual (#1049, режим-1): super-админ добавляет сотрудника без заявки.
// Проверяем весь путь persist -> scoped-показ: вложение-сирота (application_id NULL,
// is_manual), сотрудник активен (status=1), привязан к целевой таблице и виден в ней с
// меткой «добавлено вручную» (application_id пустой), но не виден в чужой таблице.
func TestCreateManualEmployees_PersistsAndScoped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "people_manual_a", "Проход A")
	otherTableID := seedPeopleTable(t, db, "people_manual_b", "Проход B")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"company_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"entry_time_from": "08:00",
		"entry_time_to": "18:00",
		"employees": [{
			"last_name": "Ivanov",
			"first_name": "Ivan",
			"citizenship_id": %d,
			"position": "Loader",
			"passport_series_number": "1234 567890",
			"target_tables": []
		}]
	}`, td.OrgID, td.CompanyID, tableID, citizenshipID)

	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	resp := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec)
	require.Len(t, resp.EmployeeIDs, 1)
	require.NotZero(t, resp.AttachmentID)

	// Вложение-сирота: application_id NULL, is_manual, org/company на вложении.
	var att models.Attachment
	require.NoError(t, db.First(&att, resp.AttachmentID).Error)
	assert.Nil(t, att.ApplicationID, "ручное вложение без заявки")
	assert.True(t, att.IsManual)
	assert.Equal(t, "people", att.AttachmentType)
	require.NotNil(t, att.OrganizationID)
	assert.Equal(t, td.OrgID, *att.OrganizationID)

	// Сотрудник активен и привязан к целевой таблице.
	var emp models.Employee
	require.NoError(t, db.First(&emp, resp.EmployeeIDs[0]).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 1, *emp.Status, "ручной сотрудник сразу активен (одобрения нет)")
	require.NotNil(t, emp.AttachmentID)
	assert.Equal(t, resp.AttachmentID, *emp.AttachmentID)
	var linkCount int64
	require.NoError(t, db.Table("employee_target_tables").
		Where("employee_id = ? AND table_id = ?", emp.ID, tableID).Count(&linkCount).Error)
	assert.EqualValues(t, 1, linkCount, "сотрудник привязан к таблице со страницы")

	// Виден в целевой таблице с меткой «добавлено вручную» (application_id пустой).
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1, "ручной сотрудник виден в целевой таблице")
	assert.Equal(t, "Ivanov", rows[0]["last_name"])
	assert.Nil(t, rows[0]["application_id"], "у ручного сотрудника нет заявки (метка «добавлено вручную»)")
	assert.Equal(t, "Test Organization", rows[0]["organization"], "org резолвится с вложения через COALESCE")

	// Не виден в чужой таблице - scope, а не «во всех сразу».
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", otherTableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.ParseSlice(t, rec), "ручной сотрудник не виден в непривязанной таблице")
}

// Ручной сотрудник виден в таблице даже с истёкшим окном действия пропуска: в отличие от
// заявочных (гейтятся CURRENT_DATE BETWEEN entry_date_from/to), ручные показываются пока
// активны (e.status=1) - зеркало поведения ручных машин, у которых окна-фильтра нет вовсе.
func TestCreateManualEmployees_VisibleDespiteExpiredWindow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "people_manual_expired", "Проход E")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"table_id": %d,
		"entry_date_from": "2020-01-01",
		"entry_date_to": "2020-12-31",
		"employees": [{
			"last_name": "Expired",
			"first_name": "Window",
			"citizenship_id": %d,
			"position": "Guest",
			"passport_series_number": "0001 112233",
			"target_tables": []
		}]
	}`, td.OrgID, tableID, citizenshipID)

	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1, "ручной сотрудник виден несмотря на истёкшее окно")
	assert.Equal(t, "Expired", rows[0]["last_name"])
}

// Двое ручных сотрудников без паспорта в одной таблице видны ОБА: пустой паспорт
// хранится как NULL (не &""), иначе одинаковый HMAC("") и dedup PARTITION BY passport_hmac
// (rn=1) молча спрятал бы одного из списка допуска охраны. Инвариант: hmac IS NULL не
// схлопывается. Кейс реальный - гости/мигранты с патентом вместо паспорта.
func TestCreateManualEmployees_NoPassportNotDeduped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "people_manual_nopass", "Проход N")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"employees": [
			{"last_name": "GuestOne", "first_name": "A", "citizenship_id": %d, "position": "Guest", "passport_series_number": ""},
			{"last_name": "GuestTwo", "first_name": "B", "citizenship_id": %d, "position": "Guest", "passport_series_number": "   "}
		]
	}`, td.OrgID, tableID, citizenshipID, citizenshipID)

	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	resp := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec)
	require.Len(t, resp.EmployeeIDs, 2)

	// Оба безпаспортных сотрудника хранят NULL, а не HMAC("").
	var hmacCount int64
	require.NoError(t, db.Table("employees").
		Where("id IN ? AND passport_series_number_hmac IS NULL", resp.EmployeeIDs).Count(&hmacCount).Error)
	assert.EqualValues(t, 2, hmacCount, "пустой паспорт -> NULL hmac (не схлопывается dedup'ом)")

	// Оба видны в таблице допуска (dedup не спрятал одного).
	rec = testutil.GET(t, e, fmt.Sprintf("/employees/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "оба гостя без паспорта видны, ни один не схлопнут")
}

// Долг ревью S1: app-detail путь к сироте (#1049) закрыт даже для super - вложение
// ручного сотрудника не принадлежит заявке, GET /attachments/:id/employees отдаёт 403
// (без гейта super байпасил бы CanAccessApplication на appID 0). Закрыто ещё в S3, здесь
// регресс на people-путь.
func TestGetAttachmentEmployees_ManualOrphan_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "people_manual_orphan", "Проход O")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"employees": [{"last_name": "Orphan", "first_name": "Att",
		"citizenship_id": %d, "position": "X", "passport_series_number": "9 9"}]}`, td.OrgID, tableID, citizenshipID)
	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	attID := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec).AttachmentID

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "сирота ручного добавления недоступна через app-detail даже super")
}

// Гейт entity.employees.manual_add: обычный пользователь без права не может добавить вручную.
func TestCreateManualEmployees_ForbiddenWithoutPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empmanualnoperm", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "people_manual_gate", "Проход G")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"employees": [{"last_name": "Gate", "first_name": "Test",
		"citizenship_id": %d, "position": "X", "passport_series_number": "3 3"}]}`, td.OrgID, tableID, citizenshipID)
	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "без права entity.employees.manual_add - 403")
}

// Валидация: без организации / без таблицы / без сотрудников / пустое ФИО - 400.
func TestCreateManualEmployees_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	tableID := seedPeopleTable(t, db, "people_manual_val", "Проход V")

	cases := map[string]string{
		"без организации": fmt.Sprintf(`{"table_id": %d, "employees":[{"last_name":"A","first_name":"B"}]}`, tableID),
		"без таблицы":     fmt.Sprintf(`{"organization_id": %d, "employees":[{"last_name":"A","first_name":"B"}]}`, td.OrgID),
		"без сотрудников": fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "employees":[]}`, td.OrgID, tableID),
		"пустое ФИО":      fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "employees":[{"last_name":"  ","first_name":""}]}`, td.OrgID, tableID),
	}
	for name, body := range cases {
		rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s -> 400 (%s)", name, rec.Body.String())
	}
}
