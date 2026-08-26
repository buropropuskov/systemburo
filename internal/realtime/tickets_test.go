package realtime

import (
	"testing"
	"time"
)

func TestTicketStore_IssueThenConsume(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(time.Minute)
	now := time.Unix(1_700_000_000, 0)

	ticket, err := s.Issue(42, now)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if ticket == "" {
		t.Fatal("пустой билет")
	}

	uid, ok := s.Consume(ticket, now)
	if !ok || uid != 42 {
		t.Fatalf("ожидали userID=42 ok=true, получили uid=%d ok=%v", uid, ok)
	}
}

func TestTicketStore_ConsumeIsOneTime(t *testing.T) {
	t.Parallel()
	s := NewTicketStore(time.Minute)
	now := time.Unix(1_700_000_000, 0)

	ticket, _ := s.Issue(1, now)
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

	ticket, _ := s.Issue(1, now)
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

	stale, _ := s.Issue(1, now)
	// Новая выдача сильно позже должна вычистить протухший билет из хранилища.
	_, _ = s.Issue(2, now.Add(10*time.Minute))
	if _, ok := s.Consume(stale, now.Add(10*time.Minute)); ok {
		t.Fatal("протухший билет должен быть вычищен на следующей выдаче")
	}
}
