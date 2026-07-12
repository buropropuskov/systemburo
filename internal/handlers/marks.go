package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// MarkHandler - HTTP-обработчики справочника марок автомобилей.
type MarkHandler struct {
	service services.MarkService
}

// NewMarkHandler создаёт обработчик.
func NewMarkHandler(service services.MarkService) *MarkHandler {
	return &MarkHandler{service: service}
}

// GetAll godoc
// @Summary      Список марок автомобилей
// @Tags         marks
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Включать архивные марки"
// @Success      200 {array} models.Mark
// @Router       /marks [get]
func (h *MarkHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	marks, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, marks)
}

// Create godoc
// @Summary      Создать марку автомобиля
// @Tags         marks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateMarkRequest true "Данные марки"
// @Success      201 {object} models.Mark
// @Router       /marks [post]
func (h *MarkHandler) Create(c echo.Context) error {
	var req models.CreateMarkRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	mark, err := h.service.Create(c.Request().Context(), req, userID)
	if err != nil {
		return err
	}
	return RespondCreated(c, mark)
}

// Update godoc
// @Summary      Переименовать марку (логируется в истории)
// @Tags         marks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID марки"
// @Param        request body models.UpdateMarkRequest true "Новое имя"
// @Success      200 {string} string "Марка обновлена"
// @Router       /marks/{id} [put]
func (h *MarkHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateMarkRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Update(c.Request().Context(), id, req, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Марка обновлена")
}

// Archive godoc
// @Summary      Архивировать марку (is_active=false)
// @Tags         marks
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID марки"
// @Success      200 {string} string "Марка архивирована"
// @Router       /marks/{id}/archive [post]
func (h *MarkHandler) Archive(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Archive(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Марка архивирована")
}

// Restore godoc
// @Summary      Разархивировать марку (is_active=true)
// @Tags         marks
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID марки"
// @Success      200 {string} string "Марка восстановлена"
// @Router       /marks/{id}/restore [post]
func (h *MarkHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Марка восстановлена")
}

// GetHistory godoc
// @Summary      История изменений марки
// @Tags         marks
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID марки"
// @Success      200 {array} models.MarkHistoryItem
// @Router       /marks/{id}/history [get]
func (h *MarkHandler) GetHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	history, err := h.service.GetHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}

// BulkArchive godoc
// @Summary      Групповая архивация марок
// @Tags         marks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID марок"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /marks/bulk/archive [post]
func (h *MarkHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны марки")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление марок
// @Tags         marks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID марок"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /marks/bulk/restore [post]
func (h *MarkHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны марки")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}
