package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// TableSnapshotHandler обслуживает версии (слепки) состояния системных таблиц.
type TableSnapshotHandler struct {
	service services.TableSnapshotService
}

// NewTableSnapshotHandler собирает хендлер версий поверх сервиса снимков.
func NewTableSnapshotHandler(service services.TableSnapshotService) *TableSnapshotHandler {
	return &TableSnapshotHandler{service: service}
}

// Create godoc
// @Summary      Ручной снимок состояния таблицы
// @Description  Сохраняет текущее состояние таблицы (машины/люди со статусами) как новую версию (reason=manual).
// @Tags         table-snapshots
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {object} map[string]interface{} "id версии, message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/snapshots [post]
func (h *TableSnapshotHandler) Create(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var actor *int
	if uid := GetUserID(c); uid > 0 {
		actor = &uid
	}

	snapID, err := h.service.SnapshotTable(c.Request().Context(), id, models.SnapshotReasonManual, actor)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      snapID,
		"message": "Версия состояния таблицы сохранена",
	})
}
