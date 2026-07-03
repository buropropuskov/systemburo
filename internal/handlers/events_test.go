package handlers

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var eventsTestSecret = []byte("test-jwt-secret-that-is-at-least-32-chars!!")

func makeAccessToken(t *testing.T, userID int, ttl time.Duration) string {
	t.Helper()
	claims := services.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "tester",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(eventsTestSecret)
	if err != nil {
		t.Fatalf("подписать токен: %v", err)
	}
	return signed
}

func newEventsServer(t *testing.T, hub *realtime.Hub) *httptest.Server {
	t.Helper()
	e := echo.New()
	h := NewEventsHandler(hub, eventsTestSecret)
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

func TestEventsStream_RejectsMissingToken(t *testing.T) {
	t.Parallel()
	srv := newEventsServer(t, realtime.NewHub())
	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("без токена ожидали 401, получили %d", resp.StatusCode)
	}
}

func TestEventsStream_RejectsInvalidToken(t *testing.T) {
	t.Parallel()
	srv := newEventsServer(t, realtime.NewHub())
	resp, err := http.Get(srv.URL + "/api/events?access_token=garbage")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("с мусорным токеном ожидали 401, получили %d", resp.StatusCode)
	}
}

func TestEventsStream_DeliversPublishedEvent(t *testing.T) {
	t.Parallel()
	hub := realtime.NewHub()
	srv := newEventsServer(t, hub)

	token := makeAccessToken(t, 42, time.Hour)
	req, _ := http.NewRequest("GET", srv.URL+"/api/events?access_token="+token, nil)
	resp, err := http.DefaultClient.Do(req)
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

	// connected-кадр приходит после hub.Subscribe в хендлере (Subscribe -> запись
	// кадра -> flush), поэтому к этому моменту подписка уже зарегистрирована и
	// публикация гарантированно долетит в поток.
	hub.Publish(42, realtime.Event{Type: "available.new", Scope: "available-attachments", Count: 3})

	frame := readFrame(t, r)
	if !strings.Contains(frame, `"type":"available.new"`) || !strings.Contains(frame, `"count":3`) {
		t.Fatalf("ожидали data-кадр с событием, получили %q", frame)
	}
}
