package passages

import (
	"testing"
	"time"
)

func TestPassage2667OrphanExitAndCorrection(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		events []Event
		grace  bool
	}{
		{[]Event{{ID: 1, Kind: Exit, RecordedAt: now}}, false},
		{[]Event{{ID: 1, Kind: Entry, RecordedAt: now.Add(-time.Hour)}, {ID: 2, Kind: Exit, RecordedAt: now.Add(-time.Minute)}, {ID: 3, Kind: Exit, RecordedAt: now}}, false},
		{[]Event{{ID: 1, Kind: Correction, RecordedAt: now}}, true},
		{[]Event{{ID: 1, Kind: Entry, RecordedAt: now.Add(-time.Hour)}, {ID: 2, Kind: Correction, RecordedAt: now, Reverted: true}}, false},
	} {
		state, err := Resolve(tc.events, now)
		if err != nil || state.InExitGrace != tc.grace {
			t.Fatalf("state=%+v err=%v", state, err)
		}
	}
}
