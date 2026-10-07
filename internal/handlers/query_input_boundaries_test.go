package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func requireInput400(t *testing.T, err error) {
	t.Helper()
	var app *apperr.Error
	var httpErr *echo.HTTPError
	if errors.As(err, &app) {
		require.Equal(t, http.StatusBadRequest, app.Code)
		return
	}
	require.True(t, errors.As(err, &httpErr), "%v", err)
	require.Equal(t, http.StatusBadRequest, httpErr.Code)
}

// Nil dependencies prove input validation completes before any DB/service call.
func TestQueryInput_ApplicationAndTrashInvalidDatesBeforeSQL(t *testing.T) {
	app := handlers.NewApplicationHandler(nil, nil)
	trash := handlers.NewTrashHandler(nil, nil)
	for _, query := range []string{"date_from=2026-02-29", "date_to=bad", "date_from=2026-10-08&date_to=2026-10-07"} {
		for _, action := range []func(echo.Context) error{app.GetApplications, app.GetUserApplications, app.GetAttachableApplications, trash.List} {
			c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/?"+query, nil), httptest.NewRecorder())
			c.Set("username", "synthetic")
			c.SetParamNames("id")
			c.SetParamValues("1")
			requireInput400(t, action(c))
		}
	}
}

func TestQueryInput_PaginationOverflowBeforeSQL(t *testing.T) {
	ctx := context.Background()
	maxInt := int(^uint(0) >> 1)
	app := services.NewApplicationService(nil, nil, nil, nil, nil, nil)
	_, _, err := app.GetApplicationsPaginated(ctx, "synthetic", services.ApplicationFilter{}, maxInt, 20)
	requireInput400(t, err)
	_, _, err = services.NewUniqueCarService(nil).GetAllPaginated(ctx, "synthetic", "my", "", maxInt, 20)
	requireInput400(t, err)
	_, _, err = services.NewUniqueEmployeeService(nil).GetAllPaginated(ctx, "synthetic", "my", "", maxInt, 20)
	requireInput400(t, err)
	_, _, err = app.GetAvailableAttachmentsForSecurity(ctx, 1, true, services.AvailableAttachmentFilters{}, maxInt, 20)
	requireInput400(t, err)
	_, _, err = services.NewRequestLogsService(nil).GetLogs(ctx, models.RequestLogsQuery{Page: maxInt, PerPage: 20})
	requireInput400(t, err)
	_, _, err = services.NewTableSnapshotService(nil, nil, nil, nil).ListSnapshots(ctx, 1, nil, nil, maxInt, 20)
	requireInput400(t, err)
	_, _, err = (&services.ArchiveDownloadService{}).ListItems(ctx, services.ArchiveItemsQuery{Page: maxInt, PerPage: 20})
	requireInput400(t, err)
	_, _, err = app.GetUserApplicationsPaginated(ctx, "synthetic", services.ApplicationFilter{}, maxInt, 20)
	requireInput400(t, err)
	_, err = services.NewAccessDenialService(nil).List(ctx, models.AccessDenialFilter{Page: maxInt, Limit: 20})
	requireInput400(t, err)
	_, err = services.NewAccessDenialService(nil).ListArchive(ctx, models.AccessDenialFilter{Page: maxInt, Limit: 20})
	requireInput400(t, err)
	_, err = services.NewAuthEventReader(nil).ListForUser(ctx, models.AuthEventFilter{Page: maxInt, Limit: 20})
	requireInput400(t, err)
	_, err = services.NewPDAuditService(nil).List(ctx, models.PDAuditFilter{Page: maxInt, Limit: 20})
	requireInput400(t, err)
	_, _, err = services.NewAuditReader(nil).List(ctx, services.AuditQuery{Page: maxInt, PerPage: 20})
	requireInput400(t, err)
	c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?page=%d&per_page=20", maxInt), nil), httptest.NewRecorder())
	c.Set("username", "synthetic")
	requireInput400(t, handlers.NewApplicationHandler(nil, nil).GetApplications(c))
	requireInput400(t, handlers.NewRequestLogsHandler(nil, nil).GetLogs(c))
}
