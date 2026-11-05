package middleware

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/admission"
	"systemburo/internal/apperr"

	"github.com/labstack/echo/v4"
)

func TestBodyAdmissionSocketCancellationInterruptsRead(t *testing.T) {
	gate := admission.New(1, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reading, finished := make(chan struct{}), make(chan struct{})
	e := bodyTestApp(gate, func(c echo.Context) error {
		close(reading)
		_, err := io.Copy(io.Discard, c.Request().Body)
		return err
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.ServeHTTP(w, r.WithContext(ctx))
		close(finished)
	}))
	defer server.Close()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "POST /api/body HTTP/1.1\r\nHost: local\r\nContent-Length: 100\r\n\r\nx")
	select {
	case <-reading:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start reading")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not interrupt blocked Read/Close")
	}
	assertBodySlotFree(t, gate)
}

func TestBodyAdmissionSocketFullyReadKeepsConnection(t *testing.T) {
	gate := admission.New(1, 0, 0)
	e := bodyTestApp(gate, func(c echo.Context) error {
		if _, err := io.Copy(io.Discard, c.Request().Body); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
	server := httptest.NewServer(e)
	defer server.Close()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	for _, request := range []string{
		"POST /api/body HTTP/1.1\r\nHost: local\r\nContent-Length: 2\r\n\r\n{}",
		"DELETE /api/body HTTP/1.1\r\nHost: local\r\nTransfer-Encoding: chunked\r\n\r\n2\r\n{}\r\n0\r\n\r\n",
		"GET /api/body HTTP/1.1\r\nHost: local\r\n\r\n",
	} {
		_, _ = io.WriteString(conn, request)
		response, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatalf("normal request lost keepalive: %v", err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNoContent || response.Close {
			t.Fatalf("normal request: status=%d close=%v", response.StatusCode, response.Close)
		}
	}
	assertBodySlotFree(t, gate)
}

func TestBodyAdmissionHTTP2UnknownLengthAndRejectKeepsSession(t *testing.T) {
	gate := admission.New(1, 0, 0)
	var consumed atomic.Int64
	e := bodyTestApp(gate, func(c echo.Context) error {
		if c.Request().ProtoMajor != 2 {
			return echo.NewHTTPError(599, "not HTTP/2")
		}
		n, err := io.Copy(io.Discard, c.Request().Body)
		consumed.Add(n)
		if err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
	server := httptest.NewUnstartedServer(e)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	makeRequest := func(body io.Reader, want int) {
		t.Helper()
		req, err := http.NewRequest("GET", server.URL+"/api/body", body)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.ProtoMajor != 2 || response.StatusCode != want || response.Header.Get("Connection") != "" {
			t.Fatalf("H2 response: proto=%s status=%d headers=%v", response.Proto, response.StatusCode, response.Header)
		}
	}
	// Hide strings.Reader's known length: transport must send DATA without CL.
	makeRequest(io.LimitReader(strings.NewReader("body"), 4), http.StatusNoContent)
	if consumed.Load() != 4 {
		t.Fatalf("unknown length payload lost: %d", consumed.Load())
	}
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	makeRequest(io.LimitReader(strings.NewReader("body"), 4), http.StatusServiceUnavailable)
	release()
	// A rejected H2 body must not terminate the client session.
	makeRequest(io.LimitReader(strings.NewReader("body"), 4), http.StatusNoContent)
}

func bodyTestErrorHandler(err error, c echo.Context) {
	status := http.StatusInternalServerError
	var app *apperr.Error
	var he *echo.HTTPError
	if errors.As(err, &app) {
		status = app.Code
		for k, v := range app.Headers {
			c.Response().Header().Set(k, v)
		}
	} else if errors.As(err, &he) {
		status = he.Code
	}
	_ = c.NoContent(status)
}

func bodyTestApp(gate *admission.Gate, handler echo.HandlerFunc) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = bodyTestErrorHandler
	e.Use(SafeRecover(), bodyAdmissionWithGate(gate))
	e.Any("/api/body", handler)
	return e
}

func assertBodySlotFree(t *testing.T, gate *admission.Gate) {
	t.Helper()
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatalf("body slot leaked: %v", err)
	}
	release()
}

func TestBodyAdmissionEveryMethodAndBodylessReads(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			gate := admission.New(1, 0, 0)
			release, err := gate.Acquire(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			called := false
			e := bodyTestApp(gate, func(c echo.Context) error {
				called = true
				return c.NoContent(http.StatusNoContent)
			})
			for _, length := range []int64{4, -1} {
				req := httptest.NewRequest(method, "/api/body", strings.NewReader("body"))
				req.ContentLength = length
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if called || rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "2" {
					t.Fatalf("potential body bypass: length=%d status=%d called=%v", length, rec.Code, called)
				}
			}
			for i := 0; i < 16; i++ {
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequest(method, "/api/body", nil))
				if rec.Code != http.StatusNoContent {
					t.Fatalf("bodyless request consumed a slot: %d", rec.Code)
				}
			}
		})
	}
}

