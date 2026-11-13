package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOptionalEntryPeriod2903(t *testing.T) {
	ptr := func(s string) *string { return &s }
	for name, period := range map[string]EntryPeriod{
		"unbounded": {}, "empty": {EntryDateFrom: ptr(""), EntryTimeTo: ptr("")},
		"leap":          {EntryDateFrom: ptr("2028-02-29"), EntryDateTo: ptr("2028-03-01")},
		"year_boundary": {EntryDateFrom: ptr("2026-12-31"), EntryDateTo: ptr("2027-01-01")},
		"night":         {EntryDateFrom: ptr("2026-12-31"), EntryDateTo: ptr("2026-12-31"), EntryTimeFrom: ptr("22:00"), EntryTimeTo: ptr("06:00")},
		"clocks":        {EntryTimeFrom: ptr("00:00"), EntryTimeTo: ptr("23:59:59")},
		"end_only":      {EntryDateTo: ptr("2027-01-01")},
	} {
		t.Run(name, func(t *testing.T) { require.NoError(t, ValidateOptionalEntryPeriod(period)) })
	}
	for name, period := range map[string]EntryPeriod{
		"invalid_date": {EntryDateFrom: ptr("not-a-date")}, "invalid_end": {EntryDateTo: ptr("not-a-date")},
		"invalid_clock": {EntryTimeFrom: ptr("not-a-time")}, "invalid_clock_end": {EntryTimeTo: ptr("25:00")},
		"not_leap": {EntryDateTo: ptr("2027-02-29")}, "invalid_month": {EntryDateFrom: ptr("2026-13-01")},
		"reverse":          {EntryDateFrom: ptr("2027-01-01"), EntryDateTo: ptr("2026-12-31")},
		"fractional_clock": {EntryTimeFrom: ptr("12:00:00.123")},
		"padded":           {EntryDateTo: ptr(" 2027-01-01 ")}, "whitespace": {EntryDateFrom: ptr(" ")},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, ValidateOptionalEntryPeriod(period), ErrInvalidIndividualPeriod)
		})
	}
}
