package handlers

import (
	"net/http"

	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// Original files preserve historical data. Resolve the effective user's current
// state instead of trusting a role recorded in a token or download ticket.
func requireArchiveOriginal(c echo.Context, resolver *services.PermissionResolver) error {
	if resolver == nil {
		return apperr.New(http.StatusServiceUnavailable, "Не удалось проверить доступ к архивному оригиналу")
	}
	set, err := resolver.ResolveFresh(c.Request().Context(), GetUserID(c))
	if err != nil {
		return err
	}
	if set.IsBanned() || set.IsInactive() || !set.IsSuperAdmin() {
		return apperr.Forbidden("Исторический оригинал доступен только суперадминистратору. Актуальный бланк можно сформировать в заявке")
	}
	return nil
}
