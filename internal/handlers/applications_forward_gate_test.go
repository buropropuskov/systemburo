package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forwardBody собирает тело POST /applications/:id/forward на одного получателя.
func forwardBody(userID int, requiredApproval, canView bool) string {
	return fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":%t,"can_view":%t}]}`,
		userID, requiredApproval, canView)
}

// viewerIDsOf возвращает идентификаторы просматривающих заявки.
func viewerIDsOf(t *testing.T, e *echo.Echo, token string, appID int) map[int]bool {
	t.Helper()
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/viewers", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	viewers := testutil.ParseResponse[[]services.ViewerWithUser](t, rec)
	ids := make(map[int]bool, len(viewers))
	for _, v := range viewers {
		ids[v.UserID] = true
	}
	return ids
}

// responsibleIDsOf возвращает идентификаторы ответственных заявки.
func responsibleIDsOf(t *testing.T, e *echo.Echo, token string, appID int) map[int]bool {
	t.Helper()
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/responsible-users", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	responsibles := testutil.ParseResponse[[]services.ResponsibleUserInfo](t, rec)
	ids := make(map[int]bool, len(responsibles))
	for _, r := range responsibles {
		ids[r.ID] = true
	}
	return ids
}

// TestForwardGate_SuperAdminForwardsForeignApplication: супер-админ пересылает заявку,
// в которой не участвует. Прежний гейт пересылки знал только отправителя и строку в
// application_responsible_users, поэтому супер-админ заявку видел, а кнопок пересылки
// не имел. Заодно фиксируем, что белый список получателей к нему не применяется:
// получатель здесь из чужой организации, то есть вне круга recipientCandidateIDs.
func TestForwardGate_SuperAdminForwardsForeignApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	foreignOrgID, foreignCompanyID := seedOrgAndCompany(t, db, "FwdSuperForeign")

	senderToken := testutil.RegisterAndLogin(t, e, "fwdgate_super_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, db, e, senderToken, td.OrgID)

	// Супер-админ: заявку не подавал, ответственным не назначен, принимающим не является.
	superToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "fwdgate_super_target", "pass123", 1, foreignOrgID, foreignCompanyID)
	targetID := getUserID(t, db, "fwdgate_super_target")

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID),
		forwardBody(targetID, true, false), testutil.AuthHeader(superToken))
	require.Equal(t, http.StatusOK, rec.Code, "супер-админ должен пересылать чужую заявку: %s", rec.Body.String())

	assert.True(t, responsibleIDsOf(t, e, senderToken, appID)[targetID],
		"назначенный супер-админом согласующий должен попасть в ответственных, даже из чужой организации")
}

// TestForwardGate_ApproverForwardsOutsideOwnCircle: принимающий (application_approvers,
// оператор бюро) пересылает заявку, в которой не участвует, получателю из чужой
// организации. Круг recipientCandidateIDs строится от организации и компании самого
// пользователя, а у оператора бюро это бюро - маршрутизация заявок по чужим
// организациям и есть его работа, поэтому белый список к нему не применяется.
func TestForwardGate_ApproverForwardsOutsideOwnCircle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	foreignOrgID, foreignCompanyID := seedOrgAndCompany(t, db, "FwdApproverForeign")

	senderToken := testutil.RegisterAndLogin(t, e, "fwdgate_appr_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, db, e, senderToken, td.OrgID)

	approverToken := testutil.RegisterAndLogin(t, e, "fwdgate_appr", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "fwdgate_appr")

	testutil.RegisterUser(t, e, "fwdgate_appr_target", "pass123", 1, foreignOrgID, foreignCompanyID)
	targetID := getUserID(t, db, "fwdgate_appr_target")

	candidates := testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-recipients", appID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, candidates.Code, candidates.Body.String())
	assert.True(t, candidateIDs(t, testutil.ParseSlice(t, candidates))[targetID])
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID),
		forwardBody(targetID, false, true), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "принимающий должен пересылать любую заявку: %s", rec.Body.String())

	assert.Equal(t, services.ForwardResult{Accepted: 1}, testutil.ParseResponse[services.ForwardResult](t, rec))
	assert.True(t, viewerIDsOf(t, e, senderToken, appID)[targetID],
		"получатель принимающего должен попасть в просматривающие независимо от его организации")
}

