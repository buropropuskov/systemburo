package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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

// List godoc
// @Summary      Список версий состояния таблицы
// @Description  Метаданные версий таблицы (дата, причина, автор, агрегаты) без payload. Пагинация + фильтр периода (from/to по дате снимка).
// @Tags         table-snapshots
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        page query int false "Страница (>=1)"
// @Param        per_page query int false "Размер страницы (1-100)"
// @Param        from query string false "Начало периода (YYYY-MM-DD или RFC3339)"
// @Param        to query string false "Конец периода (YYYY-MM-DD или RFC3339)"
// @Success      200 {object} map[string]interface{} "data: список версий, meta: пагинация"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /system-tables/{id}/snapshots [get]
func (h *TableSnapshotHandler) List(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	from, err := parseSnapshotBound(c.QueryParam("from"), false)
	if err != nil {
		return err
	}
	to, err := parseSnapshotBound(c.QueryParam("to"), true)
	if err != nil {
		return err
	}

	items, total, err := h.service.ListSnapshots(c.Request().Context(), id, from, to, page, perPage)
	if err != nil {
		return err
	}
	return RespondPaginated(c, items, models.PaginationMeta{Total: total, Page: page, PerPage: perPage})
}

// Get godoc
// @Summary      Версия состояния таблицы (полный слепок)
// @Description  Одна версия с полным payload (строки+статусы). Скоуплена по таблице из URL.
// @Tags         table-snapshots
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        sid path int true "ID версии"
// @Success      200 {object} map[string]interface{} "версия с payload"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/snapshots/{sid} [get]
func (h *TableSnapshotHandler) Get(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	sid, err := ParseID(c, "sid")
	if err != nil {
		return err
	}

	snap, err := h.service.GetSnapshot(c.Request().Context(), id, sid)
	if err != nil {
		return err
	}
	return RespondSuccess(c, snap)
}

// Cleanup godoc
// @Summary      Чистка старых версий таблицы
// @Description  Удаляет версии таблицы старше older_than месяцев. Разрушительно - только admin/super (гейт requireAdmin на роуте).
// @Tags         table-snapshots
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        older_than query int true "Порог в месяцах (>0): удалить версии старше"
// @Success      200 {object} map[string]interface{} "deleted: число удалённых"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /system-tables/{id}/snapshots [delete]
func (h *TableSnapshotHandler) Cleanup(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	months, err := strconv.Atoi(c.QueryParam("older_than"))
	if err != nil || months <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "older_than must be a positive number of months")
	}

	deleted, err := h.service.DeleteSnapshotsOlderThan(c.Request().Context(), id, months)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"deleted": deleted,
		"message": fmt.Sprintf("Удалено версий: %d", deleted),
	})
}

// parseSnapshotBound парсит границу периода фильтра: RFC3339 или YYYY-MM-DD.
// Для верхней границы (endOfDay) дата без времени трактуется как конец суток
// включительно. Пустая строка - нет границы. Непарсимое непустое значение - 400
// (молча не глотаем: кривой фильтр лучше вернуть явной ошибкой).
func parseSnapshotBound(v string, endOfDay bool) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return &t, nil
	}
	return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid date filter, expected YYYY-MM-DD or RFC3339")
}
