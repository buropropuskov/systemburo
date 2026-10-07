package api

import (
	"errors"
	"net/http"
	"testing"

	"gorm.io/gorm"
	"systemburo/internal/apperr"
)

func TestPagination_ApplyOverflowReturnsValidationError(t *testing.T) {
	db := &gorm.DB{Config: &gorm.Config{}}
	result := ApplyPagination(db, PaginationParams{Page: int(^uint(0) >> 1), Limit: 20})
	var validation *apperr.Error
	if !errors.As(result.Error, &validation) || validation.Code != http.StatusBadRequest {
		t.Fatalf("expected validation error, got %v", result.Error)
	}
}
