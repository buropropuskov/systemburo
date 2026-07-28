package handlers

import (
	"net/http"
	"time"

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
	if !IsSuperAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ только для супер-администратора")
	}
	st, err := h.service.GetStatus(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, st)
}

// MaintenanceToggleRequest — тело PUT /api/admin/maintenance.
// PlannedStart/PlannedEnd — объявленное окно работ в RFC3339; либо оба
// заданы, либо оба пусты (режим без объявленного срока).
type MaintenanceToggleRequest struct {
	Enabled      bool   `json:"enabled"`
	Message      string `json:"message" validate:"max=500"`
	PlannedStart string `json:"planned_start" validate:"max=40"`
	PlannedEnd   string `json:"planned_end" validate:"max=40"`
	SupportEmail string `json:"support_email" validate:"max=255"`
	SupportPhone string `json:"support_phone" validate:"max=40"`
}

// parseWindow разбирает объявленное окно работ и нормализует его в UTC.
// Окно опционально, но задаётся целиком: с одной половиной страница техработ
// не смогла бы показать ни срок окончания, ни прогресс.
func parseWindow(req MaintenanceToggleRequest) (start, end string, err error) {
	if req.PlannedStart == "" && req.PlannedEnd == "" {
		return "", "", nil
	}
	if req.PlannedStart == "" || req.PlannedEnd == "" {
		return "", "", echo.NewHTTPError(http.StatusBadRequest,
			"укажите и начало, и окончание технических работ")
	}
	startAt, parseErr := time.Parse(time.RFC3339, req.PlannedStart)
	if parseErr != nil {
		return "", "", echo.NewHTTPError(http.StatusBadRequest, "некорректная дата начала работ")
	}
	endAt, parseErr := time.Parse(time.RFC3339, req.PlannedEnd)
	if parseErr != nil {
		return "", "", echo.NewHTTPError(http.StatusBadRequest, "некорректная дата окончания работ")
	}
	if !endAt.After(startAt) {
		return "", "", echo.NewHTTPError(http.StatusBadRequest,
			"окончание работ должно быть позже начала")
	}
	return startAt.UTC().Format(time.RFC3339), endAt.UTC().Format(time.RFC3339), nil
}

// ToggleMaintenance включает или выключает режим обслуживания.
// При Enable=true дополнительно revoke non-admin refresh_tokens
// (см. MaintenanceService.Enable).
func (h *MaintenanceHandler) ToggleMaintenance(c echo.Context) error {
	if !IsSuperAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ только для супер-администратора")
	}
	userID, _ := c.Get("user_id").(int)
	username, _ := c.Get("username").(string)

	var req MaintenanceToggleRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if req.Enabled {
		plannedStart, plannedEnd, err := parseWindow(req)
		if err != nil {
			return err
		}
		params := services.MaintenanceParams{
			Message:      req.Message,
			PlannedStart: plannedStart,
			PlannedEnd:   plannedEnd,
			SupportEmail: req.SupportEmail,
			SupportPhone: req.SupportPhone,
		}
		if err := h.service.Enable(c.Request().Context(), userID, username, params); err != nil {
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
