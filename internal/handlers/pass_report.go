package handlers

import (
	"net/http"
	"time"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PassReportHandler - суточный отчёт охранника по проходам таблицы. Роуты
// гейтятся middleware RequireTableVerb правом table.<name>.report; здесь
// решается только скоуп строк: admin/super видят разбивку по всем охранникам,
// остальные - свои строки + итог по таблице.
type PassReportHandler struct {
	service  services.DailyPassReportService
	resolver *services.PermissionResolver
}

// NewPassReportHandler собирает хендлер отчётов по проходам.
func NewPassReportHandler(service services.DailyPassReportService, resolver *services.PermissionResolver) *PassReportHandler {
	return &PassReportHandler{service: service, resolver: resolver}
}

// scopeFor решает видимость строк по правам вызывающего.
func (h *PassReportHandler) scopeFor(c echo.Context) (services.PassReportScope, error) {
	userID := GetUserID(c)
	if userID == 0 {
		return services.PassReportScope{}, echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	set, err := h.resolver.Resolve(c.Request().Context(), userID)
	if err != nil {
		return services.PassReportScope{}, err
	}
	if set.IsSuperAdmin() || set.IsAdmin() {
		return services.PassReportScope{AllUsers: true}, nil
	}
	return services.PassReportScope{UserID: userID}, nil
}

// Live godoc
// @Summary      Живой отчёт по проходам за текущее окно
// @Description  Счётчики событий въезд/выезд машин и вход/выход людей за незакрытое окно [последние 21:30 МСК, сейчас). Охранник видит свои строки + итог по таблице, admin/super - разбивку по всем.
// @Tags         pass-reports
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {object} map[string]interface{} "period_start/period_end, rows, totals"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/pass-report/live [get]
func (h *PassReportHandler) Live(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	scope, err := h.scopeFor(c)
	if err != nil {
		return err
	}

	report, err := h.service.Live(c.Request().Context(), id, scope)
	if err != nil {
		return err
	}
	return RespondSuccess(c, report)
}

// List godoc
// @Summary      Сохранённые суточные отчёты по проходам
// @Description  История отчётов таблицы по дням (окно [21:30, 21:30) МСК, фиксируется кроном в 21:30). Фильтр периода from/to по report_date (YYYY-MM-DD); без фильтра - последний месяц. Новые дни первыми.
// @Tags         pass-reports
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        from query string false "Начало периода (YYYY-MM-DD)"
// @Param        to query string false "Конец периода (YYYY-MM-DD)"
// @Success      200 {object} map[string]interface{} "days: список дней с rows и totals"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/pass-reports [get]
func (h *PassReportHandler) List(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	scope, err := h.scopeFor(c)
	if err != nil {
		return err
	}

	from, err := parsePassReportDate(c.QueryParam("from"))
	if err != nil {
		return err
	}
	to, err := parsePassReportDate(c.QueryParam("to"))
	if err != nil {
		return err
	}

	days, err := h.service.ListDays(c.Request().Context(), id, from, to, scope)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{"days": days})
}

// parsePassReportDate парсит дату фильтра по report_date. Пустая строка - нет
// границы; непарсимое значение - явная 400, не молчаливый пропуск фильтра.
func parsePassReportDate(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid date filter, expected YYYY-MM-DD")
	}
	return &t, nil
}
