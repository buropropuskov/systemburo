package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// forward2664Surfaces reads both public surfaces instead of exercising only a
// service helper. The history surface nests forwarding metadata; the message
// surface exposes the same privacy-safe recipient fields at top level.
func forward2664Surfaces(t *testing.T, e *echo.Echo, token string, appID int) []map[string]interface{} {
	t.Helper()
	history := testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	event := findHistoryEntry(t, testutil.ParseSlice(t, history), "forwarded")
	metadata, ok := event["metadata"].(map[string]interface{})
	require.True(t, ok, "history must contain a structured forwarding metadata object")
	messages := testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, messages.Code, messages.Body.String())
	items := testutil.ParseSlice(t, messages)
	require.NotEmpty(t, items)
	return []map[string]interface{}{metadata, items[len(items)-1]}
}

func forward2664StoredEvent(t *testing.T, db *gorm.DB, appID int) (int, string) {
	t.Helper()
	var row struct {
		ID      int
		Details string
	}
	require.NoError(t, db.Raw(`SELECT id, details::text AS details FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action = ? ORDER BY id DESC LIMIT 1`,
		models.AuditEntityApplication, appID, models.AuditActionForwarded).Scan(&row).Error)
	require.NotZero(t, row.ID)
	return row.ID, row.Details
}

func forward2664Details(t *testing.T, metadata map[string]interface{}) map[int]map[string]interface{} {
	t.Helper()
	rows, ok := metadata["recipient_details"].([]interface{})
	require.True(t, ok, "recipient_details must be an array")
	result := make(map[int]map[string]interface{}, len(rows))
	for _, value := range rows {
		row, ok := value.(map[string]interface{})
		require.True(t, ok)
		id, ok := row["user_id"].(float64)
		require.True(t, ok)
		require.NotContains(t, result, int(id), "one recipient must have one final assignment in a forwarding event")
		result[int(id)] = row
	}
	return result
}

func forward2664RequireEmptyArray(t *testing.T, metadata map[string]interface{}, field string) {
	t.Helper()
	rows, ok := metadata[field].([]interface{})
	require.True(t, ok, "%s must be JSON [], not null, absent or another empty value", field)
	require.NotNil(t, rows)
	require.Len(t, rows, 0)
}

func TestForwardHistory2664_LegacyAndMalformedArePrivateWithoutStorageChanges(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "fh2664_legacy_target", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "fh2664_legacy_target")
	appID := createSimpleApplication(t, e, admin, td.OrgID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(targetID, false, true), testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	eventID, _ := forward2664StoredEvent(t, db, appID)

	fixtures := []struct {
		name     string
		metadata string
	}{
		{"legacy", `{"recipients":["Private legacy recipient"],"whole":true,"attachments":[]}`},
		{"unknown purpose", fmt.Sprintf(`{"forward_schema_version":2,"recipients":["Private legacy recipient"],"recipient_details":[{"user_id":%d,"purpose":"unknown","required_approval":false,"access_granted":true}],"whole":true}`, targetID)},
		{"missing flags", fmt.Sprintf(`{"forward_schema_version":2,"recipients":["Private legacy recipient"],"recipient_details":[{"user_id":%d,"purpose":"view"}],"whole":true}`, targetID)},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			require.NoError(t, db.Exec(`UPDATE audit_log SET details = jsonb_set(details, '{metadata}', ?::jsonb) WHERE id = ?`, fixture.metadata, eventID).Error)
			_, before := forward2664StoredEvent(t, db, appID)
			for _, metadata := range forward2664Surfaces(t, e, admin, appID) {
				assert.Equal(t, false, metadata["recipient_details_available"])
				forward2664RequireEmptyArray(t, metadata, "recipients")
				forward2664RequireEmptyArray(t, metadata, "recipient_details")
				encoded, err := json.Marshal(metadata)
				require.NoError(t, err)
				assert.NotContains(t, string(encoded), "Private legacy recipient")
			}
			_, after := forward2664StoredEvent(t, db, appID)
			assert.Equal(t, before, after, "GET must never rewrite original audit metadata")
			assert.Contains(t, after, "Private legacy recipient", "source legacy data must remain stored")
		})
	}
}

