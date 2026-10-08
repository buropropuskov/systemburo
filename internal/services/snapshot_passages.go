package services

import (
	"encoding/json"
	"fmt"
	"systemburo/internal/models"
	"time"
)

// Snapshot projections contain facts at capture time, never live permissions.
// Older rows without a passage projection keep their original wire shape.
func freezeSnapshotPassages(raw json.RawMessage, captured time.Time) (json.RawMessage, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode snapshot passage rows: %w", err)
	}
	for _, row := range rows {
		encoded, ok := row["passage_state"]
		if !ok {
			continue
		}
		var state PassageState
		if err := json.Unmarshal(encoded, &state); err != nil {
			return nil, fmt.Errorf("decode snapshot passage state: %w", err)
		}
		state.CanCorrect = false
		state.CanRevertCorrection = false
		state.NeedsAttention = state.Open && state.EntryTimeKnown && state.EntryAt != nil && captured.Sub(*state.EntryAt) > 48*time.Hour
		state.InExitGrace = !state.Open && state.GraceUntil != nil && captured.Before(*state.GraceUntil)
		stateRaw, err := json.Marshal(state)
		if err != nil {
			return nil, err
		}
		row["passage_state"] = stateRaw
		nowRaw, err := json.Marshal(captured.UTC())
		if err != nil {
			return nil, err
		}
		row["server_now"] = nowRaw
		var period models.EffectivePeriod
		reason := "snapshot"
		if encoded, ok := row["effective_period"]; ok {
			if err := json.Unmarshal(encoded, &period); err != nil {
				return nil, err
			}
			_, reason = passageWindow(period, captured)
		}
		admissionRaw, err := json.Marshal(PassageAdmission{Reason: reason})
		if err != nil {
			return nil, err
		}
		row["admission"] = admissionRaw
		row["can_revert"] = json.RawMessage("false")
	}
	return json.Marshal(rows)
}

func snapshotPassageCounts(raw json.RawMessage) (models.SnapshotCounts, error) {
	var rows []struct {
		PassageState    *PassageState `json:"passage_state"`
		TerritoryStatus *int          `json:"territory_status"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return models.SnapshotCounts{}, err
	}
	counts := models.SnapshotCounts{Total: len(rows)}
	for _, row := range rows {
		if row.PassageState != nil {
			state := row.PassageState
			switch {
			case state.Open:
				counts.OnTerritory++
			case state.LastEventKind == "exit":
				counts.Exited++
			case state.LastEventKind == PassageCorrectionAction:
				counts.Corrected++
			default:
				counts.NotEntered++
			}
		} else {
			legacy := computeSnapshotCounts([]*int{row.TerritoryStatus})
			counts.OnTerritory += legacy.OnTerritory
			counts.Exited += legacy.Exited
			counts.NotEntered += legacy.NotEntered
		}
	}
	return counts, nil
}
