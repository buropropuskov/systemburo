package handlers

import (
	"context"
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// EntityPeriodCommands is the narrow interface of the dedicated command service.
// It preserves the existing CarService and EmployeeService interfaces.
type EntityPeriodCommands interface {
	Inspect(context.Context, int, services.ElementKind, int, *int) (*services.EntityPeriodCommandResult, error)
	Change(context.Context, int, services.ElementKind, int, services.ChangeEntityPeriodRequest) (*services.EntityPeriodCommandResult, error)
}

type EntityPeriodHandler struct {
	commands EntityPeriodCommands
}

func NewEntityPeriodHandler(commands EntityPeriodCommands) *EntityPeriodHandler {
	return &EntityPeriodHandler{commands: commands}
}

// Routes are intentionally not registered by this file. The integration owner
// must first update expiry/read consumers and preserve authenticated middleware.
func (h *EntityPeriodHandler) GetEmployeePeriod(c echo.Context) error {
	return h.inspect(c, services.ElementEmployee)
}

func (h *EntityPeriodHandler) GetCarPeriod(c echo.Context) error {
	return h.inspect(c, services.ElementCar)
}

func (h *EntityPeriodHandler) ChangeEmployeePeriod(c echo.Context) error {
	return h.change(c, services.ElementEmployee)
}

func (h *EntityPeriodHandler) ChangeCarPeriod(c echo.Context) error {
	return h.change(c, services.ElementCar)
}

func (h *EntityPeriodHandler) inspect(c echo.Context, kind services.ElementKind) error {
	actorID, entityID, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис изменения срока недоступен")
	}
	var tableID *int
	if raw := c.QueryParam("table_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректная таблица")
		}
		tableID = &id
	}
	result, err := h.commands.Inspect(c.Request().Context(), actorID, kind, entityID, tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

func (h *EntityPeriodHandler) change(c echo.Context, kind services.ElementKind) error {
	actorID, entityID, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис изменения срока недоступен")
	}
	var req services.ChangeEntityPeriodRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректное тело запроса")
	}
	result, err := h.commands.Change(c.Request().Context(), actorID, kind, entityID, req)
	if err != nil {
		return err
	}
	// Notifications and blank/table invalidation must be integrated after the
	// committed service result, not during binding or before the transaction.
	return RespondSuccess(c, result)
}

func entityPeriodRequestIdentity(c echo.Context) (int, int, error) {
	actorID := GetUserID(c)
	if actorID <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "Некорректная запись")
	}
	return actorID, id, nil
}
