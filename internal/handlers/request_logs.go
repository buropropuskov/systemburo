package handlers

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// RequestLogsHandler -- HTTP-обработчики логов запросов.
type RequestLogsHandler struct {
	service  services.RequestLogsService
	recorder services.AuditRecorder
}

// NewRequestLogsHandler создаёт новый экземпляр RequestLogsHandler. Рекордер нужен
// выгрузке: снятие журнала файлом оставляет след в audit_log.
func NewRequestLogsHandler(service services.RequestLogsService, recorder services.AuditRecorder) *RequestLogsHandler {
	return &RequestLogsHandler{service: service, recorder: recorder}
}

// GetLogs godoc
// @Summary      Получение логов запросов с пагинацией и фильтрацией
// @Tags         request-logs
// @Produce      json
// @Security     BearerAuth
// @Param        user_id   query int    false "ID пользователя"
// @Param        method    query string false "HTTP метод"
// @Param        status    query int    false "HTTP статус"
// @Param        status_min query int   false "Нижняя граница кода ответа (400 -- только ошибки)"
// @Param        status_max query int   false "Верхняя граница кода ответа"
// @Param        min_duration_ms query int false "Только ответы дольше указанного времени, мс"
// @Param        from_date query string false "Дата начала (ISO 8601)"
// @Param        to_date   query string false "Дата окончания (ISO 8601)"
// @Param        search    query string false "Поиск по URL и username"
// @Param        sort      query string false "Поле сортировки" Enums(created_at, method, url, status, username, duration) default(created_at)
// @Param        order     query string false "Направление сортировки" Enums(asc, desc) default(desc)
// @Param        page      query int    false "Страница" default(1)
// @Param        per_page  query int    false "Записей на странице" default(20)
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /request-logs [get]
func (h *RequestLogsHandler) GetLogs(c echo.Context) error {
	var q models.RequestLogsQuery
	if err := c.Bind(&q); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query parameters")
	}
	logs, total, err := h.service.GetLogs(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return RespondPaginated(c, logs, models.PaginationMeta{
		Total:   total,
		Page:    q.Page,
		PerPage: q.PerPage,
	})
}

// GetUsers godoc
// @Summary      Получение уникальных пользователей для фильтра
// @Tags         request-logs
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.RequestLogsUser
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /request-logs/users [get]
func (h *RequestLogsHandler) GetUsers(c echo.Context) error {
	users, err := h.service.GetUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, users)
}

// GetStats godoc
// @Summary      Получение статистики по логам
// @Tags         request-logs
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.RequestLogsStats
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /request-logs/stats [get]
func (h *RequestLogsHandler) GetStats(c echo.Context) error {
	stats, err := h.service.GetStats(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, stats)
}

// GetRealtime godoc
// @Summary      Получение статистики в реальном времени
// @Tags         request-logs
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.RealtimeStats
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /request-logs/realtime [get]
func (h *RequestLogsHandler) GetRealtime(c echo.Context) error {
	stats, err := h.service.GetRealtime(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, stats)
}

// GetTimeline godoc
// @Summary      Получение таймлайна для графика
// @Tags         request-logs
// @Produce      json
// @Security     BearerAuth
// @Param        interval  query int    false "Интервал группировки в секундах" default(60)
// @Param        limit     query int    false "Максимум точек" default(24)
// @Param        from_date query string false "Дата начала (ISO 8601)"
// @Param        to_date   query string false "Дата окончания (ISO 8601)"
// @Success      200 {array} models.TimelinePoint
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /request-logs/timeline [get]
func (h *RequestLogsHandler) GetTimeline(c echo.Context) error {
	var q models.TimelineQuery
	if err := c.Bind(&q); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query parameters")
	}
	points, err := h.service.GetTimeline(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return RespondSuccess(c, points)
}

// GetHistory godoc
// @Summary      Агрегаты логов за период (вкладка «Аналитика»)
// @Tags         request-logs
// @Produce      json
// @Security     BearerAuth
// @Param        from_date query string false "Дата начала (YYYY-MM-DD)"
// @Param        to_date   query string false "Дата окончания (YYYY-MM-DD)"
// @Success      200 {object} models.RequestLogsHistory
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /request-logs/history [get]
func (h *RequestLogsHandler) GetHistory(c echo.Context) error {
	var q models.RequestLogsHistoryQuery
	if err := c.Bind(&q); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query parameters")
	}
	res, err := h.service.GetHistory(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return RespondSuccess(c, res)
}