type bodyTrackingReader struct {
	io.Reader
	closed bool
}

func (b *bodyTrackingReader) Close() error { b.closed = true; return nil }

func TestBodyAdmissionRetainsLeaseThroughEOFReplacementErrorAndPanic(t *testing.T) {
	for _, outcome := range []string{"ok", "error", "panic"} {
		t.Run(outcome, func(t *testing.T) {
			gate := admission.New(1, 0, 0)
			raw := &bodyTrackingReader{Reader: strings.NewReader("payload")}
			e := bodyTestApp(gate, func(c echo.Context) error {
				data, err := io.ReadAll(c.Request().Body)
				if err != nil {
					t.Fatal(err)
				}
				// Match the permission middleware's replay of the decoded body.
				c.Request().Body = io.NopCloser(strings.NewReader(string(data)))
				if release, err := gate.Acquire(context.Background()); err == nil {
					release()
					t.Fatal("lease released at EOF, before handler work ended")
				}
				if outcome == "panic" {
					panic("synthetic")
				}
				if outcome == "error" {
					return echo.ErrBadRequest
				}
				return nil
			})
			req := httptest.NewRequest("PUT", "/api/body", nil)
			req.Body, req.ContentLength = raw, -1
			e.ServeHTTP(httptest.NewRecorder(), req)
			if !raw.closed {
				t.Fatal("original transport body was not closed after replacement")
			}
			assertBodySlotFree(t, gate)
		})
	}
}

type bodyDelayedRead struct {
	started, interrupted, finish chan struct{}
	once                         sync.Once
}

func (b *bodyDelayedRead) Read([]byte) (int, error) {
	close(b.started)
	<-b.interrupted
	<-b.finish
	return 0, io.EOF
}

func (b *bodyDelayedRead) Close() error {
	b.once.Do(func() { close(b.interrupted) })
	return nil
}

func TestBodyAdmissionCloseDoesNotReleaseBeforeReadReturns(t *testing.T) {
	gate := admission.New(1, 0, 0)
	raw := &bodyDelayedRead{started: make(chan struct{}), interrupted: make(chan struct{}), finish: make(chan struct{})}
	e := bodyTestApp(gate, func(c echo.Context) error {
		// Defensively cover an accidental reader outliving its handler as well.
		body := c.Request().Body
		go func() { _, _ = body.Read(make([]byte, 1)) }()
		<-raw.started
		return echo.ErrBadRequest
	})
	req := httptest.NewRequest("POST", "/api/body", nil)
	req.Body, req.ContentLength = raw, -1
	done := make(chan struct{})
	go func() {
		e.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	select {
	case <-raw.interrupted:
	case <-time.After(3 * time.Second):
		close(raw.finish)
		t.Fatal("cleanup did not close the transport")
	}
	release, err := gate.Acquire(context.Background())
	if err == nil {
		release()
		close(raw.finish)
		t.Fatal("Close released a lease while Read was still running")
	}
	close(raw.finish)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not join the completed Read")
	}
	assertBodySlotFree(t, gate)
}

type repeatedByteReader struct{ remaining int64 }

func (r *repeatedByteReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	for i := range p[:int(n)] {
		p[i] = ' '
	}
	r.remaining -= n
	return int(n), nil
}

func TestBodyAdmissionExactLimitAndMasked413(t *testing.T) {
	for _, length := range []int64{apiBodyBytes, apiBodyBytes + 1} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			gate := admission.New(1, 0, 0)
			e := bodyTestApp(gate, func(c echo.Context) error {
				if _, err := io.Copy(io.Discard, c.Request().Body); err != nil {
					return echo.ErrBadRequest // handlers commonly mask Bind/FormFile errors
				}
				return c.NoContent(http.StatusNoContent)
			})
			req := httptest.NewRequest("POST", "/api/body", nil)
			req.Body = io.NopCloser(&repeatedByteReader{remaining: length})
			req.ContentLength = -1
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			want := http.StatusNoContent
			if length > apiBodyBytes {
				want = http.StatusRequestEntityTooLarge
			}
			if rec.Code != want {
				t.Fatalf("status=%d want=%d", rec.Code, want)
			}
			assertBodySlotFree(t, gate)
		})
	}
}

