package realtime

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func recv(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("канал закрыт, событие не получено")
		}
		return ev
	case <-time.After(time.Second):
		t.Fatal("таймаут ожидания события")
		return Event{}
	}
}

func expectNoEvent(t *testing.T, ch <-chan Event) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("не ожидали события, получили %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHub_PublishReachesSubscriber(t *testing.T) {
	t.Parallel()
	h := NewHub()
	ch, unsub := h.Subscribe(1)
	defer unsub()

	h.Publish(1, Event{Type: "available.new", Scope: "available-attachments", Count: 3})

	ev := recv(t, ch)
	if ev.Type != "available.new" || ev.Scope != "available-attachments" || ev.Count != 3 {
		t.Fatalf("неожиданное событие: %+v", ev)
	}
}

func TestHub_IsolatesUsers(t *testing.T) {
	t.Parallel()
	h := NewHub()
	chA, unsubA := h.Subscribe(1)
	defer unsubA()
	chB, unsubB := h.Subscribe(2)
	defer unsubB()

	h.Publish(1, Event{Type: "x"})

	if ev := recv(t, chA); ev.Type != "x" {
		t.Fatalf("адресат A не получил своё событие: %+v", ev)
	}
	expectNoEvent(t, chB) // чужое событие не должно долететь до B
}

func TestHub_MultipleTabsSameUser(t *testing.T) {
	t.Parallel()
	h := NewHub()
	ch1, u1 := h.Subscribe(1)
	defer u1()
	ch2, u2 := h.Subscribe(1)
	defer u2()

	h.Publish(1, Event{Type: "x"})

	if ev := recv(t, ch1); ev.Type != "x" {
		t.Fatal("вкладка 1 не получила событие")
	}
	if ev := recv(t, ch2); ev.Type != "x" {
		t.Fatal("вкладка 2 не получила событие")
	}
}

func TestHub_PublishManyFansOut(t *testing.T) {
	t.Parallel()
	h := NewHub()
	chA, ua := h.Subscribe(1)
	defer ua()
	chB, ub := h.Subscribe(2)
	defer ub()
	_, uc := h.Subscribe(3)
	defer uc()

	h.PublishMany([]int{1, 2}, Event{Type: "refresh"})

	if ev := recv(t, chA); ev.Type != "refresh" {
		t.Fatal("A не получил fan-out")
	}
	if ev := recv(t, chB); ev.Type != "refresh" {
		t.Fatal("B не получил fan-out")
	}
}

func TestHub_PublishToEachConnectedPerUserEvent(t *testing.T) {
	t.Parallel()
	h := NewHub()
	ch1, u1 := h.Subscribe(1)
	defer u1()
	ch2, u2 := h.Subscribe(2)
	defer u2()

	// Каждый подключённый получает событие, построенное по ЕГО userID (per-user scope).
	h.PublishToEachConnected(func(uid int) Event {
		return Event{Type: "user.permissions", Scope: fmt.Sprintf("user:%d", uid)}
	})

	if ev := recv(t, ch1); ev.Scope != "user:1" {
		t.Fatalf("юзер 1 должен получить свой scope, got %q", ev.Scope)
	}
	if ev := recv(t, ch2); ev.Scope != "user:2" {
		t.Fatalf("юзер 2 должен получить свой scope, got %q", ev.Scope)
	}
}

func TestHub_PublishToEachConnectedNoSubscribers(t *testing.T) {
	t.Parallel()
	h := NewHub()
	// Без подписчиков метод не паникует и ничего не шлёт (no-op).
	h.PublishToEachConnected(func(int) Event { return Event{Type: "x"} })
}

func TestHub_UnsubscribeClosesAndStops(t *testing.T) {
	t.Parallel()
	h := NewHub()
	ch, unsub := h.Subscribe(1)

	unsub()
	if _, ok := <-ch; ok {
		t.Fatal("канал должен быть закрыт после отписки")
	}

	h.Publish(1, Event{Type: "x"}) // publish после отписки не паникует
	unsub()                        // повторная отписка идемпотентна
}

func TestHub_SlowSubscriberDoesNotBlockPublish(t *testing.T) {
	t.Parallel()
	h := NewHub()
	_, unsub := h.Subscribe(1) // намеренно не читаем канал
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.Publish(1, Event{Type: "flood"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish заблокировался на медленном подписчике (нет drop-on-slow)")
	}
}

func TestHub_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	h := NewHub()
	var wg sync.WaitGroup

	for u := 0; u < 20; u++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ch, unsub := h.Subscribe(uid)
				h.Publish(uid, Event{Type: "x"})
				select {
				case <-ch:
				default:
				}
				unsub()
			}
		}(u)
	}
	for p := 0; p < 10; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				h.Publish(i%20, Event{Type: "y"})
			}
		}()
	}
	wg.Wait()
}
