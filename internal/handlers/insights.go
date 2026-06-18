package handlers

import (
	"github.com/labstack/echo/v4"
)

// GetInsights godoc
// @Summary      Инсайты аналитики
// @Description  Готовые инсайты за период: пик по часам, сравнение с прошлым периодом, топ мест/организаций, тренды
// @Tags         statistics
// @Produce      json
// @Param        from query string false "Дата начала YYYY-MM-DD"
// @Param        to query string false "Дата конца YYYY-MM-DD"
// @Success      200 {object} models.InsightsResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Security     BearerAuth
// @Router       /statistics/insights [get]
func (h *StatisticsHandler) GetInsights(c echo.Context) error {
	from, to := parseDateRange(c)
	res, err := h.service.GetInsights(
		c.Request().Context(),
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		return mapReportError(err)
	}
	return RespondSuccess(c, res)
}
