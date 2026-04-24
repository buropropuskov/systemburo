package handlers

import (
	"errors"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type BugReportHandler struct {
	service services.BugReportService
}

// NewBugReportHandler создаёт хендлер для POST /api/bug-report.
func NewBugReportHandler(service services.BugReportService) *BugReportHandler {
	return &BugReportHandler{service: service}
}

// Submit godoc
// @Summary      Отправить отчёт о баге
// @Description  Принимает с фронта bug_hash и контекст 500-ошибки, записывает
// @Description  в bug_reports и асинхронно отправляет в Telegram. Один юзер -
// @Description  один репорт на конкретный bug_hash (409 при повторе).
// @Tags         bug-report
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.BugReportRequest true "Контекст ошибки"
// @Success      200 {object} models.BugReport
// @Failure      400 {object} models.HTTPError "Invalid request"
// @Failure      401 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError "Already reported"
// @Failure      429 {object} models.HTTPError "Rate limit exceeded"
// @Router       /bug-report [post]
func (h *BugReportHandler) Submit(c echo.Context) error {
	var req models.BugReportRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID := c.Get("user_id").(int)
	username, _ := c.Get("username").(string)

	// UserAgent обрезаем на границе базы (size:255) на стороне модели,
	// но лучше обрежем тут явно, чтобы не полагаться на silent truncate БД.
	ua := c.Request().UserAgent()
	if len(ua) > 255 {
		ua = ua[:255]
	}

	report, err := h.service.Submit(c.Request().Context(), userID, username, req, ua)
	if err != nil {
		if errors.Is(err, services.ErrBugAlreadyReported) {
			return echo.NewHTTPError(http.StatusConflict, "Вы уже отправляли отчёт об этом баге")
		}
		return err
	}
	return RespondSuccess(c, report)
}
