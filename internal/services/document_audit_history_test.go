package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func TestDocumentAudit_LegacyPassageReadersKeepEvents(t *testing.T) {
	w := setupPassage2667World(t, services.ElementEmployee)
	svc := services.NewEmployeesHistoryService(w.db)
	ctx := context.Background()
	_, originalPassages, err := svc.GetAll(ctx, models.PassageHistoryQuery{Page: 1, PerPage: 20})
	require.NoError(t, err)
	encodedDetails, err := json.Marshal(map[string]any{"field_name": "patent_number", "old_value": "S6-PASSAGE-OLD", "new_value": "S6-PASSAGE-NEW", "comment": "Fixture document changed", "table_id": w.table})
	require.NoError(t, err)
	raw := json.RawMessage(encodedDetails)
	legacy := models.AuditLog{EntityType: models.AuditEntityEmployee, EntityID: &w.id, Action: "data_changed", ActorUserID: &w.actor, Details: raw}
	require.NoError(t, w.db.Create(&legacy).Error)
	check := func(rows []services.EmployeeHistoryItem, err error) {
		t.Helper()
		require.NoError(t, err)
		found := false
		for _, row := range rows {
			if row.ID != legacy.ID {
				continue
			}
			found = true
			require.Equal(t, "data_changed", row.ActionType)
			require.NotNil(t, row.FieldName)
			require.Equal(t, "patent_number", *row.FieldName)
			require.Nil(t, row.OldValue)
			require.Nil(t, row.NewValue)
			require.Equal(t, "Fixture document changed", *row.Comment)
		}
		require.True(t, found)
	}
	rows, err := svc.GetByEmployee(ctx, w.id)
	check(rows, err)
	rows, _, err = svc.GetByTable(ctx, w.table, models.PassageHistoryQuery{Page: 1, PerPage: 20})
	check(rows, err)
	passages, total, err := svc.GetAll(ctx, models.PassageHistoryQuery{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.Equal(t, originalPassages, total)
	for _, row := range passages {
		require.NotEqual(t, legacy.ID, row.ID)
	}
	items, _, err := services.NewAuditReader(w.db).List(ctx, services.AuditQuery{EntityType: models.AuditEntityEmployee, EntityID: &w.id})
	require.NoError(t, err)
	encoded, err := json.Marshal(items)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "S6-PASSAGE-OLD")
	require.NotContains(t, string(encoded), "S6-PASSAGE-NEW")
	var stored models.AuditLog
	require.NoError(t, w.db.First(&stored, legacy.ID).Error)
	require.JSONEq(t, string(raw), string(stored.Details))
}
