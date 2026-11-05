// Package realtime реализует in-process pub/sub для доставки лёгких real-time
// сигналов подключённым пользователям через SSE (issue #840). Событие - только
// сигнал "сходи обнови", без данных сущности: получив его, клиент делает обычный
// запрос, который сам применяет права (паттерн event-then-fetch).
package realtime

import "sync"

// Event - лёгкий сигнал real-time обновления. Полезные данные сущности в стрим
// не кладём (безопасность + объём) - только тип и область, которую обновить.
type Event struct {
	Type  string `json:"type"`
	Scope string `json:"scope,omitempty"`
	Count int    `json:"count,omitempty"`
}

// Publisher - контракт адресной публикации real-time событий. Реализуется *Hub;
// доменные сервисы принимают его, а не конкретный хаб (accept interfaces).
type Publisher interface {
	Publish(userID int, ev Event)
	PublishMany(userIDs []int, ev Event)
}

// subscriberBuffer - глубина буфера на подписчика. При переполнении (медленный
// клиент) сигнал дропается: при следующем событии или реконнекте клиент всё равно
// перезапросит данные, поэтому потеря отдельного сигнала не критична.
const subscriberBuffer = 16

type subscriber struct {
	ch chan Event
}

// Hub хранит подписки per-user и адресно доставляет события. Безопасен для
// конкурентного использования. In-process, на один инстанс backend; точка
// расширения на несколько инстансов (LISTEN/NOTIFY или брокер) - за интерфейсом
// Publish, менять только реализацию.
type Hub struct {
	mu          sync.RWMutex
	subs        map[int]map[*subscriber]struct{}
	owners      map[int]int
	total       int
	ownerLimit  int
	systemLimit int
	closed      bool
}

// NewHub создаёт пустой хаб.
func NewHub() *Hub {
	return NewHubWithLimits(defaultOwnerSubscriptions, defaultSystemSubscriptions)
}

// NewHubWithLimits задаёт положительные process-local пределы. Неположительные
// значения запрещают соответствующий допуск, не отключают ограничение.
func NewHubWithLimits(owner, system int) *Hub {
	return &Hub{subs: make(map[int]map[*subscriber]struct{}), owners: make(map[int]int), ownerLimit: owner, systemLimit: system}
}

// Subscribe учитывает квоту настоящего владельца ownerID, доставляя события по
// effective userID. Отказ происходит до выделения канала. Release идемпотентен.
func (h *Hub) Subscribe(ownerID, userID int) (<-chan Event, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, nil, ErrClosed
	}
	if ownerID <= 0 || userID <= 0 {
		return nil, nil, ErrInvalidIdentity
	}
	if h.owners[ownerID] >= h.ownerLimit {
		return nil, nil, ErrOwnerLimit
	}
	if h.total >= h.systemLimit {
		return nil, nil, ErrSystemLimit
	}
	sub := &subscriber{ch: make(chan Event, subscriberBuffer)}
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[*subscriber]struct{})
	}
	h.subs[userID][sub] = struct{}{}
	h.owners[ownerID]++
	h.total++

	unsub := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if set := h.subs[userID]; set != nil {
			if _, exists := set[sub]; exists {
				delete(set, sub)
				if len(set) == 0 {
					delete(h.subs, userID)
				}
				h.total--
				h.owners[ownerID]--
				if h.owners[ownerID] == 0 {
					delete(h.owners, ownerID)
				}
				close(sub.ch)
			}
		}
	}
	return sub.ch, unsub, nil
}

// Close запрещает новые подписки и закрывает существующие под publish-lock.
// Buffered events остаются в канале; lifecycle потребителя отменяется отдельно.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, set := range h.subs {
		for sub := range set {
			close(sub.ch)
		}
	}
	clear(h.subs)
	clear(h.owners)
	h.total = 0
}

// Publish адресно шлёт событие всем подпискам userID. Доставка неблокирующая:
// полный буфер подписчика (медленный клиент) не тормозит паблиш - сигнал для него
// дропается (см. subscriberBuffer). Отписка держит write-lock, поэтому close
// канала не пересекается с отправкой под read-lock.
func (h *Hub) Publish(userID int, ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subs[userID] {
		select {
		case sub.ch <- ev:
		default:
		}
	}
}

// PublishMany шлёт событие набору пользователей - аудитории события, вычисленной
// по той же логике доступа, что фильтрует соответствующий список.
func (h *Hub) PublishMany(userIDs []int, ev Event) {
	for _, uid := range userIDs {
		h.Publish(uid, ev)
	}
}

// PublishToEachConnected шлёт каждому подключённому пользователю событие,
// построенное mk(userID). Нужно для broadcast с per-user scope: смена грантов
// роли/группы затрагивает произвольный набор носителей, поэтому шлём всем онлайн
// сигнал перезапросить своё (у каждого свой scope user:<id>). Снимок подключённых
// берём под RLock, публикуем вне лока (Publish берёт RLock сам).
func (h *Hub) PublishToEachConnected(mk func(userID int) Event) {
	h.mu.RLock()
	ids := make([]int, 0, len(h.subs))
	for uid := range h.subs {
		ids = append(ids, uid)
	}
	h.mu.RUnlock()
	for _, uid := range ids {
		h.Publish(uid, mk(uid))
	}
}
