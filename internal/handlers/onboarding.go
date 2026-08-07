package handlers

import (
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// OnboardingHandler — self-service эндпоинты per-user прогресса онбординг-туров.
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
	Tour    string `json:"tour" validate:"required"`
	Version int    `json:"version" validate:"gte=1"`
	// Finished - тур доведён до финального шага. false = закрыли на середине:
	// автозапуск гасим, но «Пройден» в меню обучения не показываем.
	Finished bool `json:"finished"`
}

// resetRequest — тело POST /users/:username/onboarding/reset. Ключ опционален:
// отсутствующее поле (как и пустое тело) означает сброс всех туров пользователя.
type resetRequest struct {
	Tour string `json:"tour"`
}

// GetStatus godoc
// @Summary      Прогресс онбординг-туров текущего пользователя
// @Description  Возвращает версию по каждому туру. Ключи присутствуют все, null — тур не пройден.
// @Tags         onboarding
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{} "completed: {tour: int|null}, finished: [tour]"
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /onboarding [get]
func (h *OnboardingHandler) GetStatus(c echo.Context) error {
	completed, finished, err := h.service.GetCompleted(c.Request().Context(), GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"completed": completed, "finished": finished})
}

// MarkComplete godoc
// @Summary      Отметить прохождение онбординг-тура
// @Description  Сохраняет версию тура, которую прошёл текущий пользователь. Версия только растёт.
// @Tags         onboarding
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.markCompleteRequest true "Ключ тура и версия (>=1)"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /onboarding/complete [post]
func (h *OnboardingHandler) MarkComplete(c echo.Context) error {
	var req markCompleteRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetCompleted(c.Request().Context(), GetUserID(c), req.Tour, req.Version, req.Finished); err != nil {
		return err
	}
	return RespondMessage(c, "Onboarding marked as completed")
}

// ResetForUser godoc
// @Summary      Сбросить онбординг-тур пользователю (админ)
// @Description  Снимает прохождение тура у пользователя по username - при следующем входе
// @Description  у него снова сработает автозапуск. Без ключа tour сбрасывает все туры.
// @Description  Только для админов.
// @Tags         onboarding
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Логин пользователя"
// @Param        request body handlers.resetRequest false "Ключ тура; пустое тело сбрасывает все"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/onboarding/reset [post]
func (h *OnboardingHandler) ResetForUser(c echo.Context) error {
	var req resetRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.ResetByUsername(c.Request().Context(), c.Param("username"), req.Tour); err != nil {
		return err
	}
	return RespondMessage(c, "Onboarding reset for user")
}
