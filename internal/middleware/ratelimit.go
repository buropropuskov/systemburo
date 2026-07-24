package middleware

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
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
				ra := rl.retryAfterSeconds(key)
				c.Response().Header().Set("Retry-After", strconv.FormatInt(ra, 10))
				return echo.NewHTTPError(http.StatusTooManyRequests,
					fmt.Sprintf("Вы отправляете слишком много запросов. Подождите %d секунд.", ra))
			}
			return next(c)
		}
	}
}

func (rl *rateLimiter) getKey(c echo.Context) string {
	auth := c.Request().Header.Get("Authorization")
	if auth != "" && strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		// Хешируем ВЕСЬ токен. Префикс JWT (первые ~36 символов) - это base64
		// заголовка {"alg":"HS256","typ":"JWT"}, одинаковый у всех пользователей,
		// поэтому token[:20] схлопывал всех авторизованных в одно ведро и делил
		// лимит на всю систему. Уникальность токена в payload+signature -> хеш
		// по всей строке даёт ключ per-token (по факту per-user/сессия).
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		return "user:" + strconv.FormatUint(h.Sum64(), 16)
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

// retryAfterSeconds - сколько секунд до освобождения слота: остаток жизни самого
// старого запроса в окне. Отдаём РЕАЛЬНЫЙ остаток, а не полное окно, иначе клиентский
// таймер сбрасывался бы на максимум при каждом новом запросе. Минимум 1.
func (rl *rateLimiter) retryAfterSeconds(key string) int64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	ts := rl.requests[key]
	if len(ts) == 0 {
		return rl.window
	}
	remaining := rl.window - (time.Now().Unix() - ts[0])
	if remaining < 1 {
		return 1
	}
	return remaining
}

// LoginRateLimit - специализированный rate limiter для /login.
// Ключ - client IP, окно и лимит задаются отдельно от общего RateLimit.
// При превышении отвечает 429 + Retry-After, чтобы клиент знал через сколько
// можно повторить. Защита от онлайн brute-force до попадания в Argon2id.
func LoginRateLimit(maxAttempts int, window time.Duration) echo.MiddlewareFunc {
	rl := &rateLimiter{
		requests: make(map[string][]int64),
		limit:    maxAttempts,
		window:   int64(window.Seconds()),
	}
	go rl.cleanup(int64(window.Seconds()))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := "login:" + c.RealIP()
			if !rl.allow(key) {
				ra := rl.retryAfterSeconds(key)
				c.Response().Header().Set("Retry-After", strconv.FormatInt(ra, 10))
				return echo.NewHTTPError(http.StatusTooManyRequests,
					fmt.Sprintf("Слишком много попыток входа. Повторите через %d секунд.", ra))
			}
			return next(c)
		}
	}
}
