package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// CitizenshipHandler -- HTTP-обработчики гражданств.
type CitizenshipHandler struct {
	service services.CitizenshipService
}

// NewCitizenshipHandler создаёт новый экземпляр обработчика гражданств.
func NewCitizenshipHandler(service services.CitizenshipService) *CitizenshipHandler {
	return &CitizenshipHandler{service: service}
}

// GetAll возвращает все гражданства.
// GET /citizenships
func (h *CitizenshipHandler) GetAll(c echo.Context) error {
	citizenships, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, citizenships)
}

// Create создаёт новое гражданство.
// POST /citizenships
func (h *CitizenshipHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	var req models.CreateCitizenshipRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id, err := h.service.Create(c.Request().Context(), typeID, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":      id,
		"message": "Гражданство успешно создано",
	})
}

// Update обновляет гражданство по ID.
// PUT /citizenships/:id
func (h *CitizenshipHandler) Update(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateCitizenshipRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.Update(c.Request().Context(), typeID, id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Гражданство успешно обновлено")
}

// Delete удаляет гражданство по ID.
// DELETE /citizenships/:id
func (h *CitizenshipHandler) Delete(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), typeID, id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Гражданство успешно удалено")
}

// ClearDefaults сбрасывает флаг is_default у всех гражданств.
// POST /citizenships/clear-default
func (h *CitizenshipHandler) ClearDefaults(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	if err := h.service.ClearDefaults(c.Request().Context(), typeID); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Все гражданства по умолчанию сброшены")
}
