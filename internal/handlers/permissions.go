package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PermissionHandler -- HTTP-обработчики разрешений.
type PermissionHandler struct {
	service  services.PermissionService
	resolver *services.PermissionResolver
}

// NewPermissionHandler создаёт новый экземпляр обработчика разрешений.
// resolver используется для GetMyPermissions (новая система прав #187),
// service остаётся для legacy /permissions/tree, /user/:id и auto-generate.
func NewPermissionHandler(service services.PermissionService, resolver *services.PermissionResolver) *PermissionHandler {
	return &PermissionHandler{service: service, resolver: resolver}
}

// GetMyPermissions возвращает разрешения текущего пользователя в виде
// массива {key, value:"allow"} - формат сохранён ради backward-compat
// с usePermissionsStore на фронте. Источник данных - PermissionResolver
// из #187 (roles + permission_groups + user_groups + overrides), а не
// старая таблица user_permissions (она устарела и не отражает реальные
// права после миграции на новую систему).
func (h *PermissionHandler) GetMyPermissions(c echo.Context) error {
	userID := GetUserID(c)
	set, err := h.resolver.Resolve(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	keys := set.Keys()
	perms := make([]models.UserPermissionResponse, 0, len(keys))
	for _, k := range keys {
		perms = append(perms, models.UserPermissionResponse{
			Key:   k,
			Value: "allow",
		})
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
