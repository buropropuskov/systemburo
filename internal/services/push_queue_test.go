package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"systemburo/internal/models"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/require"
)

func waitTestPushSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("push fixture did not reach its barrier")
	}
}

func stopTestPushQueue(t *testing.T, q *pushDeliveryQueue) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q.shutdown(ctx)
	waitTestPushSignal(t, q.done)
}

func TestPushQueue_DefaultBoundsAndCancellation(t *testing.T) {
	q := &pushDeliveryQueue{}
	t.Cleanup(func() { stopTestPushQueue(t, q) })
	started := make(chan struct{}, pushMaxConcurrentDeliveries)
	process := func(ctx context.Context, _ int, _ []byte) {
		started <- struct{}{}
		<-ctx.Done()
	}
	for i := 0; i < pushMaxConcurrentDeliveries; i++ {
		q.enqueue(i, []byte("{}"), process)
	}
	for i := 0; i < pushMaxConcurrentDeliveries; i++ {
		waitTestPushSignal(t, started)
	}
	require.EqualValues(t, 30, q.remaining.Load())
	for i := 0; i < pushQueueCapacity; i++ {
		q.enqueue(i, []byte("{}"), process)
	}
	require.Len(t, q.jobs, 1024)
	require.EqualValues(t, 1054, q.counters.accepted.Load())
	q.enqueue(9999, []byte("{}"), process)
	require.Len(t, q.jobs, 1024)
	require.EqualValues(t, 1054, q.counters.accepted.Load())
	require.EqualValues(t, 1, q.counters.overload.Load())
	stopTestPushQueue(t, q)
	require.Empty(t, q.jobs)
	require.Zero(t, q.remaining.Load())
	require.EqualValues(t, 1054, q.counters.shutdown.Load())
	require.Zero(t, q.counters.processed.Load())
	// Repeated shutdown reuses the same done and starts no waiter goroutine.
	done := q.done
	q.shutdown(context.Background())
	require.Equal(t, done, q.done)
}

func TestPushQueue_DrainAndClosedAdmission(t *testing.T) {
	q := &pushDeliveryQueue{workers: 1, capacity: 3}
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); stopTestPushQueue(t, q) })
	started := make(chan struct{}, 3)
	processed := make(chan int, 3)
	process := func(ctx context.Context, id int, _ []byte) {
		started <- struct{}{}
		select {
		case <-release:
			processed <- id
		case <-ctx.Done():
		}
	}
	q.enqueue(1, []byte("{}"), process)
	waitTestPushSignal(t, started)
	q.enqueue(2, []byte("{}"), process)
	q.enqueue(3, []byte("{}"), process)
	unblock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	q.shutdown(ctx)
	waitTestPushSignal(t, q.done)
	require.EqualValues(t, 3, q.counters.processed.Load())
	for _, want := range []int{1, 2, 3} {
		select {
		case got := <-processed:
			require.Equal(t, want, got)
		default:
			t.Fatal("accepted work was not drained")
		}
	}
	q.enqueue(4, []byte("{}"), process)
	require.EqualValues(t, 3, q.counters.accepted.Load())
	require.EqualValues(t, 1, q.counters.shutdown.Load())
}

func TestPushQueue_QueuedAgeBoundaries(t *testing.T) {
	for _, age := range []time.Duration{pushQueueMaxAge - time.Nanosecond, pushQueueMaxAge, pushQueueMaxAge + time.Nanosecond} {
		t.Run(age.String(), func(t *testing.T) {
			var clock atomic.Int64
			clock.Store(time.Unix(1000, 0).UnixNano())
			q := &pushDeliveryQueue{workers: 1, capacity: 2, now: func() time.Time { return time.Unix(0, clock.Load()) }}
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			t.Cleanup(func() { unblock(); stopTestPushQueue(t, q) })
			started := make(chan struct{}, 2)
			var second atomic.Bool
			process := func(ctx context.Context, id int, _ []byte) {
				if id == 1 {
					started <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
					}
				} else {
					second.Store(true)
				}
			}
			q.enqueue(1, []byte("{}"), process)
			waitTestPushSignal(t, started)
			q.enqueue(2, []byte("{}"), process)
			clock.Add(int64(age))
			unblock()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			q.shutdown(ctx)
			waitTestPushSignal(t, q.done)
			require.Equal(t, age < pushQueueMaxAge, second.Load())
			if age >= pushQueueMaxAge {
				require.EqualValues(t, 1, q.counters.expired.Load())
			} else {
				require.Zero(t, q.counters.expired.Load())
			}
		})
	}
}

func TestPushQueue_ConcurrentEnqueueShutdown(t *testing.T) {
	q := &pushDeliveryQueue{workers: 2, capacity: 5}
	t.Cleanup(func() { stopTestPushQueue(t, q) })
	process := func(ctx context.Context, _ int, _ []byte) { <-ctx.Done() }
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; q.enqueue(1, []byte("{}"), process) }()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			q.shutdown(ctx)
		}()
	}
	close(start)
	wg.Wait()
	waitTestPushSignal(t, q.done)
	require.EqualValues(t, 200, q.counters.overload.Load()+q.counters.shutdown.Load())
	require.Empty(t, q.jobs)
	require.Zero(t, q.remaining.Load())
}

