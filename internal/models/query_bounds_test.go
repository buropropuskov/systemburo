package models

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/apperr"
)

func TestQueryBounds_MoscowHalfOpenDay(t *testing.T) {
	b, err := ParseQueryDateBounds("2026-10-07", "2026-10-07")
	if err != nil {
		t.Fatal(err)
	}
	if b.From == nil || b.To == nil {
		t.Fatal("missing bounds")
	}
	if got := b.From.UTC().Format(time.RFC3339); got != "2026-10-06T21:00:00Z" {
		t.Fatal(got)
	}
	if got := b.To.UTC().Format(time.RFC3339); got != "2026-10-07T21:00:00Z" {
		t.Fatal(got)
	}
	for _, at := range []time.Time{*b.From, b.From.Add(2*time.Hour + 59*time.Minute), b.To.Add(-500 * time.Millisecond)} {
		if at.Before(*b.From) || !at.Before(*b.To) {
			t.Fatalf("day should include %v", at)
		}
	}
}

func TestQueryBounds_InvalidDatesAndReversedRange(t *testing.T) {
	for _, pair := range [][2]string{{"2026-02-29", ""}, {"2026-04-31", ""}, {"2026-1-01", ""}, {"2026-10-07T00:00:00Z", ""}, {"", "bad"}, {"2026-10-08", "2026-10-07"}} {
		_, err := ParseQueryDateBounds(pair[0], pair[1])
		var validation *apperr.Error
		if !errors.As(err, &validation) || validation.Code != http.StatusBadRequest {
			t.Fatalf("%v: expected 400, got %v", pair, err)
		}
	}
}

func TestQueryBounds_OptionalAndCalendarRollover(t *testing.T) {
	for _, pair := range [][3]string{{"", "", ""}, {"2024-02-29", "", ""}, {"", "2024-02-29", "2024-03-01"}, {"", "2026-12-31", "2027-01-01"}} {
		b, err := ParseQueryDateBounds(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if (b.From == nil) != (pair[0] == "") || (b.To == nil) != (pair[1] == "") {
			t.Fatalf("optional bounds: %v", pair)
		}
		if b.To != nil && b.To.Format("2006-01-02") != pair[2] {
			t.Fatal(b.To)
		}
	}
}
