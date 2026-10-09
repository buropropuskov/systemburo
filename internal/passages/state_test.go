package passages

import (
	"testing"
	"time"
)

func TestResolveAttentionBoundary(t *testing.T) {
	at := time.Date(2026, time.January, 1, 23, 59, 0, 0, time.UTC)
	events := []Event{{ID: 1, Kind: Entry, RecordedAt: at}}
	for _, tc := range []struct {
		name string
		age  time.Duration
		want bool
	}{
		{"before", AttentionAfter - time.Nanosecond, false},
		{"exact", AttentionAfter, false},
		{"after", AttentionAfter + time.Nanosecond, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := Resolve(events, at.Add(tc.age))
			if err != nil || !state.Open || state.NeedsAttention != tc.want {
				t.Fatalf("state=%+v err=%v", state, err)
			}
		})
	}
}

func TestResolveExitGraceBoundary(t *testing.T) {
	at := time.Date(2026, time.December, 31, 23, 59, 0, 0, time.UTC)
	events := []Event{{ID: 1, Kind: Entry, RecordedAt: at.Add(-time.Hour)}, {ID: 2, Kind: Exit, RecordedAt: at}}
	for _, tc := range []struct {
		name string
		age  time.Duration
		want bool
	}{
		{"registration", 0, true},
		{"before", ExitGrace - time.Nanosecond, true},
		{"exact", ExitGrace, false},
		{"after", ExitGrace + time.Nanosecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := Resolve(events, at.Add(tc.age))
			if err != nil || state.Open || state.InExitGrace != tc.want || state.GraceUntil == nil || !state.GraceUntil.Equal(at.Add(ExitGrace)) {
				t.Fatalf("state=%+v err=%v", state, err)
			}
		})
	}
}

func TestResolveRevertAndOrder(t *testing.T) {
	now := time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC)
	entry := Event{ID: 1, Kind: Entry, RecordedAt: now.Add(-49 * time.Hour)}
	exit := Event{ID: 2, Kind: Exit, RecordedAt: now.Add(-time.Minute), Reverted: true}
	state, err := Resolve([]Event{exit, entry}, now)
	if err != nil || !state.Open || !state.NeedsAttention || state.InExitGrace || state.LastEventID != 1 {
		t.Fatalf("reverted exit state=%+v err=%v", state, err)
	}
	entry.Reverted = true
	state, err = Resolve([]Event{exit, entry}, now)
	if err != nil || state.HasEvent {
		t.Fatalf("all reverted state=%+v err=%v", state, err)
	}
	// A new entry supersedes the exit grace; equal timestamps use the event ID.
	state, err = Resolve([]Event{{ID: 2, Kind: Entry, RecordedAt: now}, {ID: 1, Kind: Exit, RecordedAt: now}}, now)
	if err != nil || !state.Open || state.LastEventID != 2 || state.InExitGrace {
		t.Fatalf("tie state=%+v err=%v", state, err)
	}
}

func TestResolveTimeZoneAndInvalidInput(t *testing.T) {
	now := time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC)
	at := now.Add(-49 * time.Hour).In(time.FixedZone("test", 3*60*60))
	state, err := Resolve([]Event{{ID: 1, Kind: Entry, RecordedAt: at}}, now)
	if err != nil || !state.NeedsAttention || state.EntryAt.Location() != time.UTC {
		t.Fatalf("zone state=%+v err=%v", state, err)
	}
	for _, events := range [][]Event{
		{{ID: 0, Kind: Entry, RecordedAt: now}},
		{{ID: 1, Kind: Kind("passage_close"), RecordedAt: now}},
		{{ID: 1, Kind: Entry}},
		{{ID: 1, Kind: Entry, RecordedAt: now.Add(time.Nanosecond)}},
		{{ID: 1, Kind: Entry, RecordedAt: now}, {ID: 1, Kind: Exit, RecordedAt: now}},
	} {
		if _, err := Resolve(events, now); err == nil {
			t.Fatalf("expected invalid events to fail: %+v", events)
		}
	}
	state, err = Resolve(nil, now)
	if err != nil || state.HasEvent || state.Open || state.InExitGrace {
		t.Fatalf("empty state=%+v err=%v", state, err)
	}
}
