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

// usernamesOf собирает логины из ответа списка кандидатов.
func usernamesOf(list []map[string]any) map[string]bool {
	found := make(map[string]bool, len(list))
	for _, item := range list {
		if login, ok := item["username"].(string); ok {
			found[login] = true
		}
	}
	return found
}

func seedOrgAndCompany(t *testing.T, db *gorm.DB, name string) (int, int) {
	t.Helper()
	org := models.Organization{Name: name + " Org"}
	require.NoError(t, db.Create(&org).Error)
	comp := models.Company{Name: name + " Company"}
	require.NoError(t, db.Create(&comp).Error)
	return org.ID, comp.ID
}

// TestRecipientCandidates_NoAdminRightsNeeded: арендатор без права page.admin.users
// получает список получателей. Раньше форма подачи брала его из админского /users/all
// и ловила 403 - тост «Недостаточно прав» и пустой выбор получателей.
func TestRecipientCandidates_NoAdminRightsNeeded(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	otherOrgID, otherCompanyID := seedOrgAndCompany(t, db, "Foreign")

	tenantToken := testutil.RegisterAndLogin(t, e, "rc_tenant", "pass123", 2, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rc_colleague", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterManager(t, e, "rc_boss", otherOrgID, otherCompanyID)
	testutil.RegisterUser(t, e, "rc_stranger", "pass123", 1, otherOrgID, otherCompanyID)

	rec := testutil.GET(t, e, "/users/recipient-candidates", testutil.AuthHeader(tenantToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	found := usernamesOf(testutil.ParseSlice(t, rec))
	assert.True(t, found["rc_colleague"], "коллега по организации доступен получателем")
	assert.True(t, found["rc_boss"], "руководитель доступен получателем и из чужой организации")
	assert.False(t, found["rc_stranger"], "рядовой пользователь чужой организации не показывается")
	assert.False(t, found["rc_tenant"], "себя в получателях не предлагаем")
}

// TestRecipientCandidates_SkipsArchivedAndBanned: архивным и заблокированным заявку не
// пересылают - они не должны попадать в выбор.
func TestRecipientCandidates_SkipsArchivedAndBanned(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tenantToken := testutil.RegisterAndLogin(t, e, "rc2_tenant", "pass123", 2, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rc2_archived", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rc2_banned", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rc2_active", "pass123", 1, td.OrgID, td.CompanyID)

	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "rc2_archived").
		Update("is_active", false).Error)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "rc2_banned").
		Update("is_banned", true).Error)

	rec := testutil.GET(t, e, "/users/recipient-candidates", testutil.AuthHeader(tenantToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	found := usernamesOf(testutil.ParseSlice(t, rec))
	assert.True(t, found["rc2_active"], "активный коллега на месте")
	assert.False(t, found["rc2_archived"], "архивный не предлагается")
	assert.False(t, found["rc2_banned"], "заблокированный не предлагается")
}

// TestSubmitCompleteApplication_DropsForeignReader: подделанный readers не открывает
// заявку постороннему. Форма предлагает только своих, но запрос можно собрать руками -
// бэк обязан отбросить чужой идентификатор сам, не завалив при этом подачу.
func TestSubmitCompleteApplication_DropsForeignReader(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	otherOrgID, otherCompanyID := seedOrgAndCompany(t, db, "Outsider")

	senderToken := testutil.RegisterAndLogin(t, e, "rc3_sender", "pass123", 2, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rc3_colleague", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterManager(t, e, "rc3_boss", otherOrgID, otherCompanyID)
	strangerToken := testutil.RegisterAndLogin(t, e, "rc3_stranger", "pass123", 1, otherOrgID, otherCompanyID)

	colleagueID := getUserID(t, db, "rc3_colleague")
	bossID := getUserID(t, db, "rc3_boss")
	strangerID := getUserID(t, db, "rc3_stranger")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_rc3", "Cars RC3")

	body := fmt.Sprintf(`{
		"message": "app with forged reader",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"readers": [%d, %d, %d],
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
	}`, colleagueID, bossID, strangerID, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "чужой читатель отбрасывается молча, подача проходит: %s", rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	vrec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/viewers", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, vrec.Code, vrec.Body.String())
	viewers := testutil.ParseResponse[[]services.ViewerWithUser](t, vrec)

	viewerIDs := make(map[int]bool, len(viewers))
	for _, v := range viewers {
		viewerIDs[v.UserID] = true
	}
	assert.True(t, viewerIDs[colleagueID], "свой коллега остаётся читателем")
	// Пока фронт не переехал на новый эндпоинт, он шлёт в readers руководителей из
	// любых организаций (фильтр по типу в ApplicationRecipientsRow). Валидация обязана
	// их пропускать, иначе этот срез ломает подачу до выхода фронтового.
	assert.True(t, viewerIDs[bossID], "руководитель из чужой организации остаётся допустимым читателем")
	assert.False(t, viewerIDs[strangerID], "посторонний в читатели не попадает")

	// И заявку он по-прежнему не видит.
	arec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(strangerToken))
	assert.Equal(t, http.StatusForbidden, arec.Code, "посторонний не получает доступ к заявке")
}
