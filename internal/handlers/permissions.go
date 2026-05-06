package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PermissionHandler -- HTTP-обработчики разрешений.
type PermissionHandler struct {
	service services.PermissionService
}

// NewPermissionHandler создаёт новый экземпляр обработчика разрешений.
func NewPermissionHandler(service services.PermissionService) *PermissionHandler {
	return &PermissionHandler{service: service}
}

// GetMyPermissions возвращает разрешения текущего пользователя.
func (h *PermissionHandler) GetMyPermissions(c echo.Context) error {
	username := GetUsername(c)
	perms, err := h.service.GetMyPermissions(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, perms)
}

// GetUserPermissions возвращает разрешения указанного пользователя (только super-admin).
func (h *PermissionHandler) GetUserPermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	perms, err := h.service.GetUserPermissions(c.Request().Context(), IsSuperAdmin(c), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, perms)
}

// UpdateUserPermissions обновляет разрешения указанного пользователя (только super-admin).
func (h *PermissionHandler) UpdateUserPermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdatePermissionsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateUserPermissions(c.Request().Context(), IsSuperAdmin(c), GetUserID(c), userID, req); err != nil {
		return err
	}
	return RespondMessage(c, "ok")
}

// GetPermissionTree возвращает дерево разрешений для админского UI.
func (h *PermissionHandler) GetPermissionTree(c echo.Context) error {
	tree, err := h.service.GetPermissionTree(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, tree)
}

// AutoGenerate создаёт разрешения для таблицы (только admin).
func (h *PermissionHandler) AutoGenerate(c echo.Context) error {
	var req models.AutoGenerateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.AutoGenerateForTable(c.Request().Context(), req.TableID, req.TableName); err != nil {
		return err
	}
	return RespondMessage(c, "ok")
}
