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
// По умолчанию — последние 7 дней. Границы считаются в московских сутках (как и
// бакетинг аналитики), иначе "сегодня"/"неделя" съезжают на 3 часа: from -> начало
// дня 00:00 МСК, to -> конец дня 23:59:59 МСК. Инстанты сравниваются с timestamptz
// корректно вне зависимости от хранения в UTC.
func parseDateRange(c echo.Context) (from, to time.Time) {
	loc := services.AnalyticsLocation()
	now := time.Now().In(loc)
	toDefault := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
	fromDefault := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -6)

	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	if fromStr == "" && toStr == "" {
		return fromDefault, toDefault
	}

	from = fromDefault
	to = toDefault

	if fromStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", fromStr, loc); err == nil {
			from = t // начало дня (00:00:00) в МСК
		}
	}
	if toStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", toStr, loc); err == nil {
			to = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
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

// GetProcessingSummary godoc
// @Summary      Сводка обработки заявок
// @Description  Бандл вкладки «Обработка заявок»: KPI этапов пути заявки (среднее и 90-й перцентиль) со сравнением с прошлым периодом, качество обработки, топ медленных согласующих, разбивка по организациям
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Param        from query string false "Начало периода (YYYY-MM-DD), по умолчанию 7 дней назад"
// @Param        to   query string false "Конец периода (YYYY-MM-DD), по умолчанию сегодня"
// @Success      200 {object} Response
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/processing-summary [get]
func (h *StatisticsHandler) GetProcessingSummary(c echo.Context) error {
	from, to := parseDateRange(c)
	if from.After(to) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date range")
	}
	summary, err := h.service.GetProcessingSummary(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	return RespondSuccess(c, summary)
}

// GetProcessingJournal godoc
// @Summary      Журнал обработки заявок
// @Description  Сквозная лента событий обработки за период по времени убыванием: согласования и несогласования, принятия в работу, отказы принимающего и отзывы инициатором — кто, по какой заявке, в какой роли, когда и сколько рабочего времени Бюро на это ушло. Страница задаётся limit и offset, общее число подходящих событий — в meta.total. Фильтры role и q сужают выборку и учитываются в meta.total.
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Param        from   query string false "Начало периода (YYYY-MM-DD), по умолчанию 7 дней назад"
// @Param        to     query string false "Конец периода (YYYY-MM-DD), по умолчанию сегодня"
// @Param        role   query string false "Роль события: approval, not_approved, acceptance, rejection или withdrawal (по умолчанию все)"
// @Param        q      query string false "Поиск по номеру заявки или ФИО актора (подстрока, регистр не важен)"
// @Param        limit  query int    false "Размер страницы (по умолчанию 50, максимум 200)"
// @Param        offset query int    false "Смещение от начала ленты (по умолчанию 0)"
// @Success      200 {object} Response
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/processing-journal [get]
func (h *StatisticsHandler) GetProcessingJournal(c echo.Context) error {
	from, to := parseDateRange(c)
	if from.After(to) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date range")
	}
	limit, offset := 0, 0
	if v := c.QueryParam("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.QueryParam("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	// Клампим теми же правилами, что и сервис, чтобы meta отдавала реально
	// применённый размер страницы и её номер, а не сырые значения из query.
	limit, offset = services.NormalizeProcessingJournalPaging(limit, offset)

	// Неизвестная роль — 400, а не тихий показ всех событий: иначе опечатка в
	// параметре выглядела бы как «фильтр применён, просто ничего не отсеялось».
	filter, err := services.NormalizeProcessingJournalFilter(services.ProcessingJournalFilter{
		Role:   c.QueryParam("role"),
		Search: c.QueryParam("q"),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role filter")
	}

	entries, total, err := h.service.GetProcessingJournal(c.Request().Context(), from, to, filter, limit, offset)
	if err != nil {
		return err
	}
	return RespondPaginated(c, entries, models.PaginationMeta{
		Total:   total,
		Page:    offset/limit + 1,
		PerPage: limit,
	})
}

// GetOnlinePeaks godoc
// @Summary      Дневные пики онлайна пользователей
// @Description  Серия дневных пиков одновременного онлайна за период для графика динамики пользователей
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Param        from query string false "Начало периода (YYYY-MM-DD)"
// @Param        to   query string false "Конец периода (YYYY-MM-DD)"
// @Success      200 {array} models.OnlinePeakPoint
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/online-peaks [get]
func (h *StatisticsHandler) GetOnlinePeaks(c echo.Context) error {
	from, to := parseDateRange(c)
	if from.After(to) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date range")
	}
	points, err := h.service.GetOnlinePeaks(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	return RespondSuccess(c, points)
}

// GetOnlineUsers godoc
// @Summary      Список пользователей онлайн
// @Description  Пользователи с активностью (last_seen) за окно онлайна, по убыванию свежести. Для модалки «кто онлайн» на дашборде.
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/online-users [get]
func (h *StatisticsHandler) GetOnlineUsers(c echo.Context) error {
	users, err := h.service.GetOnlineUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, users)
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
// @Description  mode=aggregate: одна/несколько метрик (metrics[]) x разрез (или none) x фильтры x период. mode=list: выгрузка строк сущности.
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

	switch req.Mode {
	case "aggregate":
		if (req.Metric == "" && len(req.Metrics) == 0) || req.Dimension == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "metric and dimension are required")
		}
		res, err := h.service.RunReport(c.Request().Context(), req)
		if err != nil {
			return mapReportError(err)
		}
		return RespondSuccess(c, res)
	case "list":
		if req.Entity == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "entity is required")
		}
		res, err := h.service.RunReportList(c.Request().Context(), req)
		if err != nil {
			return mapReportError(err)
		}
		return RespondSuccess(c, res)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported report mode")
	}
}

// mapReportError маппит ошибку движка отчётов в HTTP. Невалидный запрос (неизвестная
// метрика/разрез/сущность/фильтр) -> generic 400 без эха пользовательского ввода.
func mapReportError(err error) error {
	if errors.Is(err, services.ErrInvalidReportRequest) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report request")
	}
	return err
}
