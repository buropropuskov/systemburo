package services

import (
	"testing"
	"time"
)

// computeWorkModeStatus зависит от текущего времени; кейсы подобраны так, чтобы
// результат был детерминирован в любой момент суток (привязка к "сегодня" и к
// слотам, открытым/закрытым независимо от времени).
func TestComputeWorkModeStatus(t *testing.T) {
	today := int(time.Now().In(moscowWorkModeLoc).Weekday()+6) % 7
	otherDay := (today + 1) % 7

	cases := []struct {
		name  string
		slots []WorkModeSlot
		want  string
	}{
		{"пусто", nil, "closed"},
		{
			"круглосуточно сегодня",
			[]WorkModeSlot{{DayOfWeek: today, OpenTime: "00:00", CloseTime: "23:59", IsActive: true}},
			"open",
		},
		{
			// is_next_day: открытие в полночь -> currentTime >= "00:00" всегда true.
			"переход через полночь сегодня",
			[]WorkModeSlot{{DayOfWeek: today, OpenTime: "00:00", CloseTime: "06:00", IsNextDay: true, IsActive: true}},
			"open",
		},
		{
			"круглосуточно на другой день",
			[]WorkModeSlot{{DayOfWeek: otherDay, OpenTime: "00:00", CloseTime: "23:59", IsActive: true}},
			"closed",
		},
		{
			"неактивный круглосуточный слот сегодня",
			[]WorkModeSlot{{DayOfWeek: today, OpenTime: "00:00", CloseTime: "23:59", IsActive: false}},
			"closed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeWorkModeStatus(tc.slots); got != tc.want {
				t.Errorf("computeWorkModeStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}
