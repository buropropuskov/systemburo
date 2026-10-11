package handlers

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

func historyEntityParams(c echo.Context) (int, int, error) {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil || tableID <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "Некорректная таблица")
	}
	entityID, err := strconv.Atoi(c.Param("entityID"))
	if err != nil || entityID <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "Некорректная запись")
	}
	return tableID, entityID, nil
}
func (h *CarHandler) GetHistoryCarForTable(c echo.Context) error {
	tableID, id, err := historyEntityParams(c)
	if err != nil {
		return err
	}
	row, err := h.service.GetHistoryCarForTable(c.Request().Context(), GetUserID(c), tableID, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, row)
}
func (h *EmployeeHandler) GetHistoryEmployeeForTable(c echo.Context) error {
	tableID, id, err := historyEntityParams(c)
	if err != nil {
		return err
	}
	row, err := h.service.GetHistoryEmployeeForTable(c.Request().Context(), GetUserID(c), tableID, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, row)
}
