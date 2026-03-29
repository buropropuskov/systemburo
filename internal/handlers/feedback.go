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
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id, err := h.service.Create(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
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
	typeID := c.Get("type_id").(int)
	feedbacks, err := h.service.GetAll(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, feedbacks)
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
	typeID := c.Get("type_id").(int)
	stats, err := h.service.GetStats(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, stats)
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
	return c.JSON(http.StatusOK, feedbacks)
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
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateFeedbackStatusRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.UpdateStatus(c.Request().Context(), typeID, id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Статус обращения успешно обновлен")
}

// MarkAsRead godoc
// @Summary      Отметить обращение как прочитанное/непрочитанное
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID обращения"
// @Param        request body models.MarkAsReadRequest true "Статус прочтения"
// @Success      200 {string} string "Статус прочтения обновлен"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /feedback/{id}/read [put]
func (h *FeedbackHandler) MarkAsRead(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.MarkAsReadRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.MarkAsRead(c.Request().Context(), typeID, id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Статус прочтения обновлен")
}
