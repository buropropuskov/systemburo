package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// GetApplicationQuestions godoc
// @Summary      Вопросы к заявке
// @Description  Возвращает вопросы к заявке (#973) с вложенными ответами и вложениями. Видно всем, у кого есть доступ к заявке (включая инициатора).
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.QuestionWithAnswers
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /applications/{id}/questions [get]
func (h *ApplicationHandler) GetApplicationQuestions(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	// forwardViewerID для фильтра пер-вложенного пересыла (#680): 0 - супер-админ (видит все).
	// readerID - реальный id для флага IsNew (per-топик прочтение), у супера тоже реальный.
	readerID := GetUserID(c)
	forwardViewerID := readerID
	if IsSuperAdmin(c) {
		forwardViewerID = 0
	}
	questions, err := h.service.GetApplicationQuestions(c.Request().Context(), id, forwardViewerID, readerID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, questions)
}

// CreateApplicationQuestion godoc
// @Summary      Задать вопрос к заявке
// @Description  Создаёт вопрос-топик (#973). Доступно любому с доступом к заявке, включая инициатора. Пишется в историю, инициатору уходит уведомление (если он не автор).
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        body body services.CreateQuestionRequest true "Вопрос"
// @Success      201 {object} services.QuestionWithAnswers
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /applications/{id}/questions [post]
func (h *ApplicationHandler) CreateApplicationQuestion(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	var req services.CreateQuestionRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	question, err := h.service.CreateApplicationQuestion(c.Request().Context(), username, id, IsSuperAdmin(c), req)
	if err != nil {
		return err
	}
	return RespondCreated(c, question)
}

// CreateApplicationAnswer godoc
// @Summary      Ответить на вопрос к заявке
// @Description  Добавляет ответ в тред вопроса (#973). Доступно любому с доступом к заявке. Участникам обсуждения уходит уведомление.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        questionId path int true "ID вопроса"
// @Param        body body services.CreateAnswerRequest true "Ответ"
// @Success      201 {object} services.AnswerItem
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /applications/{id}/questions/{questionId}/answers [post]
func (h *ApplicationHandler) CreateApplicationAnswer(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	questionID, err := strconv.Atoi(c.Param("questionId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid question ID")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	var req services.CreateAnswerRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	answer, err := h.service.CreateApplicationAnswer(c.Request().Context(), username, id, questionID, req)
	if err != nil {
		return err
	}
	return RespondCreated(c, answer)
}

// MarkQuestionsSeen godoc
// @Summary      Отметить вопросы заявки просмотренными
// @Description  Обновляет last-seen пользователя по Q&A заявки (#973), гасит маркер "новые вопросы/ответы".
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]string
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /applications/{id}/questions/seen [post]
func (h *ApplicationHandler) MarkQuestionsSeen(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	if err := h.service.MarkQuestionsSeen(c.Request().Context(), username, id); err != nil {
		return err
	}
	return RespondMessage(c, "Questions marked as seen")
}

// MarkQuestionRead godoc
// @Summary      Отметить вопрос-топик прочитанным
// @Description  Помечает конкретный вопрос-топик прочитанным (#973): гасит его новизну для пользователя. Недочитанные топики остаются новыми.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        questionId path int true "ID вопроса"
// @Success      200 {object} map[string]string
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /applications/{id}/questions/{questionId}/read [post]
func (h *ApplicationHandler) MarkQuestionRead(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	questionID, err := strconv.Atoi(c.Param("questionId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid question ID")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	if err := h.service.MarkQuestionRead(c.Request().Context(), username, id, questionID); err != nil {
		return err
	}
	return RespondMessage(c, "Question marked as read")
}
