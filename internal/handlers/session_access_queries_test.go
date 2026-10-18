package handlers_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sessionQueryCounter struct {
	logger.Interface
	count atomic.Int32
}

func (l *sessionQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	l.count.Add(1)
}

func TestSessionAccess_NormalQueryBudgetAndNoMemo(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, _ := loginForRefreshTests(t, e, db, "query_owner")
	access, _ := testutil.LoginUser(t, e, "query_owner", "correctpass")
	claims, err := services.DecodeAccessToken(access, []byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	counter := &sessionQueryCounter{Interface: logger.Discard}
	resolver := services.NewSessionAccessService(db.Session(&gorm.Session{Logger: counter}), testutil.TestJWTRefreshSecret)
	identity, err := resolver.ResolveAccess(context.Background(), claims)
	require.NoError(t, err)
	require.Equal(t, int32(2), counter.count.Swap(0), "current owner plus live-family EXISTS")
	_, err = resolver.ResolveRefreshCookie(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, int32(2), counter.count.Swap(0), "exact cookie row plus current owner")
	for range 2 {
		_, err = resolver.RequireLive(context.Background(), identity)
		require.NoError(t, err)
	}
	require.Equal(t, int32(4), counter.count.Load(), "long-lived requests must not memoize the first result")
}
