package validator

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// CustomValidator integrates go-playground/validator with Echo.
type CustomValidator struct {
	validator *validator.Validate
}

// New creates a CustomValidator instance ready for use with Echo.
func New() *CustomValidator {
	return &CustomValidator{validator: validator.New()}
}

// Validate implements echo.Validator interface.
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, formatValidationErrors(err))
	}
	return nil
}

// formatValidationErrors converts validator.ValidationErrors into a human-readable string.
func formatValidationErrors(err error) string {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	msgs := make([]string, 0, len(validationErrors))
	for _, fe := range validationErrors {
		msgs = append(msgs, formatFieldError(fe))
	}
	return strings.Join(msgs, "; ")
}

func formatFieldError(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("field '%s' is required", field)
	case "min":
		return fmt.Sprintf("field '%s' must be at least %s", field, fe.Param())
	case "max":
		return fmt.Sprintf("field '%s' must be at most %s", field, fe.Param())
	case "email":
		return fmt.Sprintf("field '%s' must be a valid email", field)
	case "oneof":
		return fmt.Sprintf("field '%s' must be one of: %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("field '%s' must be >= %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("field '%s' must be <= %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("field '%s' must be > %s", field, fe.Param())
	default:
		return fmt.Sprintf("field '%s' failed on '%s' validation", field, fe.Tag())
	}
}
