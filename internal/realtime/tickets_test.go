package realtime

import (
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