// TestForwardGate_ReaderForwardsToReader: просматривающий передаёт заявку дальше тоже на
// просмотр. До правки гейт пересылки читателя не знал вовсе и отдавал 403.
func TestForwardGate_ReaderForwardsToReader(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "fwdgate_r_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, db, e, senderToken, td.OrgID)

	readerToken := testutil.RegisterAndLogin(t, e, "fwdgate_reader", "pass123", 1, td.OrgID, td.CompanyID)
	readerID := getUserID(t, db, "fwdgate_reader")
	testutil.RegisterUser(t, e, "fwdgate_reader2", "pass123", 1, td.OrgID, td.CompanyID)
	reader2ID := getUserID(t, db, "fwdgate_reader2")

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID),
		forwardBody(readerID, false, true), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "отправитель пересылает читателю: %s", rec.Body.String())

	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID),
		forwardBody(reader2ID, false, true), testutil.AuthHeader(readerToken))
	require.Equal(t, http.StatusOK, rec.Code, "читатель должен пересылать заявку читателю: %s", rec.Body.String())

	viewers := viewerIDsOf(t, e, senderToken, appID)
	assert.True(t, viewers[readerID], "первый читатель на месте")
	assert.True(t, viewers[reader2ID], "читатель, добавленный читателем, тоже просматривающий")
}

// TestForwardGate_ReaderCannotAssignResponsible: читателю доступен только просмотр,
// поэтому назначать ответственных и согласующих он не вправе - такой запрос отбивается
// целиком, а не молча превращается в просмотр.
func TestForwardGate_ReaderCannotAssignResponsible(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "fwdgate_ra_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, db, e, senderToken, td.OrgID)

	readerToken := testutil.RegisterAndLogin(t, e, "fwdgate_ra_reader", "pass123", 1, td.OrgID, td.CompanyID)
	readerID := getUserID(t, db, "fwdgate_ra_reader")
	testutil.RegisterUser(t, e, "fwdgate_ra_target", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "fwdgate_ra_target")

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID),
		forwardBody(readerID, false, true), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "подготовка: отправитель делает читателя: %s", rec.Body.String())

	tests := []struct {
		name             string
		requiredApproval bool
		canView          bool
	}{
		{name: "согласующий", requiredApproval: true, canView: false},
		{name: "ответственный без согласования", requiredApproval: false, canView: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID),
				forwardBody(targetID, tc.requiredApproval, tc.canView), testutil.AuthHeader(readerToken))
			require.Equal(t, http.StatusForbidden, rec.Code,
				"читатель не назначает %s: %s", tc.name, rec.Body.String())

			assert.False(t, responsibleIDsOf(t, e, senderToken, appID)[targetID],
				"отбитый запрос не должен оставлять назначение")
			assert.False(t, viewerIDsOf(t, e, senderToken, appID)[targetID],
				"отбитый запрос не должен добавлять и просматривающего")
		})
	}
}

