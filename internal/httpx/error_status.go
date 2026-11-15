package httpx

import (
	"errors"
	"net/http"

	"systemburo/internal/apperr"

	"github.com/labstack/echo/v5"
)

// StatusCode resolves the response status without changing the response.
// Once committed, the status actually sent takes precedence over returned errors.
func StatusCode(c *echo.Context, err error) int {
	response, _ := echo.UnwrapResponse(c.Response())
	code := http.StatusOK
	if response != nil {
		code = response.Status
	}
	minimum := 100
	if (response == nil || !response.Committed) && err != nil {
		minimum = http.StatusBadRequest
		code = http.StatusInternalServerError
		var domain *apperr.Error
		var transport interface{ StatusCode() int }
		switch {
		case errors.As(err, &domain) && domain != nil:
			code = domain.Code
		case errors.As(err, &transport) && transport != nil:
			code = transport.StatusCode()
		}
	}
	if code < minimum || code > 599 {
		return http.StatusInternalServerError
	}
	return code
}

// ResponseCommitted reports whether Echo has sent the response headers.
func ResponseCommitted(c *echo.Context) bool {
	response, _ := echo.UnwrapResponse(c.Response())
	return response != nil && response.Committed
}
