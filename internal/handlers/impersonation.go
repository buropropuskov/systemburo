package handlers

import (
	"strconv"

	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ImpersonationHandler - режим «войти как пользователь» (#1912).
type ImpersonationHandler struct {
	service services.ImpersonationService
}

// NewImpersonationHandler создаёт хендлер режима «войти как пользователь».
func NewImpersonationHandler(service services.ImpersonationService) *ImpersonationHandler {
	return &ImpersonationHandler{service: service}
}

// Start godoc
// @Summary      Войти от имени пользователя
// @Description  Выдаёт маркер доступа от имени указанного пользователя на 30 минут. Требует право user.impersonate. Войти от имени более полномочного нельзя.
// @Tags         users
// @Produce      json
// @Param        id path int true "ID пользователя"
// @Success      200 {object} models.ImpersonationResponse
// @Failure      403 {object} models.HTTPError "Нет права или у цели прав больше"
// @Failure      404 {object} models.HTTPError "Пользователь не найден"
// @Router       /users/{id}/impersonate [post]
func (h *ImpersonationHandler) Start(c echo.Context) error {
	actorUserID, _ := c.Get("user_id").(int)
	if actorUserID == 0 {
		return apperr.Unauthorized("Требуется авторизация")
	}
	targetUserID, err := strconv.Atoi(c.Param("id"))
	if err != nil || targetUserID <= 0 {
		return apperr.Validation("Некорректный идентификатор пользователя")
	}

	resp, err := h.service.Start(c.Request().Context(), actorUserID, targetUserID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// Stop godoc
// @Summary      Вернуться в свою учётную запись
// @Description  Закрывает сеанс работы от чужого имени записью в журнал. Вызывается с маркером режима; свою учётную запись клиент возвращает обычным обновлением маркера.
// @Tags         users
// @Produce      json
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError "Запрос идёт не в режиме работы от чужого имени"
// @Router       /impersonation/stop [post]
func (h *ImpersonationHandler) Stop(c echo.Context) error {
	// Инициатор берётся из маркера доступа, а не из тела запроса: иначе выход можно
	// было бы записать на чужое имя.
	actorUserID, ok := services.ImpersonatorFromContext(c.Request().Context())
	if !ok {
		return apperr.Validation("Вы не работаете от имени другого пользователя")
	}
	targetUserID, _ := c.Get("user_id").(int)

	if err := h.service.Stop(c.Request().Context(), actorUserID, targetUserID); err != nil {
		return err
	}
	return RespondMessage(c, "Сеанс работы от имени другого пользователя завершён")
}
