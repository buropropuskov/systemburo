package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A notification when a blocking select consults Done makes cancellation tests
// deterministic without sleeping or assuming when the worker was scheduled.
type slotObservedContext struct {
	context.Context
	once     sync.Once
	observed chan struct{}
}

func (c *slotObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func passwordContextAwait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("controlled password operation did not reach its barrier")
	}
}

func TestPasswordContextSlot_CancelledWaitDoesNotRunOrLoseSlot(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	occupied, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	defer func() { releaseOnce.Do(func() { close(release) }); passwordContextAwait(t, holderDone) }()
	go func() { defer close(holderDone); withArgon2Slot(func() { close(occupied); <-release }) }()
	passwordContextAwait(t, occupied)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &slotObservedContext{Context: base, observed: make(chan struct{})}
	var ran atomic.Bool
	result := make(chan error, 1)
	go func() { result <- withArgon2SlotContext(ctx, func() { ran.Store(true) }) }()
	passwordContextAwait(t, ctx.observed)
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled slot waiter remained queued")
	}
	require.False(t, ran.Load())
	sem := argon2Sem.Load().(chan struct{})
	require.Len(t, sem, 1, "cancelled waiter must neither consume nor release the holder's slot")
	releaseOnce.Do(func() { close(release) })
	passwordContextAwait(t, holderDone)
	require.NoError(t, withArgon2SlotContext(context.Background(), func() { ran.Store(true) }))
	require.True(t, ran.Load())
	require.Empty(t, sem)
}

func TestPasswordContextSlot_LegacyWaitAndPanicRelease(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	sem := argon2Sem.Load().(chan struct{})
	sem <- struct{}{}
	entered, done := make(chan struct{}), make(chan struct{})
	var ran atomic.Bool
	go func() { close(entered); defer close(done); withArgon2Slot(func() { ran.Store(true) }) }()
	passwordContextAwait(t, entered)
	require.False(t, ran.Load(), "legacy caller cannot run while the only slot is held")
	<-sem
	passwordContextAwait(t, done)
	require.True(t, ran.Load())
	require.PanicsWithValue(t, "slot panic", func() {
		_ = withArgon2SlotContext(context.Background(), func() { panic("slot panic") })
	})
	require.Empty(t, sem)
	require.NoError(t, withArgon2SlotContext(context.Background(), func() {}))
}

func TestPasswordContextSlot_HashAndVerifyCancellationKeepDistinctErrors(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	hash := HashPassword("context-slot-original")
	require.NotEmpty(t, hash, "legacy HashPassword must still produce a usable PHC hash")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	value, err := hashPasswordContext(ctx, "cancelled-value")
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, value)
	match, err := verifyPasswordContext(ctx, hash, "context-slot-original")
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, match)
	match, err = verifyPasswordContext(context.Background(), "damaged-PHC", "x")
	require.Error(t, err)
	require.NotErrorIs(t, err, context.Canceled)
	require.False(t, match)
	match, err = verifyPasswordContext(context.Background(), hash, "context-slot-original")
	require.NoError(t, err)
	require.True(t, match)
}

func TestPasswordContextSlot_RunningWorkRetainsSlotUntilCompletion(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, finish, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var finishOnce sync.Once
	defer func() { finishOnce.Do(func() { close(finish) }); passwordContextAwait(t, done) }()
	result := make(chan error, 1)
	go func() { defer close(done); result <- withArgon2SlotContext(ctx, func() { close(started); <-finish }) }()
	passwordContextAwait(t, started)
	cancel()
	require.Len(t, argon2Sem.Load().(chan struct{}), 1, "cancellation must not detach running work from its memory slot")
	select {
	case <-result:
		t.Fatal("running work returned before its controlled completion")
	default:
	}
	finishOnce.Do(func() { close(finish) })
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("completed computation retained its slot")
	}
	require.Empty(t, argon2Sem.Load().(chan struct{}))
}

func TestAuthTransitionGate_CancelledWaitIndependentStripeAndPanicRelease(t *testing.T) {
	const userID = 19
	occupied, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	defer func() { releaseOnce.Do(func() { close(release) }); passwordContextAwait(t, holderDone) }()
	go func() {
		defer close(holderDone)
		_ = withAuthTransitionContext(context.Background(), userID, func() error { close(occupied); <-release; return nil })
	}()
	passwordContextAwait(t, occupied)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &slotObservedContext{Context: base, observed: make(chan struct{})}
	var ran atomic.Bool
	result := make(chan error, 1)
	go func() {
		result <- withAuthTransitionContext(ctx, userID+64, func() error { ran.Store(true); return nil })
	}()
	passwordContextAwait(t, ctx.observed)
	// A different stripe stays usable while one identity's transition is held.
	require.NoError(t, withAuthTransitionContext(context.Background(), userID+1, func() error { return nil }))
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled transition waiter remained queued")
	}
	require.False(t, ran.Load())
	releaseOnce.Do(func() { close(release) })
	passwordContextAwait(t, holderDone)
	require.PanicsWithValue(t, "transition panic", func() {
		_ = withAuthTransitionContext(context.Background(), userID, func() error { panic("transition panic") })
	})
	require.NoError(t, withAuthTransitionContext(context.Background(), userID, func() error { return nil }))
}

func TestAuthTransitionGate_CollidingCallerRunsAfterRelease(t *testing.T) {
	const userID = 23
	occupied, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer func() { once.Do(func() { close(release) }); passwordContextAwait(t, holderDone) }()
	go func() {
		defer close(holderDone)
		_ = withAuthTransitionContext(context.Background(), userID, func() error { close(occupied); <-release; return nil })
	}()
	passwordContextAwait(t, occupied)
	ctx := &slotObservedContext{Context: context.Background(), observed: make(chan struct{})}
	var ran atomic.Bool
	result := make(chan error, 1)
	go func() {
		result <- withAuthTransitionContext(ctx, userID+64, func() error { ran.Store(true); return nil })
	}()
	passwordContextAwait(t, ctx.observed)
	require.False(t, ran.Load())
	once.Do(func() { close(release) })
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("colliding transition did not resume after release")
	}
	require.True(t, ran.Load())
}
