package handlers

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PermissionGroupHandler -- HTTP-обработчики для управления группами прав.
type PermissionGroupHandler struct {
	service *services.PermissionGroupService
}

// NewPermissionGroupHandler конструирует handler.
func NewPermissionGroupHandler(service *services.PermissionGroupService) *PermissionGroupHandler {
	return &PermissionGroupHandler{service: service}
}

// List -- GET /permission-groups.
func (h *PermissionGroupHandler) List(c echo.Context) error {
	groups, err := h.service.List(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, groups)
}

// Get -- GET /permission-groups/:id.
func (h *PermissionGroupHandler) Get(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	group, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, group)
}

// Create -- POST /permission-groups.
func (h *PermissionGroupHandler) Create(c echo.Context) error {
	var req models.CreatePermissionGroupRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	group, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondCreated(c, group)
}

// Update -- PUT /permission-groups/:id.
func (h *PermissionGroupHandler) Update(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdatePermissionGroupRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"updated": true})
}

// Delete -- DELETE /permission-groups/:id.
func (h *PermissionGroupHandler) Delete(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"deleted": true})
}

// Merge -- POST /permission-groups/merge.
func (h *PermissionGroupHandler) Merge(c echo.Context) error {
	var req models.MergePermissionGroupsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	grantedBy := GetUserID(c)
	group, err := h.service.Merge(c.Request().Context(), req, grantedBy)
	if err != nil {
		return err
	}
	return RespondCreated(c, group)
}

// ListForUser -- GET /users/:user_id/permission-groups.
// Возвращает группы прав, явно назначенные пользователю (без дефолтных групп роли).
func (h *PermissionGroupHandler) ListForUser(c echo.Context) error {
	userID, err := ParseID(c, "user_id")
	if err != nil {
		return err
	}
	groups, err := h.service.ListForUser(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, groups)
}

// SetUserRole -- PUT /users/:id/role.
func (h *PermissionGroupHandler) SetUserRole(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req struct {
		RoleID *int `json:"role_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body: "+err.Error())
	}
	if err := h.service.SetUserRole(c.Request().Context(), userID, req.RoleID); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"updated": true})
}

// SetUserAdmin -- PUT /users/:id/admin (super-only через middleware action.grant.admin).
func (h *PermissionGroupHandler) SetUserAdmin(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body: "+err.Error())
	}
	if err := h.service.SetUserAdmin(c.Request().Context(), userID, req.IsAdmin); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"updated": true})
}

// AssignToUser -- POST /users/:user_id/permission-groups/:group_id.
func (h *PermissionGroupHandler) AssignToUser(c echo.Context) error {
	userID, err := ParseID(c, "user_id")
	if err != nil {
		return err
	}
	groupID, err := ParseID(c, "group_id")
	if err != nil {
		return err
	}
	grantedBy := GetUserID(c)
	if err := h.service.AssignToUser(c.Request().Context(), userID, groupID, grantedBy); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"assigned": true})
}

// UnassignFromUser -- DELETE /users/:user_id/permission-groups/:group_id.
func (h *PermissionGroupHandler) UnassignFromUser(c echo.Context) error {
	userID, err := ParseID(c, "user_id")
	if err != nil {
		return err
	}
	groupID, err := ParseID(c, "group_id")
	if err != nil {
		return err
	}
	if err := h.service.UnassignFromUser(c.Request().Context(), userID, groupID); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"unassigned": true})
}
