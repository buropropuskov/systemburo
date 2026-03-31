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

// GetAll godoc
// @Summary      Получить все гражданства
// @Description  Возвращает список всех гражданств в системе
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.Citizenship
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /citizenships [get]
func (h *CitizenshipHandler) GetAll(c echo.Context) error {
	citizenships, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, citizenships)
}

// Create godoc
// @Summary      Создать гражданство
// @Description  Создаёт новое гражданство с указанными параметрами
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateCitizenshipRequest true "Данные нового гражданства"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships [post]
func (h *CitizenshipHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	var req models.CreateCitizenshipRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.Create(c.Request().Context(), typeID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Гражданство успешно создано",
	})
}

// Update godoc
// @Summary      Обновить гражданство
// @Description  Обновляет данные гражданства по указанному ID
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID гражданства"
// @Param        request body models.UpdateCitizenshipRequest true "Обновлённые данные гражданства"
// @Success      200 {string} string "Гражданство успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships/{id} [put]
func (h *CitizenshipHandler) Update(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateCitizenshipRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), typeID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Гражданство успешно обновлено")
}

// Delete godoc
// @Summary      Удалить гражданство
// @Description  Удаляет гражданство по указанному ID
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID гражданства"
// @Success      200 {string} string "Гражданство успешно удалено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships/{id} [delete]
func (h *CitizenshipHandler) Delete(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), typeID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Гражданство успешно удалено")
}

// ClearDefaults godoc
// @Summary      Сбросить гражданства по умолчанию
// @Description  Сбрасывает флаг is_default у всех гражданств
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {string} string "Все гражданства по умолчанию сброшены"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships/clear-default [post]
func (h *CitizenshipHandler) ClearDefaults(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	if err := h.service.ClearDefaults(c.Request().Context(), typeID); err != nil {
		return err
	}
	return RespondMessage(c, "Все гражданства по умолчанию сброшены")
}
