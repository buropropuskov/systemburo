package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// CreateSupplement godoc
// @Summary      Дополнить поданную заявку
// @Description  Добавляет людей, машины или ТМЦ во вложения уже поданной заявки (#1685).
// @Description  Пока заявка не принята в работу, добавка вливается в текущий круг согласования
// @Description  (статус раунда merged); у заявки в работе заводится отдельный раунд (pending),
// @Description  а согласование и статус самой заявки не откатываются - уже допущенные строки
// @Description  остаются на КПП.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        request body services.CreateSupplementRequest true "Состав дополнения"
// @Success      200 {object} services.CreateSupplementResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements [post]
func (h *ApplicationHandler) CreateSupplement(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.CreateSupplementRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	username := c.Get("username").(string)
	resp, err := h.service.CreateSupplement(c.Request().Context(), username, id, IsSuperAdmin(c), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetApplicationSupplements godoc
// @Summary      Дополнения заявки
// @Description  Возвращает раунды дополнения заявки (новые сверху) с составом голосующих.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.SupplementInfo
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements [get]
func (h *ApplicationHandler) GetApplicationSupplements(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	supplements, err := h.service.GetApplicationSupplements(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, supplements)
}

// ApproveSupplement godoc
// @Summary      Голос по дополнению заявки
// @Description  Согласование или отказ по раунду дополнения (#1685). Голосуют только
// @Description  согласующие раунда - снимок ответственных заявки на момент подачи дополнения;
// @Description  голосующий берётся из токена. Итог раунда пересчитывается по тому же кворуму,
// @Description  что и согласование заявки, но пишется в статус дополнения: согласование и
// @Description  статус самой заявки не откатываются.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        sid path int true "ID дополнения"
// @Param        request body services.SupplementApprovalRequest true "Голос"
// @Success      200 {object} services.SupplementVoteResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements/{sid}/approve [post]
func (h *ApplicationHandler) ApproveSupplement(c echo.Context) error {
	id, sid, err := supplementPathIDs(c)
	if err != nil {
		return err
	}

	var req services.SupplementApprovalRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	username := c.Get("username").(string)
	resp, err := h.service.ApproveSupplement(c.Request().Context(), username, id, sid, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// RevokeSupplementApproval godoc
// @Summary      Отзыв голоса по дополнению заявки
// @Description  Возвращает собственный голос по раунду дополнения в pending и пересчитывает
// @Description  итог раунда (#1685). Отозвать голос можно, пока по дополнению не принято
// @Description  решение принимающим.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        sid path int true "ID дополнения"
// @Param        request body services.SupplementRevokeApprovalRequest false "Причина отзыва"
// @Success      200 {object} services.SupplementVoteResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements/{sid}/revoke-approval [post]
func (h *ApplicationHandler) RevokeSupplementApproval(c echo.Context) error {
	id, sid, err := supplementPathIDs(c)
	if err != nil {
		return err
	}

	// Тело намеренно опциональное: отозвать голос можно и без объяснения причины.
	var req services.SupplementRevokeApprovalRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	username := c.Get("username").(string)
	resp, err := h.service.RevokeSupplementApproval(c.Request().Context(), username, id, sid, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// DecideSupplement godoc
// @Summary      Решение принимающего по дополнению заявки
// @Description  Принятие или отказ по согласованному раунду дополнения (#1685). Принятие
// @Description  активирует строки ЭТОГО раунда - с этого момента они видны на КПП; отказ
// @Description  оставляет их неактивными навсегда. Согласование и статус самой заявки не
// @Description  двигаются ни в одной ветке: от них производен допуск уже выданных пропусков.
// @Description  Доступно только принимающему; раунд обязан быть согласован, а заявка - в работе.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        sid path int true "ID дополнения"
// @Param        request body services.SupplementDecisionRequest true "Решение"
// @Success      200 {object} services.SupplementDecisionResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements/{sid}/take-to-work [post]
func (h *ApplicationHandler) DecideSupplement(c echo.Context) error {
	id, sid, err := supplementPathIDs(c)
	if err != nil {
		return err
	}

	var req services.SupplementDecisionRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	username := c.Get("username").(string)
	resp, err := h.service.DecideSupplement(c.Request().Context(), username, id, sid, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// CancelSupplement godoc
// @Summary      Снять дополнение заявки
// @Description  Автор заявки снимает собственный незакрытый раунд дополнения (#1685). Строки
// @Description  раунда остаются неактивными; заявка и её допущенный состав не задеты.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        sid path int true "ID дополнения"
// @Param        request body services.SupplementCancelRequest false "Причина снятия"
// @Success      200 {object} services.SupplementDecisionResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements/{sid}/cancel [post]
func (h *ApplicationHandler) CancelSupplement(c echo.Context) error {
	id, sid, err := supplementPathIDs(c)
	if err != nil {
		return err
	}

	// Тело намеренно опциональное: снять раунд можно и без объяснения причины.
	var req services.SupplementCancelRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	username := c.Get("username").(string)
	resp, err := h.service.CancelSupplement(c.Request().Context(), username, id, sid, IsSuperAdmin(c), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// supplementPathIDs разбирает пару «заявка + дополнение» из пути.
func supplementPathIDs(c echo.Context) (int, int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	sid, err := strconv.Atoi(c.Param("sid"))
	if err != nil {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid supplement ID")
	}
	return id, sid, nil
}
