package handlers

import (
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UserBanHandler -- HTTP-обработчики бана и разбана.
type UserBanHandler struct {
	service *services.UserBanService
}

// NewUserBanHandler конструирует handler.
func NewUserBanHandler(s *services.UserBanService) *UserBanHandler {
	return &UserBanHandler{service: s}
}

// Ban -- POST /users/:id/ban.
// Проверка прав делается через middleware (action.ban.user).
// Тело {reason} опционально -- причина блокировки для показа заблокированному в ЛК.
func (h *UserBanHandler) Ban(c echo.Context) error {
	targetID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	// Причина -- необязательная метадата: пустое/отсутствующее тело трактуем как
	// бан без причины, а не как ошибку запроса (бан срабатывает в любом случае).
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind(&req)
	actorID := GetUserID(c)
	if err := h.service.Ban(c.Request().Context(), targetID, actorID, req.Reason); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"banned": true})
}

// Unban -- POST /users/:id/unban.
func (h *UserBanHandler) Unban(c echo.Context) error {
	targetID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	actorID := GetUserID(c)
	if err := h.service.Unban(c.Request().Context(), targetID, actorID); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"unbanned": true})
}
