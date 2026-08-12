package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- helpers ---

// seedUniqueAttachment creates a unique_attachment template and returns its ID.
func seedUniqueAttachment(t *testing.T, db *gorm.DB, attType, name, displayName string) int {
	t.Helper()
	ua := models.UniqueAttachment{
		AttachmentType: attType,
		Name:           &name,
		DisplayName:    &displayName,
		IsActive:       true,
	}
	err := db.Create(&ua).Error
	require.NoError(t, err)
	return ua.ID
}

// seedCitizenship creates a citizenship and returns its ID.
func seedCitizenship(t *testing.T, db *gorm.DB) int {
	t.Helper()
	c := models.Citizenship{Name: "Russia", IsActive: true}
	err := db.Create(&c).Error
	require.NoError(t, err)
	return c.ID
}

// seedSystemTable creates a system_table and returns its ID.
func seedSystemTable(t *testing.T, db *gorm.DB) int {
	t.Helper()
	dn := "Test Table"
	st := models.SystemTable{Name: "test_table", DisplayName: &dn, TableType: "people", IsActive: true}
	err := db.Create(&st).Error
	require.NoError(t, err)
	return st.ID
}

// seedPassTableGrant создаёт таблицу КПП и выдаёт юзеру права отметки прохода
// (table.<name>.entry/.exit), затем возвращает её id для передачи в теле
// territory-status. Отметку прохода на бэке теперь гейтит RequireTablePassVerb -
// тестам нужно и право, и table_id (реальный фронт всегда шлёт table_id).
func seedPassTableGrant(t *testing.T, db *gorm.DB, userID int, tableType string) int {
	t.Helper()
	dn := "Pass Table"
	name := fmt.Sprintf("pass_tbl_u%d_%d", userID, time.Now().UnixNano()%1000000)
	tbl := models.SystemTable{Name: name, DisplayName: &dn, TableType: tableType, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	testutil.GrantTableVerb(t, userID, name, "entry")
	testutil.GrantTableVerb(t, userID, name, "exit")
	return tbl.ID
}

// assignOrgUser adds the user to organization_users so they appear as responsible.
func assignOrgUser(t *testing.T, db *gorm.DB, orgID, userID int, isPrimary bool) {
	t.Helper()
	err := db.Exec(
		"INSERT INTO organization_users (organization_id, user_id, is_primary) VALUES (?, ?, ?) ON CONFLICT DO NOTHING",
		orgID, userID, isPrimary,
	).Error
	require.NoError(t, err)
}

// assignOrgUserRequired добавляет пользователя в organization_users с признаком
// обязательного согласующего - готовит сценарий #2037, где присланный запросом
// required_approval не должен ни снимать, ни добавлять этот признак.
func assignOrgUserRequired(t *testing.T, db *gorm.DB, orgID, userID int) {
	t.Helper()
	err := db.Exec(
		"INSERT INTO organization_users (organization_id, user_id, is_primary, required_approval) VALUES (?, ?, false, true) ON CONFLICT DO NOTHING",
		orgID, userID,
	).Error
	require.NoError(t, err)
}

// getUserID returns user.ID by username.
func getUserID(t *testing.T, db *gorm.DB, username string) int {
	t.Helper()
	var u models.User
	err := db.Where("username = ?", username).First(&u).Error
	require.NoError(t, err)
	return u.ID
}

// createSimpleApplication creates a simple application via API and returns its ID.
func createSimpleApplication(t *testing.T, e *echo.Echo, token string, orgID int) int {
	t.Helper()
	body := fmt.Sprintf(`{"organization_id":%d,"data_approval":true,"message":"test message"}`, orgID)
	rec := testutil.POST(t, e, "/applications", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "create app: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.ApplicationCreateResponse](t, rec)
	return resp.ApplicationID
}

// submitCompleteApplication creates a full application with a cars attachment and returns app ID.
func submitCompleteApplication(t *testing.T, e *echo.Echo, token string, orgName string, uaID int) int {
	t.Helper()
	body := fmt.Sprintf(`{
		"message": "complete app test",
		"organization": "%s",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"vehicles": [{
					"car_number": "A001AA777",
					"car_brand": "Toyota"
				}]
			}
		}]
	}`, orgName, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit complete app: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	return resp.ApplicationID
}

// TestSubmitCompleteApplication_AddsReaders: получатели-читатели (#884) кладутся в
// application_viewers и получают view-доступ к заявке, не становясь согласующими.
func TestSubmitCompleteApplication_AddsReaders(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "rsender", "pass123", 1, td.OrgID, td.CompanyID)
	readerToken := testutil.RegisterAndLogin(t, e, "rreader", "pass123", 1, td.OrgID, td.CompanyID)
	readerID := getUserID(t, db, "rreader")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_rd", "Cars RD")

	body := fmt.Sprintf(`{
		"message": "app with readers",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"readers": [%d],
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A001AA777", "car_brand": "Toyota" }] }
		}]
	}`, readerID, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	// Читатель попал в список viewers заявки.
	vrec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/viewers", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, vrec.Code, vrec.Body.String())
	viewers := testutil.ParseResponse[[]services.ViewerWithUser](t, vrec)
	foundViewer := false
	for _, v := range viewers {
		if v.UserID == readerID {
			foundViewer = true
		}
	}
	assert.True(t, foundViewer, "читатель должен попасть в application_viewers")

	// Читатель получил view-доступ к заявке (раньше 403 - не свой/не ответственный).
	arec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(readerToken))
	assert.Equal(t, http.StatusOK, arec.Code, "читатель должен видеть заявку")

	// Но в согласующих его нет (только просмотр).
	var respCount int64
	db.Raw("SELECT COUNT(*) FROM application_responsible_users WHERE application_id = ? AND user_id = ?", appID, readerID).Scan(&respCount)
	assert.Zero(t, respCount, "читатель не должен попадать в ответственных/согласующих")
}

// TestSubmitCompleteApplication_RequiredApprovalFromOrgPersists закрепляет базовое
// поведение (#2037): признак обязательного согласующего читается из organization_users
// и переносится в application_responsible_users без участия required_users в запросе -
// именно так подаёт форма-эталон, ничего не заявляя.
func TestSubmitCompleteApplication_RequiredApprovalFromOrgPersists(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "reqsender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "reqapprover", "pass123", 1, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "reqapprover")
	assignOrgUserRequired(t, db, td.OrgID, approverID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_req", "Cars REQ")

	body := fmt.Sprintf(`{
		"message": "required approval from org",
		"organization_id": %d,
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A002AA777", "car_brand": "Toyota" }] }
		}]
	}`, td.OrgID, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var required bool
	require.NoError(t, db.Raw(
		"SELECT required_approval FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		appID, approverID).Scan(&required).Error)
	assert.True(t, required, "признак обязательного согласующего из организации должен перейти в заявку")
}

// TestSubmitCompleteApplication_RequiredApprovalNotDowngradableByClient - дефект
// #2037: заявитель не назначает согласующих сам, признак обязательности целиком
// определяется составом организации. Присланный запросом required_approval: false
// для уже обязательного согласующего не должен его снимать - до фикса строка
// application_service.go затирала прочитанное из organization_users значение тем,
// что прислал клиент.
func TestSubmitCompleteApplication_RequiredApprovalNotDowngradableByClient(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "downsender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "downapprover", "pass123", 1, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "downapprover")
	assignOrgUserRequired(t, db, td.OrgID, approverID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_down", "Cars DOWN")

	body := fmt.Sprintf(`{
		"message": "required approval downgrade attempt",
		"organization_id": %d,
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"required_users": [{"user_id": %d, "required_approval": false}],
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A003AA777", "car_brand": "Toyota" }] }
		}]
	}`, td.OrgID, approverID, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var required bool
	require.NoError(t, db.Raw(
		"SELECT required_approval FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		appID, approverID).Scan(&required).Error)
	assert.True(t, required, "клиентский required_approval:false не должен снимать обязательность, назначенную в организации")
}

// TestSubmitCompleteApplication_RequiredUsersRejectsForeignID - дефект #2048, воспроизведённый
// на стенде 12.08.2026: заявитель одной организации вписывал в required_users id работника
// ДРУГОЙ организации, заявка создавалась, и посторонний получал доступ к ней и право голосовать.
// До фикса ветка application_service.go, не найдя присланный id среди прочитанных из
// organization_users/companies_users, тихо добавляла его в ответственные вместо отказа.
// Подача с чужим id должна отклоняться целиком - заявка не создаётся.
func TestSubmitCompleteApplication_RequiredUsersRejectsForeignID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	foreignOrg := models.Organization{Name: "Foreign Organization"}
	require.NoError(t, db.Create(&foreignOrg).Error)

	senderToken := testutil.RegisterAndLogin(t, e, "foreignsender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "foreignoutsider", "pass123", 1, foreignOrg.ID, td.CompanyID)
	outsiderID := getUserID(t, db, "foreignoutsider")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_foreign", "Cars Foreign")

	body := fmt.Sprintf(`{
		"message": "required users foreign id attempt",
		"organization_id": %d,
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"required_users": [{"user_id": %d, "required_approval": true}],
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A004AA777", "car_brand": "Toyota" }] }
		}]
	}`, td.OrgID, outsiderID, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "чужой id в required_users должен отклонять подачу: %s", rec.Body.String())

	var appCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM applications WHERE message = ?", "required users foreign id attempt").Scan(&appCount).Error)
	assert.Zero(t, appCount, "заявка с посторонним в required_users не должна создаваться")
}

// TestSubmitCompleteApplication_RequiredUsersAcceptsOrgMember - парный к предыдущему тесту:
// присланный id, действительно входящий в состав организации заявки, по-прежнему принимается,
// заявка создаётся, а признак обязательности берётся из справочника (organization_users),
// а не из тела запроса (#2037).
func TestSubmitCompleteApplication_RequiredUsersAcceptsOrgMember(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "membersender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "memberapprover", "pass123", 1, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "memberapprover")
	assignOrgUserRequired(t, db, td.OrgID, approverID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_member", "Cars Member")

	body := fmt.Sprintf(`{
		"message": "required users org member",
		"organization_id": %d,
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"required_users": [{"user_id": %d, "required_approval": true}],
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A005AA777", "car_brand": "Toyota" }] }
		}]
	}`, td.OrgID, approverID, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var required bool
	require.NoError(t, db.Raw(
		"SELECT required_approval FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		appID, approverID).Scan(&required).Error)
	assert.True(t, required, "участник организации из required_users должен попасть в ответственные с признаком из справочника")
}

// TestSubmitCompleteApplication_RequiredUsersEmptyWhenLinkBroken - вопрос ревью к фиксу
// #2048: форма рвёт связь с organization_id, когда наименование организации правится
// руками (CreateApplication.vue, applyOrganizationChoice), и тогда блок сбора required_users
// в функции отправки не выполняется вовсе (guard `if (this.organizationId)`), запрос уходит
// без required_users. Организация при этом резолвится на бэке по наименованию (#1437) и
// остаётся не nil, поэтому гейт «пустая организация и компания» здесь не участвует - тест
// проверяет именно ветку required_users с отсутствующим полем. Обязательный согласующий из
// organization_users всё равно должен попасть в заявку: его сбор (строки выше в сервисе)
// не зависит от required_users в запросе, а раз клиент ничего не прислал - новой проверке
// нечего отклонять.
func TestSubmitCompleteApplication_RequiredUsersEmptyWhenLinkBroken(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "brokenlinksender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "brokenlinkapprover", "pass123", 1, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "brokenlinkapprover")
	assignOrgUserRequired(t, db, td.OrgID, approverID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_broken", "Cars Broken")

	// Наименованием, без organization_id и без required_users - ровно то, что шлёт форма
	// после ручной правки поля организации.
	body := fmt.Sprintf(`{
		"message": "broken organization link",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A006AA777", "car_brand": "Toyota" }] }
		}]
	}`, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "подача без organization_id и без required_users не должна отклоняться: %s", rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var required bool
	require.NoError(t, db.Raw(
		"SELECT required_approval FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		appID, approverID).Scan(&required).Error)
	assert.True(t, required, "обязательный согласующий из справочника должен попасть в заявку даже без required_users в запросе")
}

// --- 401 Unauthorized tests ---

func TestApplications_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/applications"},
		{"POST", "/applications"},
		{"POST", "/applications/submit-complete-application"},
		{"GET", "/applications/user"},
		{"GET", "/applications/1"},
		{"PUT", "/applications/1"},
		{"GET", "/applications/1/responsible-users"},
		{"GET", "/applications/1/details"},
		{"GET", "/applications/1/attachments"},
		{"POST", "/applications/1/update-items-status"},
		{"POST", "/applications/1/forward"},
		{"POST", "/applications/1/approve"},
		{"GET", "/applications/1/check-approval-status"},
		{"POST", "/applications/1/take-to-work"},
		{"POST", "/applications/1/revoke-from-work"},
		{"POST", "/applications/1/restore-to-work"},
		{"GET", "/applications/1/history"},
		{"POST", "/applications/1/revoke-approval"},
		{"POST", "/applications/history"},
		{"GET", "/applications/1/viewers"},
		{"GET", "/attachments/1/cars"},
		{"GET", "/attachments/1/employees"},
		{"GET", "/attachments/1/items"},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			var rec *httptest.ResponseRecorder
			switch ep.method {
			case "GET":
				rec = testutil.GET(t, e, ep.path, nil)
			case "POST":
				rec = testutil.POST(t, e, ep.path, "{}", nil)
			case "PUT":
				rec = testutil.PUT(t, e, ep.path, "{}", nil)
			}
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// --- GET /applications (list) ---

func TestGetApplications_EmptyList(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "appuser", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	apps := testutil.ParseResponse[[]interface{}](t, rec)
	assert.Empty(t, apps)
}

// TestGetApplications_FuzzySearchExecutes защищает большой SQL мега-поиска (#46):
// подзапросы по машинам/сотрудникам/местам разгрузки/согласующим + word_similarity.
// Регрессия: неверный JOIN car_unload_places.attachment_id (нет такой колонки) ронял
// весь запрос в 500. Проверяем, что разные формы запроса исполняются (200), а не падают.
func TestGetApplications_FuzzySearchExecutes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "searchuser", "pass123", 1, td.OrgID, td.CompanyID)

	for _, q := range []string{"грязевой", "А777АА", "А 777 АА", "ghbdtn", "иванов", "70", "ремонт крыши"} {
		rec := testutil.GET(t, e, "/applications?search_query="+url.QueryEscape(q), testutil.AuthHeader(token))
		assert.Equalf(t, http.StatusOK, rec.Code,
			"search_query=%q должен вернуть 200, а не %d: %s", q, rec.Code, rec.Body.String())
	}
}

// TestGetApplications_SearchFindsItemWork: поиск находит заявку по наименованию
// работ из items-вложения ("Заявка на работы"). Раньше items не джойнились в
// мега-поиске - заявка с "Ремонт крыши" не находилась.
func TestGetApplications_SearchFindsItemWork(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "itemsearch", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	var attID int
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (application_id, attachment_type, created_at, updated_at)
		 VALUES (?, 'items', now(), now()) RETURNING id`, appID).Scan(&attID).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO items (attachment_id, name, created_at, updated_at)
		 VALUES (?, ?, now(), now())`, attID, "Ремонт уникальнойкрыши zzqx").Error)

	rec := testutil.GET(t, e,
		"/applications?search_query="+url.QueryEscape("уникальнойкрыши zzqx"),
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	apps := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	found := false
	for _, a := range apps {
		if id, ok := a["id"].(float64); ok && int(id) == appID {
			found = true
		}
	}
	assert.True(t, found, "заявка должна находиться по наименованию работ из items-вложения")
}

// TestGetApplications_SearchFindsCarBrand: поиск находит заявку по марке её машины.
// Марка хранится в двух колонках: mark_name -- снимок имени марки, он появился позже и
// заполнен у единиц записей, у остальных марка лежит в устаревшей car_brand. Поиск
// смотрел только в mark_name, и заявку по марке машины было не найти.
func TestGetApplications_SearchFindsCarBrand(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carbrandsearch", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	var attID int
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (application_id, attachment_type, created_at, updated_at)
		 VALUES (?, 'cars', now(), now()) RETURNING id`, appID).Scan(&attID).Error)
	// mark_name намеренно пуст -- ровно так выглядят реальные записи.
	require.NoError(t, db.Exec(
		`INSERT INTO cars (attachment_id, car_number, car_brand, created_at, updated_at)
		 VALUES (?, ?, ?, now(), now())`, attID, "В 543 НЕ 654", "Мерседесzzqx").Error)

	rec := testutil.GET(t, e,
		"/applications?search_query="+url.QueryEscape("Мерседесzzqx"),
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	apps := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	found := false
	for _, a := range apps {
		if id, ok := a["id"].(float64); ok && int(id) == appID {
			found = true
		}
	}
	assert.True(t, found, "заявка должна находиться по марке своей машины")
}

// TestGetApplications_SearchFindsAttachmentName: поиск находит заявку по
// пользовательскому наименованию вложения (#883). Имя вложения редактируется
// при подаче и хранится в attachments.attachment_display_name; раньше в мега-поиске
// не участвовало - заявку по переименованному вложению было не найти.
func TestGetApplications_SearchFindsAttachmentName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "attnamesearch", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	require.NoError(t, db.Exec(
		`INSERT INTO attachments (application_id, attachment_type, attachment_display_name, created_at, updated_at)
		 VALUES (?, 'cars', ?, now(), now())`, appID, "Грузовикиzzqx уникальное").Error)

	rec := testutil.GET(t, e,
		"/applications?search_query="+url.QueryEscape("Грузовикиzzqx уникальное"),
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	apps := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	found := false
	for _, a := range apps {
		if id, ok := a["id"].(float64); ok && int(id) == appID {
			found = true
		}
	}
	assert.True(t, found, "заявка должна находиться по наименованию вложения")
}

// --- GET /applications/user ---

func TestGetUserApplications_ReturnsOwnApplications(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "sender1", "pass123", 1, td.OrgID, td.CompanyID)
	createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, "/applications/user", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	apps := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	assert.GreaterOrEqual(t, len(apps), 1)
}

// TestGetUserApplications_AccessScoped (#1158 срез 4): GetUserApplications раньше
// возвращал ВООБЩЕ ВСЕ заявки системы без фильтрации по пользователю - клиент лишь
// отображал подмножество через currentFilter (my/organization), не ограничивая
// реальный доступ к данным. Заявка чужой организации, отправленная чужим
// пользователем, не должна попадать ни в legacy-список, ни в пагинированный.
func TestGetUserApplications_AccessScoped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	otherOrg := models.Organization{Name: "Other Organization"}
	require.NoError(t, db.Create(&otherOrg).Error)

	ownToken := testutil.RegisterAndLogin(t, e, "scoped_owner", "pass123", 1, td.OrgID, td.CompanyID)
	otherToken := testutil.RegisterAndLogin(t, e, "scoped_stranger", "pass123", 1, otherOrg.ID, td.CompanyID)

	ownAppID := createSimpleApplication(t, e, ownToken, td.OrgID)
	strangerAppID := createSimpleApplication(t, e, otherToken, otherOrg.ID)

	containsID := func(apps []map[string]interface{}, id int) bool {
		for _, a := range apps {
			if v, ok := a["id"].(float64); ok && int(v) == id {
				return true
			}
		}
		return false
	}

	// legacy (без per_page)
	rec := testutil.GET(t, e, "/applications/user", testutil.AuthHeader(ownToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	legacyApps := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	assert.True(t, containsID(legacyApps, ownAppID), "своя заявка должна быть видна (legacy)")
	assert.False(t, containsID(legacyApps, strangerAppID), "чужая заявка чужой организации не должна быть видна (legacy)")

	// paginated (с per_page)
	rec = testutil.GET(t, e, "/applications/user?per_page=50&page=1", testutil.AuthHeader(ownToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var env struct {
		Data []map[string]interface{} `json:"data"`
		Meta map[string]interface{}   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	assert.True(t, containsID(env.Data, ownAppID), "своя заявка должна быть видна (paginated)")
	assert.False(t, containsID(env.Data, strangerAppID), "чужая заявка чужой организации не должна быть видна (paginated)")
	// total считает только доступные пользователю заявки, не всю систему.
	assert.Equal(t, float64(1), env.Meta["total"], "total должен учитывать только доступные ЛК заявки")
}

func TestGetUserApplicationsPaginated_MetaTotal(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "userpage1", "pass123", 1, td.OrgID, td.CompanyID)
	for i := 0; i < 3; i++ {
		createSimpleApplication(t, e, token, td.OrgID)
	}

	rec := testutil.GET(t, e, "/applications/user?per_page=2&page=1", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var env struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
		Meta    map[string]interface{}   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	assert.True(t, env.Success)
	require.NotNil(t, env.Meta)
	assert.Equal(t, float64(1), env.Meta["page"])
	assert.Equal(t, float64(2), env.Meta["per_page"])
	total, ok := env.Meta["total"].(float64)
	require.True(t, ok, "meta.total field must be present")
	assert.GreaterOrEqual(t, total, float64(3))
	assert.Len(t, env.Data, 2)
}

func TestGetUserApplicationsPaginated_SearchWorks(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "userpagesearch", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)
	require.NoError(t, db.Exec(
		`UPDATE applications SET message = ? WHERE id = ?`, "Уникальное сообщение zzqxsearch", appID).Error)

	rec := testutil.GET(t, e,
		"/applications/user?search_query="+url.QueryEscape("zzqxsearch")+"&per_page=10&page=1",
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var env struct {
		Data []map[string]interface{} `json:"data"`
		Meta map[string]interface{}   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	found := false
	for _, a := range env.Data {
		if id, ok := a["id"].(float64); ok && int(id) == appID {
			found = true
		}
	}
	assert.True(t, found, "заявка должна находиться поиском в пагинированном пути")
	assert.Equal(t, float64(1), env.Meta["total"])
}

// TestGetUserApplicationsPaginated_SenderFilter (#1158 срез 4): вкладка "Мои заявки"
// в ЛК сужает базовый доступ (свои + заявки организации) до заявок, отправленных
// именно этим пользователем - коллега по той же организации виден в базовом доступе,
// но не должен попадать в выдачу с sender_user_id фильтром.
func TestGetUserApplicationsPaginated_SenderFilter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownToken := testutil.RegisterAndLogin(t, e, "sender_own", "pass123", 1, td.OrgID, td.CompanyID)
	colleagueToken := testutil.RegisterAndLogin(t, e, "sender_colleague", "pass123", 1, td.OrgID, td.CompanyID)

	ownUserID := getUserID(t, db, "sender_own")
	ownAppID := createSimpleApplication(t, e, ownToken, td.OrgID)
	colleagueAppID := createSimpleApplication(t, e, colleagueToken, td.OrgID)

	rec := testutil.GET(t, e,
		fmt.Sprintf("/applications/user?sender_user_id=%d&per_page=50&page=1", ownUserID),
		testutil.AuthHeader(ownToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var env struct {
		Data []map[string]interface{} `json:"data"`
		Meta map[string]interface{}   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	found, foundColleague := false, false
	for _, a := range env.Data {
		id, _ := a["id"].(float64)
		if int(id) == ownAppID {
			found = true
		}
		if int(id) == colleagueAppID {
			foundColleague = true
		}
	}
	assert.True(t, found, "своя заявка должна быть видна при sender_user_id фильтре")
	assert.False(t, foundColleague, "заявка коллеги по организации не должна попадать в 'Мои заявки'")
	assert.Equal(t, float64(1), env.Meta["total"])
}

// --- POST /applications (simple create) ---

func TestCreateApplication_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "creator1", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{"organization_id":%d,"data_approval":true,"message":"test msg"}`, td.OrgID)
	rec := testutil.POST(t, e, "/applications", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[services.ApplicationCreateResponse](t, rec)
	assert.NotZero(t, resp.ApplicationID)
	assert.Contains(t, resp.ApplicationNumber, "/")
}

func TestCreateApplication_DataApprovalRequired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "creator2", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{"organization_id":%d,"data_approval":false}`, td.OrgID)
	rec := testutil.POST(t, e, "/applications", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- POST /applications/submit-complete-application ---

func TestSubmitCompleteApplication_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "test_cars", "Test Cars")
	token := testutil.RegisterAndLogin(t, e, "complete1", "pass123", 1, td.OrgID, td.CompanyID)

	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	assert.NotZero(t, appID)
}

func TestSubmitCompleteApplication_NoAttachments(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "complete2", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{
		"message": "no attachments",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": []
	}`
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSubmitCompleteApplication_DataApprovalRequired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "complete3", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{
		"message": "no approval",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": false,
		"attachments": [{"attachment_type":"cars","attachment_name":"x","attachment_display_name":"X","unique_attachment_id":1,"data":{"vehicles":[{"car_number":"A000AA777","car_brand":"BMW"}]}}]
	}`
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSubmitCompleteApplication_CompanyOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "co_cars", "Company Only Cars")
	token := testutil.RegisterAndLogin(t, e, "componly", "pass123", 1, td.OrgID, td.CompanyID)

	// Организация пустая, заполнена только компания - заявка должна создаться.
	body := fmt.Sprintf(`{
		"message": "company only",
		"organization": "",
		"company": "Test Company",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "co_cars",
			"attachment_display_name": "Company Only Cars",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [{"car_number": "A001AA777", "car_brand": "Toyota"}]}
		}]
	}`, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "company-only submit: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	assert.NotZero(t, resp.ApplicationID)

	// Чтение company-only заявки не должно падать: organization_id NULL в LEFT JOIN organizations.
	listRec := testutil.GET(t, e, "/applications/user", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, listRec.Code, "read company-only app: %s", listRec.Body.String())
}

func TestSubmitCompleteApplication_NoOrgNoCompany(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "noorg_cars", "No Org Cars")
	token := testutil.RegisterAndLogin(t, e, "noorg", "pass123", 1, td.OrgID, td.CompanyID)

	// Ни организация, ни компания не заполнены - заявку отклоняем (400).
	body := fmt.Sprintf(`{
		"message": "no org no company",
		"organization": "",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "noorg_cars",
			"attachment_display_name": "No Org Cars",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [{"car_number": "A002AA777", "car_brand": "Toyota"}]}
		}]
	}`, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "no org/company should be 400: %s", rec.Body.String())
}

// TestAutoMigrate_RelaxesOrganizationNotNull: на БД из старой "NOT NULL"-эры
// (свежий AutoMigrate уже делает колонку nullable, поэтому эмулируем констрейнт)
// повторный AutoMigrate должен снять NOT NULL с applications.organization_id -
// иначе company-only подача упадёт 500 на существующих staging/prod.
func TestAutoMigrate_RelaxesOrganizationNotNull(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	require.NoError(t, db.Exec(`TRUNCATE applications CASCADE`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE applications ALTER COLUMN organization_id SET NOT NULL`).Error)

	require.NoError(t, database.AutoMigrate(db))

	var nullable string
	require.NoError(t, db.Raw(
		`SELECT is_nullable FROM information_schema.columns WHERE table_name='applications' AND column_name='organization_id'`,
	).Row().Scan(&nullable))
	require.Equal(t, "YES", nullable, "organization_id должна стать nullable после миграции")
}

// --- GET /applications/:id ---

func TestGetApplicationByID_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "getter1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(appID), resp["id"])
}

func TestGetApplicationByID_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "getter2", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/applications/999999", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetApplicationByID_InvalidID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "getter3", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/applications/abc", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- PUT /applications/:id ---

func TestUpdateApplication_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "updater1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	body := `{"responsible_comment":"some comment"}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[services.ApplicationUpdateResponse](t, rec)
	assert.True(t, resp.Success)
}

// --- GET /applications/:id/details ---

func TestGetApplicationDetails_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "details1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(appID), resp["id"])
}

// --- GET /applications/:id/responsible-users ---

func TestGetResponsibleUsers_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "resp1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/responsible-users", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	testutil.ParseResponse[[]interface{}](t, rec)
	// May or may not have responsible users depending on org_users seeding
}

// TestGetResponsibleUsers_ExposesReminderFields — карточке заявки (#1315 S3) нужны
// created_at (момент назначения, от него "не отвечает N дней") и reminder_count
// ("напомнили K раз") в ответе responsible-users. Тест бьёт по реальному эндпоинту:
// go build видит поля DTO, но не проверяет, что SQL их реально селектит.
func TestGetResponsibleUsers_ExposesReminderFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "resp2", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	approverID := newReminderUser(t, db, "approver")
	past := time.Now().Add(-2 * 24 * time.Hour)
	aru := models.ApplicationResponsibleUser{
		ApplicationID:    appID,
		UserID:           approverID,
		CreatedAt:        time.Now().Add(-5 * 24 * time.Hour),
		RequiredApproval: true,
		ApprovalStatus:   pendingStatus(),
		ReminderCount:    2,
		LastReminderAt:   &past,
	}
	require.NoError(t, db.Create(&aru).Error)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/responsible-users", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	users := testutil.ParseSlice(t, rec)
	var found map[string]interface{}
	for _, u := range users {
		if int(u["id"].(float64)) == approverID {
			found = u
			break
		}
	}
	require.NotNil(t, found, "назначенный согласующий должен быть в ответе: %s", rec.Body.String())
	assert.Equal(t, float64(2), found["reminder_count"], "reminder_count должен отдаваться")
	createdAt, ok := found["created_at"].(string)
	assert.True(t, ok && createdAt != "", "created_at должен отдаваться непустой строкой")
	assert.NotContains(t, createdAt, "0001-01-01", "created_at не должен быть нулевым временем")
}

// --- GET /applications/:id/attachments ---

func TestGetApplicationAttachments_WithCompleteApp(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl", "Cars Template")
	token := testutil.RegisterAndLogin(t, e, "att1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	attachments := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(attachments), 1)
	assert.Equal(t, "cars", attachments[0]["attachment_type"])
}

// --- GET /attachments/:id/cars ---

func TestGetAttachmentCars_WithCompleteApp(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl2", "Cars Template 2")
	token := testutil.RegisterAndLogin(t, e, "attcar1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	// Get attachment ID from the application
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	attachments := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, attachments)

	attID := int(attachments[0]["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	cars := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(cars), 1)
	assert.Equal(t, "A001AA777", cars[0]["car_number"])
	assert.Equal(t, "Toyota", cars[0]["car_brand"])
}

// --- GET /attachments/:id/employees ---

func TestGetAttachmentEmployees_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl3", "Cars Template 3")
	token := testutil.RegisterAndLogin(t, e, "attemp1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	attachments := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, attachments)
	attID := int(attachments[0]["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	employees := testutil.ParseResponse[[]interface{}](t, rec)
	// cars attachment has no employees
	assert.Empty(t, employees)
}

// --- GET /attachments/:id/items ---

func TestGetAttachmentItems_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl4", "Cars Template 4")
	token := testutil.RegisterAndLogin(t, e, "attitem1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	attachments := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, attachments)
	attID := int(attachments[0]["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/items", attID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	items := testutil.ParseResponse[[]interface{}](t, rec)
	assert.Empty(t, items)
}

// --- GET /applications/:id/history ---

func TestGetApplicationHistory_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "hist_cars", "Hist Cars")
	token := testutil.RegisterAndLogin(t, e, "hist1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	// SubmitCompleteApplication writes create + assigned_responsible entries
	assert.GreaterOrEqual(t, len(history), 1)
}

// --- POST /applications/history ---

func TestAddHistoryEntry_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "histwr1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)
	userID := getUserID(t, db, "histwr1")

	body := fmt.Sprintf(`{
		"application_id": %d,
		"user_id": %d,
		"action_type": "comment",
		"comment": "manual history entry"
	}`, appID, userID)
	rec := testutil.POST(t, e, "/applications/history", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "History entry added successfully", msg)
}

// --- GET /applications/:id/viewers ---

func TestGetApplicationViewers_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "viewer1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/viewers", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	viewers := testutil.ParseResponse[[]interface{}](t, rec)
	assert.Empty(t, viewers)
}

// --- GET /applications/:id/check-approval-status ---

func TestCheckApprovalStatus_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "appstat1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
	assert.NotNil(t, resp.Confirmation)
	assert.NotNil(t, resp.Status)
}

// --- POST /applications/:id/forward ---

func TestForwardApplication_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Sender creates app
	senderToken := testutil.RegisterAndLogin(t, e, "fwdsender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "fwdsender")

	// Register approver user (responsible — will receive required_approval)
	testutil.RegisterUser(t, e, "fwdapprover", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "fwdapprover")

	// Make sender an approver so they can forward
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", senderID)

	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Register viewer user
	testutil.RegisterUser(t, e, "fwdviewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "fwdviewer")

	body := fmt.Sprintf(`{
		"users": [
			{"user_id": %d, "required_approval": true, "can_view": false},
			{"user_id": %d, "required_approval": false, "can_view": true}
		]
	}`, approverID, viewerID)

	// Sender is the application creator and can forward
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify tab.applications.view was granted to the forwarded responsible user
	var perm struct {
		PermissionKey string
		Value         string
	}
	err := db.Raw("SELECT permission_key, value FROM user_permissions WHERE user_id = ? AND permission_key = ?",
		approverID, "tab.applications.view").Scan(&perm).Error
	require.NoError(t, err)
	assert.Equal(t, "allow", perm.Value)
}

// --- POST /applications/:id/approve ---

func TestApproveApplication_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Register responsible user and assign to org
	testutil.RegisterUser(t, e, "approveresp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "approveresp")
	assignOrgUser(t, db, td.OrgID, respID, true)

	// Sender creates application (responsible will be auto-assigned from org)
	senderToken := testutil.RegisterAndLogin(t, e, "approvsender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Login as responsible
	respToken, _ := testutil.LoginUser(t, e, "approveresp", "pass123")

	body := fmt.Sprintf(`{"user_id": %d, "status": "approved", "comment": "looks good"}`, respID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), body, testutil.AuthHeader(respToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Approval status updated successfully", msg)
}

// --- POST /applications/:id/take-to-work ---

func TestTakeApplicationToWork_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Create approver user
	testutil.RegisterUser(t, e, "ttwapprover", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "ttwapprover")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "ttwapprover", "pass123")

	// Sender creates application
	senderToken := testutil.RegisterAndLogin(t, e, "ttwsender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	body := fmt.Sprintf(`{"user_id": %d, "action": "accept"}`, approverID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(approverToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Application taken to work", msg)
}

func TestTakeApplicationToWork_Reject(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "ttwrej", "pass123", 6, td.OrgID, td.CompanyID)
	rejID := getUserID(t, db, "ttwrej")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", rejID)
	rejToken, _ := testutil.LoginUser(t, e, "ttwrej", "pass123")

	senderToken := testutil.RegisterAndLogin(t, e, "ttwsenderrej", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	body := fmt.Sprintf(`{"user_id": %d, "action": "reject", "comment": "not suitable"}`, rejID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(rejToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Application rejected", msg)
}

// --- POST /applications/:id/update-items-status ---

func TestUpdateApplicationItemsStatus_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "status_cars", "Status Cars")
	token := testutil.RegisterAndLogin(t, e, "itemstat1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "All items statuses updated successfully", msg)
}

// --- POST /applications/:id/revoke-from-work ---

func TestRevokeFromWork_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Setup approver
	testutil.RegisterUser(t, e, "revokeappr", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "revokeappr")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "revokeappr", "pass123")

	// Create application and take to work first
	senderToken := testutil.RegisterAndLogin(t, e, "revokesender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	takeBody := fmt.Sprintf(`{"user_id": %d, "action": "accept"}`, approverID)
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), takeBody, testutil.AuthHeader(approverToken))

	body := fmt.Sprintf(`{"user_id": %d, "comment": "needs changes"}`, approverID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-from-work", appID), body, testutil.AuthHeader(approverToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Application revoked from work", msg)
}

// --- POST /applications/:id/restore-to-work ---

func TestRestoreToWork_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "restoreappr", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "restoreappr")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "restoreappr", "pass123")

	senderToken := testutil.RegisterAndLogin(t, e, "restoresender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Take to work, then reject, then restore
	takeBody := fmt.Sprintf(`{"user_id": %d, "action": "reject", "comment": "rejected"}`, approverID)
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), takeBody, testutil.AuthHeader(approverToken))

	body := fmt.Sprintf(`{"user_id": %d, "comment": "restoring for review"}`, approverID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/restore-to-work", appID), body, testutil.AuthHeader(approverToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Application restored, ready to take to work", msg)
}

