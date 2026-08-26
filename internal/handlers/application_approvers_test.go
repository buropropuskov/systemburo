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

func TestApprovers_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/application-approvers", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestApprovers_GetAll_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Regular user should get 403
	userToken := testutil.RegisterAndLogin(t, e, "approveruser", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/application-approvers", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Admin should succeed
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/application-approvers", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	approvers := testutil.ParseSlice(t, rec)
	// Initially empty
	assert.IsType(t, []map[string]interface{}{}, approvers)
}

func TestApprovers_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Register a target user to make an approver
	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Get target user ID
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	// Get available users
	rec := testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	available := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(available), 1, "should have at least one available user")

	// Find the target user ID
	var targetUserID int
	for _, u := range available {
		if u["username"] == "targetuser" {
			targetUserID = int(u["id"].(float64))
			break
		}
	}
	require.Greater(t, targetUserID, 0, "target user should be in available users list")

	// Create approver
	body := fmt.Sprintf(`{"user_id":%d}`, targetUserID)
	rec = testutil.POST(t, e, "/application-approvers", body, adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Approver added successfully", createResp["message"])

	// Get all approvers -- should have one
	rec = testutil.GET(t, e, "/application-approvers", adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	approvers := testutil.ParseSlice(t, rec)
	assert.Len(t, approvers, 1)
	assert.Equal(t, "targetuser", approvers[0]["username"])
	assert.Contains(t, approvers[0], "id")
	assert.Contains(t, approvers[0], "user_id")
	assert.Contains(t, approvers[0], "created_at")

	approverID := int(approvers[0]["id"].(float64))

	// Target user should no longer be in available users
	rec = testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	available = testutil.ParseSlice(t, rec)
	for _, u := range available {
		assert.NotEqual(t, "targetuser", u["username"],
			"targetuser should not be in available users after being added as approver")
	}

	// Delete approver
	rec = testutil.DELETE(t, e, fmt.Sprintf("/application-approvers/%d", approverID), adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	delResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Approver deleted successfully", delResp["message"])

	// Verify deleted
	rec = testutil.GET(t, e, "/application-approvers", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	approvers = testutil.ParseSlice(t, rec)
	assert.Len(t, approvers, 0)
}

func TestApprovers_Create_DuplicateUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "dupapprover", "password123", 1, td.OrgID, td.CompanyID)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	// Get user ID
	rec := testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	available := testutil.ParseSlice(t, rec)

	var userID int
	for _, u := range available {
		if u["username"] == "dupapprover" {
			userID = int(u["id"].(float64))
			break
		}
	}
	require.Greater(t, userID, 0)

	body := fmt.Sprintf(`{"user_id":%d}`, userID)

	rec = testutil.POST(t, e, "/application-approvers", body, adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Try adding again -- should fail
	rec = testutil.POST(t, e, "/application-approvers", body, adminH)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApprovers_Create_UserNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	rec := testutil.POST(t, e, "/application-approvers", `{"user_id":99999}`, adminH)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApprovers_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	rec := testutil.DELETE(t, e, "/application-approvers/99999", adminH)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestApprovers_GetAvailableUsers_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "nonadmin", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/application-approvers/available-users", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestApprovers_Create_RegularUserForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "noperm", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/application-approvers", `{"user_id":1}`, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestApprovers_History_CreateWritesEntry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "histuser", "password123", 1, td.OrgID, td.CompanyID)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	rec := testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	available := testutil.ParseSlice(t, rec)

	var targetUserID int
	for _, u := range available {
		if u["username"] == "histuser" {
			targetUserID = int(u["id"].(float64))
			break
		}
	}
	require.Greater(t, targetUserID, 0)

	body := fmt.Sprintf(`{"user_id":%d}`, targetUserID)
	rec = testutil.POST(t, e, "/application-approvers", body, adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	// История должна содержать запись created.
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	require.Len(t, history, 1)
	entry := history[0]
	assert.Equal(t, "created", entry["action_type"])
	assert.Equal(t, float64(targetUserID), entry["approver_user_id"])
	assert.NotEmpty(t, entry["approver_name"])
	// actor_user_id должен быть заполнен (admin создавал).
	assert.NotNil(t, entry["actor_user_id"])
}

func TestApprovers_History_DeleteWritesEntry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "delhistuser", "password123", 1, td.OrgID, td.CompanyID)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	rec := testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	available := testutil.ParseSlice(t, rec)

	var targetUserID int
	for _, u := range available {
		if u["username"] == "delhistuser" {
			targetUserID = int(u["id"].(float64))
			break
		}
	}
	require.Greater(t, targetUserID, 0)

	body := fmt.Sprintf(`{"user_id":%d}`, targetUserID)
	rec = testutil.POST(t, e, "/application-approvers", body, adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = testutil.GET(t, e, "/application-approvers", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	approvers := testutil.ParseSlice(t, rec)
	require.Len(t, approvers, 1)
	approverID := int(approvers[0]["id"].(float64))

	rec = testutil.DELETE(t, e, fmt.Sprintf("/application-approvers/%d", approverID), adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	// История: две записи (created + deleted), deleted — новее, стоит первой.
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(history), 2)

	// Первая запись — deleted (новее).
	assert.Equal(t, "deleted", history[0]["action_type"])
	assert.Equal(t, float64(targetUserID), history[0]["approver_user_id"])
	// Снимок имени не пустой.
	assert.NotEmpty(t, history[0]["approver_name"])
	// actor заполнен.
	assert.NotNil(t, history[0]["actor_user_id"])
}

func TestApprovers_History_OrderNewestFirst(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "orduser1", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "orduser2", "password123", 1, td.OrgID, td.CompanyID)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	rec := testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	available := testutil.ParseSlice(t, rec)

	var uid1, uid2 int
	for _, u := range available {
		switch u["username"] {
		case "orduser1":
			uid1 = int(u["id"].(float64))
		case "orduser2":
			uid2 = int(u["id"].(float64))
		}
	}
	require.Greater(t, uid1, 0)
	require.Greater(t, uid2, 0)

	rec = testutil.POST(t, e, "/application-approvers", fmt.Sprintf(`{"user_id":%d}`, uid1), adminH)
	require.Equal(t, http.StatusCreated, rec.Code)
	rec = testutil.POST(t, e, "/application-approvers", fmt.Sprintf(`{"user_id":%d}`, uid2), adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	require.Len(t, history, 2)

	// Новые сверху: uid2 добавлен последним -> стоит первым в журнале.
	assert.Equal(t, float64(uid2), history[0]["approver_user_id"])
	assert.Equal(t, float64(uid1), history[1]["approver_user_id"])

	// Оба — created, актор заполнен, actor_name не пустой.
	for _, h := range history {
		assert.Equal(t, "created", h["action_type"])
		assert.NotNil(t, h["actor_user_id"])
		assert.NotEmpty(t, h["actor_name"])
	}
}

// Принимающий узнаёт о своей роли САМ, без права администратора. Полный состав
// принимающих закрыт админом, и пока карточка выводила роль из него, принимающий без
// page.admin получал пустой список и не видел ни одной своей кнопки - ни приёма заявки
// в работу, ни решения по дополнению. Ошибки при этом не было нигде: 403 гасится молча.
func TestApprovers_Me_AvailableWithoutAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "plain_approver", "password123", 1, td.OrgID, td.CompanyID)

	// Пока роли нет - честный false, а не отказ.
	rec := testutil.GET(t, e, "/application-approvers/me", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "ответ про себя доступен любому авторизованному: %s", rec.Body.String())
	assert.False(t, testutil.ParseResponse[map[string]bool](t, rec)["is_approver"])

	// Тот же пользователь, назначенный принимающим, видит себя принимающим.
	makeApprover(t, db, "plain_approver")
	rec = testutil.GET(t, e, "/application-approvers/me", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, testutil.ParseResponse[map[string]bool](t, rec)["is_approver"],
		"принимающий обязан узнавать о своей роли без права администратора")

	// А полный состав ему по-прежнему закрыт - это админская информация.
	rec = testutil.GET(t, e, "/application-approvers", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "состав принимающих остаётся под правом администратора")
}

