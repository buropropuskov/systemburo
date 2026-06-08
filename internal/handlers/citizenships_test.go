package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCitizenships_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Empty list initially
	rec := testutil.GET(t, e, "/citizenships", h)
	assert.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	assert.Empty(t, list)
}

func TestCitizenships_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/citizenships", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCitizenships_CRUD_Cycle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// --- Create ---
	body := `{"name":"Российская Федерация","icon":"🇷🇺","is_default":true,"patent_required":false}`
	rec := testutil.POST(t, e, "/citizenships", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Гражданство успешно создано", createResp["message"])
	citizenshipID := int(createResp["id"].(float64))
	assert.Greater(t, citizenshipID, 0)

	// --- Read (verify created) ---
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, "Российская Федерация", list[0]["name"])
	assert.Equal(t, "🇷🇺", list[0]["icon"])
	assert.Equal(t, true, list[0]["is_default"])
	assert.Equal(t, false, list[0]["patent_required"])

	// --- Update ---
	updateBody := `{"name":"РФ обновлённое","icon":"🏳️","is_default":false,"patent_required":true}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/citizenships/%d", citizenshipID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	updateResp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Гражданство успешно обновлено", updateResp)

	// --- Read (verify updated) ---
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list = testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, "РФ обновлённое", list[0]["name"])
	assert.Equal(t, false, list[0]["is_default"])
	assert.Equal(t, true, list[0]["patent_required"])

	// --- Delete (архив, soft-delete) ---
	rec = testutil.DELETE(t, e, fmt.Sprintf("/citizenships/%d", citizenshipID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	deleteResp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Гражданство архивировано", deleteResp)

	// --- Read (активные не показывают архивное) ---
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list = testutil.ParseSlice(t, rec)
	assert.Empty(t, list)
}

func TestCitizenships_Create_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// type_id=1 is "user" — not an admin
	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"name":"Тест","icon":"🏳️"}`
	rec := testutil.POST(t, e, "/citizenships", body, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCitizenships_Update_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/citizenships/1", `{"name":"Тест"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCitizenships_Delete_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/citizenships/1", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCitizenships_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/citizenships/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCitizenships_Update_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/citizenships/99999", `{"name":"ghost"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCitizenships_ClearDefaults(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create citizenship with is_default=true
	body := `{"name":"Default","icon":"🏳️","is_default":true}`
	rec := testutil.POST(t, e, "/citizenships", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify it's default
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, true, list[0]["is_default"])

	// Clear defaults
	rec = testutil.POST(t, e, "/citizenships/clear-default", "", h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Все гражданства по умолчанию сброшены", resp)

	// Verify default cleared
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, false, list[0]["is_default"])
}

func TestCitizenships_ClearDefaults_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/citizenships/clear-default", "", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCitizenships_Create_DefaultClears_Previous(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create first default
	rec := testutil.POST(t, e, "/citizenships", `{"name":"First","is_default":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Create second default — should clear the first
	rec = testutil.POST(t, e, "/citizenships", `{"name":"Second","is_default":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify only the second is default
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 2)

	defaultCount := 0
	for _, c := range list {
		if c["is_default"] == true {
			defaultCount++
			assert.Equal(t, "Second", c["name"])
		}
	}
	assert.Equal(t, 1, defaultCount)
}

func TestCitizenships_Archive_Restore_Cycle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/citizenships", `{"name":"Узбекистан"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Архив (soft-delete)
	rec = testutil.DELETE(t, e, fmt.Sprintf("/citizenships/%d", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Гражданство архивировано", testutil.ParseMessage(t, rec))

	// Активные - пусто, архив виден через include_archived
	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.ParseSlice(t, rec))

	rec = testutil.GET(t, e, "/citizenships?include_archived=true", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, false, list[0]["is_active"])

	// Восстановление
	rec = testutil.POST(t, e, fmt.Sprintf("/citizenships/%d/restore", id), "", h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Гражданство восстановлено из архива", testutil.ParseMessage(t, rec))

	rec = testutil.GET(t, e, "/citizenships", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, true, list[0]["is_active"])
}

func TestCitizenships_Archive_Default_Conflict(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/citizenships", `{"name":"РФ","is_default":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Дефолтное гражданство архивировать нельзя - 409.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/citizenships/%d", id), h)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCitizenships_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/citizenships", `{"name":"Таджикистан","patent_required":false}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Изменение имени + патента -> запись updated с diff.
	rec = testutil.PUT(t, e, fmt.Sprintf("/citizenships/%d", id),
		`{"name":"Республика Таджикистан","patent_required":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Архив + восстановление.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/citizenships/%d", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = testutil.POST(t, e, fmt.Sprintf("/citizenships/%d/restore", id), "", h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/citizenships/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 4)

	// Новые сверху: restored, archived, updated, created.
	assert.Equal(t, "restored", hist[0]["action_type"])
	assert.Equal(t, "archived", hist[1]["action_type"])
	assert.Equal(t, "updated", hist[2]["action_type"])
	assert.Equal(t, "created", hist[3]["action_type"])

	// created - имя в details.
	created := hist[3]["details"].(map[string]interface{})
	assert.Equal(t, "Таджикистан", created["name"])

	// updated - diff по name и patent_required.
	updated := hist[2]["details"].(map[string]interface{})
	name := updated["name"].(map[string]interface{})
	assert.Equal(t, "Таджикистан", name["old"])
	assert.Equal(t, "Республика Таджикистан", name["new"])
	patent := updated["patent_required"].(map[string]interface{})
	assert.Equal(t, false, patent["old"])
	assert.Equal(t, true, patent["new"])
}
