package realtime

import (
	"errors"
	"sync"
	"systemburo/internal/models"
	"testing"
	"time"
)

func TestTicketStore_IssueThenConsume(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(time.Minute)
	now := time.Unix(1_700_000_000, 0)

	ticket, err := s.Issue(ticketIdentity(42, now), now)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if ticket == "" {
		t.Fatal("пустой билет")
	}

	uid, ok := s.Consume(ticket, now)
	if !ok || uid.EffectiveUserID != 42 {
		t.Fatalf("ожидали userID=42 ok=true, получили uid=%v ok=%v", uid, ok)
	}
}

func TestTicketStore_ConsumeIsOneTime(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(time.Minute)
	now := time.Unix(1_700_000_000, 0)

	ticket, _ := s.Issue(ticketIdentity(1, now), now)
	if _, ok := s.Consume(ticket, now); !ok {
		t.Fatal("первый consume должен пройти")
	}
	if _, ok := s.Consume(ticket, now); ok {
		t.Fatal("повторный consume того же билета должен быть отвергнут")
	}
}

func TestTicketStore_ConsumeExpired(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(60 * time.Second)
	now := time.Unix(1_700_000_000, 0)

	ticket, _ := s.Issue(ticketIdentity(1, now), now)
	if _, ok := s.Consume(ticket, now.Add(61*time.Second)); ok {
		t.Fatal("протухший билет должен быть отвергнут")
	}
}

func TestTicketStore_ConsumeUnknown(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(time.Minute)
	if _, ok := s.Consume("nope", time.Unix(1_700_000_000, 0)); ok {
		t.Fatal("несуществующий билет должен быть отвергнут")
	}
}

func TestTicketStore_IssueGCsExpired(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(60 * time.Second)
	now := time.Unix(1_700_000_000, 0)

	stale, _ := s.Issue(ticketIdentity(1, now), now)
	// Новая выдача сильно позже должна вычистить протухший билет из хранилища.
	_, _ = s.Issue(ticketIdentity(2, now.Add(10*time.Minute)), now.Add(10*time.Minute))
	if _, ok := s.Consume(stale, now.Add(10*time.Minute)); ok {
		t.Fatal("протухший билет должен быть вычищен на следующей выдаче")
	}
}

func ticketIdentity(id int, now time.Time) models.SessionIdentity {
	return models.SessionIdentity{EffectiveUserID: id, OwnerUserID: id, FamilyID: "synthetic-family", AccessExpiresAt: now.Add(time.Hour)}
}