func TestForwardHistory2664_RecordsAcceptedPurposesAndRepeatedDelivery(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "fh2664_sender", "pass123", 1, td.OrgID, td.CompanyID)
	ids := make([]int, 4)
	for i := range ids {
		username := fmt.Sprintf("fh2664_recipient_%d", i)
		testutil.RegisterUser(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
		ids[i] = getUserID(t, db, username)
	}
	foreignOrg, foreignCompany := seedOrgAndCompany(t, db, "Forward2664Foreign")
	testutil.RegisterUser(t, e, "fh2664_foreign", "pass123", 1, foreignOrg, foreignCompany)
	foreignID := getUserID(t, db, "fh2664_foreign")
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	body := fmt.Sprintf(`{"users":[
		{"user_id":%d,"required_approval":true,"can_view":false},
		{"user_id":%d,"required_approval":false,"can_view":false},
		{"user_id":%d,"required_approval":false,"can_view":true},
		{"user_id":%d,"required_approval":false,"can_view":true},
		{"user_id":%d,"required_approval":true,"can_view":false},
		{"user_id":%d,"required_approval":false,"can_view":true}],"message":"Synthetic forwarding note"}`,
		ids[0], ids[1], ids[2], ids[3], ids[3], foreignID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, stored := forward2664StoredEvent(t, db, appID)
	var audit struct {
		Metadata map[string]interface{} `json:"metadata"`
		Comment  string                 `json:"comment"`
	}
	require.NoError(t, json.Unmarshal([]byte(stored), &audit))
	assert.Equal(t, float64(2), audit.Metadata["forward_schema_version"])
	assert.Equal(t, "all", audit.Metadata["attachment_scope"])
	assert.Equal(t, "Synthetic forwarding note", audit.Comment)
	storedRecipients := forward2664Details(t, audit.Metadata)
	require.Len(t, storedRecipients, 4)
	assert.NotContains(t, storedRecipients, foreignID, "discarded recipient must not be recorded as accepted")
	for _, metadata := range forward2664Surfaces(t, e, sender, appID) {
		assert.Equal(t, true, metadata["recipient_details_available"])
		details := forward2664Details(t, metadata)
		require.Len(t, details, 4)
		assert.Equal(t, "approval", details[ids[0]]["purpose"])
		assert.Equal(t, true, details[ids[0]]["required_approval"])
		assert.Equal(t, "approval", details[ids[1]]["purpose"])
		assert.Equal(t, false, details[ids[1]]["required_approval"])
		assert.Equal(t, "view", details[ids[2]]["purpose"])
		assert.Equal(t, "approval", details[ids[3]]["purpose"], "accepted view then approval must retain the resulting approval assignment")
		assert.Equal(t, true, details[ids[3]]["required_approval"])
	}
	for _, expected := range []struct {
		table string
		ids   []int
	}{
		{"application_responsible_users", []int{ids[0], ids[1], ids[3]}},
		{"application_viewers", []int{ids[2], ids[3]}},
	} {
		var count int64
		require.NoError(t, db.Table(expected.table).Where("application_id = ? AND user_id IN ?", appID, expected.ids).Count(&count).Error)
		assert.Equal(t, int64(len(expected.ids)), count, "accepted metadata must match real assignment rows")
		require.NoError(t, db.Table(expected.table).Where("application_id = ? AND user_id = ?", appID, foreignID).Count(&count).Error)
		assert.Zero(t, count, "discarded recipient must not gain access")
	}
	// Existing viewer delivery is a new event but not a newly created access grant.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(ids[2], false, true), testutil.AuthHeader(sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, metadata := range forward2664Surfaces(t, e, sender, appID) {
		details := forward2664Details(t, metadata)
		require.Len(t, details, 1)
		assert.Equal(t, "view", details[ids[2]]["purpose"])
		assert.Equal(t, false, details[ids[2]]["access_granted"])
	}
}

