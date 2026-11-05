package workbudget

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func acquire(t *testing.T, g *Gate, ctx context.Context) (context.Context, func()) {
	t.Helper()
	ctx, release, err := g.Acquire(ctx)
	if err != nil || release == nil {
		t.Fatalf("Acquire: release=%t err=%v", release != nil, err)
	}
	return ctx, release
}

func TestConcurrentActiveCap(t *testing.T) {
	g := New(2, 0, 0)
	results := make(chan bool, 32)
	start, finish := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, release, err := g.Acquire(context.Background())
			results <- err == nil
			if err == nil {
				<-finish
				release()
				release()
			} else if !errors.Is(err, ErrBusy) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	close(start)
	accepted := 0
	for i := 0; i < 32; i++ {
		if <-results {
			accepted++
		}
	}
	close(finish)
	wg.Wait()
	if accepted != 2 {
		t.Fatalf("accepted=%d, want 2", accepted)
	}
	_, release := acquire(t, g, context.Background())
	release()
}

func TestNestedLeaseAndStaleContext(t *testing.T) {
	g := New(1, 0, 0)
	ctx, release := acquire(t, g, context.Background())
	_, nested := acquire(t, g, ctx)
	nested()
	if _, _, err := g.Acquire(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("nested release freed the outer slot: %v", err)
	}
	release()
	_, current := acquire(t, g, context.Background())
	defer current()
	release()
	if _, free, err := g.Acquire(ctx); free != nil || !errors.Is(err, ErrBusy) {
		t.Fatalf("stale marker bypassed admission: %v", err)
	}
}

func TestCancellationBeforeAndWhileWaiting(t *testing.T) {
	g := New(1, 1, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, free, err := g.Acquire(ctx); free != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled: %v", err)
	}
	_, release := acquire(t, g, context.Background())
	waitCtx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, free, err := g.Acquire(waitCtx)
		if free != nil {
			free()
		}
		done <- err
	}()
	stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled acquire did not finish")
	}
	release()
	_, after := acquire(t, g, context.Background())
	after()
}

func TestNestedCanceledContextAndDistinctGate(t *testing.T) {
	a, b := New(1, 0, 0), New(1, 0, 0)
	ctx, freeA := acquire(t, a, context.Background())
	defer freeA()
	_, freeB := acquire(t, b, context.Background())
	defer freeB()
	if _, _, err := b.Acquire(ctx); !errors.Is(err, ErrBusy) {
		t.Fatalf("another gate's marker bypassed admission: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, free, err := a.Acquire(canceled); free != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("nested canceled context: %v", err)
	}
}
