package services

import (
	"encoding/json"
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentAccessPriority(t *testing.T) {
	for _, tc := range []struct {
		name              string
		set               PermissionSet
		own, read, export bool
	}{
		{name: "own_no_grants", own: true, read: true, export: true},
		{name: "own_explicit_denies", set: PermissionSet{denies: map[string]struct{}{KeyDetailDocuments: {}, KeyDetailDocumentsExport: {}}}, own: true, read: true, export: true},
		{name: "foreign_no_grants"},
		{name: "foreign_read", set: PermissionSet{allows: map[string]string{KeyDetailDocuments: SourceOverride}}, read: true},
		{name: "foreign_export_without_read", set: PermissionSet{allows: map[string]string{KeyDetailDocumentsExport: SourceOverride}}},
		{name: "foreign_both", set: PermissionSet{allows: map[string]string{KeyDetailDocuments: SourceOverride, KeyDetailDocumentsExport: SourceOverride}}, read: true, export: true},
		{name: "admin_denied", set: PermissionSet{adminAll: true, denies: map[string]struct{}{KeyDetailDocuments: {}}}},
		{name: "super", set: PermissionSet{allowAll: true}, read: true, export: true},
		{name: "banned_own", set: PermissionSet{banned: true}, own: true},
		{name: "inactive_own", set: PermissionSet{inactive: true}, own: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := DocumentAccess{actorID: 10, checked: true, set: tc.set}
			assert.Equal(t, tc.read, p.ReadAllowed(tc.own))
			assert.Equal(t, tc.export, p.ExportAllowed(tc.own))
		})
	}
	assert.False(t, (DocumentAccess{}).ReadAllowed(true), "unverified actor never receives owner exception")
}

func TestDocumentProjectionDoesNotChangePersistentModel(t *testing.T) {
	passport, patent, other := "synthetic-passport", "synthetic-patent", "synthetic-other"
	ownerID := 11
	model := models.UniqueEmployee{UserID: &ownerID, PassportSeriesNumber: &passport, PatentNumber: &patent, OtherPermission: &other}
	response := projectEmployeeResponse(employeeToResponse(&model), DocumentAccess{actorID: 12, checked: true})
	assert.True(t, response.DocumentsHidden)
	assert.Nil(t, response.PassportSeriesNumber)
	assert.Nil(t, response.PatentNumber)
	assert.Nil(t, response.OtherPermission)
	assert.Equal(t, "synthetic-passport", *model.PassportSeriesNumber)
	assert.Equal(t, "synthetic-patent", *model.PatentNumber)
	assert.Equal(t, "synthetic-other", *model.OtherPermission)
	body, err := json.Marshal(response)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"documents_hidden":true`)
	assert.Contains(t, string(body), `"passport_series_number":null`)
	assert.NotContains(t, string(body), "synthetic-passport")
}
