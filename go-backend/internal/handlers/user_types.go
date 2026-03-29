package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UserTypesHandler — HTTP-обработчики управления типами пользователей (admin-only).
type UserTypesHandler struct {
	service services.UserTypeService
}

// NewUserTypesHandler создаёт новый экземпляр обработчика типов пользователей.
func NewUserTypesHandler(service services.UserTypeService) *UserTypesHandler {
	return &UserTypesHandler{service: service}
}

// GetAll возвращает все типы пользователей с количеством связанных пользователей.
// GET /user-types-management
func (h *UserTypesHandler) GetAll(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	result, err := h.service.GetAllWithCount(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// Create создаёт новый тип пользователя.
// POST /user-types-management
func (h *UserTypesHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	var req services.CreateUserTypeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id, err := h.service.Create(c.Request().Context(), typeID, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":      id,
		"message": "Тип пользователя успешно создан",
	})
}

// Update обновляет тип пользователя по ID.
// PUT /user-types-management/:id
func (h *UserTypesHandler) Update(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type ID")
	}
	var req services.UpdateUserTypeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.Update(c.Request().Context(), typeID, id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Тип пользователя успешно обновлен")
}

// Delete удаляет тип пользователя по ID.
// DELETE /user-types-management/:id
func (h *UserTypesHandler) Delete(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type ID")
	}
	if err := h.service.Delete(c.Request().Context(), typeID, id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Тип пользователя успешно удален")
}
