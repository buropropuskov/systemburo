package main

import (
	"testing"
	"time"
)

// Планировщик предупреждений об истечении пропуска гоняется в фиксированный час
// рабочей зоны (#1748, S4). До этого он висел на тикере от старта процесса, и
// перезапуск бэкенда переносил рассылку на время перезапуска - предупреждения
// приходили среди ночи.

func TestNextDailyRun(t *testing.T) {
	t.Parallel()

	msk, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("Europe/Moscow не загрузилась: %v", err)
	}

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "до часа рассылки - сегодня",
			now:  time.Date(2026, 8, 13, 7, 30, 0, 0, msk),
			want: time.Date(2026, 8, 13, 9, 0, 0, 0, msk),
		},
		{
			name: "после часа рассылки - завтра",
			now:  time.Date(2026, 8, 13, 9, 0, 1, 0, msk),
			want: time.Date(2026, 8, 14, 9, 0, 0, 0, msk),
		},
		{
			name: "ровно в час рассылки - следующие сутки, не мгновенный повтор",
			now:  time.Date(2026, 8, 13, 9, 0, 0, 0, msk),
			want: time.Date(2026, 8, 14, 9, 0, 0, 0, msk),
		},
		{
			name: "полночь по UTC - тот же день по рабочей зоне",
			now:  time.Date(2026, 8, 13, 0, 30, 0, 0, time.UTC),
			want: time.Date(2026, 8, 13, 9, 0, 0, 0, msk),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nextDailyRun(c.now, 9, msk)
			if !got.Equal(c.want) {
				t.Errorf("nextDailyRun(%s) = %s, ожидалось %s",
					c.now.Format(time.RFC3339), got.Format(time.RFC3339), c.want.Format(time.RFC3339))
			}
		})
	}
}
