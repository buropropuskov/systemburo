package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type rateLimiter struct {
	mu          sync.Mutex
	requests    map[string][]int64
	limit       int
	window      int64 // seconds
	now         func() time.Time
	nextCleanup int64
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
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			identifier, err := key(c)
			if err != nil {
				return err
			}
			if retryAfter := rl.take(identifier); retryAfter > 0 {
				c.Response().Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				return echo.NewHTTPError(http.StatusTooManyRequests, fmt.Sprintf(message, retryAfter))
			}
			return next(c)
		}
	}
}

// take atomically prunes, checks and records a request. Lazy cleanup avoids
// creating a permanent goroutine for each limiter or test application.
func (rl *rateLimiter) take(key string) int64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now().Unix()
	cutoff := now - rl.window
	if now >= rl.nextCleanup {
		for identifier, timestamps := range rl.requests {
			if len(timestamps) == 0 || timestamps[len(timestamps)-1] <= cutoff {
				delete(rl.requests, identifier)
			}
		}
		rl.nextCleanup = now + 60
	}
	timestamps := rl.requests[key]
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts > cutoff {
			valid = append(valid, ts)
		}
	}
	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		if len(valid) == 0 {
			return 1
		}
		remaining := rl.window - (now - valid[0])
		if remaining < 1 {
			remaining = 1
		}
		return remaining
	}
	rl.requests[key] = append(valid, now)
	return 0
}

// LoginRateLimit preserves the independent per-IP login budget.
func LoginRateLimit(maxAttempts int, window time.Duration) echo.MiddlewareFunc {
	return newRateLimit(maxAttempts, int64(window.Seconds()), time.Now,
		func(c echo.Context) (string, error) { return "login:" + c.RealIP(), nil },
		"Слишком много попыток входа. Повторите через %d секунд.")
}
