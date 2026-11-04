package middleware

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/apperr"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func stateBudgetLimiter(now *int64, limit, keys, slots int) *rateLimiter {
	return &rateLimiter{requests: make(map[string][]int64), limit: limit, window: 60,
		maxKeys: keys, maxSlots: slots, now: func() time.Time { return time.Unix(*now, 0) }}
}

func requireRateStateBudget(t *testing.T, rl *rateLimiter) {
	t.Helper()
	slots := 0
	for _, timestamps := range rl.requests {
		require.NotEmpty(t, timestamps)
		slots += cap(timestamps)
	}
	require.Equal(t, slots, rl.slots)
	require.LessOrEqual(t, len(rl.requests), rl.maxKeys)
	require.LessOrEqual(t, slots, rl.maxSlots)
}

func TestRateLimitStateBudget_DefaultKeyBoundary(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, 1, 0, 0)
	for i := 0; i < rateLimitMaxKeys; i++ {
		require.Equal(t, rateLimitResult{}, rl.take(fmt.Sprint(i)))
	}
	require.Equal(t, rateLimitResult{retryAfter: 60, capacityBusy: true}, rl.take("overflow"))
	require.NotContains(t, rl.requests, "overflow")
	require.Equal(t, rateLimitResult{retryAfter: 60}, rl.take("0"), "frequency still returns 429 at full key capacity")
	requireRateStateBudget(t, rl)
	now = 1060
	require.Equal(t, rateLimitResult{}, rl.take("replacement"))
	require.Len(t, rl.requests, 1)
	require.Equal(t, 1, rl.slots)
	requireRateStateBudget(t, rl)
}

func TestRateLimitStateBudget_GrowthPruningAndRelease(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, 5, 2, 5)
	for i, capacity := range []int{1, 2, 4, 4, 5} {
		require.Equal(t, rateLimitResult{}, rl.take("a"))
		require.Equal(t, capacity, cap(rl.requests["a"]), "request %d", i+1)
		requireRateStateBudget(t, rl)
	}
	require.Equal(t, rateLimitResult{retryAfter: 60}, rl.take("a"))
	require.Equal(t, rateLimitResult{retryAfter: 60, capacityBusy: true}, rl.take("b"))
	require.NotContains(t, rl.requests, "b")
	now = 1060
	require.Equal(t, rateLimitResult{}, rl.take("b"))
	require.Equal(t, 1, rl.slots)
	requireRateStateBudget(t, rl)

	now = 2000
	rl = stateBudgetLimiter(&now, 5, 2, 4)
	for _, ts := range []int64{2000, 2001, 2002} {
		now = ts
		require.Equal(t, rateLimitResult{}, rl.take("a"))
	}
	now = 2060
	require.Equal(t, rateLimitResult{}, rl.take("a"))
	require.Equal(t, []int64{2001, 2002, 2060}, rl.requests["a"])
	require.Equal(t, 4, rl.slots, "partial pruning retains the whole backing array")
	require.Equal(t, rateLimitResult{retryAfter: 60, capacityBusy: true}, rl.take("b"))
	requireRateStateBudget(t, rl)
	now = 2120
	require.Equal(t, rateLimitResult{}, rl.take("b"))
	require.Len(t, rl.requests, 1)
	require.Equal(t, 1, rl.slots, "expired capacity is released once")
	requireRateStateBudget(t, rl)
}

func TestRateLimitStateBudget_DefaultSlotBoundary(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, rateLimitMaxSlots+1, 0, 0)
	// Seed an already admitted full array rather than issuing a quadratic number
	// of requests just to reach the production backing-array budget.
	timestamps := make([]int64, rateLimitMaxSlots)
	for i := range timestamps {
		timestamps[i] = now
	}
	rl.requests["full"] = timestamps
	rl.slots = cap(timestamps)
	require.True(t, rl.take("full").capacityBusy)
	require.Equal(t, rateLimitMaxSlots, len(rl.requests["full"]))
	require.True(t, rl.take("new").capacityBusy)
	require.NotContains(t, rl.requests, "new")
	requireRateStateBudget(t, rl)
	now = 1060
	require.Equal(t, rateLimitResult{}, rl.take("replacement"))
	require.Equal(t, 1, rl.slots)
	require.Len(t, rl.requests, 1)
	requireRateStateBudget(t, rl)
}

