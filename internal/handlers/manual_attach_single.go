package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"systemburo/internal/services"
)

type SingleManualAttachCommands interface {
	Context(context.Context, int, services.ElementKind, int, *int) (*services.SingleManualAttachContext, error)
	Attachments(context.Context, int, services.ElementKind, int, int, *int) ([]services.SingleManualAttachAttachment, error)
	Preview(context.Context, int, services.ElementKind, int, services.SingleManualAttachRequest) (*services.SingleManualAttachResult, error)
	Attach(context.Context, int, services.ElementKind, int, services.SingleManualAttachRequest) (*services.SingleManualAttachResult, error)
}

func (h *SingleManualAttachHandler) GetEmployeeAttachments(c echo.Context) error {
	return h.attachments(c, services.ElementEmployee)
}
func (h *SingleManualAttachHandler) GetCarAttachments(c echo.Context) error {
	return h.attachments(c, services.ElementCar)
}

func (h *SingleManualAttachHandler) attachments(c echo.Context, kind services.ElementKind) error {
	actor, id, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис привязки недоступен")
	}
	applicationID, err := strconv.Atoi(c.QueryParam("application_id"))
	if err != nil || applicationID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректная заявка")
	}
	var tableID *int
	if raw := c.QueryParam("table_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректная таблица")
		}
		tableID = &id
	}
	result, err := h.commands.Attachments(c.Request().Context(), actor, kind, id, applicationID, tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

type SingleManualAttachHandler struct{ commands SingleManualAttachCommands }

func NewSingleManualAttachHandler(commands SingleManualAttachCommands) *SingleManualAttachHandler {
	return &SingleManualAttachHandler{commands: commands}
}

func (h *SingleManualAttachHandler) GetEmployeeContext(c echo.Context) error {
	return h.context(c, services.ElementEmployee)
}
func (h *SingleManualAttachHandler) GetCarContext(c echo.Context) error {
	return h.context(c, services.ElementCar)
}
func (h *SingleManualAttachHandler) PreviewEmployee(c echo.Context) error {
	return h.command(c, services.ElementEmployee, false)
}
func (h *SingleManualAttachHandler) PreviewCar(c echo.Context) error {
	return h.command(c, services.ElementCar, false)
}
func (h *SingleManualAttachHandler) AttachEmployee(c echo.Context) error {
	return h.command(c, services.ElementEmployee, true)
}
func (h *SingleManualAttachHandler) AttachCar(c echo.Context) error {
	return h.command(c, services.ElementCar, true)
}

func (h *SingleManualAttachHandler) context(c echo.Context, kind services.ElementKind) error {
	actor, id, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис привязки недоступен")
	}
	var table *int
	if raw := c.QueryParam("table_id"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректная таблица")
		}
		table = &value
	}
	result, err := h.commands.Context(c.Request().Context(), actor, kind, id, table)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

func (h *SingleManualAttachHandler) command(c echo.Context, kind services.ElementKind, execute bool) error {
	actor, id, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис привязки недоступен")
	}
	var req services.SingleManualAttachRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректное тело запроса")
	}
	var result *services.SingleManualAttachResult
	if execute {
		result, err = h.commands.Attach(c.Request().Context(), actor, kind, id, req)
	} else {
		result, err = h.commands.Preview(c.Request().Context(), actor, kind, id, req)
	}
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}