// TestForwardGate_DropsForeignRecipient: подделанный получатель не открывает заявку
// постороннему. Тот же INSERT в application_viewers закрыт на подаче (#1928), и через
// пересылку он обязан быть закрыт тем же кругом. Чужой идентификатор отбрасывается
// с указанием количества пропущенных, свой получатель из того же запроса проходит.
func TestForwardGate_DropsForeignRecipient(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	foreignOrgID, foreignCompanyID := seedOrgAndCompany(t, db, "FwdForeign")

	senderToken := testutil.RegisterAndLogin(t, e, "fwdgate_df_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, db, e, senderToken, td.OrgID)

	testutil.RegisterUser(t, e, "fwdgate_df_colleague", "pass123", 1, td.OrgID, td.CompanyID)
	colleagueID := getUserID(t, db, "fwdgate_df_colleague")
	strangerToken := testutil.RegisterAndLogin(t, e, "fwdgate_df_stranger", "pass123", 1, foreignOrgID, foreignCompanyID)
	strangerID := getUserID(t, db, "fwdgate_df_stranger")

	grantPermission(t, db, "fwdgate_df_sender", "page.admin.users")
	candidatesPath := fmt.Sprintf("/applications/%d/forward-recipients", appID)
	candidates := testutil.GET(t, e, candidatesPath, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, candidates.Code, candidates.Body.String())
	ids := candidateIDs(t, testutil.ParseSlice(t, candidates))
	assert.True(t, ids[colleagueID])
	assert.False(t, ids[strangerID])
	require.Equal(t, http.StatusForbidden, testutil.GET(t, e, candidatesPath, testutil.AuthHeader(strangerToken)).Code)

	body := fmt.Sprintf(`{"users":[
		{"user_id":%d,"required_approval":false,"can_view":true},
		{"user_id":%d,"required_approval":false,"can_view":true},
		{"user_id":%d,"required_approval":true,"can_view":false}
	]}`, colleagueID, strangerID, strangerID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, services.ForwardResult{Accepted: 1, Skipped: 1}, testutil.ParseResponse[services.ForwardResult](t, rec))
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(strangerID, false, true), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, services.ForwardResult{Skipped: 1}, testutil.ParseResponse[services.ForwardResult](t, rec))

	viewers := viewerIDsOf(t, e, senderToken, appID)
	assert.True(t, viewers[colleagueID], "свой коллега остаётся получателем")
	assert.False(t, viewers[strangerID], "посторонний в просматривающие не попадает")
	assert.False(t, responsibleIDsOf(t, e, senderToken, appID)[strangerID],
		"посторонний не попадает и в ответственные")

	arec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(strangerToken))
	assert.Equal(t, http.StatusForbidden, arec.Code, "посторонний не получает доступ к заявке через пересылку")
}

func TestForwardRecipients_HidesRestrictedProfileForDirectoryAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	author := testutil.RegisterAndLogin(t, e, "fwd_mask_author", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, db, e, author, td.OrgID)
	testutil.RegisterUser(t, e, "fwd_mask_hidden", "pass123", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "fwd_mask_hidden", "Скрытый", "Пример", "Тестовый")
	id := getUserID(t, db, "fwd_mask_hidden")
	require.NoError(t, db.Exec("UPDATE users SET position = ? WHERE id = ?", "Учебная должность", id).Error)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	response := testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-recipients", appID), testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	hidden := candidateBy(t, testutil.ParseSlice(t, response), id)
	assert.Equal(t, true, hidden["pd_hidden"])
	assert.Equal(t, false, hidden["can_select"])
	assert.Equal(t, "Имя скрыто", hidden["display_label"])
	for _, field := range []string{"first_name", "middle_name", "last_name", "position", "organization", "company"} {
		assert.Empty(t, hidden[field], field)
	}
}

func TestForwardGate_RepeatedViewerInSingleRequest(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	author := testutil.RegisterAndLogin(t, e, "forward_repeat_author", "pass123", 1, seed.OrgID, seed.CompanyID)
	testutil.RegisterUser(t, e, "forward_repeat_reader", "pass123", 1, seed.OrgID, seed.CompanyID)
	reader := getUserID(t, db, "forward_repeat_reader")
	appID := createSimpleApplication(t, db, e, author, seed.OrgID)
	path := fmt.Sprintf("/applications/%d/forward", appID)
	body := fmt.Sprintf(`{"users":[{"user_id":%d,"can_view":true},{"user_id":%d,"can_view":true}]}`, reader, reader)

	for send := int64(1); send <= 2; send++ {
		rec := testutil.POST(t, e, path, body, testutil.AuthHeader(author))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, services.ForwardResult{Accepted: 1}, testutil.ParseResponse[services.ForwardResult](t, rec))

		var assignments, events, notices int64
		require.NoError(t, db.Table("application_viewers").Where("application_id = ? AND user_id = ?", appID, reader).Count(&assignments).Error)
		assert.EqualValues(t, 1, assignments)
		require.NoError(t, db.Table("audit_log").Where("entity_type = 'application' AND entity_id = ? AND actor_user_id = ? AND action = 'assigned_viewer'", appID, reader).Count(&events).Error)
		assert.Equal(t, send, events)
		require.NoError(t, db.Table("notifications").Where("user_id = ? AND type = ?", reader, services.NotificationTypeApplicationForwarded).Select("COALESCE(SUM(count), 0)").Scan(&notices).Error)
		assert.Equal(t, send, notices)
	}
}
