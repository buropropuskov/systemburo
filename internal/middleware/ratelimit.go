package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]int64
	limit    int
	window   int64 // seconds
}

// RateLimit creates rate limiting middleware: limit requests per window seconds.
func RateLimit(limit int, windowSeconds int64) echo.MiddlewareFunc {
	rl := &rateLimiter{
		requests: make(map[string][]int64),
		limit:    limit,
		window:   windowSeconds,
	}
	go rl.cleanup(windowSeconds)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := rl.getKey(c)
			if !rl.allow(key) {
				return echo.NewHTTPError(http.StatusTooManyRequests,
					"Вы отправляете слишком много запросов. Подождите 60 секунд.")
			}
			return next(c)
		}
	}
}

func (rl *rateLimiter) getKey(c echo.Context) string {
	auth := c.Request().Header.Get("Authorization")
	if auth != "" && strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if len(token) > 20 {
			return "user:" + token[:20]
		}
		return "user:" + token
	}
	return c.RealIP()
}

func (rl *rateLimiter) cleanup(windowSec int64) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now().Unix()
		for key, timestamps := range rl.requests {
			valid := timestamps[:0]
			for _, ts := range timestamps {
				if now-ts < windowSec {
					valid = append(valid, ts)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = valid
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().Unix()
	cutoff := now - rl.window

	// Clean expired entries
	timestamps := rl.requests[key]
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts > cutoff {
			valid = append(valid, ts)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}

	rl.requests[key] = append(valid, now)
	return true
}
