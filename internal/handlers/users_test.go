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

func TestUsers_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Register a second user
	testutil.RegisterUser(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.GreaterOrEqual(t, len(list), 2) // admin + regularuser

	// Verify response structure
	for _, u := range list {
		assert.Contains(t, u, "id")
		assert.Contains(t, u, "username")
		assert.Contains(t, u, "type_id")
		assert.Contains(t, u, "user_type")
	}
}

func TestUsers_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/users/all", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsers_GetAll_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/users/all", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdateType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Get user types to find the "renter" type_id
	rec := testutil.GET(t, e, "/user-types-management", h)
	require.Equal(t, http.StatusOK, rec.Code)
	var types []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &types))

	var renterTypeID int
	for _, ut := range types {
		if ut["code"] == "renter" {
			renterTypeID = int(ut["id"].(float64))
			break
		}
	}
	require.Greater(t, renterTypeID, 0)

	// Update type
	body := fmt.Sprintf(`{"type_id":%d}`, renterTypeID)
	rec = testutil.PUT(t, e, "/users/targetuser/type", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "User type updated successfully", resp)

	// Verify the change via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	var users []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, float64(renterTypeID), u["type_id"])
			break
		}
	}
}

func TestUsers_UpdateType_InvalidType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/users/targetuser/type", `{"type_id":99999}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUsers_UpdateType_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/users/regularuser/type", `{"type_id":2}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdatePassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "oldpassword", 1, td.OrgID, td.CompanyID)

	// Update password
	rec := testutil.PUT(t, e, "/users/targetuser/password", `{"password":"newpassword123"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Password updated successfully", resp)

	// Verify new password works by logging in
	rec = testutil.POST(t, e, "/login", `{"username":"targetuser","password":"newpassword123"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify old password no longer works
	rec = testutil.POST(t, e, "/login", `{"username":"targetuser","password":"oldpassword"}`, nil)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestUsers_UpdatePassword_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/users/regularuser/password", `{"password":"newpass"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdateInfo(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	body := `{
		"last_name": "Иванов",
		"first_name": "Иван",
		"middle_name": "Иванович",
		"position": "Инженер",
		"email": "ivanov@test.com",
		"phone": "+79001234567"
	}`
	rec := testutil.PUT(t, e, "/users/targetuser/info", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "User info updated successfully", resp)

	// Verify via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var users []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, "Иванов", u["last_name"])
			assert.Equal(t, "Иван", u["first_name"])
			assert.Equal(t, "Иванович", u["middle_name"])
			assert.Equal(t, "Инженер", u["position"])
			assert.Equal(t, "ivanov@test.com", u["email"])
			assert.Equal(t, "+79001234567", u["phone"])
			break
		}
	}
}

func TestUsers_UpdateInfo_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/users/regularuser/info", `{"last_name":"Test"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_UpdateOrganization(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Create a second organization
	rec := testutil.POST(t, e, "/organizations", `{"name":"New Organization"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var orgResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &orgResp))
	newOrgID := int(orgResp["id"].(float64))

	// Update user's organization
	body := fmt.Sprintf(`{"organization_id":%d}`, newOrgID)
	rec = testutil.PUT(t, e, "/users/targetuser/organization", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Organization updated successfully", resp)

	// Verify via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var users []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, float64(newOrgID), u["organization_id"])
			break
		}
	}
}

func TestUsers_UpdateCompany(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Create a second company
	rec := testutil.POST(t, e, "/companies", `{"name":"New Company"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var compResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &compResp))
	newCompID := int(compResp["id"].(float64))

	// Update user's company
	body := fmt.Sprintf(`{"company_id":%d}`, newCompID)
	rec = testutil.PUT(t, e, "/users/targetuser/company", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Company updated successfully", resp)

	// Verify via GetAll
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var users []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))

	for _, u := range users {
		if u["username"] == "targetuser" {
			assert.Equal(t, float64(newCompID), u["company_id"])
			break
		}
	}
}

func TestUsers_Delete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "targetuser", "password123", 1, td.OrgID, td.CompanyID)

	// Verify user exists
	rec := testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	var users []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))

	found := false
	for _, u := range users {
		if u["username"] == "targetuser" {
			found = true
			break
		}
	}
	require.True(t, found, "targetuser should exist before deletion")

	// Delete
	rec = testutil.DELETE(t, e, "/users/targetuser", h)
	require.Equal(t, http.StatusOK, rec.Code)

	var deleteResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &deleteResp))
	assert.Equal(t, "User deleted successfully", deleteResp["message"])

	// Verify gone
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))

	for _, u := range users {
		assert.NotEqual(t, "targetuser", u["username"])
	}
}

func TestUsers_Delete_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.DELETE(t, e, "/users/someuser", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsers_Delete_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/users/regularuser", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsers_ManagerAlsoHasAdminAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// type_id=5 is "manager" -- also has admin privileges
	managerToken := testutil.RegisterAndLogin(t, e, "manager1", "password123", 5, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(managerToken)

	rec := testutil.GET(t, e, "/users/all", h)
	assert.Equal(t, http.StatusOK, rec.Code)
}
