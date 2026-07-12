package handlers

import (
	"net/http"

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

// BulkBan godoc
// @Summary      Групповая блокировка пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUserBanRequest true "Список username и причина"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/ban [post]
//
// BulkBan -- POST /users/bulk/ban. Групповая блокировка по списку username.
// Тот же гейт action.ban.user, что у одиночного. Частичный успех -> 207.
func (h *UserBanHandler) BulkBan(c echo.Context) error {
	var req services.BulkUserBanRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	actorID := GetUserID(c)
	res, err := h.service.BulkBan(c.Request().Context(), actorID, req.Usernames, req.Reason)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkUnban godoc
// @Summary      Групповая разблокировка пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUsernamesRequest true "Список username"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/unban [post]
//
// BulkUnban -- POST /users/bulk/unban. Групповая разблокировка по списку username.
func (h *UserBanHandler) BulkUnban(c echo.Context) error {
	var req services.BulkUsernamesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	actorID := GetUserID(c)
	res, err := h.service.BulkUnban(c.Request().Context(), actorID, req.Usernames)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}
