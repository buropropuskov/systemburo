package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/workbudget"

	"github.com/labstack/echo/v4"
)

func TestOperationClassifyKnownRoutesAndExcludeOrdinaryRequests(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         OperationClass
	}{
		{http.MethodPost, "/api/applications/submit-complete-application", OperationCommand},
		{http.MethodPost, "/api/attachments/:id/import-list", OperationImport},
		{http.MethodPost, "/api/applications/files", OperationUpload},
		{http.MethodGet, "/api/applications/:id/archive", OperationExport},
		{http.MethodGet, "/api/file-archive/download", OperationExport},
		{http.MethodGet, "/api/events", OperationNone},
		{http.MethodGet, "/api/applications", OperationNone},
		{http.MethodPost, "/api/applications/:id/read", OperationNone},
		{http.MethodGet, "/api/settings/upload", OperationNone},
		{http.MethodGet, "/api/applications/42/archive", OperationNone},
	} {
		if got := ClassifyRoute(tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s: got %d want %d", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestOperationBudgetRejectsBeforeBodyAndReleasesOnHandlerError(t *testing.T) {
	gate := workbudget.New(1, 0, 0)
	_, free, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/applications", strings.NewReader("unread"))
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetPath("/api/applications")
	called := false
	handlerErr := errors.New("handler failed")
	h := OperationBudgetWithGate(gate)(func(c echo.Context) error {
		called = true
		_, nested, err := gate.Acquire(c.Request().Context())
		if err != nil {
			t.Fatalf("nested service: %v", err)
		}
		nested()
		return handlerErr
	})
	err = h(c)
	var busy *apperr.Error
	if called || !errors.As(err, &busy) || busy.Code != http.StatusServiceUnavailable || busy.Headers["Retry-After"] != "2" {
		t.Fatalf("busy admission: called=%t err=%v", called, err)
	}
	remaining, err := io.ReadAll(req.Body)
	if err != nil || string(remaining) != "unread" {
		t.Fatal("admission consumed body")
	}
	free()
	if err := h(c); err != handlerErr || !called {
		t.Fatalf("handler result changed: %v", err)
	}
	_, after, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatalf("handler error leaked slot: %v", err)
	}
	after()
}

func TestOperationDeadlineDoesNotTruncateArchiveStream(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		bounded      bool
	}{
		{http.MethodPost, "/api/applications", true},
		{http.MethodPost, "/api/attachments/:id/import-list", true},
		{http.MethodGet, "/api/applications/:id/archive", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(tc.method, "/", nil), httptest.NewRecorder())
			c.SetPath(tc.path)
			err := OperationBudgetWithGate(workbudget.New(1, 0, 0))(func(c echo.Context) error {
				deadline, ok := c.Request().Context().Deadline()
				if ok != tc.bounded {
					t.Fatalf("deadline set=%t want %t", ok, tc.bounded)
				}
				if ok && (time.Until(deadline) > commandLifetime || time.Until(deadline) < commandLifetime-time.Second) {
					t.Fatal("unexpected command lifetime")
				}
				return nil
			})(c)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSmallAuthBodyExactAndChunkedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		bytes             int
		chunked, rejected bool
	}{
		{int(SmallAuthBodyLimit), false, false},
		{int(SmallAuthBodyLimit) + 1, false, true},
		{int(SmallAuthBodyLimit), true, false},
		{int(SmallAuthBodyLimit) + 1, true, true},
	} {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(strings.Repeat("x", tc.bytes)))
		if tc.chunked {
			req.ContentLength = -1
			req.TransferEncoding = []string{"chunked"}
		}
		c := e.NewContext(req, httptest.NewRecorder())
		var bytesRead int
		err := SmallAuthBodyBudget(func(c echo.Context) error {
			data, err := io.ReadAll(c.Request().Body)
			bytesRead = len(data)
			if err != nil {
				return apperr.Validation("invalid body")
			}
			return nil
		})(c)
		if tc.rejected {
			var public *apperr.Error
			if !errors.As(err, &public) || public.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("size=%d chunked=%t: %v", tc.bytes, tc.chunked, err)
			}
		} else if err != nil || bytesRead != tc.bytes {
			t.Fatalf("exact boundary size=%d read=%d err=%v", tc.bytes, bytesRead, err)
		}
		if bytesRead > int(SmallAuthBodyLimit) {
			t.Fatal("read beyond body budget")
		}
	}
}

func TestSmallAuthBodyDoesNotReadBeforeHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/events/ticket", strings.NewReader("still unread"))
	req.ContentLength = -1
	c := e.NewContext(req, httptest.NewRecorder())
	if err := SmallAuthBodyBudget(func(echo.Context) error { return nil })(c); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(req.Body)
	if err != nil || string(data) != "still unread" {
		t.Fatal("body read ahead of authorized handler")
	}
}