// --- POST /applications/:id/revoke-approval ---

func TestRevokeApproval_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Setup responsible user
	testutil.RegisterUser(t, e, "revokeapprvl", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "revokeapprvl")
	assignOrgUser(t, db, td.OrgID, respID, false)

	senderToken := testutil.RegisterAndLogin(t, e, "revokeapprsend", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// First approve
	respToken, _ := testutil.LoginUser(t, e, "revokeapprvl", "pass123")
	approveBody := fmt.Sprintf(`{"user_id": %d, "status": "approved", "comment": "ok"}`, respID)
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(respToken))

	// Then revoke
	revokeBody := `{"comment": "changed my mind"}`
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", appID), revokeBody, testutil.AuthHeader(respToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	testutil.ParseMap(t, rec)
	// success=true guaranteed by envelope

}

// --- Full lifecycle test ---

func TestApplicationLifecycle_CreateSubmitForwardApproveTakeToWork(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "lifecycle_cars", "Lifecycle Cars")

	// 1. Register sender
	senderToken := testutil.RegisterAndLogin(t, e, "lcsender", "pass123", 1, td.OrgID, td.CompanyID)

	// 2. Register responsible/approver user assigned to org
	testutil.RegisterUser(t, e, "lcresp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "lcresp")
	assignOrgUser(t, db, td.OrgID, respID, true)
	respToken, _ := testutil.LoginUser(t, e, "lcresp", "pass123")

	// 3. Register approver (buropropuskov)
	testutil.RegisterUser(t, e, "lcapprover", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "lcapprover")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "lcapprover", "pass123")

	// 4. Submit complete application
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)
	require.NotZero(t, appID)

	// 5. Verify application appears in user list
	rec := testutil.GET(t, e, "/applications/user", testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)
	userApps := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	assert.GreaterOrEqual(t, len(userApps), 1)

	// 6. Get by ID
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 7. Check approval status
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	approvalStatus := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
	require.NotNil(t, approvalStatus.Confirmation)
	assert.Equal(t, "Согласование", *approvalStatus.Confirmation)

	// 8. Responsible user approves
	approveBody := fmt.Sprintf(`{"user_id": %d, "status": "approved", "comment": "approved"}`, respID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(respToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 9. Check approval status changed
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 10. Approver takes to work
	takeBody := fmt.Sprintf(`{"user_id": %d, "action": "accept"}`, approverID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), takeBody, testutil.AuthHeader(approverToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 11. Update items status (activate all cars/employees)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(approverToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 12. Check history has multiple entries
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseResponse[[]interface{}](t, rec)
	assert.GreaterOrEqual(t, len(history), 2, "history should have at least create + approve entries")

	// 13. Verify attachments
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)
	atts := testutil.ParseSlice(t, rec)
	assert.NotEmpty(t, atts)

	// 14. Verify cars in attachment
	if len(atts) > 0 {
		attID := int(atts[0]["id"].(float64))
		rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(senderToken))
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 15. Details endpoint
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Complete application with employees and items ---

func TestSubmitCompleteApplication_WithEmployeesAndItems(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaIDPeople := seedUniqueAttachment(t, db, "people", "people_tmpl", "People Template")
	uaIDItems := seedUniqueAttachment(t, db, "items", "items_tmpl", "Items Template")

	token := testutil.RegisterAndLogin(t, e, "fullappsender", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{
		"message": "full application",
		"organization": "Test Organization",
		"responsible_person": "Full Test",
		"contact_phone": "+79009876543",
		"data_approval": true,
		"attachments": [
			{
				"attachment_type": "people",
				"attachment_name": "people_tmpl",
				"attachment_display_name": "People Template",
				"unique_attachment_id": %d,
				"entry_date_from": "2026-04-01",
				"entry_date_to": "2099-12-31",
				"data": {
					"employees": [{
						"last_name": "Ivanov",
						"first_name": "Ivan",
						"middle_name": "Ivanovich",
						"citizenship_id": %d,
						"position": "Engineer",
						"passport_series_number": "1234 567890",
						"target_tables": [%d]
					}]
				}
			},
			{
				"attachment_type": "items",
				"attachment_name": "items_tmpl",
				"attachment_display_name": "Items Template",
				"unique_attachment_id": %d,
				"entry_date_from": "2026-04-01",
				"entry_date_to": "2099-12-31",
				"data": {
					"items": [{
						"name": "Cement bags",
						"count": 100,
						"order_index": 1
					}]
				}
			}
		]
	}`, uaIDPeople, citizenshipID, tableID, uaIDItems)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	appID := createResp.ApplicationID

	// Verify attachments
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	atts := testutil.ParseSlice(t, rec)
	assert.Equal(t, 2, len(atts))

	// Find people attachment and check employees
	for _, att := range atts {
		attID := int(att["id"].(float64))
		attType := att["attachment_type"].(string)

		if attType == "people" {
			rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(token))
			assert.Equal(t, http.StatusOK, rec.Code)
			emps := testutil.ParseSlice(t, rec)
			assert.GreaterOrEqual(t, len(emps), 1)
			assert.Equal(t, "Ivanov", emps[0]["last_name"])
		}

		if attType == "items" {
			rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/items", attID), testutil.AuthHeader(token))
			assert.Equal(t, http.StatusOK, rec.Code)
			items := testutil.ParseSlice(t, rec)
			assert.GreaterOrEqual(t, len(items), 1)
			assert.Equal(t, "Cement bags", items[0]["name"])
		}
	}
}

func TestGetApplications_Paginated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/applications?per_page=5&page=1", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var env struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
		Meta    map[string]interface{}   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	assert.True(t, env.Success)
	assert.NotNil(t, env.Meta)
	assert.Equal(t, float64(1), env.Meta["page"])
	assert.Equal(t, float64(5), env.Meta["per_page"])
	_, hasTotalField := env.Meta["total"]
	assert.True(t, hasTotalField, "meta.total field must be present")
}
