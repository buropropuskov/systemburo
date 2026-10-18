package services

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestHistoryReadPolicy_Families(t *testing.T) {
	passages := []string{"entry", "exit", "entry_revert", "exit_revert", "passage_close", "passage_close_revert"}
	changes := []string{"create", "update", "delete", "dates_changed", "unknown_future_action", ""}
	for _, p := range []HistoryReadPolicy{{}, {Changes: true}, {Passages: true}, {Changes: true, Passages: true}} {
		for _, action := range passages {
			require.Equal(t, p.Passages, p.Allows(action), action)
		}
		for _, action := range changes {
			require.Equal(t, p.Changes, p.Allows(action), action)
		}
		cars := p.Cars([]CarHistoryItemResponse{{ActionType: "entry"}, {ActionType: "update"}})
		employees := p.Employees([]EmployeeHistoryItem{{ActionType: "exit_revert"}, {ActionType: "dates_changed"}})
		n := 0
		if p.Changes {
			n++
		}
		if p.Passages {
			n++
		}
		require.Len(t, cars, n)
		require.Len(t, employees, n)
		require.NotNil(t, p.Cars(nil))
		require.NotNil(t, p.Employees(nil))
	}
}

func TestHistoryReadPolicy_UnavailableResolver(t *testing.T) {
	_, err := ResolveHistoryReadPolicy(context.Background(), nil, 1)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusServiceUnavailable, httpErr.Code)
}
