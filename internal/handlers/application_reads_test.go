package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func makeApprover(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	db.Create(&models.ApplicationApprover{UserID: user.ID})
}

// --- Part 1: Individual Reads ---

func TestMarkAsRead_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestMarkAsRead_Idempotent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	// First call
	rec1 := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Second call — should also succeed (idempotent)
	rec2 := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec2.Code)

	// Verify only one read record exists
	var count int64
	db.Model(&models.ApplicationRead{}).Where("application_id = ?", appID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestMarkAsRead_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/applications/99999/read", "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetReads_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, adminToken, td.OrgID)

	// Mark as read
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	// Get reads
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/reads", appID), testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	var reads []models.ApplicationReadResponse
	err := json.Unmarshal(rec.Body.Bytes(), &reads)
	require.NoError(t, err)
	assert.Len(t, reads, 1)
	assert.Equal(t, "testadmin", reads[0].Username)
}

func TestGetReads_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/reads", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	var reads []models.ApplicationReadResponse
	err := json.Unmarshal(rec.Body.Bytes(), &reads)
	require.NoError(t, err)
	assert.Empty(t, reads)
}

func TestGetUnreadCount_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create two applications
	appID1 := createSimpleApplication(t, e, token, td.OrgID)
	_ = createSimpleApplication(t, e, token, td.OrgID)

	// Check unread count before reading
	rec := testutil.GET(t, e, "/applications/unread-count", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp models.UnreadCountResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)

	// Mark one as read
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID1), "", testutil.AuthHeader(token))

	// Check unread count after reading one
	rec = testutil.GET(t, e, "/applications/unread-count", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Count)
}

// --- Part 2: Archive ---

func TestGetApplications_ArchiveDefault_ExcludesArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	// Create an application that will be archived
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl", "Cars")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	// Make it archived: set status to Завершено and entry_date_to to > 1 month ago
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Create a normal active application
	_ = createSimpleApplication(t, e, token, td.OrgID)

	// Default (no archive param) should exclude archived
	rec := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	var apps []map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &apps)
	require.NoError(t, err)

	// The archived application should NOT be in the results
	for _, app := range apps {
		assert.NotEqual(t, float64(appID), app["id"], "archived application should be excluded by default")
	}
}

func TestGetApplications_ArchiveTrue_ShowsOnlyArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	// Create an archived application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl2", "Cars2")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Create a normal active application
	_ = createSimpleApplication(t, e, token, td.OrgID)

	// archive=true should show only archived
	rec := testutil.GET(t, e, "/applications?archive=true", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	var apps []map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &apps)
	require.NoError(t, err)

	// Should contain only the archived application
	assert.Len(t, apps, 1)
	assert.Equal(t, float64(appID), apps[0]["id"])
}

func TestForwardApplication_ArchivedReturns403(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create and archive an application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl3", "Cars3")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Try to forward — should get 403
	body := `{"users":[{"user_id":1,"required_approval":true}]}`
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestApproveApplication_ArchivedReturns403(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "testadmin")

	// Create and archive an application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl4", "Cars4")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Try to approve — should get 403
	body := fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, adminID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTakeToWork_ArchivedReturns403(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "testadmin")

	// Create and archive an application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl5", "Cars5")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Try to take to work — should get 403
	body := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, adminID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- Unauthorized tests for new endpoints ---

func TestReads_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rec := testutil.POST(t, e, "/applications/1/read", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = testutil.GET(t, e, "/applications/1/reads", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = testutil.GET(t, e, "/applications/unread-count", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
