package handlers

import (
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// OnboardingHandler — self-service эндпоинты per-user статуса онбординг-тура.
// Каждый авторизованный пользователь читает и помечает прохождение ДЛЯ СЕБЯ
// (userID берётся из JWT-контекста, а не из path/body).
type OnboardingHandler struct {
	service *services.OnboardingService
}

// NewOnboardingHandler создаёт новый экземпляр обработчика онбординга.
func NewOnboardingHandler(service *services.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{service: service}
}

// markCompleteRequest — тело POST /onboarding/complete.
type markCompleteRequest struct {
	Version int `json:"version" validate:"gte=1"`
}

// GetStatus godoc
// @Summary      Статус онбординг-тура текущего пользователя
// @Description  Возвращает версию тура, которую прошёл пользователь. null — не проходил.
// @Tags         onboarding
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{} "completed_version: int|null"
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /onboarding [get]
func (h *OnboardingHandler) GetStatus(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	version, err := h.service.GetCompletedVersion(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"completed_version": version})
}

// MarkComplete godoc
// @Summary      Отметить прохождение онбординг-тура
// @Description  Сохраняет версию тура, которую прошёл текущий пользователь.
// @Tags         onboarding
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.markCompleteRequest true "Версия пройденного тура (>=1)"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /onboarding/complete [post]
func (h *OnboardingHandler) MarkComplete(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	var req markCompleteRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetCompleted(c.Request().Context(), userID, req.Version); err != nil {
		return err
	}
	return RespondMessage(c, "Onboarding marked as completed")
}
