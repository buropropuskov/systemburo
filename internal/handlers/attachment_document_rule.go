package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

func (h *AttachmentTemplateHandler) DocumentAtPost(resolver *services.PermissionResolver) echo.HandlerFunc {
	return func(c echo.Context) error {
		templateID, err := strconv.Atoi(c.Param("id"))
		if err != nil || templateID <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректный вид заявки")
		}
		ctx := c.Request().Context()
		userID := GetUserID(c)
		set, err := resolver.ResolveFresh(ctx, userID)
		if err != nil {
			return err
		}
		if set.IsBanned() || set.IsInactive() {
			return echo.NewHTTPError(http.StatusForbidden)
		}
		allowed, err := h.fieldConfig.DocumentAtPostAllowed(ctx, templateID, userID)
		if err != nil {
			return err
		}
		return RespondSuccess(c, map[string]bool{"allowed": allowed})
	}
}

func (h *AttachmentTemplateHandler) DocumentRule(resolver *services.PermissionResolver) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		set, err := resolver.ResolveFresh(ctx, GetUserID(c))
		if err != nil {
			return err
		}
		if set.IsBanned() || set.IsInactive() || (!set.IsAdmin() && !set.IsSuperAdmin()) {
			return echo.NewHTTPError(http.StatusForbidden)
		}
		templateID, err := strconv.Atoi(c.Param("template_id"))
		if err != nil || templateID <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректный вид заявки")
		}
		id, err := ParseID(c, "id")
		if err != nil {
			return err
		}
		var request struct {
			Enabled *bool `json:"enabled"`
		}
		if c.Request().Method == http.MethodPut {
			if err := c.Bind(&request); err != nil || request.Enabled == nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Укажите состояние правила")
			}
		}
		enabled, err := h.fieldConfig.DocumentRule(ctx, templateID, id, request.Enabled)
		if err != nil {
			return err
		}
		return RespondSuccess(c, map[string]bool{"enabled": enabled})
	}
}