func TestRateLimitStateBudget_KnownKeyGrowthCanBeBusy(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, 5, 4, 3)
	require.Equal(t, rateLimitResult{}, rl.take("a"))
	require.Equal(t, rateLimitResult{}, rl.take("a"))
	require.Equal(t, rateLimitResult{}, rl.take("b"))
	before := append([]int64(nil), rl.requests["a"]...)
	now = 1010
	for i := 0; i < 3; i++ {
		require.Equal(t, rateLimitResult{retryAfter: 50, capacityBusy: true}, rl.take("a"))
		require.Equal(t, before, rl.requests["a"])
		require.Equal(t, 2, cap(rl.requests["a"]))
		requireRateStateBudget(t, rl)
	}
	other := stateBudgetLimiter(&now, 5, 4, 3)
	require.Equal(t, rateLimitResult{}, other.take("a"), "instances have independent capacity")

	now = 2000
	rl = stateBudgetLimiter(&now, 5, 4, 5)
	for i := 0; i < 3; i++ {
		require.Equal(t, rateLimitResult{}, rl.take("a"))
	}
	require.Equal(t, rateLimitResult{}, rl.take("b"))
	require.True(t, rl.take("c").capacityBusy)
	require.Equal(t, rateLimitResult{}, rl.take("a"), "reserved spare capacity remains usable")
	require.Equal(t, []int64{2000, 2000, 2000, 2000}, rl.requests["a"])
	requireRateStateBudget(t, rl)
}

func TestRateLimitStateBudget_CleanupClockBackAndSchedule(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, 5, 2, 8)
	require.Equal(t, rateLimitResult{}, rl.take("a"))
	now = 1030
	require.Equal(t, rateLimitResult{}, rl.take("a"))
	now = 900
	require.Equal(t, rateLimitResult{}, rl.take("a"))
	now = 1060
	require.Equal(t, rateLimitResult{}, rl.take("b"))
	require.Equal(t, []int64{1000, 1030, 900}, rl.requests["a"], "cleanup must retain the live middle timestamp")
	require.True(t, rl.take("c").capacityBusy)
	now = 1090
	for i := 0; i < 3; i++ {
		require.Equal(t, rateLimitResult{retryAfter: 30, capacityBusy: true}, rl.take("c"))
	}
	require.Contains(t, rl.requests, "a", "capacity rejection must not scan and expire unrelated keys before scheduled cleanup")
	require.EqualValues(t, 1120, rl.nextCleanup)
	now = 1120
	require.Equal(t, rateLimitResult{}, rl.take("c"))
	require.NotContains(t, rl.requests, "a")
	require.NotContains(t, rl.requests, "b")
	requireRateStateBudget(t, rl)
}

func TestRateLimitStateBudget_ConcurrentKeyAndSlotSaturation(t *testing.T) {
	for _, tc := range []struct {
		keys, slots int
		sameKey     bool
		accepted    int64
	}{{37, 100, false, 37}, {100, 37, false, 37}, {100, 37, true, 32}} {
		t.Run(fmt.Sprintf("keys%d-slots%d-same%t", tc.keys, tc.slots, tc.sameKey), func(t *testing.T) {
			now := int64(1000)
			rl := stateBudgetLimiter(&now, 1000, tc.keys, tc.slots)
			var accepted atomic.Int64
			var busy atomic.Int64
			var wg sync.WaitGroup
			for i := 0; i < 200; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					key := fmt.Sprint(i)
					if tc.sameKey {
						key = "same"
					}
					result := rl.take(key)
					if result.capacityBusy {
						busy.Add(1)
					} else if result.retryAfter == 0 {
						accepted.Add(1)
					}
				}(i)
			}
			wg.Wait()
			require.EqualValues(t, tc.accepted, accepted.Load())
			require.EqualValues(t, 200-tc.accepted, busy.Load())
			if tc.sameKey {
				require.Len(t, rl.requests, 1)
				require.Len(t, rl.requests["same"], 32)
			} else {
				require.Len(t, rl.requests, 37)
			}
			requireRateStateBudget(t, rl)
		})
	}
}

