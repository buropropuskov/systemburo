package handlers

import (
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ThemeHandler — self-service эндпоинты темы оформления (#1415). Каждый
// авторизованный пользователь читает и меняет тему ДЛЯ СЕБЯ: userID берётся из
// JWT-контекста, а не из path/body.
type ThemeHandler struct {
	service *services.ThemeService
}

// NewThemeHandler создаёт новый экземпляр обработчика темы.
func NewThemeHandler(service *services.ThemeService) *ThemeHandler {
	return &ThemeHandler{service: service}
}

// setThemeRequest — тело PUT /users/me/theme. Значение валидируется по реестру
// тем в сервисе (models.IsValidTheme), а не тегом: список живёт в одном месте.
type setThemeRequest struct {
	Theme string `json:"theme" validate:"required"`
}

// GetTheme godoc
// @Summary      Тема оформления текущего пользователя
// @Description  Возвращает выбранную тему. null — пользователь тему не выбирал.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{} "theme: string|null"
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/me/theme [get]
func (h *ThemeHandler) GetTheme(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	theme, err := h.service.Get(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"theme": theme})
}

// SetTheme godoc
// @Summary      Выбрать тему оформления
// @Description  Сохраняет тему в профиле текущего пользователя, чтобы выбор ехал за ним между устройствами.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.setThemeRequest true "Идентификатор темы"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/me/theme [put]
func (h *ThemeHandler) SetTheme(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	var req setThemeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Set(c.Request().Context(), userID, req.Theme); err != nil {
		return err
	}
	return RespondMessage(c, "Theme saved")
}
