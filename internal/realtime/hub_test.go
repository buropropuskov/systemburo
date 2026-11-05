package realtime

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func mustSubscribe(t *testing.T, h *Hub, ownerID, userID int) (<-chan Event, func()) {
	t.Helper()
	ch, release, err := h.Subscribe(ownerID, userID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	return ch, release
}

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
	ch, unsub := mustSubscribe(t, h, 1, 1)
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
	chA, unsubA := mustSubscribe(t, h, 1, 1)
	defer unsubA()
	chB, unsubB := mustSubscribe(t, h, 2, 2)
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
	ch1, u1 := mustSubscribe(t, h, 1, 1)
	defer u1()
	ch2, u2 := mustSubscribe(t, h, 1, 1)
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
	chA, ua := mustSubscribe(t, h, 1, 1)
	defer ua()
	chB, ub := mustSubscribe(t, h, 2, 2)
	defer ub()
	_, uc := mustSubscribe(t, h, 3, 3)
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
	ch1, u1 := mustSubscribe(t, h, 1, 1)
	defer u1()
	ch2, u2 := mustSubscribe(t, h, 2, 2)
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
	ch, unsub := mustSubscribe(t, h, 1, 1)

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
	_, unsub := mustSubscribe(t, h, 1, 1) // намеренно не читаем канал
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

	for u := 1; u <= 20; u++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ch, unsub := mustSubscribe(t, h, uid, uid)
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
				h.Publish(i%20+1, Event{Type: "y"})
			}
		}()
	}
	wg.Wait()
}

func TestHub_AdmissionByOwnerDeliveryByEffectiveUser(t *testing.T) {
	h := NewHubWithLimits(2, 3)
	defer h.Close()
	chA, releaseA := mustSubscribe(t, h, 1, 11)
	chB, releaseB := mustSubscribe(t, h, 1, 12)
	defer releaseB()
	if ch, release, err := h.Subscribe(1, 13); !errors.Is(err, ErrOwnerLimit) || ch != nil || release != nil {
		t.Fatalf("owner limit did not reject before allocation: %v", err)
	}
	chC, releaseC := mustSubscribe(t, h, 2, 11)
	defer releaseC()
	if _, _, err := h.Subscribe(3, 13); !errors.Is(err, ErrSystemLimit) {
		t.Fatalf("system limit: %v", err)
	}
	h.Publish(11, Event{Type: "effective"})
	if recv(t, chA).Type != "effective" || recv(t, chC).Type != "effective" {
		t.Fatal("effective recipient did not receive event")
	}
	select {
	case ev := <-chB:
		t.Fatalf("other effective user received event: %+v", ev)
	default:
	}
	releaseA()
	releaseA()
	_, release := mustSubscribe(t, h, 1, 13)
	release()
	if h.total != 2 || h.owners[1] != 1 || h.owners[2] != 1 || len(h.subs) != 2 {
		t.Fatal("release did not restore exact quota")
	}
}

func TestHub_InvalidIdentityAndClose(t *testing.T) {
	h := NewHubWithLimits(1, 2)
	for _, ids := range [][2]int{{0, 1}, {-1, 1}, {1, 0}, {1, -1}} {
		if _, _, err := h.Subscribe(ids[0], ids[1]); !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("identity %v: %v", ids, err)
		}
	}
	if len(h.subs) != 0 || len(h.owners) != 0 || h.total != 0 {
		t.Fatal("invalid identities allocated quota")
	}
	ch, release := mustSubscribe(t, h, 1, 2)
	h.Close()
	h.Close()
	release()
	release()
	h.Publish(2, Event{Type: "after-close"})
	h.PublishToEachConnected(func(int) Event { t.Fatal("closed hub retained user"); return Event{} })
	if _, ok := <-ch; ok {
		t.Fatal("close did not close channel")
	}
	if _, _, err := h.Subscribe(1, 2); !errors.Is(err, ErrClosed) {
		t.Fatalf("subscribe after close: %v", err)
	}
	if h.total != 0 || len(h.owners) != 0 || len(h.subs) != 0 {
		t.Fatal("close retained counters")
	}
}

