package middleware

import (
	"sync"
	"testing"
	"time"
)

func TestSeenThrottle_ShouldWrite(t *testing.T) {
	base := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	window := 60 * time.Second

	tests := []struct {
		name string
		// steps: каждый шаг - запрос пользователя в момент base+offset; want - ожидаемое shouldWrite.
		steps []struct {
			userID int
			offset time.Duration
			want   bool
		}
	}{
		{
			name: "первый запрос юзера всегда пишет",
			steps: []struct {
				userID int
				offset time.Duration
				want   bool
			}{
				{1, 0, true},
			},
		},
		{
			name: "два быстрых запроса в окне - одна запись",
			steps: []struct {
				userID int
				offset time.Duration
				want   bool
			}{
				{1, 0, true},
				{1, 5 * time.Second, false},
				{1, 59 * time.Second, false},
			},
		},
		{
			name: "запрос за окном - вторая запись",
			steps: []struct {
				userID int
				offset time.Duration
				want   bool
			}{
				{1, 0, true},
				{1, 30 * time.Second, false},
				{1, 60 * time.Second, true},
				{1, 90 * time.Second, false},
				{1, 120 * time.Second, true},
			},
		},
		{
			name: "разные юзеры не мешают друг другу",
			steps: []struct {
				userID int
				offset time.Duration
				want   bool
			}{
				{1, 0, true},
				{2, 1 * time.Second, true},
				{1, 2 * time.Second, false},
				{2, 3 * time.Second, false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th := newSeenThrottle(window)
			for i, s := range tc.steps {
				got := th.shouldWrite(s.userID, base.Add(s.offset))
				if got != s.want {
					t.Errorf("step %d (user=%d offset=%s): shouldWrite=%v, want %v",
						i, s.userID, s.offset, got, s.want)
				}
			}
		})
	}
}

// TestSeenThrottle_Concurrent проверяет, что всплеск конкурентных запросов одного
// юзера в одном окне приводит ровно к одной записи (защита горячего пути от гонки).
func TestSeenThrottle_Concurrent(t *testing.T) {
	th := newSeenThrottle(60 * time.Second)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)

	const goroutines = 100
	var wg sync.WaitGroup
	var mu sync.Mutex
	writes := 0

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if th.shouldWrite(7, now) {
				mu.Lock()
				writes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if writes != 1 {
		t.Errorf("конкурентный всплеск дал %d записей, ожидалась 1", writes)
	}
}
