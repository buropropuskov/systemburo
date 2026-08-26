package services

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// db=nil переводит periodCache в чисто in-memory режим (persist/warmup - no-op),
// что позволяет тестировать логику кэширования без БД.

func TestPeriodCache_GetCachesPerPeriod(t *testing.T) {
	var calls int64
	compute := func(_ context.Context, from, _ time.Time) (int, error) {
		atomic.AddInt64(&calls, 1)
		return int(from.Unix()), nil
	}
	c := newPeriodCache[int](nil, "test", time.Hour, compute)
	ctx := context.Background()
	from := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	if _, err := c.get(ctx, from, to); err != nil {
		t.Fatal(err)
	}
	if _, err := c.get(ctx, from, to); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Fatalf("compute calls = %d, want 1 (повторный get того же периода должен брать из кэша)", got)
	}

	if _, err := c.get(ctx, from, to.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Fatalf("compute calls = %d, want 2 (другой период - новый расчёт)", got)
	}
}

func TestPeriodCache_RefreshRecomputesHotKey(t *testing.T) {
	var calls int64
	compute := func(_ context.Context, _, _ time.Time) (int, error) {
		return int(atomic.AddInt64(&calls, 1)), nil
	}
	c := newPeriodCache[int](nil, "test", time.Hour, compute)
	ctx := context.Background()
	from, to := time.Now(), time.Now().Add(time.Hour)

	v1, _ := c.get(ctx, from, to) // calls=1
	c.refresh(ctx)                // горячий ключ пересчитан, calls=2
	v2, _ := c.get(ctx, from, to) // из кэша - уже обновлённое значение
	if v1 == v2 {
		t.Fatalf("refresh должен пересчитать горячий ключ: v1=%d v2=%d", v1, v2)
	}
}

func TestPeriodCache_RefreshEvictsCold(t *testing.T) {
	var calls int64
	compute := func(_ context.Context, _, _ time.Time) (int, error) {
		return int(atomic.AddInt64(&calls, 1)), nil
	}
	// evict=0 - любой ключ при refresh считается холодным и выселяется.
	c := newPeriodCache[int](nil, "test", 0, compute)
	ctx := context.Background()
	from, to := time.Now(), time.Now().Add(time.Hour)

	c.get(ctx, from, to) // calls=1, ключ в кэше
	c.refresh(ctx)       // выселяет (lastAccess старше evict=0)
	c.get(ctx, from, to) // miss -> снова считает, calls=2
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Fatalf("compute calls = %d, want 2 (после выселения - пересчёт)", got)
	}
}

func TestPeriodCache_GetPropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	c := newPeriodCache[int](nil, "test", time.Hour,
		func(_ context.Context, _, _ time.Time) (int, error) { return 0, wantErr })
	if _, err := c.get(context.Background(), time.Now(), time.Now()); !errors.Is(err, wantErr) {
		t.Fatalf("get error = %v, want %v", err, wantErr)
	}
}