func TestHub_ConcurrentAdmissionReleasePublishClose(t *testing.T) {
	h := NewHubWithLimits(2, 5)
	_, seededRelease := mustSubscribe(t, h, 99, 42)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(owner int) {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				_, release, err := h.Subscribe(owner, 42)
				if err != nil {
					if !errors.Is(err, ErrOwnerLimit) && !errors.Is(err, ErrSystemLimit) && !errors.Is(err, ErrClosed) {
						t.Errorf("unexpected admission: %v", err)
					}
					continue
				}
				h.mu.RLock()
				if h.total > 5 || len(h.owners) > h.total || len(h.subs) > h.total {
					t.Error("hub exceeded bounded aggregate state")
				}
				for id, count := range h.owners {
					if id <= 0 || count <= 0 || count > 2 {
						t.Error("hub exceeded owner quota")
					}
				}
				h.mu.RUnlock()
				h.Publish(42, Event{Type: "race"})
				release()
				release()
			}
		}(i%8 + 1)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for j := 0; j < 200; j++ {
			h.Publish(42, Event{Type: "concurrent-publish"})
			seededRelease()
		}
	}()
	wg.Add(1)
	go func() { defer wg.Done(); <-start; h.Close() }()
	close(start)
	wg.Wait()
	h.Close()
	if h.total != 0 || len(h.owners) != 0 || len(h.subs) != 0 {
		t.Fatal("concurrent close retained subscriptions")
	}
}

func TestHub_DefaultLimits(t *testing.T) {
	h := NewHub()
	defer h.Close()
	for owner := 1; owner <= 25; owner++ {
		for tab := 0; tab < 8; tab++ {
			mustSubscribe(t, h, owner, owner)
		}
		if _, _, err := h.Subscribe(owner, 999); !errors.Is(err, ErrOwnerLimit) {
			t.Fatalf("default owner limit: %v", err)
		}
	}
	if _, _, err := h.Subscribe(26, 26); !errors.Is(err, ErrSystemLimit) {
		t.Fatalf("default system limit: %v", err)
	}
}

func TestHub_ConcurrentAdmissionHasExactCaps(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		ownerLimit, systemLimit, owners, want int
	}{
		{name: "owner", ownerLimit: 2, systemLimit: 10, owners: 1, want: 2},
		{name: "system", ownerLimit: 2, systemLimit: 5, owners: 16, want: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHubWithLimits(tc.ownerLimit, tc.systemLimit)
			defer h.Close()
			start, releaseAll := make(chan struct{}), make(chan struct{})
			results := make(chan bool, 64)
			var wg sync.WaitGroup
			for i := 0; i < 64; i++ {
				wg.Add(1)
				go func(owner int) {
					defer wg.Done()
					<-start
					_, release, err := h.Subscribe(owner, owner+100)
					results <- err == nil
					if err == nil {
						<-releaseAll
						release()
						release()
					} else if !errors.Is(err, ErrOwnerLimit) && !errors.Is(err, ErrSystemLimit) {
						t.Errorf("unexpected admission: %v", err)
					}
				}(i%tc.owners + 1)
			}
			close(start)
			accepted := 0
			for i := 0; i < 64; i++ {
				if <-results {
					accepted++
				}
			}
			close(releaseAll)
			wg.Wait()
			if accepted != tc.want || h.total != 0 || len(h.owners) != 0 || len(h.subs) != 0 {
				t.Fatalf("accepted=%d want=%d, retained=%d", accepted, tc.want, h.total)
			}
		})
	}
}

func TestHub_ConcurrentReleaseSameSubscription(t *testing.T) {
	h := NewHub()
	ch, release := mustSubscribe(t, h, 1, 2)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); release() }()
	}
	wg.Wait()
	if _, ok := <-ch; ok {
		t.Fatal("concurrent release did not close channel")
	}
	if h.total != 0 || len(h.owners) != 0 || len(h.subs) != 0 {
		t.Fatal("concurrent release retained quota")
	}
}
