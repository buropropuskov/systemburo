package services

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type ticketSessionStub struct {
	SessionAccess
	err error
}

func (s ticketSessionStub) RequireLive(_ context.Context, identity models.SessionIdentity) (models.SessionIdentity, error) {
	return identity, s.err
}

func TestArchiveTicketOwner_ConcurrentConsumeAndSourceFailure(t *testing.T) {
	for _, sourceFailure := range []bool{false, true} {
		s := NewArchiveDownloadService(nil, nil, nil, &ticketOwnerStatusStub{active: true}, ticketSessionStub{})
		if sourceFailure {
			s.sessions = ticketSessionStub{err: apperr.New(503, "synthetic source failure")}
		}
		s.tickets["one-ticket"] = archiveDownloadTicket{userID: 42, identity: models.SessionIdentity{EffectiveUserID: 42}, expiresAt: time.Now().Add(time.Minute)}
		var authorizations, failures, replays atomic.Int32
		var workers sync.WaitGroup
		for range 12 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				_, _, _, err := s.ConsumeAndCollect(context.Background(), "one-ticket", time.Now(), func(models.SessionIdentity) error {
					authorizations.Add(1)
					return apperr.New(503, "synthetic authorization failure")
				})
				var he *echo.HTTPError
				var domain *apperr.Error
				if errors.As(err, &he) && he.Code == 401 {
					replays.Add(1)
				}
				if errors.As(err, &domain) && domain.Code == 503 {
					failures.Add(1)
				}
			}()
		}
		workers.Wait()
		require.Equal(t, int32(11), replays.Load())
		require.Equal(t, int32(1), failures.Load())
		wantAuthorizations := int32(1)
		if sourceFailure {
			wantAuthorizations = 0
		}
		require.Equal(t, wantAuthorizations, authorizations.Load(), "source failure must precede authorization and file reads")
	}
}

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
			s := NewArchiveDownloadService(nil, nil, nil, tc.status, nil)
			s.tickets["synthetic-ticket"] = archiveDownloadTicket{userID: 42, expiresAt: time.Now().Add(time.Minute)}
			entries, name, owner, err := s.ConsumeAndCollect(context.Background(), "synthetic-ticket", time.Now(), nil)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			require.Equal(t, tc.want, he.Code)
			require.Nil(t, entries)
			require.Empty(t, name)
			require.Equal(t, 42, owner)
			_, _, _, err = s.ConsumeAndCollect(context.Background(), "synthetic-ticket", time.Now(), nil)
			require.ErrorAs(t, err, &he)
			require.Equal(t, http.StatusUnauthorized, he.Code)
		})
	}
}
