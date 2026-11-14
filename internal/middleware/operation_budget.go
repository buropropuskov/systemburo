package middleware

import (
	"systemburo/internal/httpx"

	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/services"
	"systemburo/internal/workbudget"

	"github.com/labstack/echo/v5"
)

type OperationClass uint8

const (
	OperationNone OperationClass = iota
	OperationCommand
	OperationImport
	OperationUpload
	OperationExport
	SmallAuthBodyLimit int64 = 64 << 10
	commandLifetime          = 120 * time.Second
)

// Route keys use Echo's registered path (c.Path()), never the raw request URL.
// This list controls admission only and does not grant access to any route.
var operationRoutes = map[string]OperationClass{
	"POST /api/applications":                                      OperationCommand,
	"POST /api/applications/submit-complete-application":          OperationCommand,
	"POST /api/applications/:id/forward":                          OperationCommand,
	"POST /api/applications/:id/approve":                          OperationCommand,
	"POST /api/applications/:id/take-to-work":                     OperationCommand,
	"POST /api/applications/:id/revoke-from-work":                 OperationCommand,
	"POST /api/applications/:id/restore-to-work":                  OperationCommand,
	"POST /api/applications/:id/withdraw":                         OperationCommand,
	"POST /api/applications/:id/revoke-approval":                  OperationCommand,
	"PUT /api/applications/:id/elements/tables":                   OperationCommand,
	"DELETE /api/applications/:id/elements":                       OperationCommand,
	"PUT /api/applications/:id/elements/unload-places":            OperationCommand,
	"PUT /api/applications/:id/dates":                             OperationCommand,
	"POST /api/applications/:id/attachment-period/preview":        OperationCommand,
	"PUT /api/applications/:id/attachment-period":                 OperationCommand,
	"POST /api/applications/:id/supplements":                      OperationCommand,
	"POST /api/applications/:id/supplements/:sid/approve":         OperationCommand,
	"POST /api/applications/:id/supplements/:sid/revoke-approval": OperationCommand,
	"POST /api/applications/:id/supplements/:sid/take-to-work":    OperationCommand,
	"POST /api/applications/:id/supplements/:sid/cancel":          OperationCommand,
	"POST /api/file-archive/applications/:id/reexport":            OperationCommand,
	"POST /api/file-archive/backfill":                             OperationCommand,
	"POST /api/file-archive/estimate":                             OperationCommand,
	"POST /api/statistics/report":                                 OperationCommand,
	"POST /api/attachments/:id/import-list":                       OperationImport,
	"POST /api/attachments/:id/template":                          OperationUpload,
	"POST /api/applications/files":                                OperationUpload,
	"POST /api/unload-places/:id/photos":                          OperationUpload,
	"POST /api/system-tables/:id/photos":                          OperationUpload,
	"POST /api/documents":                                         OperationUpload,
	"PUT /api/documents/:id/file":                                 OperationUpload,
	"PUT /api/guide/admin/sections/:role/file":                    OperationUpload,
	"POST /api/settings/data-processing/document":                 OperationUpload,
	"GET /api/applications/export":                                OperationExport,
	"GET /api/applications/:id/blank":                             OperationExport,
	"GET /api/applications/:id/archive":                           OperationExport,
	"GET /api/attachments/:id/blank-template":                     OperationExport,
	"GET /api/system-tables/:id/snapshots/:sid/export":            OperationExport,
	"GET /api/request-logs/export":                                OperationExport,
	"POST /api/pd-subject/export":                                 OperationExport,
	"GET /api/file-archive/download":                              OperationExport,
	"GET /api/file-archive/files/:id":                             OperationExport,
}

func ClassifyRoute(method, registeredPath string) OperationClass {
	return operationRoutes[method+" "+registeredPath]
}

// OperationBudget must be attached AFTER authentication and route permissions,
// before the handler reads/binds a body. Ticket streams require their verified
// ticket/live policy before acquiring; do not attach this before ticket consume.
func OperationBudget() echo.MiddlewareFunc {
	return OperationBudgetWithGate(workbudget.Default())
}

func OperationBudgetWithGate(gate *workbudget.Gate) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			class := ClassifyRoute(c.Request().Method, c.Path())
			if class == OperationNone {
				return next(c)
			}
			ctx := c.Request().Context()
			if class == OperationCommand || class == OperationImport {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, commandLifetime)
				defer cancel()
			}
			ctx, release, err := gate.Acquire(ctx)
			if err != nil {
				return services.ResourceAdmissionError(err)
			}
			defer release()
			c.SetRequest(c.Request().WithContext(ctx))
			return services.ResourceAdmissionError(next(c))
		}
	}
}

// SmallAuthBodyBudget limits only routes where explicitly attached. It never
// reads the body before the authorized handler. MaxBytesReader also bounds
// chunked requests, for which Content-Length provides no usable upper bound.
func SmallAuthBodyBudget(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		req := c.Request()
		tooLarge := func() error {
			return apperr.New(http.StatusRequestEntityTooLarge, "Слишком большой запрос.")
		}
		if req.ContentLength > SmallAuthBodyLimit {
			return tooLarge()
		}
		if req.Body == nil {
			return next(c)
		}
		body := &budgetBody{ReadCloser: http.MaxBytesReader(c.Response(), req.Body, SmallAuthBodyLimit)}
		req.Body = body
		err := next(c)
		if body.exceeded && !httpx.ResponseCommitted(c) {
			return tooLarge()
		}
		return err
	}
}

type budgetBody struct {
	io.ReadCloser
	exceeded bool
}

func (b *budgetBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		b.exceeded = true
	}
	return n, err
}