// Согласующий назначается per-application, глобального признака у него не было -
// гейтить обучающий тур согласования было нечем (#1737). Признак выводится из
// назначений в организациях и компаниях, и он не должен путаться с принимающим.
func TestApprovers_Me_IsReviewerFromRequiredApproval(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	plainToken := testutil.RegisterAndLogin(t, e, "plain_member", "password123", 1, td.OrgID, td.CompanyID)
	orgToken := testutil.RegisterAndLogin(t, e, "org_reviewer", "password123", 1, td.OrgID, td.CompanyID)
	compToken := testutil.RegisterAndLogin(t, e, "comp_reviewer", "password123", 1, td.OrgID, td.CompanyID)

	userID := func(username string) int {
		var user models.User
		require.NoError(t, db.Where("username = ?", username).First(&user).Error)
		return user.ID
	}

	// Обычное членство в организации согласующим не делает - только required_approval.
	require.NoError(t, db.Create(&models.OrganizationUser{
		OrganizationID: td.OrgID, UserID: userID("plain_member"),
	}).Error)
	require.NoError(t, db.Create(&models.OrganizationUser{
		OrganizationID: td.OrgID, UserID: userID("org_reviewer"), RequiredApproval: true,
	}).Error)
	require.NoError(t, db.Create(&models.CompaniesUser{
		CompanyID: td.CompanyID, UserID: userID("comp_reviewer"), RequiredApproval: true,
	}).Error)

	roles := func(token string) map[string]bool {
		rec := testutil.GET(t, e, "/application-approvers/me", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return testutil.ParseResponse[map[string]bool](t, rec)
	}

	plain := roles(plainToken)
	assert.False(t, plain["is_reviewer"], "членство без required_approval согласующим не делает")
	assert.False(t, plain["is_approver"])

	assert.True(t, roles(orgToken)["is_reviewer"], "согласующий организации")
	assert.True(t, roles(compToken)["is_reviewer"], "согласующий компании")

	// Принимающий - другая роль: назначение в неё признак согласующего не поднимает.
	makeApprover(t, db, "plain_member")
	afterApprover := roles(plainToken)
	assert.True(t, afterApprover["is_approver"], "поле принимающего сохраняет прежний смысл")
	assert.False(t, afterApprover["is_reviewer"], "принимающий не становится согласующим")
}
