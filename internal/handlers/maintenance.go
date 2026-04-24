package handlers

import (
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// MaintenanceHandler обслуживает три endpoint'а:
//   - GET  /api/settings/maintenance  (public, без JWT) - статус для /maintenance и /login
//   - GET  /api/admin/maintenance     (admin) - полный статус + message
//   - PUT  /api/admin/maintenance     (admin) - переключение режима
type MaintenanceHandler struct {
	service services.MaintenanceService
}

func NewMaintenanceHandler(service services.MaintenanceService) *MaintenanceHandler {
	return &MaintenanceHandler{service: service}
}

// GetPublicStatus возвращает публичный статус режима техработ (без auth).
// Используется страницей /maintenance для авто-рефреша и формой /login для
// проверки, стоит ли давать возможность логиниться.
func (h *MaintenanceHandler) GetPublicStatus(c echo.Context) error {
	st, err := h.service.GetStatus(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, st)
}

// GetAdminStatus возвращает тот же статус для админского экрана SystemControl.
// Оставлен как отдельный метод - если позже понадобятся доп. поля только для
// админа (например, кто включил и когда), их можно добавить только сюда.
func (h *MaintenanceHandler) GetAdminStatus(c echo.Context) error {
	typeID, _ := c.Get("type_id").(int)
	if typeID != 6 {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ только для бюро пропусков")
	}
	st, err := h.service.GetStatus(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, st)
}

// MaintenanceToggleRequest — тело PUT /api/admin/maintenance.
type MaintenanceToggleRequest struct {
	Enabled      bool   `json:"enabled"`
	Message      string `json:"message" validate:"max=500"`
	SupportEmail string `json:"support_email" validate:"max=255"`
}

// ToggleMaintenance включает или выключает режим обслуживания.
// При Enable=true дополнительно revoke non-admin refresh_tokens
// (см. MaintenanceService.Enable).
func (h *MaintenanceHandler) ToggleMaintenance(c echo.Context) error {
	typeID, _ := c.Get("type_id").(int)
	if typeID != 6 {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ только для бюро пропусков")
	}
	userID, _ := c.Get("user_id").(int)
	username, _ := c.Get("username").(string)

	var req MaintenanceToggleRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if req.Enabled {
		if err := h.service.Enable(c.Request().Context(), userID, username, req.Message, req.SupportEmail); err != nil {
			return err
		}
	} else {
		if err := h.service.Disable(c.Request().Context(), userID, username); err != nil {
			return err
		}
	}
	st, err := h.service.GetStatus(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, st)
}
