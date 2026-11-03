// Package admission bounds concurrent work and the number of callers waiting for it.
package admission

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrBusy means the waiting limit was reached or the gate's wait expired.
var ErrBusy = errors.New("admission gate busy")

// Gate bounds active work and pending acquisitions.
type Gate struct {
	active  chan struct{}
	pending chan struct{}
	wait    time.Duration
}

// New constructs a gate. Zero waiting capacity or a zero wait disables queuing.
func New(active, waiting int, wait time.Duration) *Gate {
	if active <= 0 {
		panic("invalid admission gate configuration")
	}
	return NewWithSlots(make(chan struct{}, active), waiting, wait)
}

// NewWithSlots uses an existing semaphore. Other users must acquire by sending
// a value and release by receiving one. The channel must stay open.
func NewWithSlots(slots chan struct{}, waiting int, wait time.Duration) *Gate {
	if cap(slots) == 0 || waiting < 0 || wait < 0 {
		panic("invalid admission gate configuration")
	}
	return &Gate{
		active:  slots,
		pending: make(chan struct{}, waiting),
		wait:    wait,
	}
}

// Acquire reserves an active slot. The caller must release the slot when work
// ends, even if ctx has since been canceled. The release function is idempotent.
func (g *Gate) Acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case g.active <- struct{}{}:
		return g.acquired(ctx)
	default:
	}
	if g.wait == 0 {
		return nil, busyOrCanceled(ctx)
	}
	select {
	case g.pending <- struct{}{}:
		defer func() { <-g.pending }()
	default:
		return nil, busyOrCanceled(ctx)
	}

	waitCtx, cancel := context.WithTimeout(ctx, g.wait)
	defer cancel()
	select {
	case g.active <- struct{}{}:
		release, err := g.acquired(waitCtx)
		if err != nil {
			return nil, busyOrCanceled(ctx)
		}
		return release, nil
	case <-waitCtx.Done():
		return nil, busyOrCanceled(ctx)
	}
}

func (g *Gate) acquired(ctx context.Context) (func(), error) {
	// Cancellation and an available slot can become ready together.
	if err := ctx.Err(); err != nil {
		<-g.active
		return nil, err
	}
	return sync.OnceFunc(func() { <-g.active }), nil
}

func busyOrCanceled(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrBusy
}
