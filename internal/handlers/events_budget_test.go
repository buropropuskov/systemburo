package handlers

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

type countingEventsOwner struct{ calls atomic.Int32 }

func (s *countingEventsOwner) Status(context.Context, int) (bool, bool, error) {
	s.calls.Add(1)
	return false, true, nil
}

func TestEventsBudgetRejectsBeforeLive(t *testing.T) {
	for _, system := range []bool{false, true} {
		t.Run(map[bool]string{false: "owner", true: "system"}[system], func(t *testing.T) {
			h, _, tickets := newEventsHandler()
			h.hub = realtime.NewHubWithLimits(1, 1)
			_, release, err := h.hub.Subscribe(42, 42)
			require.NoError(t, err)
			defer release()
			checker := &countingEventsOwner{}
			h.ownerStatus = checker
			id := 42
			want := http.StatusTooManyRequests
			if system {
				id = 43
				want = http.StatusServiceUnavailable
			}
			ticket, err := tickets.Issue(eventsIdentity(id), time.Now())
			require.NoError(t, err)
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/events?ticket="+ticket, nil), rec)
			err = h.Stream(c)
			require.Error(t, err)
			e.HTTPErrorHandler(c, err)
			require.Equal(t, want, rec.Code)
			require.Equal(t, "5", rec.Header().Get("Retry-After"))
			require.NotContains(t, rec.Body.String(), ": connected")
			require.Zero(t, checker.calls.Load())
			_, ok := tickets.Consume(ticket, time.Now())
			require.False(t, ok)
		})
	}
}

func TestEventsTicketCapacityIsRetryable(t *testing.T) {
	h, _, _ := newEventsHandler()
	h.tickets = realtime.NewTicketStoreWithLimits(time.Minute, 1, 2)
	_, err := h.tickets.Issue(eventsIdentity(42), time.Now())
	require.NoError(t, err)
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events/ticket", nil)
	req = req.WithContext(services.WithSessionIdentity(req.Context(), eventsIdentity(42)))
	c := e.NewContext(req, rec)
	c.Set("user_id", 42)
	err = h.IssueTicket(c)
	require.Error(t, err)
	e.HTTPErrorHandler(c, err)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "5", rec.Header().Get("Retry-After"))
}

func TestEventsUnsupportedDeadlineReleasesReservation(t *testing.T) {
	h, hub, tickets := newEventsHandler()
	ticket, err := tickets.Issue(eventsIdentity(42), time.Now())
	require.NoError(t, err)
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/events?ticket="+ticket, nil), rec)
	err = h.Stream(c)
	require.Error(t, err)
	e.HTTPErrorHandler(c, err)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.NotContains(t, rec.Body.String(), ": connected")
	// All owner slots must be available after the failed writer setup.
	for i := 0; i < 8; i++ {
		_, release, err := hub.Subscribe(42, 42)
		require.NoError(t, err)
		defer release()
	}
}

type blockingEventsSession struct {
	eventsSessionStub
	calls   atomic.Int32
	entered chan struct{}
}

func (s *blockingEventsSession) RequireLive(ctx context.Context, id models.SessionIdentity) (models.SessionIdentity, error) {
	if s.calls.Add(1) == 1 {
		return id, nil
	}
	select {
	case s.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return models.SessionIdentity{}, ctx.Err()
}

func TestEventsShutdownCancelsBufferedChecks(t *testing.T) {
	h, hub, tickets := newEventsHandler()
	checker := &blockingEventsSession{entered: make(chan struct{}, 1)}
	h.sessions = checker
	srv := newEventsServer(t, h)
	ticket, err := tickets.Issue(eventsIdentity(42), time.Now())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events?ticket="+ticket, nil)
	require.NoError(t, err)
	response, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	require.Contains(t, readFrame(t, reader), ": connected")
	for i := 0; i < 17; i++ {
		hub.Publish(42, realtime.Event{Type: "must-not-follow-shutdown", Scope: "notifications"})
	}
	select {
	case <-checker.entered:
	case <-ctx.Done():
		t.Fatal("live checker not entered")
	}
	h.Close()
	h.Close()
	rest, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.LessOrEqual(t, checker.calls.Load(), int32(2))
	_, _, err = hub.Subscribe(42, 42)
	require.ErrorIs(t, err, realtime.ErrClosed)
}

type blockedEventWriter struct {
	conn   net.Conn
	header http.Header
	flush  bool
}

func (w *blockedEventWriter) Header() http.Header { return w.header }
func (w *blockedEventWriter) WriteHeader(int)     {}
func (w *blockedEventWriter) Write(b []byte) (int, error) {
	if w.flush {
		return len(b), nil
	}
	return w.conn.Write(b)
}
func (w *blockedEventWriter) FlushError() error {
	if !w.flush {
		return nil
	}
	_, err := w.conn.Write([]byte("flush"))
	return err
}
func (w *blockedEventWriter) SetWriteDeadline(t time.Time) error { return w.conn.SetWriteDeadline(t) }

func TestEventsBoundedWriteAndFlushReleaseSlots(t *testing.T) {
	for _, flush := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "flush"}[flush], func(t *testing.T) {
			server, client := net.Pipe()
			defer server.Close()
			defer client.Close()
			h, hub, tickets := newEventsHandler()
			h.writeTimeout = 20 * time.Millisecond
			ticket, err := tickets.Issue(eventsIdentity(42), time.Now())
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req := httptest.NewRequest(http.MethodGet, "/api/events?ticket="+ticket, nil).WithContext(ctx)
			e := echo.New()
			c := e.NewContext(req, &blockedEventWriter{conn: server, header: make(http.Header), flush: flush})
			finished := make(chan error, 1)
			go func() { finished <- h.Stream(c) }()
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-ctx.Done():
				server.Close()
				<-finished
				t.Fatal("writer exceeded its deadline")
			}
			for i := 0; i < 8; i++ {
				_, release, err := hub.Subscribe(42, 42)
				require.NoError(t, err)
				defer release()
			}
		})
	}
}

func TestEventsLifetimeIndependentOfHeartbeat(t *testing.T) {
	h, _, tickets := newEventsHandler()
	h.maxLifetime = 40 * time.Millisecond
	h.newHeartbeat = func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }
	srv := newEventsServer(t, h)
	ticket, err := tickets.Issue(eventsIdentity(42), time.Now())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events?ticket="+ticket, nil)
	require.NoError(t, err)
	r, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	require.Equal(t, ": connected\n\n", string(body))
}
