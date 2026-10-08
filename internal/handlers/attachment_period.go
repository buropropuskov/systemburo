package handlers

import (
	"context"
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type AttachmentPeriodCommands interface {
	Preview(context.Context, int, int, services.ChangeAttachmentPeriodRequest) (*services.AttachmentPeriodCommandResult, error)
	Change(context.Context, int, int, services.ChangeAttachmentPeriodRequest) (*services.AttachmentPeriodCommandResult, error)
}

type AttachmentPeriodHandler struct {
	commands AttachmentPeriodCommands
}

func NewAttachmentPeriodHandler(commands AttachmentPeriodCommands) *AttachmentPeriodHandler {
	return &AttachmentPeriodHandler{commands: commands}
}

// The integration owner registers these authenticated routes after DB review.
// Actor identity is read exclusively from the authenticated request context.
func (h *AttachmentPeriodHandler) Preview(c echo.Context) error {
	return h.run(c, false)
}

func (h *AttachmentPeriodHandler) Change(c echo.Context) error {
	return h.run(c, true)
}

func (h *AttachmentPeriodHandler) run(c echo.Context, change bool) error {
	actorID, applicationID, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис изменения сроков недоступен")
	}
	var req services.ChangeAttachmentPeriodRequest
	// Selection uniqueness, policy default and preview/change-specific revision
	// requirements are conditional domain validation in the service.
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректное тело запроса")
	}
	var result *services.AttachmentPeriodCommandResult
	if change {
		result, err = h.commands.Change(c.Request().Context(), actorID, applicationID, req)
	} else {
		result, err = h.commands.Preview(c.Request().Context(), actorID, applicationID, req)
	}
	if err != nil {
		return err
	}
	// Post-commit notifications/table/blank hooks belong to the integration owner.
	return RespondSuccess(c, result)
}
