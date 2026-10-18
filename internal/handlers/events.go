package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/httpx"
	"systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// eventsHeartbeatInterval - период heartbeat-комментария в SSE-поток. Держит
// соединение живым (прокси не рвёт idle) и служит тиком проверки времени жизни.
const eventsHeartbeatInterval = 25 * time.Second

// eventsStreamMaxLifetime - максимальное время жизни одного SSE-соединения. По
// истечении сервер закрывает поток сигналом reconnect; фронт берёт новый билет
// (через защищённый POST -> JWTAuth+banCheck) и переоткрывает. Так отзыв доступа
// истёкшей сессии отрабатывает на выдаче билета. Бан/архив дополнительно
// проверяются при открытии потока; уже открытый поток получает штатные сигналы.
const eventsStreamMaxLifetime = 10 * time.Minute

// EventsHandler отдаёт SSE-поток лёгких real-time сигналов (issue #840) и выдаёт
// одноразовые билеты для его установления. См. realtime.TicketStore про то, почему
// билет, а не access-токен в query.
type EventsHandler struct {
	hub           *realtime.Hub
	tickets       *realtime.TicketStore
	ownerStatus   services.TicketOwnerStatus
	sessions      services.SessionAccess
	requestPolicy *middleware.SessionRequestPolicy
	newHeartbeat  func() (<-chan time.Time, func())
}

// NewEventsHandler создаёт хендлер SSE-потока.
func NewEventsHandler(hub *realtime.Hub, tickets *realtime.TicketStore, ownerStatus services.TicketOwnerStatus, sessions services.SessionAccess) *EventsHandler {
	return &EventsHandler{hub: hub, tickets: tickets, ownerStatus: ownerStatus, sessions: sessions, newHeartbeat: func() (<-chan time.Time, func()) {
		ticker := time.NewTicker(eventsHeartbeatInterval)
		return ticker.C, ticker.Stop
	}}
}

func (h *EventsHandler) SetRequestPolicy(policy *middleware.SessionRequestPolicy) {
	h.requestPolicy = policy
}

func (h *EventsHandler) live(ctx context.Context, identity models.SessionIdentity, request *http.Request) (models.SessionIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := services.RequireActiveTicketOwner(ctx, identity.EffectiveUserID, h.ownerStatus); err != nil {
		return models.SessionIdentity{}, err
	}
	if h.sessions == nil {
		return models.SessionIdentity{}, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка сессии временно недоступна")
	}
	fresh, err := h.sessions.RequireLive(ctx, identity)
	if err != nil {
		return models.SessionIdentity{}, err
	}
	// The ticket endpoint defines the existing stream policy, including the
	// consent-notification exception. Do not invent a second whitelist for GET.
	if err := h.requestPolicy.Check(ctx, fresh, http.MethodPost, "/api/events/ticket", request); err != nil {
		return models.SessionIdentity{}, err
	}
	return fresh, nil
}

// IssueTicket выдаёт одноразовый билет для подключения к потоку.
// POST /api/events/ticket (protected: userID берём из JWT-контекста).
//
// IssueTicket godoc
// @Summary      Выдать одноразовый билет для SSE-потока
// @Description  Короткоживущий одноразовый билет для подключения к /events. EventSource не шлёт Authorization, поэтому подключение авторизуется билетом, а не access-токеном.
// @Tags         events
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{} "ticket: string"
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /events/ticket [post]
func (h *EventsHandler) IssueTicket(c echo.Context) error {
	userID := GetUserID(c)
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	identity, ok := services.SessionIdentityFromContext(c.Request().Context())
	if !ok || identity.EffectiveUserID != userID {
		return echo.ErrUnauthorized
	}
	ticket, err := h.tickets.Issue(identity, time.Now())
	if err != nil {
		slog.Error("events: issue ticket failed", "user_id", userID, "error", err)
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
//
// Stream godoc
// @Summary      SSE-поток real-time сигналов
// @Description  Server-Sent Events: лёгкие сигналы "сходи обнови" (event-then-fetch). Авторизация одноразовым билетом из query (см. POST /events/ticket). Поток живёт до 10 минут, затем закрывается событием reconnect.
// @Tags         events
// @Produce      text/event-stream
// @Param        ticket query string true "Одноразовый билет из POST /events/ticket"
// @Success      200 {string} string "SSE-поток событий"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      503 {object} models.HTTPError
// @Router       /events [get]
func (h *EventsHandler) Stream(c echo.Context) error {
	ticket := c.QueryParam("ticket")
	if ticket == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing ticket")
	}
	identity, ok := h.tickets.Consume(ticket, time.Now())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired ticket")
	}
	identity, err := h.live(c.Request().Context(), identity, c.Request())
	if err != nil {
		return err
	}
	middleware.SetSessionIdentity(c, identity)
	userID := identity.EffectiveUserID

	// Поток живёт eventsStreamMaxLifetime, то есть заведомо дольше общего
	// HTTP_WRITE_TIMEOUT. Без снятия срока сервер закрыл бы соединение молча,
	// посреди потока, и фронт увидел бы это как разрыв сети.
	if err := httpx.AllowLongResponse(c); err != nil {
		slog.Warn("events: не удалось снять срок записи, поток оборвётся по HTTP_WRITE_TIMEOUT", "user_id", userID, "error", err)
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

	heartbeat, stopHeartbeat := h.newHeartbeat()
	defer stopHeartbeat()
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
			if _, err := h.live(ctx, identity, c.Request()); err != nil {
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
		case <-heartbeat:
			if _, err := h.live(ctx, identity, c.Request()); err != nil {
				return nil
			}
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
