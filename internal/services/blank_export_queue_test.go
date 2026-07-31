package services

import (
	"testing"
	"time"
)

// Очередь файлового архива (#1615, B1) - чистая in-memory структура, без БД:
// табличные тесты по testing.md, не в internal/handlers.

func TestBlankExportQueue_PushDrain(t *testing.T) {
	q := newBlankExportQueue()

	if got := q.drain(); got != nil {
		t.Fatalf("drain() на пустой очереди = %v, хочу nil", got)
	}

	q.push(1, BlankExportReasonSubmit)
	q.push(2, BlankExportReasonUpdate)

	got := q.drain()
	want := map[int]string{1: BlankExportReasonSubmit, 2: BlankExportReasonUpdate}
	if len(got) != len(want) {
		t.Fatalf("drain() = %v, хочу %v", got, want)
	}
	for id, reason := range want {
		if got[id] != reason {
			t.Errorf("drain()[%d] = %q, хочу %q", id, got[id], reason)
		}
	}

	// Разбор очищает очередь - повторный drain пуст.
	if got := q.drain(); got != nil {
		t.Fatalf("drain() после разбора = %v, хочу nil", got)
	}
}

func TestBlankExportQueue_PushOverwritesReason(t *testing.T) {
	q := newBlankExportQueue()
	q.push(1, BlankExportReasonSubmit)
	q.push(1, BlankExportReasonUpdate)

	got := q.drain()
	if got[1] != BlankExportReasonUpdate {
		t.Fatalf("повторный push не перебил причину: got %q, хочу %q", got[1], BlankExportReasonUpdate)
	}
	if len(got) != 1 {
		t.Fatalf("повторный push той же заявки создал лишнюю запись: %v", got)
	}
}

func TestBlankExportQueue_PushWakes(t *testing.T) {
	q := newBlankExportQueue()
	q.push(1, BlankExportReasonSubmit)

	select {
	case <-q.wake:
	case <-time.After(time.Second):
		t.Fatal("push не разбудил канал wake")
	}
}

func TestBlankExportQueue_NudgeNonBlocking(t *testing.T) {
	q := newBlankExportQueue()
	// Канал ёмкостью 1: второй nudge подряд не должен блокироваться и не должен
	// паниковать, даже если никто не читает из wake.
	q.nudge()
	done := make(chan struct{})
	go func() {
		q.nudge()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("nudge() заблокировался при полном канале")
	}
}

// EnqueueApplication/EnqueueApplications/Nudge обязаны быть no-op на nil-получателе:
// точки изменения заявки не должны знать, поднялся ли писатель архива.
func TestBlankExportService_NilReceiverSafe(t *testing.T) {
	var svc *BlankExportService

	svc.EnqueueApplication(1, BlankExportReasonSubmit)
	svc.EnqueueApplications([]int{1, 2, 3}, BlankExportReasonUpdate)
	svc.Nudge()
}
