package httpx

import (
	"errors"
	"net/http"

	"systemburo/internal/apperr"

	"github.com/labstack/echo/v4"
)

// StatusCode resolves the response status without changing the response.
// Once committed, the status actually sent takes precedence over returned errors.
func StatusCode(c echo.Context, err error) int {
	code := c.Response().Status
	minimum := 100
	if !c.Response().Committed && err != nil {
		minimum = http.StatusBadRequest
		code = http.StatusInternalServerError
		var domain *apperr.Error
		var transport *echo.HTTPError
		switch {
		case errors.As(err, &domain) && domain != nil:
			code = domain.Code
		case errors.As(err, &transport) && transport != nil:
			code = transport.Code
		}
	}
	if code < minimum || code > 599 {
		return http.StatusInternalServerError
	}
	return code
}
