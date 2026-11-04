package services

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pushQueueCapacity = 1024
	pushQueueMaxAge   = 10 * time.Minute
)

type pushJob struct {
	userID     int
	body       []byte
	enqueuedAt time.Time
}

type pushQueueCounters struct {
	accepted  atomic.Uint64
	overload  atomic.Uint64
	expired   atomic.Uint64
	shutdown  atomic.Uint64
	invalid   atomic.Uint64
	processed atomic.Uint64
	timeout   atomic.Uint64
	nextLog   atomic.Int64
}

// pushDeliveryQueue owns admission and worker lifetime. The zero value starts
// no workers; only a valid enqueue starts the pool. All channel sends and closing
// the channel are serialized by mu, including Shutdown before the first Send.
type pushDeliveryQueue struct {
	mu        sync.Mutex
	stopping  bool
	jobs      chan pushJob
	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	remaining atomic.Int32
	counters  pushQueueCounters
	// Private controls permit deterministic unit fixtures without public options.
	workers  int
	capacity int
	now      func() time.Time
}

func (q *pushDeliveryQueue) enqueue(userID int, body []byte, process func(context.Context, int, []byte)) {
	q.mu.Lock()
	if q.stopping {
		q.counters.shutdown.Add(1)
		q.mu.Unlock()
		q.reportLoss()
		return
	}
	if q.jobs == nil {
		workers, capacity := q.workers, q.capacity
		if workers <= 0 {
			workers = pushMaxConcurrentDeliveries
		}
		if capacity <= 0 {
			capacity = pushQueueCapacity
		}
		if q.now == nil {
			q.now = time.Now
		}
		q.jobs = make(chan pushJob, capacity)
		q.done = make(chan struct{})
		q.ctx, q.cancel = context.WithCancel(context.Background())
		q.remaining.Store(int32(workers))
		for i := 0; i < workers; i++ {
			go q.run(process)
		}
	}
	select {
	case q.jobs <- pushJob{userID: userID, body: body, enqueuedAt: q.now()}:
		q.counters.accepted.Add(1)
		q.mu.Unlock()
	default:
		q.counters.overload.Add(1)
		q.mu.Unlock()
		q.reportLoss()
	}
}

func (q *pushDeliveryQueue) run(process func(context.Context, int, []byte)) {
	defer func() {
		if q.remaining.Add(-1) == 0 {
			q.cancel()
			close(q.done)
		}
	}()
	for {
		select {
		case <-q.ctx.Done():
			return
		case job, ok := <-q.jobs:
			if !ok {
				return
			}
			if q.ctx.Err() != nil {
				q.counters.shutdown.Add(1)
				continue
			}
			if q.now().Sub(job.enqueuedAt) >= pushQueueMaxAge {
				q.counters.expired.Add(1)
				q.reportLoss()
				continue
			}
			ctx, cancel := context.WithTimeout(q.ctx, PushSendTimeout)
			process(ctx, job.userID, job.body)
			err := ctx.Err()
			cancel()
			if err == nil {
				q.counters.processed.Add(1)
			} else if q.ctx.Err() != nil {
				q.counters.shutdown.Add(1)
			} else {
				q.counters.timeout.Add(1)
			}
		}
	}
}

func (q *pushDeliveryQueue) shutdown(ctx context.Context) {
	q.mu.Lock()
	if !q.stopping {
		q.stopping = true
		if q.jobs == nil {
			q.done = make(chan struct{})
			close(q.done)
		} else {
			close(q.jobs)
		}
	}
	done := q.done
	q.mu.Unlock()
	// Prefer already completed drain when the caller's context is also done.
	select {
	case <-done:
		return
	default:
	}
	select {
	case <-done:
		return
	case <-ctx.Done():
		q.abort()
	}
}

func (q *pushDeliveryQueue) abort() {
	q.mu.Lock()
	if q.cancel != nil {
		q.cancel()
	}
	// shutdown already closed jobs under the same lock. Discard without waiting
	// for a transport that ignores cancellation; cooperative workers close done.
	if q.jobs != nil {
		for range q.jobs {
			q.counters.shutdown.Add(1)
		}
	}
	q.mu.Unlock()
	q.reportLoss()
}

func (q *pushDeliveryQueue) reportLoss() {
	now := time.Now().Unix()
	next := q.counters.nextLog.Load()
	if now < next || !q.counters.nextLog.CompareAndSwap(next, now+60) {
		return
	}
	// Aggregate only: no payload, user IDs, endpoint labels or per-drop logs.
	slog.Warn("push: сводка ограниченной фоновой доставки",
		"accepted", q.counters.accepted.Load(), "overload", q.counters.overload.Load(),
		"expired", q.counters.expired.Load(), "shutdown", q.counters.shutdown.Load(),
		"invalid", q.counters.invalid.Load(), "processed", q.counters.processed.Load(),
		"timeout", q.counters.timeout.Load())
}
