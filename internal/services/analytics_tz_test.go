package services

import (
	"testing"
	"time"
)

func TestAnalyticsLocation(t *testing.T) {
	loc := AnalyticsLocation()
	if loc == nil {
		t.Fatal("AnalyticsLocation вернул nil")
	}
	if loc.String() != "Europe/Moscow" {
		t.Fatalf("ожидали Europe/Moscow, получили %q", loc.String())
	}
}

func TestTzColumn(t *testing.T) {
	got := tzColumn("sending_datetime")
	want := "(sending_datetime AT TIME ZONE 'Europe/Moscow')"
	if got != want {
		t.Fatalf("tzColumn: ожидали %q, получили %q", want, got)
	}
}

// parseReportDate должен трактовать YYYY-MM-DD как границу московских суток, а не
// UTC: 00:00 МСК (= 21:00 UTC предыдущих суток), конец дня — 23:59:59 МСК.
func TestParseReportDate_MoscowBoundaries(t *testing.T) {
	loc := AnalyticsLocation()
	tests := []struct {
		name     string
		in       string
		endOfDay bool
		want     time.Time
	}{
		{"начало суток МСК", "2026-06-15", false, time.Date(2026, 6, 15, 0, 0, 0, 0, loc)},
		{"конец суток МСК", "2026-06-15", true, time.Date(2026, 6, 15, 23, 59, 59, 0, loc)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseReportDate(tt.in, tt.endOfDay)
			if !ok {
				t.Fatal("parseReportDate вернул ok=false")
			}
			if !got.Equal(tt.want) {
				t.Fatalf("ожидали %v, получили %v", tt.want, got)
			}
			// Начало московских суток в UTC — это 21:00 предыдущего дня.
			if !tt.endOfDay {
				utc := got.UTC()
				if utc.Hour() != 21 {
					t.Fatalf("00:00 МСК в UTC должно быть 21:00, получили %02d:00", utc.Hour())
				}
			}
		})
	}
}
