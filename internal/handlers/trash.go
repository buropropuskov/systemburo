package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// TrashHandler - HTTP API корзины таблиц (#186).
type TrashHandler struct {
	service services.TrashService
	db      DBRef // нужен для определения table_type (cars/people) по systemTableID
}

// DBRef - минимальный интерфейс для одного запроса (выявить тип таблицы).
// Не зависит от gorm импорта в handler-пакете.
type DBRef interface {
	GetTableType(tableID int) (string, error)
}

// NewTrashHandler создаёт handler.
func NewTrashHandler(s services.TrashService, dbRef DBRef) *TrashHandler {
	return &TrashHandler{service: s, db: dbRef}
}

// List godoc
// @Summary      Список элементов в корзине таблицы
// @Tags         trash
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID SystemTable"
// @Param        search query string false "Поиск по номеру/ФИО"
// @Param        organization_id query int false "Фильтр по организации"
// @Param        organization_ids query string false "ID организаций через запятую (мультивыбор)"
// @Param        date_from query string false "Дата удаления с (YYYY-MM-DD)"
// @Param        date_to query string false "Дата удаления по (YYYY-MM-DD)"
// @Success      200 {array} models.TrashItem
// @Router       /system-tables/{id}/trash [get]
func (h *TrashHandler) List(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table id")
	}
	tableType, err := h.db.GetTableType(tableID)
	if err != nil {
		return err
	}
	filter := models.TrashFilter{
		Search:          c.QueryParam("search"),
		OrganizationIDs: c.QueryParam("organization_ids"),
		DateFrom:        c.QueryParam("date_from"),
		DateTo:          c.QueryParam("date_to"),
	}
	if oid, _ := strconv.Atoi(c.QueryParam("organization_id")); oid > 0 {
		filter.OrganizationID = oid
	}
	var items []models.TrashItem
	switch tableType {
	case "cars":
		items, err = h.service.ListCarsTrash(c.Request().Context(), tableID, filter)
	case "people":
		items, err = h.service.ListEmployeesTrash(c.Request().Context(), tableID, filter)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Тип таблицы не поддерживает корзину")
	}
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Restore godoc
// @Summary      Восстановить элементы из корзины
// @Tags         trash
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID SystemTable"
// @Param        request body models.RestoreTrashRequest true "IDs"
// @Success      200 {object} map[string]int
// @Router       /system-tables/{id}/trash/restore [post]
func (h *TrashHandler) Restore(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table id")
	}
	var req models.RestoreTrashRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	tableType, err := h.db.GetTableType(tableID)
	if err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	var restored int
	switch tableType {
	case "cars":
		restored, err = h.service.RestoreCars(c.Request().Context(), tableID, req.IDs, userID)
	case "people":
		restored, err = h.service.RestoreEmployees(c.Request().Context(), tableID, req.IDs, userID)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Тип таблицы не поддерживает корзину")
	}
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{
		"restored":  restored,
		"requested": len(req.IDs),
	})
}

// PurgeOne godoc
// @Summary      Окончательно удалить элемент из корзины
// @Tags         trash
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID SystemTable"
// @Param        item_id path int true "ID элемента"
// @Success      200 {string} string "Удалено безвозвратно"
// @Router       /system-tables/{id}/trash/{item_id} [delete]
func (h *TrashHandler) PurgeOne(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table id")
	}
	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid item id")
	}
	tableType, err := h.db.GetTableType(tableID)
	if err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	switch tableType {
	case "cars":
		err = h.service.PurgeCar(c.Request().Context(), tableID, itemID, userID)
	case "people":
		err = h.service.PurgeEmployee(c.Request().Context(), tableID, itemID, userID)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Тип таблицы не поддерживает корзину")
	}
	if err != nil {
		return err
	}
	return RespondMessage(c, "Удалено безвозвратно")
}

// History godoc
// @Summary      Лог массовых действий с корзиной таблицы
// @Tags         trash
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID SystemTable"
// @Success      200 {array} models.TrashHistoryItem
// @Router       /system-tables/{id}/trash/history [get]
func (h *TrashHandler) History(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table id")
	}
	items, err := h.service.ListTrashHistory(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// ClearAll godoc
// @Summary      Очистить корзину таблицы целиком
// @Tags         trash
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID SystemTable"
// @Success      200 {object} map[string]int
// @Router       /system-tables/{id}/trash [delete]
func (h *TrashHandler) ClearAll(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table id")
	}
	tableType, err := h.db.GetTableType(tableID)
	if err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	var purged int
	switch tableType {
	case "cars":
		purged, err = h.service.ClearCarsTrash(c.Request().Context(), tableID, userID)
	case "people":
		purged, err = h.service.ClearEmployeesTrash(c.Request().Context(), tableID, userID)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Тип таблицы не поддерживает корзину")
	}
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]int{"purged": purged})
}
