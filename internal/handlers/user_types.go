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

// GetAll godoc
// @Summary      Получить все типы пользователей
// @Description  Возвращает список всех типов пользователей с количеством связанных пользователей
// @Tags         user-types-management
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.UserTypeWithCount
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /user-types-management [get]
func (h *UserTypesHandler) GetAll(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	result, err := h.service.GetAllWithCount(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// Create godoc
// @Summary      Создать тип пользователя
// @Description  Создаёт новый тип пользователя с указанными именем и кодом
// @Tags         user-types-management
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateUserTypeRequest true "Данные нового типа пользователя"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /user-types-management [post]
func (h *UserTypesHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	userID, _ := c.Get("user_id").(int)
	var req services.CreateUserTypeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.Create(c.Request().Context(), typeID, userID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Тип пользователя успешно создан",
	})
}

// Update godoc
// @Summary      Обновить тип пользователя
// @Description  Обновляет имя и код типа пользователя по указанному ID
// @Tags         user-types-management
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID типа пользователя"
// @Param        request body services.UpdateUserTypeRequest true "Обновлённые данные типа пользователя"
// @Success      200 {string} string "Тип пользователя успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /user-types-management/{id} [put]
func (h *UserTypesHandler) Update(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type ID")
	}
	var req services.UpdateUserTypeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), typeID, userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Тип пользователя успешно обновлен")
}

// Delete godoc
// @Summary      Удалить тип пользователя
// @Description  Удаляет тип пользователя по указанному ID. Нельзя удалить если есть связанные пользователи
// @Tags         user-types-management
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID типа пользователя"
// @Success      200 {string} string "Тип пользователя успешно удален"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /user-types-management/{id} [delete]
func (h *UserTypesHandler) Delete(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type ID")
	}
	if err := h.service.Delete(c.Request().Context(), typeID, userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Тип пользователя успешно удален")
}

// GetHistory godoc
// @Summary      История изменений типа пользователя
// @Tags         user-types-management
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID типа пользователя"
// @Success      200 {array} models.UserTypeHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /user-types-management/{id}/history [get]
func (h *UserTypesHandler) GetHistory(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type ID")
	}
	items, err := h.service.GetHistory(c.Request().Context(), typeID, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}
