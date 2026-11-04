package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/admission"
	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A nil DB is deliberate: saturation must reject before lookup, password work,
// or any database state transition. Reaching the DB fails this test immediately.
func TestLoginStateBudgetRejectsBeforeDatabase(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	guard.maxEntries = 1
	guard.now = func() time.Time { return now }
	lease, err := guard.reserve("192.0.2.1")
	require.NoError(t, err)
	defer lease.release()
	for i := 0; i < 4; i++ {
		guard.recordFailure(lease, "retained-user")
	}
	entry := guard.entries["192.0.2.1"]
	require.Equal(t, 4, entry.failures)

	service := &authService{loginGuard: guard}
	response, err := service.Login(context.Background(), models.LoginRequest{
		Username: "another-user", Password: "unverified-password",
	}, &RequestMeta{IPAddress: "192.0.2.2"})
	require.Nil(t, response)
	var domain *apperr.Error
	require.ErrorAs(t, err, &domain)
	require.Equal(t, http.StatusServiceUnavailable, domain.Code)
	require.Equal(t, "300", domain.Headers["Retry-After"])
	require.NotContains(t, domain.Headers, "X-Auth-Attempts-Remaining")
	require.Nil(t, errors.Unwrap(domain))
	require.Len(t, guard.entries, 1)
	require.Same(t, entry, guard.entries["192.0.2.1"])
	require.Equal(t, 4, entry.failures)
}

func TestLoginStateBudgetPreCanceledDoesNotReserve(t *testing.T) {
	guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	service := &authService{loginGuard: guard}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Login(ctx, models.LoginRequest{Username: "cancelled-user", Password: "unused"},
		&RequestMeta{IPAddress: "192.0.2.3"})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, guard.entries)
}

func TestLoginStateBudgetCancellationReleasesReservation(t *testing.T) {
	guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel at the reservation boundary without a timer or real database.
	guard.now = func() time.Time {
		cancel()
		return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	}
	service := &authService{loginGuard: guard}
	_, err := service.Login(ctx, models.LoginRequest{Username: "cancelled-user", Password: "unused"},
		&RequestMeta{IPAddress: "192.0.2.4"})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, guard.entries)
}
func TestLoginStateBudgetPasswordBusyReleasesReservation(t *testing.T) {
	original := passwordLimit.Load()
	gate := admission.New(1, 0, 0)
	release, err := gate.Acquire(context.Background())
	require.NoError(t, err)
	defer release()
	passwordLimit.Store(&passwordLimiter{slots: make(chan struct{}, 1), gate: gate})
	defer passwordLimit.Store(original)
	guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	guard.maxEntries = 1
	service := &authService{loginGuard: guard}
	_, err = service.Login(context.Background(), models.LoginRequest{Username: "waiting-user", Password: "unused"},
		&RequestMeta{IPAddress: "192.0.2.5"})
	var domain *apperr.Error
	require.ErrorAs(t, err, &domain)
	require.Equal(t, http.StatusServiceUnavailable, domain.Code)
	require.Equal(t, "2", domain.Headers["Retry-After"], "password gate, not the guard budget, rejected this attempt")
	require.Empty(t, guard.entries)
	next, err := guard.reserve("192.0.2.6")
	require.NoError(t, err)
	next.release()
}

func TestLoginStateBudgetLookupAndBadHashReleaseReservation(t *testing.T) {
	for _, badHash := range []bool{false, true} {
		name := "lookup-error"
		if badHash {
			name = "bad-hash"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			db := loginBudgetFaultDB(t, func(tx *gorm.DB) {
				calls++
				if !badHash {
					tx.AddError(errors.New("synthetic lookup unavailable"))
					return
				}
				user, ok := tx.Statement.Dest.(*models.User)
				require.True(t, ok, "unexpected query in controlled lookup")
				*user = models.User{ID: 7, Username: "synthetic-user", Password: "invalid-phc", IsActive: true}
				tx.RowsAffected = 1
			})
			guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
			guard.maxEntries = 1
			service := &authService{loginGuard: guard, db: db}
			_, err := service.Login(context.Background(), models.LoginRequest{Username: "synthetic-user", Password: "unused"},
				&RequestMeta{IPAddress: "192.0.2.7"})
			var domain *apperr.Error
			require.ErrorAs(t, err, &domain)
			require.Equal(t, http.StatusInternalServerError, domain.Code)
			require.Equal(t, 1, calls)
			require.Empty(t, guard.entries, "an unsuccessful technical operation must not retain capacity or failed credentials")
			next, err := guard.reserve("192.0.2.8")
			require.NoError(t, err)
			next.release()
		})
	}
}

// These callbacks exercise the real service/GORM chain without a DB server.
// Automatic transactions/ping are disabled only for this fault fixture; the
// query and audit write callbacks never execute SQL. Transaction success remains
// covered by the existing real-PostgreSQL auth integrity suite and staging.
func loginBudgetFaultDB(t *testing.T, query func(*gorm.DB)) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=unused dbname=unused sslmode=disable"}),
		&gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: logger.Discard})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, db.Callback().Query().Replace("gorm:query", query))
	require.NoError(t, db.Callback().Create().Replace("gorm:create", func(tx *gorm.DB) { tx.RowsAffected = 1 }))
	return db
}
