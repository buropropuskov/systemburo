package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

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