func TestForwardHistory2664_ConsentAndApproverMasksOnBothSurfaces(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	usernames := []string{"fh2664_hidden", "fh2664_approver", "fh2664_consented"}
	ids := make([]int, len(usernames))
	tokens := make([]string, len(usernames))
	for i, username := range usernames {
		tokens[i] = testutil.RegisterAndLogin(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
		ids[i] = setUserName(t, db, username, fmt.Sprintf("SyntheticName%d", i), "Recipient", "")
	}
	appID := createSimpleApplication(t, e, admin, td.OrgID)
	body := fmt.Sprintf(`{"users":[{"user_id":%d,"can_view":true},{"user_id":%d,"can_view":true},{"user_id":%d,"can_view":true}]}`, ids[0], ids[1], ids[2])
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, storedBefore := forward2664StoredEvent(t, db, appID)
	// First verify names are visible while consent is not required.
	for _, metadata := range forward2664Surfaces(t, e, admin, appID) {
		assert.Equal(t, "SyntheticName0 Recipient", forward2664Details(t, metadata)[ids[0]]["display_name"])
	}
	makeApprover(t, db, usernames[1])
	require.NoError(t, db.Table("application_approvers").Where("user_id = ?", ids[1]).Update("display_name", "Duty operator").Error)
	enableConsent(t, e, admin, "<p>Synthetic consent text</p>")
	rec = testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(tokens[2]))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, metadata := range forward2664Surfaces(t, e, admin, appID) {
		details := forward2664Details(t, metadata)
		require.Len(t, details, 3)
		assert.Equal(t, "@fh2664_hidden", details[ids[0]]["display_name"])
		assert.Equal(t, "Duty operator", details[ids[1]]["display_name"], "configured approver mask takes precedence over consent mask")
		assert.Equal(t, "SyntheticName2 Recipient", details[ids[2]]["display_name"])
		encoded, err := json.Marshal(metadata["recipients"])
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "SyntheticName0")
		assert.NotContains(t, string(encoded), "SyntheticName1")
		for _, detail := range details {
			assert.Equal(t, "view", detail["purpose"], "current approver role must not rewrite historical forwarding purpose")
		}
	}
	_, storedAfter := forward2664StoredEvent(t, db, appID)
	assert.Equal(t, storedBefore, storedAfter)
}

func TestForwardHistory2664_MissingAccountKeepsPurposeWithoutGuessingIdentity(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "fh2664_missing_seed", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "fh2664_missing_seed")
	appID := createSimpleApplication(t, e, admin, td.OrgID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(targetID, false, true), testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	eventID, _ := forward2664StoredEvent(t, db, appID)
	missingID := targetID + 10000000
	var count int64
	require.NoError(t, db.Table("users").Where("id = ?", missingID).Count(&count).Error)
	require.Zero(t, count)
	metadata := fmt.Sprintf(`{"forward_schema_version":2,"recipients":["Stale private snapshot"],"recipient_details":[{"user_id":%d,"purpose":"approval","required_approval":true,"access_granted":true}],"whole":true}`, missingID)
	require.NoError(t, db.Exec(`UPDATE audit_log SET details = jsonb_set(details, '{metadata}', ?::jsonb) WHERE id = ?`, metadata, eventID).Error)
	_, before := forward2664StoredEvent(t, db, appID)
	for _, surface := range forward2664Surfaces(t, e, admin, appID) {
		details := forward2664Details(t, surface)
		require.Len(t, details, 1)
		assert.Equal(t, "Сведения о получателе недоступны", details[missingID]["display_name"])
		assert.Equal(t, "approval", details[missingID]["purpose"])
		assert.Equal(t, true, details[missingID]["required_approval"])
		assert.Equal(t, true, surface["recipient_details_available"], "identity unavailable does not erase known historical purpose")
		encoded, err := json.Marshal(surface)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "Stale private snapshot")
	}
	_, after := forward2664StoredEvent(t, db, appID)
	assert.Equal(t, before, after)
}

