package services

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/apperr"

	"github.com/stretchr/testify/require"
)

func budgetGuard(capacity int) (*loginGuard, *time.Time) {
	now := time.Date(2040, 2, 3, 4, 5, 6, 0, time.UTC)
	g := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	g.maxEntries = capacity
	g.now = func() time.Time { return now }
	return g, &now
}

func budgetReserve(t *testing.T, g *loginGuard, ip string) *loginGuardLease {
	t.Helper()
	lease, err := g.reserve(ip)
	require.NoError(t, err)
	require.NotNil(t, lease)
	return lease
}

func budgetBusy(t *testing.T, g *loginGuard, ip string, seconds string) {
	t.Helper()
	lease, err := g.reserve(ip)
	require.Nil(t, lease)
	var domain *apperr.Error
	require.ErrorAs(t, err, &domain)
	require.Equal(t, http.StatusServiceUnavailable, domain.Code)
	require.Equal(t, seconds, domain.Headers["Retry-After"])
	require.NotContains(t, domain.Headers, "X-Auth-Attempts-Remaining")
}

func TestLoginGuardBudget_ConcurrentReservationsRespectProductionCap(t *testing.T) {
	g, _ := budgetGuard(guardMaxEntries)
	const attempts = guardMaxEntries + 1
	leases := make(chan *loginGuardLease, attempts)
	var next, busy atomic.Int32
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := int(next.Add(1)); i <= attempts; i = int(next.Add(1)) {
				lease, err := g.reserve(fmt.Sprintf("address-%d", i))
				if err != nil {
					var domain *apperr.Error
					if errors.As(err, &domain) && domain.Code == http.StatusServiceUnavailable {
						busy.Add(1)
					}
					continue
				}
				leases <- lease
			}
		}()
	}
	workers.Wait()
	close(leases)
	require.Len(t, g.entries, guardMaxEntries)
	require.EqualValues(t, 1, busy.Load())
	for lease := range leases {
		lease.release()
		lease.release()
	}
	require.Empty(t, g.entries)
}

func TestLoginGuardBudget_RetainsFourFailuresAndFifthBlocks(t *testing.T) {
	g, now := budgetGuard(1)
	for i := 0; i < 4; i++ {
		left, _, blocked := guardTestFailure(t, g, "office", "worker")
		require.Equal(t, 4-i, left)
		require.False(t, blocked)
	}
	before := g.entries["office"].users[sha256.Sum256([]byte("worker"))].failures
	*now = now.Add(500 * time.Millisecond)
	budgetBusy(t, g, "new-address", "300")
	require.Len(t, g.entries, 1)
	require.Equal(t, before, g.entries["office"].users[sha256.Sum256([]byte("worker"))].failures)
	left, seconds, blocked := guardTestFailure(t, g, "office", "worker")
	require.Zero(t, left)
	require.Positive(t, seconds)
	require.True(t, blocked)
	budgetBusy(t, g, "another-address", "300")
	_, blocked = g.blockedSeconds("office", "worker")
	require.True(t, blocked)
}

func TestLoginGuardBudget_ResetRetainsLeasesAndLateFailure(t *testing.T) {
	for _, byUser := range []bool{false, true} {
		t.Run(fmt.Sprintf("by-user=%t", byUser), func(t *testing.T) {
			g, _ := budgetGuard(1)
			first := budgetReserve(t, g, "office")
			second := budgetReserve(t, g, "office")
			g.recordFailure(first, "Работник")
			entry := first.entry
			if byUser {
				require.Equal(t, 1, g.resetUser("Работник"))
			} else {
				g.reset("office")
			}
			require.Same(t, entry, g.entries["office"])
			require.Equal(t, 2, entry.refs)
			require.Empty(t, entry.users)
			first.release()
			first.release()
			require.Equal(t, 1, entry.refs)
			budgetBusy(t, g, "new-address", "300")
			left, _, blocked := g.recordFailure(second, "Работник")
			require.Equal(t, 4, left)
			require.False(t, blocked)
			second.release()
			require.Equal(t, 1, entry.failures)
			require.Zero(t, entry.refs)
		})
	}
}

func TestLoginGuardBudget_ResetAndAbandonedReservationFreeSpace(t *testing.T) {
	g, _ := budgetGuard(1)
	lease := budgetReserve(t, g, "office")
	g.recordFailure(lease, "worker")
	g.reset("office")
	require.Len(t, g.entries, 1)
	lease.release()
	require.Empty(t, g.entries)

	next := budgetReserve(t, g, "next")
	next.release()
	require.Empty(t, g.entries)
	empty := budgetReserve(t, g, "")
	left, _, blocked := g.recordFailure(empty, "worker")
	require.Equal(t, 5, left)
	require.False(t, blocked)
	empty.release()
	require.Empty(t, g.entries)
}

