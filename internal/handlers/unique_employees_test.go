package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniqueEmployees_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/unique-employees", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUniqueEmployees_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create
	body := fmt.Sprintf(`{
		"last_name":"Ivanov",
		"first_name":"Ivan",
		"middle_name":"Ivanovich",
		"passport_series_number":"1234 567890",
		"position":"Engineer",
		"organization_id":%d,
		"company_id":%d
	}`, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var createResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createResp))
	assert.NotNil(t, createResp["id"])
	empID := int(createResp["id"].(float64))
	assert.Greater(t, empID, 0)
	assert.Equal(t, "Ivanov", createResp["last_name"])
	assert.Equal(t, "Ivan", createResp["first_name"])
	assert.Equal(t, false, createResp["status"])

	// Get all (default filter_type=user)
	rec = testutil.GET(t, e, "/unique-employees", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var listResp []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	assert.GreaterOrEqual(t, len(listResp), 1)

	// Update
	updateBody := fmt.Sprintf(`{
		"last_name":"Petrov",
		"first_name":"Petr",
		"passport_series_number":"9999 111111",
		"organization_id":%d,
		"company_id":%d
	}`, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", empID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var updateResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updateResp))
	assert.Equal(t, "Petrov", updateResp["last_name"])
	assert.Equal(t, "Petr", updateResp["first_name"])

	// Delete
	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-employees/%d", empID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	var delResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &delResp))
	assert.Equal(t, "Employee deleted successfully", delResp["message"])
}

func TestUniqueEmployees_DuplicatePassport(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"last_name":"Dup","first_name":"Test","passport_series_number":"DUP 123456"}`
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Same passport = duplicate
	rec = testutil.POST(t, e, "/unique-employees", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUniqueEmployees_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/unique-employees/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUniqueEmployees_GetOwnershipInfo(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/unique-employees/ownership-info", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var info map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	assert.Contains(t, info, "has_organization")
	assert.Contains(t, info, "has_company")
	assert.Contains(t, info, "user_id")
	assert.Contains(t, info, "organization_id")
	assert.Contains(t, info, "company_id")
}

func TestUniqueEmployees_FilterTypes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create an employee
	body := fmt.Sprintf(`{"last_name":"Filter","first_name":"Test","organization_id":%d}`, td.OrgID)
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	filters := []string{"user", "organization", "company", "all"}
	for _, f := range filters {
		rec = testutil.GET(t, e, "/unique-employees?filter_type="+f, h)
		assert.Equal(t, http.StatusOK, rec.Code, "filter_type=%s should return 200", f)
	}
}

func TestUniqueEmployees_CreateWithoutPassport(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Should work without passport (no uniqueness check triggered)
	body := `{"last_name":"NoPassport","first_name":"Worker"}`
	rec := testutil.POST(t, e, "/unique-employees", body, h)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUniqueEmployees_Update_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/unique-employees/99999", `{"last_name":"X"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
