// Package passages provides a deterministic projection of recorded passage events.
// It does not authorize access, resolve permit dates, or read a database.
package passages

import (
	"fmt"
	"time"
)

const (
	ExitGrace      = 5 * time.Minute
	AttentionAfter = 48 * time.Hour
)

type Kind string

const (
	Entry      Kind = "entry"
	Exit       Kind = "exit"
	Correction Kind = "correction"
)

// Event must come from an authorized adapter with the current revert projection.
// RecordedAt is the registration time, not an estimated physical departure time.
type Event struct {
	ID         int64
	Kind       Kind
	RecordedAt time.Time
	Reverted   bool
}

type State struct {
	LastEventID    int64
	HasEvent       bool
	Open           bool
	EntryAt        *time.Time
	ExitRecordedAt *time.Time
	GraceUntil     *time.Time
	InExitGrace    bool
	NeedsAttention bool
}

// Resolve ignores reverted events and orders live events by recorded time, then
// ID. Empty history does not imply presence or absence on the physical territory.
// A correction closes accounting with its own registration clock; it is not an observed exit.
func Resolve(events []Event, now time.Time) (State, error) {
	var latest *Event
	seen := make(map[int64]struct{}, len(events))
	for i := range events {
		event := &events[i]
		if event.ID <= 0 {
			return State{}, fmt.Errorf("invalid passage event ID")
		}
		if _, exists := seen[event.ID]; exists {
			return State{}, fmt.Errorf("duplicate passage event ID %d", event.ID)
		}
		seen[event.ID] = struct{}{}
		if event.Kind != Entry && event.Kind != Exit && event.Kind != Correction {
			return State{}, fmt.Errorf("unsupported passage event kind %q", event.Kind)
		}
		if event.RecordedAt.IsZero() || event.RecordedAt.After(now) {
			return State{}, fmt.Errorf("invalid passage registration time for event %d", event.ID)
		}
		if event.Reverted {
			continue
		}
		if latest == nil || event.RecordedAt.After(latest.RecordedAt) ||
			(event.RecordedAt.Equal(latest.RecordedAt) && event.ID > latest.ID) {
			latest = event
		}
	}
	if latest == nil {
		return State{}, nil
	}
	state := State{HasEvent: true, LastEventID: latest.ID, Open: latest.Kind == Entry}
	at := latest.RecordedAt.UTC()
	if state.Open {
		state.EntryAt = &at
		state.NeedsAttention = now.Sub(at) > AttentionAfter
	} else {
		state.ExitRecordedAt = &at
		// An orphan exit is a recorded fact, not proof of a preceding open
		// admission. Corrections explicitly close known/unknown open accounting.
		var previous *Event
		for i := range events {
			e := &events[i]
			if e.Reverted || e.ID == latest.ID || e.RecordedAt.After(latest.RecordedAt) || (e.RecordedAt.Equal(latest.RecordedAt) && e.ID > latest.ID) {
				continue
			}
			if previous == nil || e.RecordedAt.After(previous.RecordedAt) || (e.RecordedAt.Equal(previous.RecordedAt) && e.ID > previous.ID) {
				previous = e
			}
		}
		if latest.Kind == Correction || (previous != nil && previous.Kind == Entry) {
			until := at.Add(ExitGrace)
			state.GraceUntil = &until
			state.InExitGrace = now.Before(until)
		}
	}
	return state, nil
}