func TestPushQueue_LazyUnconfiguredSyncAndShutdownBeforeStart(t *testing.T) {
	unconfigured := NewPushService(nil, "", "", "").(*pushService)
	unconfigured.Send(context.Background(), 1, PushPayload{Title: "title"})
	require.Nil(t, unconfigured.delivery.jobs)
	unconfigured.Shutdown(context.Background())
	syncService := NewPushService(nil, "public", "private", "", WithPushSyncSend()).(*pushService)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	syncService.Send(ctx, 1, PushPayload{Title: "title"})
	require.Nil(t, syncService.delivery.jobs)
	syncService.Shutdown(context.Background())
	s := NewPushService(nil, "public", "private", "").(*pushService)
	s.Shutdown(context.Background())
	s.Send(context.Background(), 1, PushPayload{Title: "title"})
	require.Nil(t, s.delivery.jobs, "stopped service must never start the pool")
	require.EqualValues(t, 1, s.delivery.counters.shutdown.Load())
	invalid := NewPushService(nil, "public", "private", "").(*pushService)
	invalid.Send(context.Background(), 1, PushPayload{Type: strings.Repeat("я", 51)})
	require.Nil(t, invalid.delivery.jobs)
	require.EqualValues(t, 1, invalid.delivery.counters.invalid.Load())
	invalid.Shutdown(context.Background())
}

func TestPushQueue_PayloadSnapshotAndNormalization(t *testing.T) {
	id := 123
	payload := PushPayload{Title: strings.Repeat("я", 101), Message: strings.Repeat("😀", 501), Type: strings.Repeat("я", 50), ApplicationID: &id}
	body, ok := preparePushMessage(payload)
	require.True(t, ok)
	id = 456
	payload.Title = "changed"
	var got PushPayload
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, 123, *got.ApplicationID)
	require.Equal(t, strings.Repeat("я", 100)+"…", got.Title)
	require.Equal(t, strings.Repeat("😀", 500)+"…", got.Message)
	require.Equal(t, strings.Repeat("я", 50), got.Type)
	_, ok = preparePushMessage(PushPayload{Type: strings.Repeat("я", 51)})
	require.False(t, ok)
	body, ok = preparePushMessage(PushPayload{Message: "a\xffb", Type: "ordinary"})
	require.True(t, ok)
	require.True(t, utf8.Valid(body))
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "a�b", got.Message)
}

func TestPushQueue_PreparedJSONByteBoundary(t *testing.T) {
	// JSON escaping makes byte length independent of rune limits. The supported
	// server targets use 64-bit IDs; these values also exercise their longest form.
	largeID := int64(9223372036854775807)
	id := int(largeID)
	for _, tc := range []struct {
		suffix string
		size   int
		valid  bool
	}{{"xxx😀", 3993, true}, {"xxé😀", 3994, false}} {
		body, valid := preparePushMessage(PushPayload{
			Title: strings.Repeat("\x00", 96) + tc.suffix, Message: strings.Repeat("\x00", 500),
			Type: strings.Repeat("\x00", 50), NotificationID: id, ApplicationID: &id,
		})
		require.Equal(t, tc.size, len(body))
		require.Equal(t, tc.valid, valid)
		require.True(t, json.Valid(body))
	}
}

func TestPushQueue_DeliveryCancellationDoesNotTouchSubscriptions(t *testing.T) {
	// nil DB is intentional: any attempted failure/success update panics, so these
	// checks cannot pass merely because a canceled DB write happened to fail.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	(&pushService{}).deliver(ctx, models.PushSubscription{}, []byte("{}"))

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	resolved := false
	s := &pushService{endpointGuard: &pushEndpointGuard{resolve: func(context.Context, string) ([]netip.Addr, error) {
		resolved = true
		cancel()
		return nil, context.Canceled
	}}}
	s.deliver(ctx, models.PushSubscription{Endpoint: "https://push.example.invalid/message"}, []byte("{}"))
	require.True(t, resolved)

	p256dh, auth := testSubscriberKeys(t)
	private, public, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	called := false
	s = &pushService{publicKey: public, privateKey: private, subscriber: "test@example.invalid",
		endpointGuard: &pushEndpointGuard{allowLocal: true},
		httpClient: pushQueueCodecClient(func(*http.Request) (*http.Response, error) {
			called = true
			cancel()
			return nil, context.Canceled
		}),
	}
	s.deliver(ctx, models.PushSubscription{Endpoint: "http://127.0.0.1/push", P256dh: p256dh, Auth: auth}, []byte("{}"))
	require.True(t, called)
}

type pushQueueCodecClient func(*http.Request) (*http.Response, error)

func (f pushQueueCodecClient) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestPushQueue_WebPushActualRecordBoundary(t *testing.T) {
	p256dh, auth := testSubscriberKeys(t)
	private, public, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	for _, size := range []int{pushPayloadMaxBytes, pushPayloadMaxBytes + 1} {
		calls := 0
		body := []byte(`"` + strings.Repeat("x", size-2) + `"`)
		require.True(t, json.Valid(body))
		response, err := webpush.SendNotificationWithContext(context.Background(), body, &webpush.Subscription{
			Endpoint: "https://push.example.invalid/message", Keys: webpush.Keys{P256dh: p256dh, Auth: auth},
		}, &webpush.Options{VAPIDPublicKey: public, VAPIDPrivateKey: private, Subscriber: "test@example.invalid",
			HTTPClient: pushQueueCodecClient(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})})
		if response != nil {
			response.Body.Close()
		}
		if size == pushPayloadMaxBytes {
			require.NoError(t, err)
			require.Equal(t, 1, calls)
		} else {
			require.True(t, errors.Is(err, webpush.ErrMaxPadExceeded))
			require.Zero(t, calls)
		}
	}
}
