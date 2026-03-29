package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// POST makes a POST request through the Echo app and returns the recorder.
func POST(t *testing.T, e *echo.Echo, path, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, e, http.MethodPost, path, body, headers)
}

// GET makes a GET request through the Echo app and returns the recorder.
func GET(t *testing.T, e *echo.Echo, path string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, e, http.MethodGet, path, "", headers)
}

// PUT makes a PUT request through the Echo app and returns the recorder.
func PUT(t *testing.T, e *echo.Echo, path, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, e, http.MethodPut, path, body, headers)
}

// DELETE makes a DELETE request through the Echo app and returns the recorder.
func DELETE(t *testing.T, e *echo.Echo, path string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, e, http.MethodDelete, path, "", headers)
}

// DELETEWithBody makes a DELETE request with a body through the Echo app.
func DELETEWithBody(t *testing.T, e *echo.Echo, path, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, e, http.MethodDelete, path, body, headers)
}

func doRequest(t *testing.T, e *echo.Echo, method, path, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	// Merge custom headers
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
