package middleware

import (
	"context"
	"net/http"
	"net/url"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// SessionRequestPolicy reuses the normal request gates without writing their
// denial response into an already-open event stream.
type SessionRequestPolicy struct {
	e     *echo.Echo
	gates []echo.MiddlewareFunc
}

func NewSessionRequestPolicy(gates ...echo.MiddlewareFunc) *SessionRequestPolicy {
	p := &SessionRequestPolicy{e: echo.New()}
	for _, gate := range gates {
		if gate != nil {
			p.gates = append(p.gates, gate)
		}
	}
	return p
}

type policyResponse struct {
	header http.Header
	status int
}

func (w *policyResponse) Header() http.Header { return w.header }
func (w *policyResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *policyResponse) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return len(b), nil
}

func (p *SessionRequestPolicy) Check(ctx context.Context, identity models.SessionIdentity, method, route string, original ...*http.Request) error {
	if p == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
	}
	req := (&http.Request{Method: method, URL: &url.URL{Path: route}, Header: make(http.Header)}).WithContext(ctx)
	if len(original) > 0 && original[0] != nil {
		req = original[0].Clone(ctx)
		req.Method = method
		req.URL.Path, req.URL.RawPath, req.URL.RawQuery = route, "", ""
		req.RequestURI = route
	}
	writer := &policyResponse{header: make(http.Header)}
	c := p.e.NewContext(req, writer)
	c.SetPath(route)
	SetSessionIdentity(c, identity)
	passed := false
	next := echo.HandlerFunc(func(echo.Context) error { passed = true; return nil })
	for i := len(p.gates) - 1; i >= 0; i-- {
		next = p.gates[i](next)
	}
	if err := next(c); err != nil {
		return err
	}
	if !passed {
		status := writer.status
		if status < 400 {
			status = http.StatusServiceUnavailable
		}
		return echo.NewHTTPError(status, "Доступ временно ограничен")
	}
	return nil
}
