package services

import (
	"strings"
	"testing"
	"time"
)

// Юниты без БД для текстовых хелперов уведомлений контента/системы (#1748).

func TestTruncateForNotification(t *testing.T) {
	t.Parallel()

	short := "Короткое сообщение"
	if got := truncateForNotification(short, 160); got != short {
		t.Errorf("truncateForNotification(short) = %q, want unchanged %q", got, short)
	}

	long := strings.Repeat("а", 200)
	got := truncateForNotification(long, 160)
	runes := []rune(got)
	if len(runes) != 163 { // 160 символов + "..."
		t.Errorf("truncateForNotification(long) length = %d, want 163", len(runes))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncateForNotification(long) = %q, want suffix '...'", got)
	}

	if got := truncateForNotification("  пробелы по краям  ", 160); got != "пробелы по краям" {
		t.Errorf("truncateForNotification не обрезал пробелы: %q", got)
	}
}

func TestFormatMaintenanceDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"нулевая", 0, "менее минуты"},
		{"только минуты", 45 * time.Minute, "45 мин"},
		{"ровно час", 1 * time.Hour, "1 ч"},
		{"час с минутами", 2*time.Hour + 30*time.Minute, "2 ч 30 мин"},
		{"меньше минуты", 20 * time.Second, "менее минуты"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatMaintenanceDuration(tc.d); got != tc.want {
				t.Errorf("formatMaintenanceDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
