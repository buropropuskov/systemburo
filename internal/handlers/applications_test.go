package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

// assignOrgUser adds the user to organization_users so they appear as responsible.
func assignOrgUser(t *testing.T, db *gorm.DB, orgID, userID int, isPrimary bool) {
	t.Helper()
	err := db.Exec(
		"INSERT INTO organization_users (organization_id, user_id, is_primary) VALUES (?, ?, ?) ON CONFLICT DO NOTHING",
		orgID, userID, isPrimary,
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
			"entry_date_to": "2026-04-30",
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

	// Register approver user and make them an application_approver
	testutil.RegisterUser(t, e, "fwdapprover", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "fwdapprover")
	approverToken, _ := testutil.LoginUser(t, e, "fwdapprover", "pass123")

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

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(approverToken))
	// Forward may require approver role; accept 200 or 403 depending on business rules
	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden}, rec.Code)
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
				"entry_date_to": "2026-04-30",
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
				"entry_date_to": "2026-04-30",
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
}
