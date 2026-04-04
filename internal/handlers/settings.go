package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type SettingsHandler struct {
	service services.SettingsService
}

func NewSettingsHandler(service services.SettingsService) *SettingsHandler {
	return &SettingsHandler{service: service}
}

func (h *SettingsHandler) GetAll(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	settings, err := h.service.GetAll(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

func (h *SettingsHandler) Update(c echo.Context) error {
	key := c.Param("key")
	var req models.UpdateSettingRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	typeID := c.Get("type_id").(int)
	setting, err := h.service.Update(c.Request().Context(), typeID, key, req.Value)
	if err != nil {
		return err
	}
	return RespondSuccess(c, setting)
}

func (h *SettingsHandler) GetUploadSettings(c echo.Context) error {
	result, err := h.service.GetUploadSettings(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}
