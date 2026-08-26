package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// FeedbackHandler -- HTTP-обработчики обратной связи.
type FeedbackHandler struct {
	service services.FeedbackService
}

// NewFeedbackHandler создаёт новый экземпляр FeedbackHandler.
func NewFeedbackHandler(service services.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{service: service}
}

// Create godoc
// @Summary      Создание нового обращения
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateFeedbackRequest true "Текст обращения"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /feedback [post]
func (h *FeedbackHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)
	var req models.CreateFeedbackRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.Create(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Сообщение отправлено успешно",
	})
}

// GetAll godoc
// @Summary      Получение всех обращений (для администраторов)
// @Tags         feedback
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.FeedbackWithUser
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /feedback/all [get]
func (h *FeedbackHandler) GetAll(c echo.Context) error {
	username := c.Get("username").(string)
	feedbacks, err := h.service.GetAll(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, feedbacks)
}

// GetStats godoc
// @Summary      Получение статистики по обращениям
// @Tags         feedback
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.FeedbackStats
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /feedback/stats [get]
func (h *FeedbackHandler) GetStats(c echo.Context) error {
	username := c.Get("username").(string)
	stats, err := h.service.GetStats(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, stats)
}

// GetMy godoc
// @Summary      Получение обращений текущего пользователя
// @Tags         feedback
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.MyFeedback
// @Failure      401 {object} models.HTTPError
// @Router       /feedback/my [get]
func (h *FeedbackHandler) GetMy(c echo.Context) error {
	username := c.Get("username").(string)
	feedbacks, err := h.service.GetMy(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, feedbacks)
}

// UpdateStatus godoc
// @Summary      Обновление статуса обращения
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID обращения"
// @Param        request body models.UpdateFeedbackStatusRequest true "Новый статус"
// @Success      200 {string} string "Статус обращения успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /feedback/{id}/status [put]
func (h *FeedbackHandler) UpdateStatus(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateFeedbackStatusRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID := GetUserID(c)
	if err := h.service.UpdateStatus(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Статус обращения успешно обновлен")
}

// MarkAsRead godoc
// @Summary      Отметить обращение прочитанным (персонально)
// @Description  Фиксирует прочтение обращения текущим администратором. Идемпотентно.
// @Tags         feedback
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID обращения"
// @Success      200 {string} string "Обращение отмечено прочитанным"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /feedback/{id}/read [put]
func (h *FeedbackHandler) MarkAsRead(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	username := c.Get("username").(string)
	if err := h.service.MarkAsRead(c.Request().Context(), id, username); err != nil {
		return err
	}
	return RespondMessage(c, "Обращение отмечено прочитанным")
}

// SetFlag godoc
// @Summary      Установить/снять общий флажок обращения
// @Description  Общий флажок "важное / взять в работу", виден всем администраторам.
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID обращения"
// @Param        request body models.SetFlagRequest true "Состояние флажка"
// @Success      200 {string} string "Флажок обращения обновлён"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /feedback/{id}/flag [put]
func (h *FeedbackHandler) SetFlag(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.SetFlagRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetFlag(c.Request().Context(), id, req.Flagged); err != nil {
		return err
	}
	return RespondMessage(c, "Флажок обращения обновлён")
}
