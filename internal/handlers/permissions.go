package handlers

import (
	"net/http"

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
	return c.JSON(http.StatusOK, perms)
}

// GetUserPermissions возвращает разрешения указанного пользователя (только admin).
func (h *PermissionHandler) GetUserPermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	typeID := GetTypeID(c)

	perms, err := h.service.GetUserPermissions(c.Request().Context(), typeID, userID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, perms)
}

// UpdateUserPermissions обновляет разрешения указанного пользователя (только admin).
func (h *PermissionHandler) UpdateUserPermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	typeID := GetTypeID(c)

	var req models.UpdatePermissionsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.UpdateUserPermissions(c.Request().Context(), typeID, userID, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// GetPermissionTree возвращает дерево разрешений для админского UI.
func (h *PermissionHandler) GetPermissionTree(c echo.Context) error {
	tree, err := h.service.GetPermissionTree(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tree)
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
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
