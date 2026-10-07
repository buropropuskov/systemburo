package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

type eventsOwnerStatusStub struct {
	banned, active bool
	err            error
	userID         int
}

func (s *eventsOwnerStatusStub) Status(_ context.Context, userID int) (bool, bool, error) {
	s.userID = userID
	return s.banned, s.active, s.err
}

func TestEventsStream_DeniesInactiveOwnerBeforeOpening(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status services.TicketOwnerStatus
		want   int
	}{
		{name: "ban", status: &eventsOwnerStatusStub{banned: true, active: true}, want: http.StatusForbidden},
		{name: "archive", status: &eventsOwnerStatusStub{}, want: http.StatusForbidden},
		{name: "lookup-error", status: &eventsOwnerStatusStub{active: true, err: errors.New("synthetic lookup failure")}, want: http.StatusServiceUnavailable},
		{name: "missing-checker", want: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tickets := realtime.NewTicketStore(time.Minute)
			h := NewEventsHandler(realtime.NewHub(), tickets, tc.status)
			srv := newEventsServer(t, h)
			ticket, err := tickets.Issue(42, time.Now())
			require.NoError(t, err)
			resp, err := srv.Client().Get(srv.URL + "/api/events?ticket=" + ticket)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, tc.want, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NotContains(t, string(body), ": connected")
			require.NotContains(t, resp.Header.Get("Content-Type"), "text/event-stream")
			if status, ok := tc.status.(*eventsOwnerStatusStub); ok {
				require.Equal(t, 42, status.userID)
			}
			_, ok := tickets.Consume(ticket, time.Now())
			require.False(t, ok, "отказанный билет должен остаться погашенным")
		})
	}
}