func TestForwardHistory2664_SelectedAttachmentNamesRemainHistoricalAfterRename(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "fh2664_material_sender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "fh2664_material_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "fh2664_material_viewer")
	uniqueID := seedUniqueAttachment(t, db, "cars", "fh2664_material_template", "Historical material name")
	appID := submitCompleteApplication(t, e, sender, "Test Organization", uniqueID)
	var attachmentID int
	require.NoError(t, db.Raw("SELECT id FROM attachments WHERE application_id = ? ORDER BY id LIMIT 1", appID).Scan(&attachmentID).Error)
	require.NotZero(t, attachmentID)
	require.NoError(t, db.Table("attachments").Where("id = ?", attachmentID).Update("attachment_display_name", "Historical material name").Error)
	var historicalName string
	require.NoError(t, db.Raw("SELECT attachment_display_name FROM attachments WHERE id = ?", attachmentID).Scan(&historicalName).Error)
	require.Equal(t, "Historical material name", historicalName)
	body := fmt.Sprintf(`{"users":[{"user_id":%d,"can_view":true}],"attachment_ids":[%d]}`, viewerID, attachmentID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, before := forward2664StoredEvent(t, db, appID)
	require.NoError(t, db.Table("attachments").Where("id = ?", attachmentID).Update("attachment_display_name", "Renamed current material").Error)
	surfaces := forward2664Surfaces(t, e, sender, appID)
	for _, surface := range surfaces {
		assert.Equal(t, false, surface["whole"])
		assert.Equal(t, []interface{}{"Historical material name"}, surface["attachments"])
	}
	assert.Equal(t, "selected", surfaces[0]["attachment_scope"])
	materials, ok := surfaces[0]["attachment_details"].([]interface{})
	require.True(t, ok)
	require.Len(t, materials, 1)
	assert.Equal(t, map[string]interface{}{"id": float64(attachmentID), "name": "Historical material name"}, materials[0])
	_, after := forward2664StoredEvent(t, db, appID)
	assert.Equal(t, before, after)
}

type forward2664FaultContextKey struct{}

func TestForwardHistory2664_MaskQueryFailuresDoNotExposeRecipients(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "fh2664_fault_target", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := setUserName(t, db, "fh2664_fault_target", "PrivateFaultName", "Recipient", "")
	appID := createSimpleApplication(t, e, admin, td.OrgID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(targetID, false, true), testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	enableConsent(t, e, admin, "<p>Synthetic fault test consent</p>")
	_, before := forward2664StoredEvent(t, db, appID)
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if target := c.Request().Header.Get("X-Forward2664-Test-Fault"); target != "" {
				ctx := context.WithValue(c.Request().Context(), forward2664FaultContextKey{}, target)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	})
	var injected atomic.Int64
	const callbackName = "test:forward2664_privacy_fault"
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register(callbackName, func(tx *gorm.DB) {
		target, _ := tx.Statement.Context.Value(forward2664FaultContextKey{}).(string)
		if target == "" {
			return
		}
		selection := strings.Join(tx.Statement.Selects, ",")
		matches := target == "users" && tx.Statement.Table == "users" && selection == "id, username, last_name, first_name, middle_name" ||
			target == "settings" && tx.Statement.Table == "system_settings" && selection == "key, value" ||
			target == "consent" && tx.Statement.Table == "users" && selection == "users.id, users.username" ||
			target == "approver" && tx.Statement.Table == "application_approvers" && selection == "user_id, display_name"
		if matches {
			injected.Add(1)
			tx.AddError(errors.New("synthetic forwarding privacy query failure"))
		}
	}))
	defer func() { require.NoError(t, db.Callback().Row().Remove(callbackName)) }()
	for _, target := range []string{"users", "settings", "consent", "approver"} {
		for _, endpoint := range []string{"history", "forward-messages"} {
			t.Run(target+"/"+endpoint, func(t *testing.T) {
				injected.Store(0)
				headers := testutil.AuthHeader(admin)
				headers.Set("X-Forward2664-Test-Fault", target)
				response := testutil.GET(t, e, fmt.Sprintf("/applications/%d/%s", appID, endpoint), headers)
				require.Positive(t, injected.Load(), "failure must actually hit the intended query")
				assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
				assert.NotContains(t, response.Body.String(), "PrivateFaultName")
				assert.NotContains(t, response.Body.String(), "@fh2664_fault_target")
			})
		}
	}
	// A request without the test context still succeeds with the callback installed.
	for _, surface := range forward2664Surfaces(t, e, admin, appID) {
		assert.Equal(t, "@fh2664_fault_target", forward2664Details(t, surface)[targetID]["display_name"])
	}
	_, after := forward2664StoredEvent(t, db, appID)
	assert.Equal(t, before, after)
}
