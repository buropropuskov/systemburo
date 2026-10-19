package services

import (
	"context"
	"encoding/json"
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

func TestDocumentAudit_FlatPersonDetailsAndBatchEntry(t *testing.T) {
	for _, entity := range []string{models.AuditEntityEmployee, models.AuditEntityUniqueEmployee} {
		for _, field := range []string{"passport_series_number", "patent_number", "other_permission"} {
			t.Run(entity+"/"+field, func(t *testing.T) {
				raw := json.RawMessage(`{"field_name":"` + field + `","old_value":"S6-OLD-DOCUMENT","new_value":"S6-NEW-DOCUMENT","comment":"Fixture reason","subject":"Fixture person","table_id":12}`)
				original := append([]byte(nil), raw...)
				id, actor := 71, 9
				entry, err := buildAuditLogEntry(WithImpersonator(context.Background(), 23), entity, &id, "data_changed", &actor, raw)
				require.NoError(t, err)
				require.Equal(t, original, []byte(raw))
				require.Equal(t, entity, entry.EntityType)
				require.Equal(t, &actor, entry.ActorUserID)
				require.Equal(t, &id, entry.EntityID)
				require.Equal(t, "data_changed", entry.Action)
				require.NotContains(t, string(entry.Details), "S6-OLD-DOCUMENT")
				require.NotContains(t, string(entry.Details), "S6-NEW-DOCUMENT")
				var details map[string]any
				require.NoError(t, json.Unmarshal(entry.Details, &details))
				require.Equal(t, field, details["field_name"])
				require.Equal(t, "Fixture reason", details["comment"])
				require.Equal(t, "Fixture person", details["subject"])
				require.EqualValues(t, 12, details["table_id"])
				require.EqualValues(t, 23, details["impersonated_by"])
				require.NotContains(t, details, "old_value")
				require.NotContains(t, details, "new_value")
			})
		}
	}
}

func TestDocumentAudit_UnrelatedSchemasAndFieldsRemain(t *testing.T) {
	for _, scenario := range []struct{ entity, raw string }{
		{models.AuditEntityCar, `{"field_name":"passport_series_number","old_value":"fixture","new_value":"next"}`},
		{models.AuditEntityEmployee, `{"field_name":"position","old_value":"fixture","new_value":"next"}`},
		{models.AuditEntityEmployee, `{"comment":"passport_series_number old_value fixture","metadata":{"old_value":"fixture"}}`},
		{models.AuditEntityEmployee, `{"passport_series_number":{"old":"fixture","new":"next"}}`},
		{models.AuditEntityEmployee, `["fixture"]`},
		{models.AuditEntityEmployee, `null`},
	} {
		raw := json.RawMessage(scenario.raw)
		require.Equal(t, raw, documentAuditDetails(scenario.entity, raw))
	}
}

func TestDocumentAudit_DiffKeepsFactWithoutValues(t *testing.T) {
	old, next, role, nextRole := "S6-OLD-DOCUMENT", "S6-NEW-DOCUMENT", "Fixture old role", "Fixture next role"
	before := models.UniqueEmployee{PassportSeriesNumber: &old, PatentNumber: &old, OtherPermission: &old, Position: &role}
	after := models.UniqueEmployee{PassportSeriesNumber: &next, PatentNumber: &next, OtherPermission: nil, Position: &nextRole}
	changes := diffUniqueEmployee(&before, &after)
	require.Len(t, changes, 4)
	for _, change := range changes {
		if isEmployeeDocumentField(change.Field) {
			require.Nil(t, change.Old)
			require.Nil(t, change.New)
		} else {
			require.Equal(t, "position", change.Field)
			require.Equal(t, &role, change.Old)
			require.Equal(t, &nextRole, change.New)
		}
	}
	require.Empty(t, diffUniqueEmployee(&before, &before))
	require.Equal(t, old, *before.PassportSeriesNumber)
	require.Equal(t, next, *after.PassportSeriesNumber)
}
