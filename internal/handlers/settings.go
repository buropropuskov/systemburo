package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type SettingsHandler struct {
	service services.SettingsService
}

// NewSettingsHandler создаёт хендлер для управления системными настройками.
func NewSettingsHandler(service services.SettingsService) *SettingsHandler {
	return &SettingsHandler{service: service}
}

// GetAll возвращает все системные настройки (только для super-admin).
func (h *SettingsHandler) GetAll(c echo.Context) error {
	settings, err := h.service.GetAll(c.Request().Context(), IsSuperAdmin(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// Update обновляет значение конкретной настройки по ключу.
func (h *SettingsHandler) Update(c echo.Context) error {
	key := c.Param("key")
	var req models.UpdateSettingRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	setting, err := h.service.Update(c.Request().Context(), IsSuperAdmin(c), key, req.Value)
	if err != nil {
		return err
	}
	return RespondSuccess(c, setting)
}

// GetUploadSettings возвращает настройки загрузки файлов для фронтенда.
func (h *SettingsHandler) GetUploadSettings(c echo.Context) error {
	result, err := h.service.GetUploadSettings(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}
