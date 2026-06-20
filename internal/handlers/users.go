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
	userID, _ := c.Get("user_id").(int)
	var req models.RegisterRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Create(c.Request().Context(), typeID, userID, req); err != nil {
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
	includeArchived := c.QueryParam("include_archived") == "true"
	result, err := h.service.GetAll(c.Request().Context(), typeID, includeArchived)
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
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserTypeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateType(c.Request().Context(), typeID, userID, username, req); err != nil {
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
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdatePasswordRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdatePassword(c.Request().Context(), typeID, userID, username, req); err != nil {
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
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserInfoRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateInfo(c.Request().Context(), typeID, userID, username, req); err != nil {
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
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserOrganizationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateOrganization(c.Request().Context(), typeID, userID, username, req); err != nil {
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
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateCompany(c.Request().Context(), typeID, userID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company updated successfully")
}

// Delete godoc
// @Summary      Архивация пользователя (soft-delete)
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
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	if err := h.service.Delete(c.Request().Context(), typeID, userID, username); err != nil {
		return err
	}
	return RespondMessage(c, "User archived successfully")
}

// Restore godoc
// @Summary      Восстановление пользователя из архива
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string true "Имя пользователя"
// @Success      200 {object} map[string]string "message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/restore [post]
func (h *UsersHandler) Restore(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	if err := h.service.Restore(c.Request().Context(), typeID, userID, username); err != nil {
		return err
	}
	return RespondMessage(c, "User restored successfully")
}

// GetHistory godoc
// @Summary      История изменений пользователя
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path int true "Имя пользователя"
// @Success      200 {array} models.UserHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/history [get]
func (h *UsersHandler) GetHistory(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	items, err := h.service.GetHistory(c.Request().Context(), typeID, username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetUserUnloadPlaces godoc
// @Summary      Получение мест разгрузки охранника
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Success      200 {array} models.UnloadPlace
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/unload-places [get]
func (h *UsersHandler) GetUserUnloadPlaces(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	places, err := h.service.GetUserUnloadPlaces(c.Request().Context(), typeID, username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// SetUserUnloadPlaces godoc
// @Summary      Замена мест разгрузки охранника
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Param        request body models.SetUserUnloadPlacesRequest true "Список ID мест разгрузки"
// @Success      200 {string} string "Unload places updated successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/unload-places [put]
func (h *UsersHandler) SetUserUnloadPlaces(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.SetUserUnloadPlacesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetUserUnloadPlaces(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Unload places updated successfully")
}

// GetUserTables godoc
// @Summary      Получение мест прохода охранника
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Success      200 {array} models.SystemTable
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/tables [get]
func (h *UsersHandler) GetUserTables(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	tables, err := h.service.GetUserTables(c.Request().Context(), typeID, username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, tables)
}

// SetUserTables godoc
// @Summary      Замена мест прохода охранника
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Param        request body models.SetUserTablesRequest true "Список ID мест прохода"
// @Success      200 {string} string "Tables updated successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/tables [put]
func (h *UsersHandler) SetUserTables(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Param("username")
	var req models.SetUserTablesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetUserTables(c.Request().Context(), typeID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Tables updated successfully")
}
