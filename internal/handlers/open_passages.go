package handlers

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
	"systemburo/internal/services"
)

type OpenPassageReader interface {
	List(context.Context, int, services.ElementKind, *int, services.PassageOpenFilter) (*services.PassageOpenList, error)
}
type OpenPassageCommands interface {
	Correct(context.Context, int, services.ElementKind, int, services.PassageCorrectionRequest) (*services.PassageResult, error)
	RevertCorrection(context.Context, int, services.ElementKind, int, services.PassageCorrectionRequest) (*services.PassageResult, error)
}
type OpenPassagesHandler struct {
	reader   OpenPassageReader
	commands OpenPassageCommands
}

func NewOpenPassagesHandler(reader OpenPassageReader, commands OpenPassageCommands) *OpenPassagesHandler {
	return &OpenPassagesHandler{reader: reader, commands: commands}
}

func (h *OpenPassagesHandler) ListCars(c echo.Context) error {
	return h.list(c, services.ElementCar, false)
}
func (h *OpenPassagesHandler) ListEmployees(c echo.Context) error {
	return h.list(c, services.ElementEmployee, false)
}
func (h *OpenPassagesHandler) SummaryCars(c echo.Context) error {
	return h.list(c, services.ElementCar, true)
}
func (h *OpenPassagesHandler) SummaryEmployees(c echo.Context) error {
	return h.list(c, services.ElementEmployee, true)
}
func (h *OpenPassagesHandler) CloseCar(c echo.Context) error {
	return h.correct(c, services.ElementCar, false)
}
func (h *OpenPassagesHandler) CloseEmployee(c echo.Context) error {
	return h.correct(c, services.ElementEmployee, false)
}
func (h *OpenPassagesHandler) RevertCar(c echo.Context) error {
	return h.correct(c, services.ElementCar, true)
}
func (h *OpenPassagesHandler) RevertEmployee(c echo.Context) error {
	return h.correct(c, services.ElementEmployee, true)
}

func (h *OpenPassagesHandler) list(c echo.Context, kind services.ElementKind, summary bool) error {
	actor := GetUserID(c)
	if actor <= 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	if h == nil || h.reader == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис учёта недоступен")
	}
	var tableID *int
	if !summary {
		id, err := strconv.Atoi(c.Param("table_id"))
		if err != nil || id <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректная таблица")
		}
		tableID = &id
	}
	f := services.PassageOpenFilter{View: c.QueryParam("view"), Search: c.QueryParam("search"), Page: 1, PerPage: 50}
	if f.View == "" {
		f.View = "open"
	}
	f.AttentionOnly = f.View == "open"
	for name, dest := range map[string]*int{"page": &f.Page, "per_page": &f.PerPage} {
		if value := c.QueryParam(name); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "Некорректная пагинация")
			}
			*dest = parsed
		}
	}
	if value := c.QueryParam("attention_only"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректный фильтр внимания")
		}
		f.AttentionOnly = parsed
	}
	if value := c.QueryParam("organization_id"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректная организация")
		}
		f.OrganizationID = &parsed
	}
	if value := c.QueryParam("expired_only"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Некорректный фильтр просроченных отметок")
		}
		f.ExpiredOnly = parsed
	}
	result, err := h.reader.List(c.Request().Context(), actor, kind, tableID, f)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}
func (h *OpenPassagesHandler) correct(c echo.Context, kind services.ElementKind, revert bool) error {
	actor, id, err := entityPeriodRequestIdentity(c)
	if err != nil {
		return err
	}
	if h == nil || h.commands == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Сервис учёта недоступен")
	}
	var req services.PassageCorrectionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректное тело запроса")
	}
	var result *services.PassageResult
	if revert {
		result, err = h.commands.RevertCorrection(c.Request().Context(), actor, kind, id, req)
	} else {
		result, err = h.commands.Correct(c.Request().Context(), actor, kind, id, req)
	}
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}
