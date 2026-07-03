package realtime

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

type ticketEntry struct {
	userID    int
	expiresAt time.Time
}

// TicketStore выдаёт короткоживущие одноразовые билеты для установления SSE-соединения.
// EventSource не умеет слать заголовок Authorization, а класть access-токен в query
// нельзя - он утечёт в журналы (request_logs, stdout, nginx access-log). Вместо этого
// фронт получает билет обычным защищённым запросом (Bearer -> JWTAuth+banCheck) и
// открывает поток уже с ним. Билет одноразовый и живёт секунды, поэтому его попадание
// в журналы безвредно. In-process, на один инстанс backend.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketEntry
	ttl     time.Duration
}

// NewTicketStore создаёт хранилище билетов с временем жизни ttl.
func NewTicketStore(ttl time.Duration) *TicketStore {
	return &TicketStore{tickets: make(map[string]ticketEntry), ttl: ttl}
}

// Issue создаёт новый одноразовый билет для userID и попутно чистит протухшие.
// now инъектируется для детерминизма в тестах; в проде - time.Now().
func (s *TicketStore) Issue(userID int, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	s.gcLocked(now)
	s.tickets[ticket] = ticketEntry{userID: userID, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()
	return ticket, nil
}

// Consume проверяет билет и сразу удаляет его (одноразовость). Возвращает userID и
// true только если билет существовал и не протух.
func (s *TicketStore) Consume(ticket string, now time.Time) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tickets[ticket]
	if !ok {
		return 0, false
	}
	delete(s.tickets, ticket)
	if now.After(e.expiresAt) {
		return 0, false
	}
	return e.userID, true
}

func (s *TicketStore) gcLocked(now time.Time) {
	for k, e := range s.tickets {
		if now.After(e.expiresAt) {
			delete(s.tickets, k)
		}
	}
}
