// Package workbudget shares one bounded budget across HTTP and background work.
package workbudget

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"systemburo/internal/admission"
)

// ErrBusy denotes a full queue or an expired admission wait.
var ErrBusy = admission.ErrBusy

// Gate adds synchronous nested-call ownership to the existing admission gate.
type Gate struct {
	gate *admission.Gate
}

type leaseKey struct{}

type lease struct {
	gate *Gate
	held atomic.Bool
}

var defaultGate = newDefault()

func newDefault() *Gate {
	active := min(2, max(1, runtime.GOMAXPROCS(0)))
	return New(active, 2*active, 2*time.Second)
}

// Default is process-local. It is shared by commands, imports, exports and uploads;
// the initial conservative profile is not a measured capacity guarantee.
func Default() *Gate { return defaultGate }

// New constructs an explicit gate for isolated uses/tests. Invalid limits panic;
// zero waiting capacity or wait disables queuing, never active-work limits.
func New(active, waiting int, wait time.Duration) *Gate {
	return &Gate{gate: admission.New(active, waiting, wait)}
}

// Acquire uses the common process budget. Always defer release after success.
func Acquire(ctx context.Context) (context.Context, func(), error) {
	return defaultGate.Acquire(ctx)
}

// Acquire preserves a live lease through synchronous nested calls. Never pass
// the derived context to detached work that outlives this release; workers must
// acquire independently before coordination locks and before transactions.
func (g *Gate) Acquire(ctx context.Context) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if current, ok := ctx.Value(leaseKey{}).(*lease); ok && current.gate == g && current.held.Load() {
		return ctx, func() {}, nil
	}
	free, err := g.gate.Acquire(ctx)
	if err != nil {
		return ctx, nil, err
	}
	owner := &lease{gate: g}
	owner.held.Store(true)
	release := sync.OnceFunc(func() {
		owner.held.Store(false)
		free()
	})
	return context.WithValue(ctx, leaseKey{}, owner), release, nil
}
