package services

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Одиночные дата и время в бланке идут в том же виде, что и диапазоны (#1454).
// Раньше они отдавались сырыми: в бланке соседствовали "2026-07-15" и
// "15.07.2026 - 17.07.2026", а время приезжало с секундами.
func TestResolveValue_DateTimeFormat(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	bctx := &BlankContext{
		Attachment: &models.Attachment{
			EntryDateFrom: strPtr("2026-07-15"),
			EntryDateTo:   strPtr("2026-07-17"),
			EntryTimeFrom: strPtr("09:00:00"),
			EntryTimeTo:   strPtr("18:30:00"),
		},
		Cars: []models.Car{{
			EntryDateFrom: strPtr("2026-07-15"),
			EntryDateTo:   strPtr("2026-07-17"),
			EntryTimeFrom: strPtr("09:00:00"),
			EntryTimeTo:   strPtr("18:30:00"),
		}},
	}

	cases := []struct {
		path string
		want string
	}{
		{"attachment.entry_date_from", "15.07.2026"},
		{"attachment.entry_date_to", "17.07.2026"},
		{"attachment.entry_time_from", "09:00"},
		{"attachment.entry_time_to", "18:30"},
		{"attachment.entry_date_range", "15.07.2026 - 17.07.2026"},
		{"attachment.entry_time_range", "09:00 - 18:30"},
		{"car.entry_date_from", "15.07.2026"},
		{"car.entry_date_to", "17.07.2026"},
		{"car.entry_time_from", "09:00"},
		{"car.entry_time_to", "18:30"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			require.Equal(t, tc.want, resolveValue(bctx, tc.path, 0))
		})
	}
}

// Дата, пришедшая из БД с временной частью, тоже приводится к дд.мм.гггг,
// а нераспознанное значение остаётся как есть - в бланке лучше исходная строка,
// чем пустая ячейка.
func TestResolveValue_DateFallback(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	withDate := func(v string) *BlankContext {
		return &BlankContext{Attachment: &models.Attachment{EntryDateFrom: strPtr(v)}}
	}
	require.Equal(t, "15.07.2026", resolveValue(withDate("2026-07-15T00:00:00Z"), "attachment.entry_date_from", 0))
	require.Equal(t, "не дата", resolveValue(withDate("не дата"), "attachment.entry_date_from", 0))
	require.Empty(t, resolveValue(withDate(""), "attachment.entry_date_from", 0))
}
