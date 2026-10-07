package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type ticketOwnerStatusStub struct {
	banned, active bool
	err            error
	userID         int
}

func (s *ticketOwnerStatusStub) Status(_ context.Context, userID int) (bool, bool, error) {
	s.userID = userID
	return s.banned, s.active, s.err
}

func TestRequireActiveTicketOwner(t *testing.T) {
	for _, tc := range []struct {
		name           string
		banned, active bool
		err            error
		want           int
	}{
		{name: "active", active: true},
		{name: "banned", banned: true, active: true, want: http.StatusForbidden},
		{name: "archived", want: http.StatusForbidden},
		{name: "banned-and-archived", banned: true, want: http.StatusForbidden},
		{name: "lookup-error", active: true, err: errors.New("synthetic lookup failure"), want: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := &ticketOwnerStatusStub{banned: tc.banned, active: tc.active, err: tc.err}
			err := RequireActiveTicketOwner(context.Background(), 42, status)
			require.Equal(t, 42, status.userID)
			if tc.want == 0 {
				require.NoError(t, err)
			} else {
				var he *echo.HTTPError
				require.ErrorAs(t, err, &he)
				require.Equal(t, tc.want, he.Code)
			}
		})
	}
	t.Run("missing-checker", func(t *testing.T) {
		var he *echo.HTTPError
		require.ErrorAs(t, RequireActiveTicketOwner(context.Background(), 42, nil), &he)
		require.Equal(t, http.StatusServiceUnavailable, he.Code)
	})
}

func TestArchiveTicketOwner_DenialPrecedesCollection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status TicketOwnerStatus
		want   int
	}{
		{name: "ban", status: &ticketOwnerStatusStub{banned: true, active: true}, want: http.StatusForbidden},
		{name: "archive", status: &ticketOwnerStatusStub{}, want: http.StatusForbidden},
		{name: "lookup-error", status: &ticketOwnerStatusStub{active: true, err: errors.New("synthetic lookup failure")}, want: http.StatusServiceUnavailable},
		{name: "missing-checker", want: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// nil DB/writer: отказ должен произойти раньше любых запросов и файлов.
			s := NewArchiveDownloadService(nil, nil, nil, tc.status)
			s.tickets["synthetic-ticket"] = archiveDownloadTicket{userID: 42, expiresAt: time.Now().Add(time.Minute)}
			entries, name, owner, err := s.ConsumeAndCollect(context.Background(), "synthetic-ticket", time.Now())
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			require.Equal(t, tc.want, he.Code)
			require.Nil(t, entries)
			require.Empty(t, name)
			require.Equal(t, 42, owner)
			_, _, _, err = s.ConsumeAndCollect(context.Background(), "synthetic-ticket", time.Now())
			require.ErrorAs(t, err, &he)
			require.Equal(t, http.StatusUnauthorized, he.Code)
		})
	}
}