func TestRateLimitStateBudget_HTTPBusyIsSafeAndNot429(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, 1, 1, 1)
	require.Equal(t, rateLimitResult{}, rl.take("existing"))
	now = 1017
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/private", nil), rec)
	called := false
	key := "private-identifier"
	h := rl.middleware(func(echo.Context) (string, error) { return key, nil }, "frequency %d")(func(echo.Context) error {
		called = true
		return nil
	})
	err := h(c)
	var busy *apperr.Error
	require.ErrorAs(t, err, &busy)
	require.Equal(t, http.StatusServiceUnavailable, busy.Code)
	require.Equal(t, "43", busy.Headers["Retry-After"])
	require.Equal(t, "43", rec.Header().Get("Retry-After"))
	require.Equal(t, "Сервис временно занят. Повторите запрос позже.", busy.Message)
	require.NotContains(t, busy.Message, key)
	require.False(t, called)
	require.NotContains(t, rl.requests, key)
	key = "existing"
	err = h(c)
	var frequency *echo.HTTPError
	require.ErrorAs(t, err, &frequency)
	require.Equal(t, http.StatusTooManyRequests, frequency.Code)
	require.Equal(t, "frequency 43", frequency.Message)
	require.Equal(t, "43", rec.Header().Get("Retry-After"))
	require.Equal(t, []int64{1000}, rl.requests[key])
	requireRateStateBudget(t, rl)
}

func TestRateLimitStateBudget_InvalidConfigurationDoesNotAllocate(t *testing.T) {
	for _, tc := range []struct {
		limit, keys, slots int
		window             int64
	}{
		{0, 1, 1, 60}, {-1, 1, 1, 60}, {1, -1, 1, 60}, {1, 1, -1, 60}, {1, 1, 1, 0},
	} {
		now := int64(1000)
		rl := stateBudgetLimiter(&now, tc.limit, tc.keys, tc.slots)
		rl.window = tc.window
		require.True(t, rl.take("a").capacityBusy)
		require.Empty(t, rl.requests)
		require.Zero(t, rl.slots)
	}
}

// The reference preserves ordered timestamps, the cutoff rule, and scheduled
// deletion of fully expired keys. A later clock rollback cannot resurrect a key
// that was already deleted. It uses no production pruning or allocation helpers.
func TestRateLimitStateBudget_ExactWindowDifferential(t *testing.T) {
	now := int64(1000)
	rl := stateBudgetLimiter(&now, 7, 100, 1000)
	reference := make(map[string][]int64)
	nextSweep := int64(0)
	rng := rand.New(rand.NewSource(2843))
	for i := 0; i < 2000; i++ {
		now += int64(rng.Intn(31) - 10)
		key := fmt.Sprint(rng.Intn(20))
		if now >= nextSweep {
			for candidate, events := range reference {
				liveCount := 0
				for _, event := range events {
					if event > now-60 {
						liveCount++
					}
				}
				if liveCount == 0 {
					delete(reference, candidate)
				}
			}
			nextSweep = now + 60
		}
		live := make([]int64, 0)
		for _, ts := range reference[key] {
			if ts > now-60 {
				live = append(live, ts)
			}
		}
		retry := int64(0)
		if len(live) >= 7 {
			retry = 60 - (now - live[0])
			if retry < 1 {
				retry = 1
			}
		} else {
			live = append(live, now)
		}
		reference[key] = live
		result := rl.take(key)
		require.False(t, result.capacityBusy, "step %d", i)
		require.Equal(t, retry, result.retryAfter, "step %d", i)
		require.Equal(t, live, rl.requests[key], "step %d", i)
		requireRateStateBudget(t, rl)
	}
}
