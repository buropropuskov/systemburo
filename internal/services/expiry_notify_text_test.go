package services

import (
	"testing"
	"time"
)

// Юниты без БД для текста предупреждения об истекающем пропуске (#1748, S4).

func TestRussianDays(t *testing.T) {
	t.Parallel()

	cases := map[int]string{
		1:  "1 день",
		2:  "2 дня",
		3:  "3 дня",
		4:  "4 дня",
		5:  "5 дней",
		11: "11 дней",
		12: "12 дней",
		14: "14 дней",
		21: "21 день",
		22: "22 дня",
		25: "25 дней",
	}
	for n, want := range cases {
		if got := russianDays(n); got != want {
			t.Errorf("russianDays(%d) = %q, ожидалось %q", n, got, want)
		}
	}
}

func TestExpiryNotifyBody(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)

	// Накануне срок называется словом: "через 1 день" человек читает медленнее.
	if got, want := expiryNotifyBody("№ 2026-0431", date, 1),
		"Срок действия пропуска по заявке № 2026-0431 истекает завтра, 16.08.2026."; got != want {
		t.Errorf("порог накануне: %q, ожидалось %q", got, want)
	}

	if got, want := expiryNotifyBody("№ 2026-0431", date, 3),
		"Срок действия пропуска по заявке № 2026-0431 истекает через 3 дня, 16.08.2026."; got != want {
		t.Errorf("порог за три дня: %q, ожидалось %q", got, want)
	}
}
