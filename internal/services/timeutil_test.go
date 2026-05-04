package services

import (
	"strings"
	"testing"
	"time"
)

// TestFormatUTC_AlwaysUTC проверяет что FormatUTC выдаёт строку в UTC (Z-суффикс)
// независимо от Location входного значения. Это гарантирует, что фронт через
// new Date(s).toLocaleString() корректно конвертирует в локальную зону пользователя.
func TestFormatUTC_AlwaysUTC(t *testing.T) {
	// Будущая дата, чтобы избежать совпадений с существующими тестовыми
	// фикстурами и не вводить flakiness вокруг "сейчас".
	base := time.Date(2099, 12, 31, 21, 22, 0, 0, time.UTC)

	t.Run("UTC input returns Z-suffix", func(t *testing.T) {
		got := FormatUTC(base)
		if !strings.HasSuffix(got, "Z") {
			t.Errorf("FormatUTC(UTC) = %q, want suffix Z", got)
		}
	})

	t.Run("Asia/Almaty input is converted to UTC", func(t *testing.T) {
		almaty, err := time.LoadLocation("Asia/Almaty")
		if err != nil {
			t.Skip("Asia/Almaty timezone unavailable")
		}
		// 2099-12-31 21:22 по Алматы (UTC+5) = 16:22 UTC.
		local := time.Date(2099, 12, 31, 21, 22, 0, 0, almaty)
		got := FormatUTC(local)
		if !strings.HasSuffix(got, "Z") {
			t.Errorf("FormatUTC(Asia/Almaty) = %q, want suffix Z", got)
		}
		want := "2099-12-31T16:22:00Z"
		if got != want {
			t.Errorf("FormatUTC(Asia/Almaty) = %q, want %q", got, want)
		}
	})

	t.Run("Europe/Moscow input is converted to UTC", func(t *testing.T) {
		moscow, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			t.Skip("Europe/Moscow timezone unavailable")
		}
		// 2099-12-31 18:22 по Москве (UTC+3) = 15:22 UTC.
		local := time.Date(2099, 12, 31, 18, 22, 0, 0, moscow)
		got := FormatUTC(local)
		want := "2099-12-31T15:22:00Z"
		if got != want {
			t.Errorf("FormatUTC(Europe/Moscow) = %q, want %q", got, want)
		}
	})
}

// TestFormatUTC_RoundTripParseable проверяет, что результат корректно
// парсится обратно через time.Parse с тем же моментом во времени.
// Это гарантирует совместимость с new Date(...) на фронте.
func TestFormatUTC_RoundTripParseable(t *testing.T) {
	original := time.Date(2099, 12, 31, 15, 22, 0, 0, time.UTC)
	formatted := FormatUTC(original)

	parsed, err := time.Parse(time.RFC3339, formatted)
	if err != nil {
		t.Fatalf("time.Parse(RFC3339, %q) failed: %v", formatted, err)
	}
	if !parsed.Equal(original) {
		t.Errorf("round-trip mismatch: got %v, want %v", parsed, original)
	}
}

// TestFormatUTCPtr_NilSafe проверяет nil-входы.
func TestFormatUTCPtr_NilSafe(t *testing.T) {
	if got := FormatUTCPtr(nil); got != nil {
		t.Errorf("FormatUTCPtr(nil) = %v, want nil", got)
	}

	moment := time.Date(2099, 12, 31, 15, 22, 0, 0, time.UTC)
	got := FormatUTCPtr(&moment)
	if got == nil {
		t.Fatal("FormatUTCPtr(&t) = nil, want non-nil")
	}
	want := "2099-12-31T15:22:00Z"
	if *got != want {
		t.Errorf("FormatUTCPtr(&t) = %q, want %q", *got, want)
	}
}

// TestNowUTC_ReturnsUTCLocation проверяет, что NowUTC всегда возвращает
// значение в UTC-зоне -- даже если хост-машина в другой TZ.
func TestNowUTC_ReturnsUTCLocation(t *testing.T) {
	got := NowUTC()
	if got.Location() != time.UTC {
		t.Errorf("NowUTC().Location() = %v, want UTC", got.Location())
	}
}

// TestFormatUTC_PreservesInstant проверяет, что момент времени сохраняется
// даже если Location другой -- баг "удаление в 21:22 фиксируется как 15:22"
// возникает именно при потере точного момента после конверсий.
func TestFormatUTC_PreservesInstant(t *testing.T) {
	almaty, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Skip("Asia/Almaty timezone unavailable")
	}

	// Один и тот же момент в двух разных зонах должен дать одинаковую UTC-строку.
	utcMoment := time.Date(2099, 12, 31, 16, 22, 0, 0, time.UTC)
	almatyMoment := utcMoment.In(almaty) // тот же instant, другая Location

	if !utcMoment.Equal(almatyMoment) {
		t.Fatal("test setup invalid: moments must be equal")
	}

	if FormatUTC(utcMoment) != FormatUTC(almatyMoment) {
		t.Errorf("FormatUTC must produce identical strings for same instant: %q vs %q",
			FormatUTC(utcMoment), FormatUTC(almatyMoment))
	}
}
