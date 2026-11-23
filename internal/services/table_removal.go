package services

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// TableRemovalTargets supports the existing single-table contract and the
// explicit set used by the post picker. Ambiguous or invalid sets are rejected.
func TableRemovalTargets(tableID int, tableIDs []int) ([]int, error) {
	if tableID != 0 && len(tableIDs) != 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите один способ выбора постов")
	}
	if tableID != 0 {
		tableIDs = []int{tableID}
	}
	if len(tableIDs) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не выбраны посты")
	}
	for _, id := range tableIDs {
		if id <= 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный пост")
		}
	}
	return uniqueInts(tableIDs), nil
}
