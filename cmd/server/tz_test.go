package main

import (
	"testing"
	"time"
)

// TestMoscowTimezoneResolves гарантирует, что зона планировщика (Europe/Moscow)
// резолвится в самом бинаре. Прод-образ - alpine без системной tzdata, сборка
// CGO_ENABLED=0; без импорта _ "time/tzdata" тут LoadLocation падает в рантайме,
// startDailyStatusReset уходит в фолбэк на UTC и снимает слепки в 06:00 UTC =
// 09:00 МСК вместо 06:00 МСК. В CI (Debian с OS-tzdata) тест зелёный и без
// импорта - он фиксирует ожидаемое смещение, а не наличие embedded-фолбэка;
// alpine-проверка бинаря делается на этапе верификации.
func TestMoscowTimezoneResolves(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("Europe/Moscow не загрузилась (нет tzdata в бинаре): %v", err)
	}

	// МСК = UTC+3 без перехода на летнее время с 2014.
	at := time.Date(2026, 7, 6, 6, 0, 0, 0, loc)
	if _, offset := at.Zone(); offset != 3*60*60 {
		t.Fatalf("ожидали смещение МСК +10800с, получили %d", offset)
	}

	// 06:00 МСК соответствует 03:00 UTC - именно этот момент должен вычислять
	// планировщик; если бы зона упала на UTC, 06:00 "МСК" стало бы 06:00 UTC.
	if h := at.UTC().Hour(); h != 3 {
		t.Fatalf("06:00 МСК ожидается как 03:00 UTC, получили %02d:00 UTC", h)
	}
}