func TestBodyAdmissionBindErrorStillReturns413(t *testing.T) {
	gate := admission.New(1, 0, 0)
	e := bodyTestApp(gate, func(c echo.Context) error {
		var value map[string]any
		if err := c.Bind(&value); err != nil {
			return echo.ErrBadRequest
		}
		return nil
	})
	req := httptest.NewRequest("DELETE", "/api/body", nil)
	req.Body = io.NopCloser(&repeatedByteReader{remaining: apiBodyBytes + 1})
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Bind masked the transport limit: %d", rec.Code)
	}
	assertBodySlotFree(t, gate)
}

func TestBodyAdmissionMultipartCleanupBeforeRelease(t *testing.T) {
	gate := admission.New(1, 0, 0)
	var filename string
	e := bodyTestApp(gate, func(c echo.Context) error {
		if err := c.Request().ParseMultipartForm(1); err != nil {
			return err
		}
		f, err := c.Request().MultipartForm.File["file"][0].Open()
		if err != nil {
			return err
		}
		filename = f.(*os.File).Name()
		_ = f.Close()
		return echo.ErrBadRequest
	})
	var data strings.Builder
	writer := multipart.NewWriter(&data)
	part, err := writer.CreateFormFile("file", "synthetic.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, strings.Repeat("x", 128))
	_ = writer.Close()
	req := httptest.NewRequest("POST", "/api/body", strings.NewReader(data.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	e.ServeHTTP(httptest.NewRecorder(), req)
	if filename == "" {
		t.Fatal("multipart never reached disk")
	}
	if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("multipart temporary file remained: %v", err)
	}
	assertBodySlotFree(t, gate)
}

// Use real net/http sockets: ResponseRecorder cannot expose implicit server
// drain, Expect handling, or the read deadline needed before Body.Close.
func TestBodyAdmissionSocketUnreadRejectExpectAndPartial(t *testing.T) {
	for _, mode := range []string{"busy", "auth", "partial", "early-response", "panic"} {
		for _, framing := range []string{"length", "chunked", "expect"} {
			t.Run(mode+"/"+framing, func(t *testing.T) {
				gate := admission.New(1, 0, 0)
				if mode == "busy" {
					release, err := gate.Acquire(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					defer release()
				}
				var calls atomic.Int32
				e := bodyTestApp(gate, func(c echo.Context) error {
					calls.Add(1)
					if mode == "partial" {
						var one [1]byte
						_, _ = c.Request().Body.Read(one[:])
					}
					if mode == "early-response" {
						return c.NoContent(http.StatusAccepted)
					}
					if mode == "panic" {
						panic("synthetic")
					}
					return echo.ErrUnauthorized
				})
				server := httptest.NewUnstartedServer(e)
				// Deliberately no server ReadTimeout: the middleware abort must suffice.
				server.Start()
				defer server.Close()
				conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				headers, prefix := "Content-Length: 100\r\n", "x"
				if framing == "chunked" {
					headers, prefix = "Transfer-Encoding: chunked\r\n", "64\r\nx"
				}
				if framing == "expect" {
					headers += "Expect: 100-continue\r\n"
					prefix = ""
				}
				_, err = fmt.Fprintf(conn, "DELETE /api/body HTTP/1.1\r\nHost: local\r\n%s\r\n%s", headers, prefix)
				if err != nil {
					t.Fatal(err)
				}
				reader := bufio.NewReader(conn)
				response, err := http.ReadResponse(reader, &http.Request{Method: "DELETE"})
				if err != nil {
					t.Fatalf("response blocked on unread body: %v", err)
				}
				if response.StatusCode == http.StatusContinue {
					if mode != "partial" || framing != "expect" {
						t.Fatal("rejected request solicited a body with 100 Continue")
					}
					_, _ = io.WriteString(conn, "x")
					response, err = http.ReadResponse(reader, &http.Request{Method: "DELETE"})
					if err != nil {
						t.Fatal(err)
					}
				}
				want := http.StatusUnauthorized
				switch mode {
				case "busy":
					want = http.StatusServiceUnavailable
				case "early-response":
					want = http.StatusAccepted
				case "panic":
					want = http.StatusInternalServerError
				}
				if response.StatusCode != want {
					t.Fatalf("status=%d want=%d", response.StatusCode, want)
				}
				_ = response.Body.Close()
				if _, err := reader.ReadByte(); err == nil {
					t.Fatal("unread-body connection unexpectedly stayed usable")
				} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
					t.Fatal("server cleanup blocked on unread transport body")
				}
				if mode == "busy" {
					if calls.Load() != 0 {
						t.Fatal("busy request entered handler")
					}
				} else {
					assertBodySlotFree(t, gate)
				}
			})
		}
	}
}
