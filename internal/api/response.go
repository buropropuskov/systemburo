// Package api provides unified response helpers for HTTP handlers.
//
// These utilities are designed for gradual adoption: existing handlers
// continue to work unchanged, and new or refactored handlers can use
// these helpers to produce a consistent JSON envelope.
package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// SuccessResponse wraps a payload in a standard envelope.
type SuccessResponse struct {
	Data interface{} `json:"data"`
}

// ErrorResponse represents a standardized error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// PaginatedResponse wraps a paginated payload with metadata.
type PaginatedResponse struct {
	Data  interface{} `json:"data"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

// RespondSuccess sends a 200 JSON response with {"data": ...} envelope.
func RespondSuccess(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, SuccessResponse{Data: data})
}

// RespondError sends an error JSON response with {"error": msg} envelope.
func RespondError(c echo.Context, code int, msg string) error {
	return c.JSON(code, ErrorResponse{Error: msg})
}

// RespondPaginated sends a 200 JSON response with pagination metadata.
func RespondPaginated(c echo.Context, data interface{}, total int64, page, limit int) error {
	return c.JSON(http.StatusOK, PaginatedResponse{
		Data:  data,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