func TestLoginGuardBudget_ExpiresLevelZeroButRetainsLadder(t *testing.T) {
	g, now := budgetGuard(1)
	guardTestFailure(t, g, "office", "worker")
	*now = now.Add(loginFailureWindow) // Existing counter expires strictly after the window.
	budgetBusy(t, g, "new-address", "300")
	*now = now.Add(guardCleanupInterval)
	lease := budgetReserve(t, g, "new-address")
	require.NotContains(t, g.entries, "office")
	lease.release()

	for i := 0; i < 5; i++ {
		guardTestFailure(t, g, "office", "worker")
	}
	lastFailure := *now
	*now = now.Add(guardCleanupInterval + time.Nanosecond)
	budgetBusy(t, g, "new-address", "300")
	require.Equal(t, 1, g.entries["office"].users[sha256.Sum256([]byte("worker"))].level)
	*now = lastFailure.Add(lockoutLevelDecay)
	budgetBusy(t, g, "new-address", "300")
	*now = now.Add(guardCleanupInterval + time.Nanosecond)
	lease = budgetReserve(t, g, "new-address")
	require.NotContains(t, g.entries, "office")
	lease.release()
}

func TestLoginGuardBudget_ClockBackDoesNotForgetState(t *testing.T) {
	g, now := budgetGuard(1)
	for i := 0; i < 4; i++ {
		guardTestFailure(t, g, "office", "worker")
	}
	*now = now.Add(-time.Hour)
	budgetBusy(t, g, "new-address", "3900")
	require.Equal(t, 4, g.entries["office"].failures)
	require.Equal(t, 4, g.entries["office"].users[sha256.Sum256([]byte("worker"))].failures)
}

func TestLoginGuardBudget_DeferredLeaseReleaseOnPanic(t *testing.T) {
	g, _ := budgetGuard(1)
	require.Panics(t, func() {
		lease := budgetReserve(t, g, "office")
		defer lease.release()
		panic("synthetic operation panic")
	})
	require.Empty(t, g.entries)
}

func TestLoginGuardBudget_ActiveBlockAndRefsSurviveCleanup(t *testing.T) {
	g, now := budgetGuard(1)
	lease := budgetReserve(t, g, "office")
	entry := lease.entry
	*now = now.Add(time.Hour)
	budgetBusy(t, g, "new-address", "300")
	require.Same(t, entry, g.entries["office"])
	lease.release()
	require.Empty(t, g.entries)

	lease = budgetReserve(t, g, "office")
	lease.entry.blockedUntil = now.Add(2 * time.Hour)
	lease.entry.lastFailure = *now
	lease.release()
	*now = now.Add(time.Hour)
	budgetBusy(t, g, "new-address", "300")
	_, blocked := g.blockedSeconds("office", "")
	require.True(t, blocked)
}

func TestLoginGuardBudget_UsernameDigestIsExactAndBounded(t *testing.T) {
	g, _ := budgetGuard(1)
	g.max = 100
	names := []string{"", "User", "user", " user", "Работник", "é", "e\u0301", strings.Repeat("界", 10000)}
	for _, name := range names {
		guardTestFailure(t, g, "office", name)
	}
	entry := g.entries["office"]
	require.Len(t, entry.users, len(names)-1)
	require.NotContains(t, entry.users, sha256.Sum256(nil))
	for _, name := range names[1:] {
		require.Equal(t, 1, entry.users[sha256.Sum256([]byte(name))].failures)
	}
	for i := len(entry.users); i < guardUsernamesPerIP+3; i++ {
		guardTestFailure(t, g, "office", fmt.Sprintf("extra-%d", i))
	}
	require.Len(t, entry.users, guardUsernamesPerIP)
	require.Zero(t, g.resetUser(" USER"))
	require.Equal(t, 1, g.resetUser("Работник"))
	require.Empty(t, g.entries)
}

func TestLoginGuardBudget_RefusedKeysDoNotRepeatCleanup(t *testing.T) {
	g, now := budgetGuard(2)
	a := budgetReserve(t, g, "first")
	b := budgetReserve(t, g, "second")
	g.recordFailure(a, "worker")
	a.release()
	b.entry.lastFailure = now.Add(-2 * loginFailureWindow)
	b.release()
	// Mark this entry stale after cleanup has already been scheduled. Repeated
	// full-budget refusals must leave it until the scheduled sweep.
	for i := 0; i < 20; i++ {
		budgetBusy(t, g, fmt.Sprintf("refused-%d", i), "300")
	}
	require.Len(t, g.entries, 2)
	*now = now.Add(guardCleanupInterval)
	lease := budgetReserve(t, g, "accepted")
	lease.release()
}
