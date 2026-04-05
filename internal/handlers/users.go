package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UsersHandler — HTTP-обработчики управления пользователями (admin-only).
type UsersHandler struct {
	service services.UserService
}

// NewUsersHandler создаёт новый экземпляр обработчика пользователей.
func NewUsersHandler(service services.UserService) *UsersHandler {
	return &UsersHandler{service: service}
}

// Create godoc
// @Summary      Создание пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.RegisterRequest true "Данные нового пользователя"
// @Success      200 {string} string "User created successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users [post]
func (h *UsersHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	var req models.RegisterRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Create(c.Request().Context(), typeID, req); err != nil {
		return err
	}
	return RespondMessage(c, "User created successfully")
}

// GetAll godoc
// @Summary      Получение списка всех пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  models.UserInfoResponse
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/all [get]
func (h *UsersHandler) GetAll(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	result, err := h.service.GetAll(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// UpdateType godoc
// @Summary      Обновление типа пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                     true "Имя пользователя"
// @Param        request  body   models.UpdateUserTypeRequest true "Новый type_id"
// @Success      200 {string} string "User type updated successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/type [put]
func (h *UsersHandler) UpdateType(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.UpdateUserTypeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateType(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "User type updated successfully")
}

// UpdatePassword godoc
// @Summary      Обновление пароля пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                       true "Имя пользователя"
// @Param        request  body   models.UpdatePasswordRequest true "Новый пароль"
// @Success      200 {string} string "Password updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/password [put]
func (h *UsersHandler) UpdatePassword(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.UpdatePasswordRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdatePassword(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Password updated successfully")
}

// UpdateInfo godoc
// @Summary      Обновление персональных данных пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                       true "Имя пользователя"
// @Param        request  body   models.UpdateUserInfoRequest true "ФИО, должность, email, телефон"
// @Success      200 {string} string "User info updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/info [put]
func (h *UsersHandler) UpdateInfo(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.UpdateUserInfoRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateInfo(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "User info updated successfully")
}

// UpdateOrganization godoc
// @Summary      Обновление организации пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                              true "Имя пользователя"
// @Param        request  body   models.UpdateUserOrganizationRequest true "Новый organization_id"
// @Success      200 {string} string "Organization updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/organization [put]
func (h *UsersHandler) UpdateOrganization(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.UpdateUserOrganizationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateOrganization(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Organization updated successfully")
}

// UpdateCompany godoc
// @Summary      Обновление компании пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                          true "Имя пользователя"
// @Param        request  body   models.UpdateUserCompanyRequest true "Новый company_id"
// @Success      200 {string} string "Company updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/company [put]
func (h *UsersHandler) UpdateCompany(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.UpdateUserCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateCompany(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company updated successfully")
}

// Delete godoc
// @Summary      Удаление пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string true "Имя пользователя"
// @Success      200 {object} map[string]string "message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username} [delete]
func (h *UsersHandler) Delete(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	if err := h.service.Delete(c.Request().Context(), typeID, username); err != nil {
		return err
	}
	return RespondMessage(c, "User deleted successfully")
}
