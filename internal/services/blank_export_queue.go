package services

import "sync"

// BlankExportEnqueuer - минимальный контракт постановки заявки в очередь на
// выгрузку в файловый архив (#1615, B1). Точки изменения заявки (подача, правка,
// согласование, назначение, bulk-операции, разбор справочника) зависят от этого
// интерфейса, а не от *BlankExportService целиком - иначе каждая такая точка
// тянула бы за собой генератор бланков, писателя и настройки, которые ей не нужны.
type BlankExportEnqueuer interface {
	EnqueueApplication(applicationID int, reason string)
	EnqueueApplications(applicationIDs []int, reason string)
}

// blankExportQueue - набор заявок в процессе, ожидающих выгрузки. In-memory и
// намеренно best-effort: постановка идёт ПОСЛЕ commit транзакции, чтобы полный
// диск или сбой архива не мог уронить подачу заявки (Outbox внутри той же
// транзакции отравил бы её целиком одной ошибкой записи). Обратная сторона -
// перезапуск процесса между enqueue и разбором теряет содержимое очереди;
// потерю новых заявок лечит ночная сверка (Recheck), потерю уже известных
// неудач - подметатель повторов (Sweep, по next_attempt_at из реестра).
type blankExportQueue struct {
	mu      sync.Mutex
	pending map[int]string
	wake    chan struct{}
}

// newBlankExportQueue создаёт пустую очередь с каналом пробуждения воркера
// ёмкостью 1: несколько enqueue подряд между тиками воркера схлопываются в одно
// пробуждение, воркеру всё равно предстоит разобрать всю карту разом.
func newBlankExportQueue() *blankExportQueue {
	return &blankExportQueue{pending: make(map[int]string), wake: make(chan struct{}, 1)}
}

// push добавляет заявку в очередь и будит воркер. Повторная постановка одной и
// той же заявки до разбора не теряется - позднейшая причина побеждает раннюю,
// поскольку интересен актуальный повод («правка» важнее «подачи», если заявку
// успели поправить до первого прогона).
func (q *blankExportQueue) push(applicationID int, reason string) {
	q.mu.Lock()
	q.pending[applicationID] = reason
	q.mu.Unlock()
	q.nudge()
}

// nudge будит воркер немедленно, не дожидаясь ближайшего тика. Неблокирующий:
// уже взведённый сигнал не переполняет канал, воркер и так разберёт всё, что
// накопилось к моменту пробуждения.
func (q *blankExportQueue) nudge() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// empty сообщает, пуста ли очередь, ничего из неё не забирая. Нужен воркеру, чтобы
// не платить за проверку порогов места на каждом тике, когда разбирать нечего.
func (q *blankExportQueue) empty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending) == 0
}

// drain забирает всё накопленное и опустошает очередь. Возвращает nil, если
// пусто, - вызывающему не нужно отличать nil от пустой карты.
func (q *blankExportQueue) drain() map[int]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return nil
	}
	out := q.pending
	q.pending = make(map[int]string)
	return out
}

// EnqueueApplication ставит заявку в очередь на выгрузку в файловый архив.
// Безопасен на nil-получателе (архив не поднят - каталог не настроен): точки
// изменения заявки не обязаны знать, поднят ли писатель, чтобы вызвать метод.
func (s *BlankExportService) EnqueueApplication(applicationID int, reason string) {
	if s == nil {
		return
	}
	s.queue.push(applicationID, reason)
}

// EnqueueApplications ставит в очередь набор заявок одной причиной - для
// bulk-операций и разбора справочника, затрагивающих сразу несколько заявок.
func (s *BlankExportService) EnqueueApplications(applicationIDs []int, reason string) {
	if s == nil {
		return
	}
	for _, id := range applicationIDs {
		s.queue.push(id, reason)
	}
}

// Nudge будит фоновый воркер вне обычного тика - используется, когда рубильник
// архива включили и накопившуюся очередь стоит разобрать сразу, не дожидаясь
// ArchiveWorkerTick.
func (s *BlankExportService) Nudge() {
	if s == nil {
		return
	}
	s.queue.nudge()
}
