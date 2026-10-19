package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestDocumentAudit_HTTPWritesFactsAndReadsLegacy(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, seed.OrgID, seed.CompanyID)
	headers := testutil.AuthHeader(token)
	create := `{"pd_consent":true,"last_name":"S6DocumentFixture","first_name":"Fixture","position":"Fixture old role","passport_series_number":"S6-OLD-PASSPORT","patent_number":"S6-OLD-PATENT","other_permission":"S6-OLD-PERMIT"}`
	response := testutil.POST(t, e, "/unique-employees", create, headers)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	id := int(testutil.ParseMap(t, response)["id"].(float64))
	update := `{"position":"Fixture next role","passport_series_number":"S6-NEW-PASSPORT","patent_number":"S6-NEW-PATENT","other_permission":"S6-NEW-PERMIT"}`
	response = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", id), update, headers)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	current := testutil.ParseMap(t, response)
	require.Equal(t, "S6-NEW-PASSPORT", current["passport_series_number"])
	require.Equal(t, "S6-NEW-PATENT", current["patent_number"])
	require.Equal(t, "S6-NEW-PERMIT", current["other_permission"])
	var written []models.AuditLog
	require.NoError(t, db.Where("entity_type=? AND entity_id=?", models.AuditEntityUniqueEmployee, id).Find(&written).Error)
	changed := 0
	for _, row := range written {
		require.NotContains(t, string(row.Details), "S6-OLD-")
		require.NotContains(t, string(row.Details), "S6-NEW-")
		var detail map[string]any
		require.NoError(t, json.Unmarshal(row.Details, &detail))
		if field, ok := detail["field_name"].(string); ok && field != "position" {
			require.Contains(t, []string{"passport_series_number", "patent_number", "other_permission"}, field)
			require.NotContains(t, detail, "old_value")
			require.NotContains(t, detail, "new_value")
			changed++
		}
	}
	require.Equal(t, 3, changed)

	// Insert an old-format row directly into the isolated test DB. Reading must
	// redact its response, preserving the original persisted JSON and event.
	legacyRaw := json.RawMessage(`{"field_name":"passport_series_number","old_value":"S6-LEGACY-OLD","new_value":"S6-LEGACY-NEW","comment":"Fixture legacy reason","subject":"Fixture legacy person"}`)
	legacy := models.AuditLog{EntityType: models.AuditEntityUniqueEmployee, EntityID: &id, Action: "data_changed", Details: legacyRaw}
	require.NoError(t, db.Create(&legacy).Error)
	for _, endpoint := range []string{fmt.Sprintf("/unique-employees/%d/history", id), "/unique-employees/history?limit=500", fmt.Sprintf("/audit?entity_type=unique_employee&entity_id=%d", id)} {
		response = testutil.GET(t, e, endpoint, headers)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.NotContains(t, response.Body.String(), "S6-LEGACY-OLD")
		require.NotContains(t, response.Body.String(), "S6-LEGACY-NEW")
		require.Contains(t, response.Body.String(), "passport_series_number")
		require.Contains(t, response.Body.String(), "Fixture legacy reason")
		require.Contains(t, response.Body.String(), "Fixture next role")
	}
	var stored models.AuditLog
	require.NoError(t, db.First(&stored, legacy.ID).Error)
	require.JSONEq(t, string(legacyRaw), string(stored.Details))
}
