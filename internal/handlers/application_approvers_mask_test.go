package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApprovers_Update_SetAndClearMask: PATCH задаёт/снимает маску отображаемого имени,
// GET её возвращает, пустая строка снимает, несуществующий id -> 404, не-админ -> 403.
func TestApprovers_Update_SetAndClearMask(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "maskuser", "password123", 1, td.OrgID, td.CompanyID)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	userID := getUserID(t, db, "maskuser")
	rec := testutil.POST(t, e, "/application-approvers", fmt.Sprintf(`{"user_id":%d}`, userID), adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = testutil.GET(t, e, "/application-approvers", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	approvers := testutil.ParseSlice(t, rec)
	require.Len(t, approvers, 1)
	assert.Nil(t, approvers[0]["display_name"], "маска изначально пуста")
	approverID := int(approvers[0]["id"].(float64))
	path := fmt.Sprintf("/application-approvers/%d", approverID)

	// Задать маску.
	rec = testutil.PATCH(t, e, path, `{"display_name":"Оператор Бюро"}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.GET(t, e, "/application-approvers", adminH)
	approvers = testutil.ParseSlice(t, rec)
	assert.Equal(t, "Оператор Бюро", approvers[0]["display_name"])

	// Пустая строка/пробелы снимают маску.
	rec = testutil.PATCH(t, e, path, `{"display_name":"   "}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = testutil.GET(t, e, "/application-approvers", adminH)
	approvers = testutil.ParseSlice(t, rec)
	assert.Nil(t, approvers[0]["display_name"], "пустая строка снимает маску")

	// Несуществующий id -> 404.
	rec = testutil.PATCH(t, e, "/application-approvers/99999", `{"display_name":"X"}`, adminH)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Не-админ -> 403.
	userToken := testutil.RegisterAndLogin(t, e, "masknonadmin", "password123", 1, td.OrgID, td.CompanyID)
	rec = testutil.PATCH(t, e, path, `{"display_name":"X"}`, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestApprovers_Mask_AppliedToResponsibleAndHistory: маска принимающего подменяет ФИО в
// заявитель-видимых местах (деталь "Принял", /details, список, история заявки) и НЕ трогает
// внутренний аудит принимающих. Снятие маски возвращает реальное ФИО.
func TestApprovers_Mask_AppliedToResponsibleAndHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Принимающий (тип 6) с реальным ФИО.
	testutil.RegisterUser(t, e, "maskappr", "pass123", 6, td.OrgID, td.CompanyID)
	approverUserID := getUserID(t, db, "maskappr")
	require.NoError(t, db.Exec(
		"UPDATE users SET last_name='Мякотных', first_name='Сергей', middle_name='Михайлович' WHERE id=?",
		approverUserID).Error)
	approverToken, _ := testutil.LoginUser(t, e, "maskappr", "pass123")

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	// Добавить принимающего, получить id записи.
	rec := testutil.POST(t, e, "/application-approvers", fmt.Sprintf(`{"user_id":%d}`, approverUserID), adminH)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = testutil.GET(t, e, "/application-approvers", adminH)
	approvers := testutil.ParseSlice(t, rec)
	require.Len(t, approvers, 1)
	approverRecordID := int(approvers[0]["id"].(float64))

	// Сендер создаёт заявку; принимающий читает и берёт в работу -> responsible = принимающий.
	senderToken := testutil.RegisterAndLogin(t, e, "masksndr", "pass123", 1, td.OrgID, td.CompanyID)
	senderH := testutil.AuthHeader(senderToken)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "approver read: %s", rec.Body.String())
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
		fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverUserID),
		testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "take-to-work: %s", rec.Body.String())

	// Без маски: сендер видит реальное ФИО в "Принял".
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), senderH)
	require.Equal(t, http.StatusOK, rec.Code)
	detail := testutil.ParseMap(t, rec)
	assert.Contains(t, detail["responsible_name"].(string), "Мякотных")

	// Ставим маску.
	maskPath := fmt.Sprintf("/application-approvers/%d", approverRecordID)
	rec = testutil.PATCH(t, e, maskPath, `{"display_name":"Оператор Бюро"}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Деталь заявки: responsible_name и responsible_full_name замаскированы.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), senderH)
	detail = testutil.ParseMap(t, rec)
	assert.Equal(t, "Оператор Бюро", detail["responsible_name"])
	assert.Equal(t, "Оператор Бюро", detail["responsible_full_name"])

	// /details тоже маскируется.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), senderH)
	det2 := testutil.ParseMap(t, rec)
	assert.Equal(t, "Оператор Бюро", det2["responsible_name"])

	// Список заявок сендера — responsible_name замаскирован.
	rec = testutil.GET(t, e, "/applications/user", senderH)
	list := testutil.ParseSlice(t, rec)
	var foundInList bool
	for _, a := range list {
		if int(a["id"].(float64)) == appID {
			foundInList = true
			assert.Equal(t, "Оператор Бюро", a["responsible_name"])
		}
	}
	require.True(t, foundInList, "заявка должна быть в списке сендера")

	// История заявки: актор take_to_work замаскирован.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), senderH)
	history := testutil.ParseSlice(t, rec)
	var sawTake bool
	for _, h := range history {
		if h["action_type"] == "take_to_work" {
			sawTake = true
			assert.Equal(t, "Оператор Бюро", h["user_name"])
		}
	}
	require.True(t, sawTake, "должна быть запись take_to_work в истории")

	// Guard: внутренний аудит принимающих показывает РЕАЛЬНОЕ ФИО, не маску.
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	apprHist := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, apprHist)
	var sawRenamed bool
	for _, h := range apprHist {
		assert.NotEqual(t, "Оператор Бюро", h["approver_name"], "аудит принимающих не должен маскироваться")
		if h["action_type"] == "renamed" {
			sawRenamed = true
			assert.Contains(t, h["approver_name"].(string), "Мякотных")
		}
	}
	assert.True(t, sawRenamed, "должна быть запись renamed в аудите принимающих")

	// Снятие маски возвращает реальное ФИО в деталь.
	rec = testutil.PATCH(t, e, maskPath, `{"display_name":null}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), senderH)
	detail = testutil.ParseMap(t, rec)
	assert.Contains(t, detail["responsible_name"].(string), "Мякотных")
}
