package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
)

// eventsHeartbeatInterval - период heartbeat-комментария в SSE-поток. Держит
// соединение живым (прокси не рвёт idle) и служит тиком проверки времени жизни.
const eventsHeartbeatInterval = 25 * time.Second

// eventsStreamMaxLifetime - максимальное время жизни одного SSE-соединения. По
// истечении сервер закрывает поток; фронт берёт новый билет
// (через защищённый POST -> JWTAuth+banCheck) и переоткрывает. Так отзыв доступа
// истёкшей сессии отрабатывает на выдаче билета. Бан/архив дополнительно
// проверяются при открытии потока; уже открытый поток получает штатные сигналы.
const eventsStreamMaxLifetime = 10 * time.Minute

const eventsWriteTimeout = 5 * time.Second

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
	lifecycle     context.Context
	stop          context.CancelFunc
	maxLifetime   time.Duration
	writeTimeout  time.Duration
}

// NewEventsHandler создаёт хендлер SSE-потока.
func NewEventsHandler(hub *realtime.Hub, tickets *realtime.TicketStore, ownerStatus services.TicketOwnerStatus, sessions services.SessionAccess) *EventsHandler {
	lifecycle, stop := context.WithCancel(context.Background())
	return &EventsHandler{hub: hub, tickets: tickets, ownerStatus: ownerStatus, sessions: sessions, lifecycle: lifecycle, stop: stop, maxLifetime: eventsStreamMaxLifetime, writeTimeout: eventsWriteTimeout, newHeartbeat: func() (<-chan time.Time, func()) {
		ticker := time.NewTicker(eventsHeartbeatInterval)
		return ticker.C, ticker.Stop
	}}
}

// Close cancels checks and buffered delivery before HTTP shutdown starts.
func (h *EventsHandler) Close() {
	h.stop()
	h.tickets.Close()
	h.hub.Close()
}

func (h *EventsHandler) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(eventsHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.lifecycle.Done():
			return
		case now := <-ticker.C:
			h.tickets.Cleanup(now)
		}
	}
}

func eventsAdmissionError(c *echo.Context, err error) error {
	if errors.Is(err, realtime.ErrOwnerLimit) || errors.Is(err, realtime.ErrSystemLimit) || errors.Is(err, realtime.ErrClosed) {
		c.Response().Header().Set("Retry-After", "5")
		status := http.StatusServiceUnavailable
		if errors.Is(err, realtime.ErrOwnerLimit) {
			status = http.StatusTooManyRequests
		}
		return echo.NewHTTPError(status, "Обновление данных временно недоступно. Повторите позже")
	}
	if errors.Is(err, realtime.ErrInvalidIdentity) {
		return echo.ErrUnauthorized
	}
	return err
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
func (h *EventsHandler) IssueTicket(c *echo.Context) error {
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
		if errors.Is(err, realtime.ErrOwnerLimit) || errors.Is(err, realtime.ErrSystemLimit) || errors.Is(err, realtime.ErrClosed) || errors.Is(err, realtime.ErrInvalidIdentity) {
			return eventsAdmissionError(c, err)
		}
		slog.Error("events: issue ticket failed", "user_id", userID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to issue ticket")
	}
	return RespondSuccess(c, map[string]string{"ticket": ticket})
}

// Stream держит SSE-соединение: GET /api/events?ticket=<one-time>.
//
// В поток идут только сигналы вида {"type":"available.new",...} - клиент по ним
// делает обычный запрос (event-then-fetch). Билет одноразовый: consume привязывает
// соединение к userID. По eventsStreamMaxLifetime поток закрывается;
// фронт переоткрывает его с новым билетом через общий reconnect-механизм.
//
// Stream godoc
// @Summary      SSE-поток real-time сигналов
// @Description  Server-Sent Events: лёгкие сигналы "сходи обнови" (event-then-fetch). Авторизация одноразовым билетом из query (см. POST /events/ticket). Поток живёт до 10 минут, затем закрывается для подключения с новым билетом.
// @Tags         events
// @Produce      text/event-stream
// @Param        ticket query string true "Одноразовый билет из POST /events/ticket"
// @Success      200 {string} string "SSE-поток событий"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      503 {object} models.HTTPError
// @Router       /events [get]
func (h *EventsHandler) Stream(c *echo.Context) error {
	if h.lifecycle.Err() != nil {
		return eventsAdmissionError(c, realtime.ErrClosed)
	}
	ticket := c.QueryParam("ticket")
	if ticket == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing ticket")
	}
	identity, ok := h.tickets.Consume(ticket, time.Now())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired ticket")
	}
	ch, release, err := h.hub.Subscribe(identity.OwnerUserID, identity.EffectiveUserID)
	if err != nil {
		return eventsAdmissionError(c, err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(c.Request().Context(), h.maxLifetime)
	defer cancel()
	stopLifecycle := context.AfterFunc(h.lifecycle, cancel)
	defer stopLifecycle()
	identity, err = h.live(ctx, identity, c.Request())
	if err != nil {
		if h.lifecycle.Err() != nil {
			return eventsAdmissionError(c, realtime.ErrClosed)
		}
		return err
	}
	if ctx.Err() != nil {
		return eventsAdmissionError(c, realtime.ErrClosed)
	}
	middleware.SetSessionIdentity(c, identity)
	res := c.Response()
	// Echo.Response.Flush has no error result and discards writer timeouts.
	// Control the wrapped writer directly so a failed flush releases the slot.
	response, responseErr := echo.UnwrapResponse(res)
	if responseErr != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Обновление данных временно недоступно")
	}
	controller := http.NewResponseController(response.Unwrap())
	setDeadline := func() error {
		end := time.Now().Add(h.writeTimeout)
		if limit, ok := ctx.Deadline(); ok && limit.Before(end) {
			end = limit
		}
		return controller.SetWriteDeadline(end)
	}
	// No bytes have been written: unsupported writers fail closed, not halfway through a stream.
	if err := setDeadline(); err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Обновление данных временно недоступно")
	}
	defer controller.SetWriteDeadline(time.Time{})
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set(echo.HeaderCacheControl, "no-cache")
	res.Header().Set(echo.HeaderConnection, "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)
	writeFrame := func(frame string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := setDeadline(); err != nil {
			return err
		}
		if _, err := fmt.Fprint(res, frame); err != nil {
			return err
		}
		return controller.Flush()
	}
	if err := writeFrame(": connected\n\n"); err != nil {
		return nil
	}
	heartbeat, stopHeartbeat := h.newHeartbeat()
	defer stopHeartbeat()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, open := <-ch:
			if !open || ctx.Err() != nil {
				return nil
			}
			if _, err := h.live(ctx, identity, c.Request()); err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if err := writeFrame(fmt.Sprintf("data: %s\n\n", data)); err != nil {
				return nil
			}
		case <-heartbeat:
			if ctx.Err() != nil {
				return nil
			}
			if _, err := h.live(ctx, identity, c.Request()); err != nil {
				return nil
			}
			if err := writeFrame(": ping\n\n"); err != nil {
				return nil
			}
		}
	}
}
