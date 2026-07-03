package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
)

// eventsHeartbeatInterval - период heartbeat-комментария в SSE-поток. Держит
// соединение живым (прокси не рвёт idle) и служит тиком проверки времени жизни.
const eventsHeartbeatInterval = 25 * time.Second

// eventsStreamMaxLifetime - максимальное время жизни одного SSE-соединения. По
// истечении сервер закрывает поток сигналом reconnect; фронт берёт новый билет
// (через защищённый POST -> JWTAuth+banCheck) и переоткрывает. Так отзыв доступа
// (истёкшая сессия, бан) отрабатывает на выдаче билета, а не тянется в стриме.
const eventsStreamMaxLifetime = 10 * time.Minute

// EventsHandler отдаёт SSE-поток лёгких real-time сигналов (issue #840) и выдаёт
// одноразовые билеты для его установления. См. realtime.TicketStore про то, почему
// билет, а не access-токен в query.
type EventsHandler struct {
	hub     *realtime.Hub
	tickets *realtime.TicketStore
}

// NewEventsHandler создаёт хендлер SSE-потока.
func NewEventsHandler(hub *realtime.Hub, tickets *realtime.TicketStore) *EventsHandler {
	return &EventsHandler{hub: hub, tickets: tickets}
}

// IssueTicket выдаёт одноразовый билет для подключения к потоку.
// POST /api/events/ticket (protected: userID берём из JWT-контекста).
func (h *EventsHandler) IssueTicket(c echo.Context) error {
	userID := GetUserID(c)
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	ticket, err := h.tickets.Issue(userID, time.Now())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to issue ticket")
	}
	return RespondSuccess(c, map[string]string{"ticket": ticket})
}

// Stream держит SSE-соединение: GET /api/events?ticket=<one-time>.
//
// В поток идут только сигналы вида {"type":"available.new",...} - клиент по ним
// делает обычный запрос (event-then-fetch). Билет одноразовый: consume привязывает
// соединение к userID. По eventsStreamMaxLifetime поток закрывается сигналом
// reconnect - фронт переоткрывает с новым билетом.
func (h *EventsHandler) Stream(c echo.Context) error {
	ticket := c.QueryParam("ticket")
	if ticket == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing ticket")
	}
	userID, ok := h.tickets.Consume(ticket, time.Now())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired ticket")
	}

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set(echo.HeaderCacheControl, "no-cache")
	res.Header().Set(echo.HeaderConnection, "keep-alive")
	// Подсказка nginx не буферизовать поток - работает и до правки nginx-конфига.
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)

	ch, unsub := h.hub.Subscribe(userID)
	defer unsub()

	// Первый кадр сразу - у клиента срабатывает onopen, соединение "поднято".
	if _, err := fmt.Fprint(res, ": connected\n\n"); err != nil {
		return nil
	}
	res.Flush()

	heartbeat := time.NewTicker(eventsHeartbeatInterval)
	defer heartbeat.Stop()
	start := time.Now()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done(): // клиент отключился
			return nil
		case ev, ok := <-ch:
			if !ok { // хаб закрыл подписку
				return nil
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(res, "data: %s\n\n", data); err != nil {
				return nil
			}
			res.Flush()
		case <-heartbeat.C:
			if time.Since(start) > eventsStreamMaxLifetime {
				fmt.Fprint(res, "event: reconnect\ndata: {}\n\n")
				res.Flush()
				return nil
			}
			if _, err := fmt.Fprint(res, ": ping\n\n"); err != nil {
				return nil
			}
			res.Flush()
		}
	}
}
