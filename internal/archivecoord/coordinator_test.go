package archivecoord

import (
	"context"
	"errors"
	"testing"
)

func TestAcquire_CancelAndRelease(t *testing.T) {
	release, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter: %v", err)
	}
	release()
	release() // Идемпотентность не освобождает чужой слот.
	second, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	if _, err := Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("slot still held: %v", err)
	}
}

func TestAcquire_WaiterCancellationDoesNotReleaseOwner(t *testing.T) {
	release, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		acquired, err := Acquire(ctx)
		if err == nil {
			acquired()
		}
		done <- err
	}()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter: %v", err)
	}
	// Освобождение владельца даёт следующему вызову ровно один слот.
	release()
	next, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	next()
}
