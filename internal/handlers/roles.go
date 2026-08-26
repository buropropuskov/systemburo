package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// RoleHandler -- HTTP-обработчики для управления ролями.
type RoleHandler struct {
	service *services.RoleService
}

// NewRoleHandler конструирует handler.
func NewRoleHandler(service *services.RoleService) *RoleHandler {
	return &RoleHandler{service: service}
}

// List -- GET /roles.
func (h *RoleHandler) List(c echo.Context) error {
	roles, err := h.service.List(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, roles)
}

// Create -- POST /roles.
func (h *RoleHandler) Create(c echo.Context) error {
	var req models.CreateRoleRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	role, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondCreated(c, role)
}

// Update -- PUT /roles/:id.
func (h *RoleHandler) Update(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdateRoleRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"updated": true})
}

// Delete -- DELETE /roles/:id.
func (h *RoleHandler) Delete(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"deleted": true})
}

// SetDefaultGroups -- PUT /roles/:id/default-groups.
func (h *RoleHandler) SetDefaultGroups(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.SetRoleGroupsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetDefaultGroups(c.Request().Context(), id, req.GroupIDs); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"updated": true})
}

// SetPermissions -- PUT /roles/:id/permissions. Полная замена прямых грантов роли.
func (h *RoleHandler) SetPermissions(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.SetRolePermissionsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetPermissions(c.Request().Context(), id, req.Keys); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"updated": true})
}
