package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

const (
	rateLimitMaxKeys = 20000
	// Retained int64 backing arrays only; map/key memory and old arrays awaiting
	// garbage collection are outside this budget.
	rateLimitMaxSlots = 1 << 20
)

type rateLimiter struct {
	mu          sync.Mutex
	requests    map[string][]int64
	limit       int
	window      int64 // seconds
	now         func() time.Time
	nextCleanup int64
	maxKeys     int
	maxSlots    int
	slots       int // sum of cap(timestamps), not the number of live timestamps
}

type rateLimitResult struct {
	retryAfter   int64
	capacityBusy bool
}

// RateLimit limits requests by the address selected by Echo.IPExtractor.
// Unverified request headers never select a separate account budget.
func RateLimit(limit int, windowSeconds int64) echo.MiddlewareFunc {
	return newRateLimit(limit, windowSeconds, time.Now, func(c echo.Context) (string, error) {
		return "ip:" + c.RealIP(), nil
	}, "Вы отправляете слишком много запросов. Подождите %d секунд.")
}

// AccountRateLimit must run after JWTAuth or FileAccess verifies the session.
// Reuse this instance across routes to share one budget across all devices.
func AccountRateLimit(limit int, windowSeconds int64) echo.MiddlewareFunc {
	return newRateLimit(limit, windowSeconds, time.Now, accountRateLimitKey,
		"Вы отправляете слишком много запросов. Подождите %d секунд.")
}

func accountRateLimitKey(c echo.Context) (string, error) {
	identity, ok := services.SessionIdentityFromContext(c.Request().Context())
	if !ok || identity.OwnerUserID <= 0 {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Сессия завершена. Войдите снова.")
	}
	return "account:" + strconv.Itoa(identity.OwnerUserID), nil
}

func newRateLimit(limit int, windowSeconds int64, now func() time.Time,
	key func(echo.Context) (string, error), message string,
) echo.MiddlewareFunc {
	rl := &rateLimiter{requests: make(map[string][]int64), limit: limit, window: windowSeconds, now: now}
	return rl.middleware(key, message)
}

func (rl *rateLimiter) middleware(key func(echo.Context) (string, error), message string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			identifier, err := key(c)
			if err != nil {
				return err
			}
			result := rl.take(identifier)
			if result.capacityBusy {
				retry := strconv.FormatInt(result.retryAfter, 10)
				c.Response().Header().Set("Retry-After", retry)
				return apperr.New(http.StatusServiceUnavailable, "Сервис временно занят. Повторите запрос позже.").
					WithHeader("Retry-After", retry)
			}
			if result.retryAfter > 0 {
				c.Response().Header().Set("Retry-After", strconv.FormatInt(result.retryAfter, 10))
				return echo.NewHTTPError(http.StatusTooManyRequests, fmt.Sprintf(message, result.retryAfter))
			}
			return next(c)
		}
	}
}

// take atomically prunes, checks and records a request. Lazy cleanup avoids
// creating a permanent goroutine for each limiter or test application.
// Frequency (429) and state-budget (503) rejections record no request.
func (rl *rateLimiter) take(key string) rateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now().Unix()
	cutoff := now - rl.window
	// Zero fields retain the production defaults for existing private callers.
	if rl.maxKeys == 0 {
		rl.maxKeys = rateLimitMaxKeys
	}
	if rl.maxSlots == 0 {
		rl.maxSlots = rateLimitMaxSlots
	}
	if now >= rl.nextCleanup {
		for identifier, timestamps := range rl.requests {
			live := false
			for _, ts := range timestamps {
				if ts > cutoff {
					live = true
					break
				}
			}
			if !live {
				rl.slots -= cap(timestamps)
				delete(rl.requests, identifier)
			}
		}
		rl.nextCleanup = now + 60
	}
	busy := func() rateLimitResult {
		retryAfter := rl.nextCleanup - now
		if retryAfter < 1 {
			retryAfter = 1
		}
		return rateLimitResult{retryAfter: retryAfter, capacityBusy: true}
	}
	if rl.limit <= 0 || rl.window <= 0 || rl.maxKeys < 0 || rl.maxSlots < 0 {
		return busy()
	}
	timestamps, exists := rl.requests[key]
	validCount := 0
	for _, ts := range timestamps {
		if ts > cutoff {
			validCount++
		}
	}
	if validCount >= rl.limit {
		valid := pruneRateTimestamps(timestamps, cutoff)
		rl.requests[key] = valid
		remaining := rl.window - (now - valid[0])
		if remaining < 1 {
			remaining = 1
		}
		return rateLimitResult{retryAfter: remaining}
	}
	if exists && validCount == 0 {
		rl.slots -= cap(timestamps)
		delete(rl.requests, key)
		timestamps = nil
		exists = false
	}
	if !exists && len(rl.requests) >= rl.maxKeys {
		return busy()
	}
	if validCount == cap(timestamps) {
		oldCap := cap(timestamps)
		newCap := 1
		if oldCap > 0 {
			// Clamp before doubling so even an extreme configured limit cannot overflow.
			newCap = rl.limit
			if oldCap < rl.limit-oldCap {
				newCap = 2 * oldCap
			}
		}
		delta := newCap - oldCap
		if delta > rl.maxSlots-rl.slots {
			return busy()
		}
		grown := make([]int64, len(timestamps), newCap)
		copy(grown, timestamps)
		timestamps = grown
		rl.slots += delta
	}
	valid := pruneRateTimestamps(timestamps, cutoff)
	// Capacity was reserved above; this append cannot allocate.
	valid = append(valid, now)
	if rl.requests == nil {
		rl.requests = make(map[string][]int64)
	}
	rl.requests[key] = valid
	return rateLimitResult{}
}

func pruneRateTimestamps(timestamps []int64, cutoff int64) []int64 {
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts > cutoff {
			valid = append(valid, ts)
		}
	}
	return valid
}

// LoginRateLimit preserves the independent per-IP login budget.
func LoginRateLimit(maxAttempts int, window time.Duration) echo.MiddlewareFunc {
	return newRateLimit(maxAttempts, int64(window.Seconds()), time.Now,
		func(c echo.Context) (string, error) { return "login:" + c.RealIP(), nil },
		"Слишком много попыток входа. Повторите через %d секунд.")
}
