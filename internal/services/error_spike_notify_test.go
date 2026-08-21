package services

import (
	"testing"
	"time"
)

// Пауза между уведомлениями: всплеск живёт минутами и десятками ошибок в минуту, и
// без неё каждый прогон слал бы новое уведомление.
func TestErrorSpikeCooldown(t *testing.T) {
	s := &errorSpikeNotifyService{}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	if !s.takeCooldownSlot(now) {
		t.Fatal("первое уведомление должно проходить")
	}
	if s.takeCooldownSlot(now.Add(errorSpikeCooldown - time.Minute)) {
		t.Fatal("внутри паузы второе уведомление проходить не должно")
	}
	if !s.takeCooldownSlot(now.Add(errorSpikeCooldown + time.Minute)) {
		t.Fatal("после паузы уведомление снова должно проходить")
	}
}

// Порог и нижняя граница потока. Ночью за пять минут проходит десяток запросов, и
// одна ошибка из двух дала бы «50% ошибок» - тревогу на ровном месте.
func TestErrorSpikeThreshold(t *testing.T) {
	cases := []struct {
		name   string
		counts errorSpikeCounts
		want   bool
	}{
		{"поток ниже границы, но всё ошибки", errorSpikeCounts{Total: 2, Errors: 2}, false},
		{"ровно на границе потока и выше порога", errorSpikeCounts{Total: errorSpikeMinRequests, Errors: 10}, true},
		{"поток есть, ошибок в норме", errorSpikeCounts{Total: 1000, Errors: 10}, false},
		{"ровно порог", errorSpikeCounts{Total: 1000, Errors: 50}, true},
		{"ошибок нет вовсе", errorSpikeCounts{Total: 1000, Errors: 0}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := spikeReached(tc.counts); got != tc.want {
				t.Fatalf("spikeReached(%+v) = %v, ожидалось %v", tc.counts, got, tc.want)
			}
		})
	}
}
