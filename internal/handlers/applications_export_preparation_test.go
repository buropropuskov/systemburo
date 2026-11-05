package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type registryPreparationService struct {
	services.ApplicationService
	queries []context.Context
	err     error
}

func (s *registryPreparationService) GetApplications(ctx context.Context, _ string, _ services.ApplicationFilter) ([]services.ApplicationWithDetails, error) {
	s.queries = append(s.queries, ctx)
	return []services.ApplicationWithDetails{{ID: 1}}, nil
}

func (s *registryPreparationService) GetRegistryExtras(ctx context.Context, _ []int) (map[int]services.ApplicationRegistryExtras, error) {
	s.queries = append(s.queries, ctx)
	return map[int]services.ApplicationRegistryExtras{}, s.err
}

type registryDeliveryWriter struct {
	*httptest.ResponseRecorder
	beforeWrite func()
}

func (w *registryDeliveryWriter) Write(data []byte) (int, error) {
	w.beforeWrite()
	return w.ResponseRecorder.Write(data)
}

func TestApplicationsExportPreparationDoesNotExpireWorkbookDelivery(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivery", true: "SQL-error"}[fail], func(t *testing.T) {
			service := &registryPreparationService{}
			if fail {
				service.err = errors.New("synthetic SQL preparation error")
			}
			parent := context.Background()
			writer := &registryDeliveryWriter{ResponseRecorder: httptest.NewRecorder()}
			writer.beforeWrite = func() {
				require.Len(t, service.queries, 2)
				require.ErrorIs(t, service.queries[0].Err(), context.Canceled, "SQL timer ends before workbook delivery")
				require.NoError(t, parent.Err())
			}
			e := echo.New()
			request := httptest.NewRequest(http.MethodGet, "/api/applications/export", nil).WithContext(parent)
			c := e.NewContext(request, writer)
			c.Set("username", "synthetic-export-viewer")
			err := NewApplicationHandler(service, nil).ExportApplications(c)
			if fail {
				require.ErrorIs(t, err, service.err)
			} else {
				require.NoError(t, err)
				require.Greater(t, writer.Body.Len(), 0)
			}
			require.Len(t, service.queries, 2)
			require.Equal(t, service.queries[0], service.queries[1], "both SQL phases share one deadline")
			deadline, ok := service.queries[0].Deadline()
			require.True(t, ok)
			require.LessOrEqual(t, time.Until(deadline), 120*time.Second)
			require.ErrorIs(t, service.queries[0].Err(), context.Canceled)
			require.Equal(t, parent, c.Request().Context(), "HTTP context retains its original lifetime")
			require.NoError(t, parent.Err())
		})
	}
}