func TestTicketStore_AccessExpiryBoundary(t *testing.T) {
	now := time.Date(2032, 12, 31, 23, 59, 59, 0, time.UTC)
	identity := ticketIdentity(42, now)
	identity.AccessExpiresAt = now.Add(time.Second)
	store := NewTicketStore(time.Minute)
	ticket, err := store.Issue(identity, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Consume(ticket, identity.AccessExpiresAt); ok {
		t.Fatal("ticket outlived access expiry boundary")
	}
	if _, ok := store.Consume(ticket, now); ok {
		t.Fatal("expired redemption did not consume the ticket")
	}
}

func mustIssue(t *testing.T, s *TicketStore, identity models.SessionIdentity, now time.Time) string {
	t.Helper()
	ticket, err := s.Issue(identity, now)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return ticket
}

func TestTicketStore_OwnerAndSystemLimits(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStoreWithLimits(time.Minute, 2, 3)
	defer s.Close()
	identity := ticketIdentity(1, now)
	identity.EffectiveUserID = 11
	a := mustIssue(t, s, identity, now)
	identity.EffectiveUserID = 12
	b := mustIssue(t, s, identity, now)
	identity.EffectiveUserID = 13
	if ticket, err := s.Issue(identity, now); !errors.Is(err, ErrOwnerLimit) || ticket != "" {
		t.Fatalf("impersonation bypassed owner cap: %v", err)
	}
	c := mustIssue(t, s, ticketIdentity(2, now), now)
	if ticket, err := s.Issue(ticketIdentity(3, now), now); !errors.Is(err, ErrSystemLimit) || ticket != "" {
		t.Fatalf("system cap: %v", err)
	}
	if got, ok := s.Consume(a, now); !ok || got.OwnerUserID != 1 || got.EffectiveUserID != 11 {
		t.Fatal("admission changed stored identity")
	}
	if _, ok := s.Consume(a, now); ok {
		t.Fatal("double consume restored quota twice")
	}
	d := mustIssue(t, s, identity, now)
	for _, ticket := range []string{b, c, d} {
		if _, ok := s.Consume(ticket, now); !ok {
			t.Fatal("new admission evicted live ticket")
		}
	}
	if len(s.tickets) != 0 || len(s.owners) != 0 {
		t.Fatal("consume retained owner counters")
	}
}

func TestTicketStore_CleanupExactExpiryAndClose(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStoreWithLimits(time.Minute, 1, 2)
	a := mustIssue(t, s, ticketIdentity(1, now), now)
	identity := ticketIdentity(2, now)
	identity.AccessExpiresAt = now.Add(2 * time.Minute)
	b := mustIssue(t, s, identity, now.Add(time.Second))
	s.Cleanup(now.Add(time.Minute - time.Nanosecond))
	if len(s.tickets) != 2 || len(s.owners) != 2 {
		t.Fatal("cleanup removed live ticket")
	}
	s.Cleanup(now.Add(time.Minute))
	s.Cleanup(now.Add(time.Minute))
	if len(s.tickets) != 1 || len(s.owners) != 1 || s.owners[2] != 1 {
		t.Fatal("cleanup failed exact boundary accounting")
	}
	if _, ok := s.Consume(a, now); ok {
		t.Fatal("expired ticket survived cleanup")
	}
	mustIssue(t, s, ticketIdentity(1, now), now.Add(time.Minute))
	s.Close()
	s.Close()
	s.Cleanup(now.Add(time.Hour))
	if _, ok := s.Consume(b, now); ok {
		t.Fatal("closed store redeemed ticket")
	}
	if ticket, err := s.Issue(identity, now); !errors.Is(err, ErrClosed) || ticket != "" {
		t.Fatalf("issue after close: %v", err)
	}
	if len(s.tickets) != 0 || len(s.owners) != 0 {
		t.Fatal("closed store retained entries")
	}
}

func TestTicketStore_ExpiryFreesCapacityBeforeIssue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStoreWithLimits(time.Minute, 1, 1)
	a := mustIssue(t, s, ticketIdentity(1, now), now)
	b := mustIssue(t, s, ticketIdentity(2, now), now.Add(time.Minute))
	if _, ok := s.Consume(a, now); ok {
		t.Fatal("full-store GC retained expired ticket")
	}
	if _, ok := s.Consume(b, now.Add(time.Minute)); !ok {
		t.Fatal("capacity not returned by GC")
	}
	if len(s.owners) != 0 {
		t.Fatal("GC retained old owner bucket")
	}
}

func TestTicketStore_ConsumeAtTTLBoundaryFreesOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStoreWithLimits(time.Minute, 1, 1)
	ticket := mustIssue(t, s, ticketIdentity(1, now), now)
	if _, ok := s.Consume(ticket, now.Add(time.Minute)); ok {
		t.Fatal("ticket valid at exact TTL boundary")
	}
	if len(s.tickets) != 0 || len(s.owners) != 0 {
		t.Fatal("expired consume retained quota")
	}
	mustIssue(t, s, ticketIdentity(1, now), now.Add(time.Minute))
}

func TestTicketStore_InvalidIdentitiesDoNotAllocate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStore(time.Minute)
	for _, ids := range [][2]int{{0, 1}, {-1, 1}, {1, 0}, {1, -1}} {
		identity := ticketIdentity(ids[0], now)
		identity.EffectiveUserID = ids[1]
		if ticket, err := s.Issue(identity, now); !errors.Is(err, ErrInvalidIdentity) || ticket != "" {
			t.Fatalf("invalid identity %v: %v", ids, err)
		}
	}
	identity := ticketIdentity(1, now)
	identity.FamilyID = ""
	if _, err := s.Issue(identity, now); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("missing family: %v", err)
	}
	identity = ticketIdentity(1, now)
	identity.AccessExpiresAt = now
	if _, err := s.Issue(identity, now); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expired access: %v", err)
	}
	if len(s.tickets) != 0 || len(s.owners) != 0 {
		t.Fatal("invalid identity allocated quota")
	}
}

