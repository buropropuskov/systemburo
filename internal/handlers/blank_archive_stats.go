package handlers

import (
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// BlankArchiveStatsHandler отдаёт сводку файлового архива: занятое место, состав
// диска и разбивку по месяцам (#1615, срез B2).
type BlankArchiveStatsHandler struct {
	quota *services.BlankExportQuotaService
}

// NewBlankArchiveStatsHandler создаёт хендлер сводки файлового архива.
func NewBlankArchiveStatsHandler(quota *services.BlankExportQuotaService) *BlankArchiveStatsHandler {
	return &BlankArchiveStatsHandler{quota: quota}
}

// GetStats отдаёт сводку файлового архива (кэш 5 минут - см. BlankExportQuotaService.Stats).
func (h *BlankArchiveStatsHandler) GetStats(c echo.Context) error {
	if h.quota == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Файловый архив недоступен: каталог не настроен")
	}
	stats, err := h.quota.Stats(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, stats)
}
