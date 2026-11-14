package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestAccountRateLimit_SharedIdentity(t *testing.T) {
	e := echo.New()
	limiter := AccountRateLimit(2, 60)
	call := func(owner, effective int, family, token string) int {
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		r.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		if owner != 0 {
			r = r.WithContext(services.WithSessionIdentity(r.Context(), models.SessionIdentity{
				OwnerUserID: owner, EffectiveUserID: effective, FamilyID: family,
			}))
		}
		rec := httptest.NewRecorder()
		c := e.NewContext(r, rec)
		// A loose context field is not a verified session identity.
		c.Set("user_id", 123)
		if err := limiter(func(c *echo.Context) error { return c.NoContent(http.StatusOK) })(c); err != nil {
			e.HTTPErrorHandler(c, err)
		}
		return rec.Code
	}
	require.Equal(t, 401, call(0, 0, "", "unverified"))
	require.Equal(t, 200, call(1, 1, "device-a", "token-a"))
	require.Equal(t, 200, call(1, 1, "device-b", "token-refreshed"))
	require.Equal(t, 429, call(1, 99, "impersonation", "token-other"))
	require.Equal(t, 200, call(2, 1, "other-account", "token-a"))
}

func TestRateLimit_WindowAndRetryAfter(t *testing.T) {
	now := time.Unix(1000, 0)
	rl := &rateLimiter{requests: map[string][]int64{}, limit: 2, window: 60, now: func() time.Time { return now }}
	require.Zero(t, rl.take("a").retryAfter)
	now = now.Add(10 * time.Second)
	require.Zero(t, rl.take("a").retryAfter)
	require.EqualValues(t, 50, rl.take("a").retryAfter)
	now = now.Add(49 * time.Second)
	require.EqualValues(t, 1, rl.take("a").retryAfter)
	now = now.Add(time.Second)
	require.Zero(t, rl.take("a").retryAfter, "oldest entry expires exactly at the window boundary")
	require.EqualValues(t, 10, rl.take("a").retryAfter)
	now = now.Add(120 * time.Second)
	require.Zero(t, rl.take("b").retryAfter)
	require.NotContains(t, rl.requests, "a", "inactive keys are pruned without background goroutines")
}

func TestRateLimit_ConcurrentBudget(t *testing.T) {
	rl := &rateLimiter{requests: map[string][]int64{}, limit: 10, window: 60,
		now: func() time.Time { return time.Unix(1000, 0) }}
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.take("same-account").retryAfter == 0 {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 10, admitted.Load())
}
