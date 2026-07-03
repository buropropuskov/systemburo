package handlers

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
)

func newEventsHandler() (*EventsHandler, *realtime.Hub, *realtime.TicketStore) {
	hub := realtime.NewHub()
	tickets := realtime.NewTicketStore(time.Minute)
	return NewEventsHandler(hub, tickets), hub, tickets
}

func newEventsServer(t *testing.T, h *EventsHandler) *httptest.Server {
	t.Helper()
	e := echo.New()
	e.GET("/api/events", h.Stream)
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return srv
}

// readFrame читает из потока до пустой строки-разделителя SSE-кадра (или таймаута).
func readFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	type res struct {
		s   string
		err error
	}
	out := make(chan res, 1)
	go func() {
		var b strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				out <- res{b.String(), err}
				return
			}
			if line == "\n" || line == "\r\n" {
				out <- res{b.String(), nil}
				return
			}
			b.WriteString(line)
		}
	}()
	select {
	case rr := <-out:
		if rr.err != nil {
			t.Fatalf("чтение кадра: %v", rr.err)
		}
		return rr.s
	case <-time.After(2 * time.Second):
		t.Fatal("таймаут ожидания SSE-кадра")
		return ""
	}
}

func TestEventsIssueTicket_ReturnsTicketForAuthedUser(t *testing.T) {
	t.Parallel()
	h, _, tickets := newEventsHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/events/ticket", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", 42)

	if err := h.IssueTicket(c); err != nil {
		t.Fatalf("IssueTicket: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ticket"`) {
		t.Fatalf("в ответе нет билета: %s", rec.Body.String())
	}
	// Выданный билет должен резолвиться в того же пользователя ровно один раз.
	// Достаём его из ответа косвенно: любой валидный билет из store consume-ится.
	uid, ok := tickets.Consume(extractTicket(t, rec.Body.String()), time.Now())
	if !ok || uid != 42 {
		t.Fatalf("билет не резолвится в userID=42: uid=%d ok=%v", uid, ok)
	}
}

func TestEventsIssueTicket_RejectsAnonymous(t *testing.T) {
	t.Parallel()
	h, _, _ := newEventsHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/events/ticket", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec) // user_id не выставлен

	err := h.IssueTicket(c)
	if err == nil {
		t.Fatal("ожидали ошибку авторизации для анонима")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401, получили %v", err)
	}
}

func TestEventsStream_RejectsMissingTicket(t *testing.T) {
	t.Parallel()
	h, _, _ := newEventsHandler()
	srv := newEventsServer(t, h)

	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("без билета ожидали 401, получили %d", resp.StatusCode)
	}
}

func TestEventsStream_RejectsInvalidTicket(t *testing.T) {
	t.Parallel()
	h, _, _ := newEventsHandler()
	srv := newEventsServer(t, h)

	resp, err := http.Get(srv.URL + "/api/events?ticket=garbage")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("с невалидным билетом ожидали 401, получили %d", resp.StatusCode)
	}
}

func TestEventsStream_DeliversPublishedEvent(t *testing.T) {
	t.Parallel()
	h, hub, tickets := newEventsHandler()
	srv := newEventsServer(t, h)

	ticket, err := tickets.Issue(42, time.Now())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/events?ticket=" + ticket)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("ожидали text/event-stream, получили %q", ct)
	}

	r := bufio.NewReader(resp.Body)
	if first := readFrame(t, r); !strings.Contains(first, ": connected") {
		t.Fatalf("ожидали connected-кадр, получили %q", first)
	}

	// connected-кадр приходит после hub.Subscribe в хендлере, поэтому к этому
	// моменту подписка зарегистрирована и публикация гарантированно долетит.
	hub.Publish(42, realtime.Event{Type: "available.new", Scope: "available-attachments", Count: 3})

	frame := readFrame(t, r)
	if !strings.Contains(frame, `"type":"available.new"`) || !strings.Contains(frame, `"count":3`) {
		t.Fatalf("ожидали data-кадр с событием, получили %q", frame)
	}
}

func TestEventsStream_TicketIsOneTime(t *testing.T) {
	t.Parallel()
	h, _, tickets := newEventsHandler()
	srv := newEventsServer(t, h)

	ticket, _ := tickets.Issue(7, time.Now())

	// Первый коннект использует билет.
	resp1, err := http.Get(srv.URL + "/api/events?ticket=" + ticket)
	if err != nil {
		t.Fatalf("GET1: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("первый коннект ожидали 200, получили %d", resp1.StatusCode)
	}

	// Повторное использование того же билета отвергается.
	resp2, err := http.Get(srv.URL + "/api/events?ticket=" + ticket)
	if err != nil {
		t.Fatalf("GET2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("повторный билет ожидали 401, получили %d", resp2.StatusCode)
	}
}

// extractTicket достаёт значение билета из JSON-ответа {success,data:{ticket}}.
func extractTicket(t *testing.T, body string) string {
	t.Helper()
	const key = `"ticket":"`
	i := strings.Index(body, key)
	if i < 0 {
		t.Fatalf("билет не найден в %s", body)
	}
	rest := body[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		t.Fatalf("билет не закрыт в %s", body)
	}
	return rest[:j]
}
