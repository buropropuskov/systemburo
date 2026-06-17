package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// StatisticsHandler — HTTP-обработчики статистики дашборда.
type StatisticsHandler struct {
	service services.StatisticsService
}

// NewStatisticsHandler создаёт новый экземпляр StatisticsHandler.
func NewStatisticsHandler(service services.StatisticsService) *StatisticsHandler {
	return &StatisticsHandler{service: service}
}

// parseDateRange парсит from/to из query-параметров (формат YYYY-MM-DD).
// По умолчанию — последние 7 дней. from -> начало дня, to -> конец дня (23:59:59).
func parseDateRange(c echo.Context) (from, to time.Time) {
	now := time.Now().UTC()
	toDefault := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
	fromDefault := toDefault.AddDate(0, 0, -6).Truncate(24 * time.Hour)

	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	if fromStr == "" && toStr == "" {
		return fromDefault, toDefault
	}

	from = fromDefault
	to = toDefault

	if fromStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", fromStr, time.UTC); err == nil {
			from = t // уже начало дня (00:00:00) в UTC
		}
	}
	if toStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", toStr, time.UTC); err == nil {
			to = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
		}
	}

	return from, to
}

// GetSummary godoc
// @Summary      Сводная статистика дашборда
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Param        from query string false "Начало периода (YYYY-MM-DD), по умолчанию 7 дней назад"
// @Param        to   query string false "Конец периода (YYYY-MM-DD), по умолчанию сегодня"
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/summary [get]
func (h *StatisticsHandler) GetSummary(c echo.Context) error {
	from, to := parseDateRange(c)
	if from.After(to) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date range")
	}
	summary, err := h.service.GetSummary(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	return RespondSuccess(c, summary)
}

// GetTimeline godoc
// @Summary      Данные для графика по метрике
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Param        from        query string false "Начало периода (YYYY-MM-DD)"
// @Param        to          query string false "Конец периода (YYYY-MM-DD)"
// @Param        metric      query string false "Метрика: applications, car_entries, people_entries" default(applications)
// @Param        granularity query string false "Гранулярность: day, week, month" default(day)
// @Success      200 {array} models.StatsTimelinePoint
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/timeline [get]
func (h *StatisticsHandler) GetTimeline(c echo.Context) error {
	from, to := parseDateRange(c)
	if from.After(to) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date range")
	}

	metric := c.QueryParam("metric")
	if metric == "" {
		metric = "applications"
	}
	granularity := c.QueryParam("granularity")
	if granularity == "" {
		granularity = "day"
	}

	points, err := h.service.GetTimeline(c.Request().Context(), from, to, metric, granularity)
	if err != nil {
		// Неизвестные metric/granularity -> 400 без эха пользовательского ввода в ответ.
		return echo.NewHTTPError(http.StatusBadRequest, "invalid metric or granularity")
	}
	return RespondSuccess(c, points)
}

// GetRecentPassages godoc
// @Summary      Последние проходы людей и проезды машин (живые ленты)
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Param        limit query int false "Количество последних записей (1-50, по умолчанию 15)"
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/recent-passages [get]
func (h *StatisticsHandler) GetRecentPassages(c echo.Context) error {
	limit := 0
	if v := c.QueryParam("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	passages, err := h.service.GetRecentPassages(c.Request().Context(), limit)
	if err != nil {
		return err
	}
	return RespondSuccess(c, passages)
}

// GetMetrics godoc
// @Summary      Каталог конструктора отчётов
// @Description  Whitelist метрик, разрезов, фильтров и list-сущностей со значениями динамических справочников
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/metrics [get]
func (h *StatisticsHandler) GetMetrics(c echo.Context) error {
	catalog, err := h.service.GetReportCatalog(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, catalog)
}

// RunReport godoc
// @Summary      Исполнение отчёта конструктора
// @Description  Агрегатный отчёт: метрика x разрез x фильтры x период (mode=aggregate)
// @Tags         statistics
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ReportRequest true "Параметры отчёта"
// @Success      200 {object} Response
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/report [post]
func (h *StatisticsHandler) RunReport(c echo.Context) error {
	var req models.ReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Mode == "" {
		req.Mode = "aggregate"
	}
	if req.Mode != "aggregate" {
		// list-режим (выгрузка строк) добавляется отдельным срезом.
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported report mode")
	}
	if req.Metric == "" || req.Dimension == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "metric and dimension are required")
	}

	res, err := h.service.RunReport(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidReportRequest) {
			// Не эхаем ввод: неизвестная метрика/разрез/фильтр -> generic 400.
			return echo.NewHTTPError(http.StatusBadRequest, "invalid report request")
		}
		return err
	}
	return RespondSuccess(c, res)
}
