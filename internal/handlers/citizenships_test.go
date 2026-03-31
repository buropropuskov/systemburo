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
	updateBody := fmt.Sprintf(
		`{"name":"РФ обновлённое","icon":"🏳️","is_active":true,"is_default":false,"patent_required":true}`)
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

	// --- Delete ---
	rec = testutil.DELETE(t, e, fmt.Sprintf("/citizenships/%d", citizenshipID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	deleteResp := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Гражданство успешно удалено", deleteResp)

	// --- Read (verify gone) ---
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
