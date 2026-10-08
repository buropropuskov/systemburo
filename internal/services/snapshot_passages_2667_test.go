package services

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestPassage2667SnapshotFreezesFactsAndRemovesGrants(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entry := now.Add(-49 * time.Hour)
	grace := now
	rows := []map[string]any{
		{"id": 1, "passage_state": PassageState{Open: true, EntryAt: &entry, EntryTimeKnown: true, CanCorrect: true}, "admission": PassageAdmission{CanEnter: true, CanExit: true}, "can_revert": true},
		{"id": 2, "passage_state": PassageState{HasEvent: true, GraceUntil: &grace, InExitGrace: true, CanRevertCorrection: true}},
		{"id": 3, "territory_status": 1},
	}
	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	frozen, err := freezeSnapshotPassages(raw, now)
	require.NoError(t, err)
	var out []struct {
		PassageState PassageState     `json:"passage_state"`
		Admission    PassageAdmission `json:"admission"`
		CanRevert    bool             `json:"can_revert"`
		ServerNow    time.Time        `json:"server_now"`
	}
	require.NoError(t, json.Unmarshal(frozen, &out))
	require.True(t, out[0].PassageState.NeedsAttention)
	require.False(t, out[0].PassageState.CanCorrect)
	require.False(t, out[0].Admission.CanEnter)
	require.False(t, out[0].Admission.CanExit)
	require.False(t, out[0].CanRevert)
	require.Equal(t, now, out[0].ServerNow)
	require.False(t, out[1].PassageState.InExitGrace)
	require.False(t, out[1].PassageState.CanRevertCorrection)
	var legacy []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(frozen, &legacy))
	require.NotContains(t, legacy[2], "passage_state")
}

func TestPassage2667SnapshotExportCorrectionAndLegacyLabels(t *testing.T) {
	closed := 2
	state := PassageState{HasEvent: true, LastEventKind: PassageCorrectionAction}
	require.Equal(t, "Учёт исправлен", snapshotPassageStatusLabel(state, &closed))
	require.Equal(t, "Выехал", snapshotPassageStatusLabel(PassageState{}, &closed))
	require.Equal(t, "Не въезжал", snapshotPassageStatusLabel(PassageState{}, nil))
	state.LastEventKind = "exit"
	require.Equal(t, "Выехал", snapshotPassageStatusLabel(state, &closed))
}
