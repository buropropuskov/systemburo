package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// extractKeys recursively extracts all JSON keys from a value, producing
// dot-separated paths. Arrays get a "[]" suffix on their parent key.
func extractKeys(prefix string, v interface{}) []string {
	var keys []string

	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			full := k
			if prefix != "" {
				full = prefix + "." + k
			}
			keys = append(keys, full)
			keys = append(keys, extractKeys(full, child)...)
		}
	case []interface{}:
		if len(val) > 0 {
			// Use first element as representative shape
			keys = append(keys, extractKeys(prefix+"[]", val[0])...)
		}
	}

	return keys
}

// goldenDir returns the absolute path to testdata/ next to this test file.
func goldenDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// assertResponseShape compares the JSON key structure of a response against a golden file.
// If UPDATE_GOLDEN is set, it writes the golden file instead of comparing.
func assertResponseShape(t *testing.T, rec *httptest.ResponseRecorder, goldenFile string) {
	t.Helper()

	// Parse the full envelope
	var envelope map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope),
		"failed to parse response JSON: %s", rec.Body.String())
	require.True(t, envelope["success"].(bool), "expected success=true: %s", rec.Body.String())

	// Extract keys from the data field
	data := envelope["data"]
	var allKeys []string
	switch d := data.(type) {
	case []interface{}:
		// Array response: top-level is data[]
		allKeys = append(allKeys, "data[]")
		if len(d) > 0 {
			allKeys = append(allKeys, extractKeys("data[]", d[0])...)
		}
	case map[string]interface{}:
		for k, child := range d {
			full := "data." + k
			allKeys = append(allKeys, full)
			allKeys = append(allKeys, extractKeys(full, child)...)
		}
	default:
		t.Fatalf("unexpected data type %T", data)
	}

	sort.Strings(allKeys)

	// Deduplicate
	unique := make([]string, 0, len(allKeys))
	seen := make(map[string]bool)
	for _, k := range allKeys {
		if !seen[k] {
			seen[k] = true
			unique = append(unique, k)
		}
	}

	got := strings.Join(unique, "\n") + "\n"

	goldenPath := filepath.Join(goldenDir(), goldenFile)

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o644))
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not found: %s\nRun with UPDATE_GOLDEN=1 to create it.\nGot keys:\n%s", goldenPath, got)
	}

	if !reflect.DeepEqual(got, string(want)) {
		t.Errorf("API contract mismatch for %s.\nWant:\n%s\nGot:\n%s\nRun with UPDATE_GOLDEN=1 to update.", goldenFile, string(want), got)
	}
}

func TestAPIContract_ApplicationsList(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Use admin (approver) so GetApplications returns all apps (non-approvers see nothing)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create an application so the list is not empty
	userToken := testutil.RegisterAndLogin(t, e, "contract_app_list", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", fmt.Sprintf("contract_tmpl_%s", t.Name()), "Contract Template")
	body := fmt.Sprintf(`{
		"message": "contract test",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Contract Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"vehicles": [{"car_number": "A001AA777", "car_brand": "Test"}]
			}
		}]
	}`, uaID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code, "seed app: %s", rec.Body.String())

	// Admin is an approver and sees all applications
	adminID := getUserID(t, db, "testadmin")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", adminID)

	rec = testutil.GET(t, e, "/applications", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	assertResponseShape(t, rec, "applications_list.golden")
}

func TestAPIContract_ApplicationDetail(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "contract_app_detail", "pass123", 1, td.OrgID, td.CompanyID)

	uaID := seedUniqueAttachment(t, db, "cars", fmt.Sprintf("contract_dtl_%s", t.Name()), "Detail Template")
	body := fmt.Sprintf(`{
		"message": "detail test",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Detail Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"vehicles": [{"car_number": "A002AA777", "car_brand": "TestDetail"}]
			}
		}]
	}`, uaID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "seed app: %s", rec.Body.String())

	// Extract application ID
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	data := env["data"].(map[string]interface{})
	appID := int(data["application_id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	assertResponseShape(t, rec, "application_detail.golden")
}

// Контракт GET /applications/:id/details - расширенной детали заявки. Не путать с
// application_detail.golden: тот бьёт по GET /applications/:id, и набор ключей у ручек
// разный. Раунд дополнения (#1685) заводится намеренно: при отсутствии открытого раунда
// open_supplement приезжает null, и форма вложенного объекта осталась бы незафиксированной.
func TestAPIContract_ApplicationDetails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "contract_app_details", "pass123", 1, td.OrgID, td.CompanyID)

	uaID := seedUniqueAttachment(t, db, "cars", fmt.Sprintf("contract_dtls_%s", t.Name()), "Details Template")
	body := fmt.Sprintf(`{
		"message": "details test",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Details Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"vehicles": [{"car_number": "A003AA777", "car_brand": "TestDetails"}]
			}
		}]
	}`, uaID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "seed app: %s", rec.Body.String())

	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	data := env["data"].(map[string]interface{})
	appID := int(data["application_id"].(float64))

	comment := "Добавили машину"
	suppRound(t, db, appID, getUserID(t, db, "contract_app_details"), 1, models.SupplementPending, &comment)

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	assertResponseShape(t, rec, "application_details.golden")
}

func TestAPIContract_UsersMe(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "contract_me", "pass123", 2, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/users/me", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	assertResponseShape(t, rec, "users_me.golden")
}

func TestAPIContract_Attachments(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Create an attachment template so the list is not empty
	seedUniqueAttachment(t, db, "cars", "contract_att", "Contract Attachment")

	token := testutil.RegisterAndLogin(t, e, "contract_att", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/attachments", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	assertResponseShape(t, rec, "attachments_list.golden")
}
