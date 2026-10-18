package handlers

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

type changingEventSession struct {
	eventsSessionStub
	deny   atomic.Bool
	status int
}

func (s *changingEventSession) RequireLive(ctx context.Context, identity models.SessionIdentity) (models.SessionIdentity, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 3*time.Second {
		return models.SessionIdentity{}, apperr.Internal("missing bounded session check")
	}
	if s.deny.Load() {
		return models.SessionIdentity{}, apperr.New(s.status, "synthetic session failure")
	}
	return identity, nil
}

func TestEventsSession_IdleHeartbeatClosesWithoutPayload(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			h, _, tickets := newEventsHandler()
			checker := &changingEventSession{status: status}
			h.sessions = checker
			ticks := make(chan time.Time, 1)
			h.newHeartbeat = func() (<-chan time.Time, func()) { return ticks, func() {} }
			srv := newEventsServer(t, h)
			ticket, err := tickets.Issue(eventsIdentity(42), time.Now())
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events?ticket="+ticket, nil)
			require.NoError(t, err)
			res, err := srv.Client().Do(req)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, http.StatusOK, res.StatusCode)
			reader := bufio.NewReader(res.Body)
			require.Contains(t, readFrame(t, reader), ": connected")
			checker.deny.Store(true)
			ticks <- time.Now()
			remaining, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Empty(t, remaining)
		})
	}
}
