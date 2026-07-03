package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// eventsHeartbeatInterval - период heartbeat-комментария в SSE-поток. Держит
// соединение живым (прокси не рвёт idle) и служит тиком ре-валидации токена.
// Меньше access-TTL, чтобы протухание ловилось задолго до накопления.
const eventsHeartbeatInterval = 25 * time.Second

// EventsHandler отдаёт SSE-поток лёгких real-time сигналов (issue #840).
// Аутентификация - своя, по query-токену: EventSource не умеет слать заголовок
// Authorization, поэтому этот эндпоинт висит на публичной группе /api (вне
// JWTAuth-middleware) и валидирует токен сам тем же services.DecodeAccessToken.
type EventsHandler struct {
	hub       *realtime.Hub
	jwtSecret []byte
}

// NewEventsHandler создаёт хендлер SSE-потока.
func NewEventsHandler(hub *realtime.Hub, jwtSecret []byte) *EventsHandler {
	return &EventsHandler{hub: hub, jwtSecret: jwtSecret}
}

// Stream держит SSE-соединение: GET /api/events?access_token=<jwt>.
//
// В поток идут только сигналы вида {"type":"available.new",...} - клиент по ним
// делает обычный запрос (event-then-fetch). Токен ре-валидируется на heartbeat:
// протух за время стрима - шлём событие auth.expired и закрываем, фронт обновляет
// access через свой single-flight refresh и переоткрывает EventSource.
func (h *EventsHandler) Stream(c echo.Context) error {
	token := c.QueryParam("access_token")
	if token == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing access_token")
	}
	claims, err := services.DecodeAccessToken(token, h.jwtSecret)
	if err != nil || claims.UserID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
	}

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set(echo.HeaderCacheControl, "no-cache")
	res.Header().Set(echo.HeaderConnection, "keep-alive")
	// Подсказка nginx не буферизовать поток - работает и до правки nginx-конфига.
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)

	ch, unsub := h.hub.Subscribe(claims.UserID)
	defer unsub()

	// Первый кадр сразу - у клиента срабатывает onopen, соединение "поднято".
	if _, err := fmt.Fprint(res, ": connected\n\n"); err != nil {
		return nil
	}
	res.Flush()

	heartbeat := time.NewTicker(eventsHeartbeatInterval)
	defer heartbeat.Stop()

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
			if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
				fmt.Fprint(res, "event: auth.expired\ndata: {}\n\n")
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
