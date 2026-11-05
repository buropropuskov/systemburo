package realtime

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"systemburo/internal/models"
	"time"
)

type ticketEntry struct {
	identity  models.SessionIdentity
	expiresAt time.Time
}

// TicketStore выдаёт короткоживущие одноразовые билеты для установления SSE-соединения.
// EventSource не умеет слать заголовок Authorization, а класть access-токен в query
// нельзя - он утечёт в журналы (request_logs, stdout, nginx access-log). Вместо этого
// фронт получает билет обычным защищённым запросом (Bearer -> JWTAuth+banCheck) и
// открывает поток уже с ним. Билет одноразовый и живёт секунды, поэтому его попадание
// в журналы безвредно. In-process, на один инстанс backend.
type TicketStore struct {
	mu          sync.Mutex
	tickets     map[string]ticketEntry
	ttl         time.Duration
	owners      map[int]int
	ownerLimit  int
	systemLimit int
	closed      bool
}

// NewTicketStore создаёт хранилище билетов с временем жизни ttl.
func NewTicketStore(ttl time.Duration) *TicketStore {
	return NewTicketStoreWithLimits(ttl, defaultOwnerTickets, defaultSystemTickets)
}

// NewTicketStoreWithLimits задаёт process-local пределы. Неположительный предел
// запрещает соответствующий допуск, не делает хранилище неограниченным.
func NewTicketStoreWithLimits(ttl time.Duration, owner, system int) *TicketStore {
	return &TicketStore{tickets: make(map[string]ticketEntry), ttl: ttl, owners: make(map[int]int), ownerLimit: owner, systemLimit: system}
}

// Issue создаёт новый одноразовый билет для userID и попутно чистит протухшие.
// now инъектируется для детерминизма в тестах; в проде - time.Now().
func (s *TicketStore) Issue(identity models.SessionIdentity, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", ErrClosed
	}
	if identity.OwnerUserID <= 0 || identity.EffectiveUserID <= 0 || identity.FamilyID == "" || !now.Before(identity.AccessExpiresAt) {
		return "", ErrInvalidIdentity
	}
	s.gcLocked(now)
	if s.owners[identity.OwnerUserID] >= s.ownerLimit {
		return "", ErrOwnerLimit
	}
	if len(s.tickets) >= s.systemLimit {
		return "", ErrSystemLimit
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	// Даже случайная коллизия не должна заменять живой билет.
	if _, exists := s.tickets[ticket]; exists {
		return "", fmt.Errorf("ticket collision")
	}
	expiresAt := now.Add(s.ttl)
	if identity.AccessExpiresAt.Before(expiresAt) {
		expiresAt = identity.AccessExpiresAt
	}
	if identity.ImpersonatedBy != nil {
		actor := *identity.ImpersonatedBy
		identity.ImpersonatedBy = &actor
	}
	s.tickets[ticket] = ticketEntry{identity: identity, expiresAt: expiresAt}
	s.owners[identity.OwnerUserID]++
	return ticket, nil
}

// Consume проверяет билет и сразу удаляет его (одноразовость). Возвращает userID и
// true только если билет существовал и не протух.
func (s *TicketStore) Consume(ticket string, now time.Time) (models.SessionIdentity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tickets[ticket]
	if !ok {
		return models.SessionIdentity{}, false
	}
	s.deleteLocked(ticket, e)
	if !now.Before(e.expiresAt) {
		return models.SessionIdentity{}, false
	}
	return e.identity, true
}

func (s *TicketStore) gcLocked(now time.Time) {
	for k, e := range s.tickets {
		if !now.Before(e.expiresAt) {
			s.deleteLocked(k, e)
		}
	}
}

func (s *TicketStore) deleteLocked(ticket string, e ticketEntry) {
	delete(s.tickets, ticket)
	owner := e.identity.OwnerUserID
	s.owners[owner]--
	if s.owners[owner] == 0 {
		delete(s.owners, owner)
	}
}

// Cleanup освобождает истёкшие билеты без фоновых goroutine на запись.
func (s *TicketStore) Cleanup(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked(now)
}

// Close идемпотентно запрещает выдачу и удаляет все непогашенные билеты.
func (s *TicketStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	clear(s.tickets)
	clear(s.owners)
}