func TestTicketStore_CopiesImpersonator(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStore(time.Minute)
	identity := ticketIdentity(1, now)
	actor := 42
	identity.ImpersonatedBy = &actor
	ticket := mustIssue(t, s, identity, now)
	actor = 99
	got, ok := s.Consume(ticket, now)
	if !ok || got.ImpersonatedBy == nil || *got.ImpersonatedBy != 42 || got.ImpersonatedBy == identity.ImpersonatedBy {
		t.Fatal("ticket retained caller impersonator pointer")
	}
}

func TestTicketStore_ConcurrentIssueConsumeCleanupClose(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStoreWithLimits(time.Minute, 2, 5)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(owner int) {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				ticket, err := s.Issue(ticketIdentity(owner, now), now)
				if err != nil {
					if !errors.Is(err, ErrOwnerLimit) && !errors.Is(err, ErrSystemLimit) && !errors.Is(err, ErrClosed) {
						t.Errorf("unexpected admission: %v", err)
					}
					continue
				}
				s.mu.Lock()
				if len(s.tickets) > 5 || len(s.owners) > len(s.tickets) {
					t.Error("ticket store exceeded bounded state")
				}
				for id, count := range s.owners {
					if id <= 0 || count <= 0 || count > 2 {
						t.Error("ticket store exceeded owner cap")
					}
				}
				s.mu.Unlock()
				s.Cleanup(now.Add(time.Minute))
				s.Consume(ticket, now)
				s.Consume(ticket, now)
			}
		}(i%8 + 1)
	}
	wg.Add(1)
	go func() { defer wg.Done(); <-start; s.Close() }()
	close(start)
	wg.Wait()
	s.Close()
	if len(s.tickets) != 0 || len(s.owners) != 0 {
		t.Fatal("concurrent close retained tickets")
	}
}

func TestTicketStore_DefaultLimits(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewTicketStore(time.Minute)
	defer s.Close()
	for owner := 1; owner <= 25; owner++ {
		for tab := 0; tab < 16; tab++ {
			mustIssue(t, s, ticketIdentity(owner, now), now)
		}
		if _, err := s.Issue(ticketIdentity(owner, now), now); !errors.Is(err, ErrOwnerLimit) {
			t.Fatalf("default owner limit: %v", err)
		}
	}
	if _, err := s.Issue(ticketIdentity(26, now), now); !errors.Is(err, ErrSystemLimit) {
		t.Fatalf("default system limit: %v", err)
	}
}

func TestTicketStore_ConcurrentAdmissionHasExactCaps(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		ownerLimit, systemLimit, owners, want int
	}{
		{name: "owner", ownerLimit: 2, systemLimit: 10, owners: 1, want: 2},
		{name: "system", ownerLimit: 2, systemLimit: 5, owners: 16, want: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_700_000_000, 0)
			s := NewTicketStoreWithLimits(time.Minute, tc.ownerLimit, tc.systemLimit)
			defer s.Close()
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := 0; i < 64; i++ {
				wg.Add(1)
				go func(owner int) {
					defer wg.Done()
					<-start
					_, err := s.Issue(ticketIdentity(owner, now), now)
					if err != nil && !errors.Is(err, ErrOwnerLimit) && !errors.Is(err, ErrSystemLimit) {
						t.Errorf("unexpected admission: %v", err)
					}
				}(i%tc.owners + 1)
			}
			close(start)
			wg.Wait()
			if len(s.tickets) != tc.want {
				t.Fatalf("accepted=%d want=%d", len(s.tickets), tc.want)
			}
			for owner, count := range s.owners {
				if owner <= 0 || count <= 0 || count > tc.ownerLimit {
					t.Fatal("owner quota violated")
				}
			}
			s.Cleanup(now.Add(time.Minute))
			if len(s.tickets) != 0 || len(s.owners) != 0 {
				t.Fatal("cleanup retained owner buckets")
			}
		})
	}
}
