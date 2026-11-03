package admission

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestGateBoundsPendingAndAdmitsAfterRelease(t *testing.T) {
	g := New(1, 1, time.Minute)
	release := acquireForTest(t, g)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type result struct {
		release func()
		err     error
	}
	done := make(chan result, 1)
	go func() {
		release, err := g.Acquire(ctx)
		done <- result{release, err}
	}()
	waitUntilPending(t, ctx, g)
	if extra, err := g.Acquire(ctx); extra != nil || !errors.Is(err, ErrBusy) {
		t.Fatalf("full queue: release present=%t, error=%v", extra != nil, err)
	}

	release()
	select {
	case got := <-done:
		if got.err != nil || got.release == nil {
			t.Fatalf("pending caller: release present=%t, error=%v", got.release != nil, got.err)
		}
		got.release()
	case <-ctx.Done():
		t.Fatal("pending caller did not finish")
	}
}

func TestGateCancellationRestoresWaitingCapacity(t *testing.T) {
	g := New(1, 1, time.Minute)
	release := acquireForTest(t, g)
	defer release()
	deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithCancel(deadline)
		done := make(chan error, 1)
		go func() {
			free, err := g.Acquire(ctx)
			if free != nil {
				free()
			}
			done <- err
		}()
		waitUntilPending(t, deadline, g)
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("pending caller error=%v, want context.Canceled", err)
			}
		case <-deadline.Done():
			t.Fatal("canceled caller did not finish")
		}
	}
}

func TestGateTimeoutAndContextError(t *testing.T) {
	g := New(1, 1, time.Millisecond)
	release := acquireForTest(t, g)
	defer release()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if free, err := g.Acquire(ctx); free != nil || !errors.Is(err, ErrBusy) {
		t.Fatalf("gate timeout: release present=%t, error=%v", free != nil, err)
	}
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if free, err := g.Acquire(expired); free != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context deadline: release present=%t, error=%v", free != nil, err)
	}
}

func TestGateSharesSlotsAndReleaseIsIdempotent(t *testing.T) {
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	g := NewWithSlots(slots, 0, time.Second)
	if free, err := g.Acquire(context.Background()); free != nil || !errors.Is(err, ErrBusy) {
		t.Fatalf("external slot in use: release present=%t, error=%v", free != nil, err)
	}
	<-slots

	first := acquireForTest(t, g)
	first()
	second := acquireForTest(t, g)
	first()
	select {
	case slots <- struct{}{}:
		<-slots
		t.Fatal("duplicate release freed the current caller's slot")
	default:
		second()
	}
}

func acquireForTest(t *testing.T, g *Gate) func() {
	t.Helper()
	release, err := g.Acquire(context.Background())
	if err != nil || release == nil {
		t.Fatalf("acquire: release present=%t, error=%v", release != nil, err)
	}
	return release
}

func waitUntilPending(t *testing.T, ctx context.Context, g *Gate) {
	t.Helper()
	for len(g.pending) == 0 {
		if ctx.Err() != nil {
			t.Fatal("caller did not enter the pending queue")
		}
		runtime.Gosched()
	}
}
