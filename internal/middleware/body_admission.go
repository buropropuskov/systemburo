package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"systemburo/internal/admission"
	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// Match the existing edge API ceiling, without lowering route/file limits.
const apiBodyBytes int64 = 50 << 20

// BodyAdmission belongs before CORS and route middleware: even permission checks
// can read and replace the body. Bodyless reads/streams never acquire this gate.
// This gate deliberately does not share the expensive-operation budget.
func BodyAdmission() echo.MiddlewareFunc {
	return bodyAdmissionWithGate(admission.New(8, 0, 0))
}

func bodyAdmissionWithGate(gate *admission.Gate) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if !strings.HasPrefix(req.URL.Path, "/api/") || req.Body == nil || req.Body == http.NoBody {
				return next(c)
			}
			// Capture the original transport body; TablePassGate replaces req.Body.
			body := &admittedBody{raw: req.Body, req: req, response: c.Response(), length: req.ContentLength}
			body.reader = http.MaxBytesReader(c.Response(), req.Body, apiBodyBytes)
			req.Body = body
			// Prevent net/http's implicit pre-response drain. Cleanup below explicitly
			// closes the transport before returning to the server's final cleanup.
			if req.ProtoMajor == 1 {
				_ = http.NewResponseController(c.Response()).EnableFullDuplex()
				// Declare shutdown before headers are copied by net/http. On Go 1.26
				// the server cannot detect an early body close through our wrapper;
				// changing headers only in deferred Close is too late for early replies.
				// Keep reading/closing under the lease in cleanup, not in this hook.
				c.Response().Before(func() {
					if !body.complete.Load() {
						req.Close = true
						c.Response().Header().Set("Connection", "close")
					}
				})
			}
			var release func()
			defer func() {
				// Removal must precede slot reuse, including forms parsed by middleware.
				if form := c.Request().MultipartForm; form != nil {
					_ = form.RemoveAll()
				}
				_ = body.Close()
				if release != nil {
					release()
				}
			}()
			// Cancellation interrupts a blocked socket Read, but never releases its
			// lease. Join this callback before returning/reusing the response writer.
			interrupted := make(chan struct{})
			stop := context.AfterFunc(req.Context(), func() {
				if req.ProtoMajor == 1 {
					_ = http.NewResponseController(c.Response()).SetReadDeadline(time.Now())
				}
				close(interrupted)
			})
			defer func() {
				if !stop() {
					<-interrupted
				}
			}()
			if req.ContentLength > apiBodyBytes {
				return bodyTooLarge()
			}
			var err error
			release, err = gate.Acquire(req.Context())
			if err != nil {
				return services.ResourceAdmissionError(err)
			}
			err = next(c)
			// Bind/FormFile and permission middleware sometimes replace a read error
			// with 400. Preserve the actual transport ceiling in the public response.
			if body.exceeded.Load() && !c.Response().Committed {
				return bodyTooLarge()
			}
			return err
		}
	}
}

func bodyTooLarge() error {
	return apperr.New(http.StatusRequestEntityTooLarge, "Слишком большой запрос.")
}

type admittedBody struct {
	raw      io.ReadCloser
	reader   io.Reader
	req      *http.Request
	response *echo.Response
	length   int64
	read     int64 // guarded by readMu
	readMu   sync.Mutex
	closing  atomic.Bool
	complete atomic.Bool
	exceeded atomic.Bool
	close    sync.Once
	closeErr error
}

func (b *admittedBody) Read(p []byte) (int, error) {
	b.readMu.Lock()
	defer b.readMu.Unlock()
	if b.closing.Load() {
		return 0, http.ErrBodyReadAfterClose
	}
	n, err := b.reader.Read(p)
	b.read += int64(n)
	if err == io.EOF || (b.length > 0 && b.read == b.length) {
		b.complete.Store(true)
	}
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		b.exceeded.Store(true)
	}
	return n, err
}

func (b *admittedBody) Close() error {
	b.close.Do(func() {
		b.closing.Store(true)
		if !b.complete.Load() && b.req.ProtoMajor == 1 {
			// Close can itself drain an unread HTTP/1 body. Interrupt socket reads
			// first. The Before hook declares connection shutdown for early replies;
			// this header also covers rejection paths before a handler writes.
			// This cancels reading, rather than imposing a body-read timeout.
			b.req.Close = true
			b.response.Header().Set("Connection", "close")
			_ = http.NewResponseController(b.response).SetReadDeadline(time.Now())
		}
		b.closeErr = b.raw.Close()
		// Close/ctx cancellation alone is not proof that an in-flight Read ended.
		b.readMu.Lock()
		b.readMu.Unlock()
	})
	return b.closeErr
}
