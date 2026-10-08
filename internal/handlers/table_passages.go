package handlers

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"systemburo/internal/services"
	"time"
)

func passageTerritoryState(state services.PassageState) *int {
	value := 0
	if state.Open {
		value = 1
	} else if state.HasEvent {
		value = 2
	}
	return &value
}
func (h *CarHandler) enrichCarPassages(c echo.Context, tableID int, rows []services.TableCarResponse) ([]services.TableCarResponse, error) {
	if h.passageDB == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Сервис учёта недоступен")
	}
	ids := make([]int, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	results, err := services.LoadTablePassageResults(c.Request().Context(), h.passageDB, GetUserID(c), services.ElementCar, ids, tableID, time.Time{})
	if err != nil {
		return nil, err
	}
	out := make([]services.TableCarResponse, 0, len(rows))
	for _, row := range rows {
		result, ok := results[row.ID]
		if !ok {
			continue
		}
		row.PassageState = result.PassageState
		row.EffectivePeriod = result.EffectivePeriod
		row.Admission = result.Admission
		row.ServerNow = result.ServerNow
		row.EntryDateTo = result.EffectivePeriod.EntryDateTo
		row.EntryTimeFrom = result.EffectivePeriod.EntryTimeFrom
		row.EntryTimeTo = result.EffectivePeriod.EntryTimeTo
		row.TerritoryStatus = passageTerritoryState(result.PassageState)
		row.TerritoryEntryTime = services.FormatUTCPtr(result.PassageState.EntryAt)
		out = append(out, row)
	}
	return out, nil
}
func (h *EmployeeHandler) enrichEmployeePassages(c echo.Context, tableID int, rows []services.TableEmployeeResponse) ([]services.TableEmployeeResponse, error) {
	if h.passageDB == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Сервис учёта недоступен")
	}
	ids := make([]int, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	results, err := services.LoadTablePassageResults(c.Request().Context(), h.passageDB, GetUserID(c), services.ElementEmployee, ids, tableID, time.Time{})
	if err != nil {
		return nil, err
	}
	out := make([]services.TableEmployeeResponse, 0, len(rows))
	for _, row := range rows {
		result, ok := results[row.ID]
		if !ok {
			continue
		}
		row.PassageState = result.PassageState
		row.EffectivePeriod = result.EffectivePeriod
		row.Admission = result.Admission
		row.ServerNow = result.ServerNow
		row.EntryDateTo = result.EffectivePeriod.EntryDateTo
		row.TerritoryStatus = passageTerritoryState(result.PassageState)
		from, to := "", ""
		if result.EffectivePeriod.EntryTimeFrom != nil {
			from = *result.EffectivePeriod.EntryTimeFrom
		}
		if result.EffectivePeriod.EntryTimeTo != nil {
			to = *result.EffectivePeriod.EntryTimeTo
		}
		passTime := from + " - " + to
		row.PassTime = &passTime
		out = append(out, row)
	}
	return out, nil
}
