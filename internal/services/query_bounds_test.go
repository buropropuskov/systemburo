package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
)

func TestQueryBounds_TrashRejectsInvalidDatesWithoutDB(t *testing.T) {
	svc := NewTrashService(nil, nil)
	for _, filter := range []models.TrashFilter{{DateFrom: "2026-02-29"}, {DateTo: "bad"}, {DateFrom: "2026-10-08", DateTo: "2026-10-07"}} {
		for _, list := range []func(context.Context, int, models.TrashFilter) ([]models.TrashItem, error){svc.ListCarsTrash, svc.ListEmployeesTrash} {
			_, err := list(context.Background(), 1, filter)
			var validation *apperr.Error
			if !errors.As(err, &validation) || validation.Code != http.StatusBadRequest {
				t.Fatalf("got %v", err)
			}
		}
	}
}

func TestPagination_PassageHistoryRejectsOverflowWithoutDB(t *testing.T) {
	q := models.PassageHistoryQuery{Page: int(^uint(0) >> 1), PerPage: 20}
	_, _, err := (&carService{}).queryCarsHistory(context.Background(), q, "", nil)
	var validation *apperr.Error
	if !errors.As(err, &validation) || validation.Code != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
	_, _, err = (&employeesHistoryService{}).queryHistoryPage(context.Background(), q, "", nil)
	if !errors.As(err, &validation) || validation.Code != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
}
