package middleware

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/admission"

	"github.com/labstack/echo/v4"
)

// Opt-in disposable fixture only. Run this test in a container listening on
// :8080, reachable as backend-edge on the private nginx network. Point the
// production nginx.conf backend upstream at that alias and provide synthetic
// TLS certificates at its existing certificate paths. No application/DB runs.
func TestBodyAdmissionNginxHTTP2(t *testing.T) {
	base := os.Getenv("BODY_ADMISSION_NGINX_URL")
	if base == "" {
		t.Skip("set BODY_ADMISSION_NGINX_URL for an isolated nginx HTTP/2 -> HTTP/1.1 fixture")
	}
	if !strings.HasPrefix(base, "https://") {
		t.Fatal("fixture URL must use HTTPS to test ALPN HTTP/2")
	}
	base = strings.TrimRight(base, "/")
	gate := admission.New(8, 0, 0)
	held, release := make(chan struct{}, 8), make(chan struct{})
	var releaseOnce sync.Once
	finishHeld := func() { releaseOnce.Do(func() { close(release) }) }
	e := echo.New()
	e.HTTPErrorHandler = bodyTestErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Fixture-Upstream-Proto", c.Request().Proto)
			return next(c)
		}
	}, bodyAdmissionWithGate(gate))
	e.DELETE("/api/body-fixture/hold", func(c echo.Context) error {
		if c.Request().ContentLength != -1 {
			return echo.NewHTTPError(599, "unknown length was not preserved")
		}
		var prefix [1]byte
		if _, err := io.ReadFull(c.Request().Body, prefix[:]); err != nil {
			return err
		}
		held <- struct{}{}
		select {
		case <-release:
		case <-c.Request().Context().Done():
			return c.Request().Context().Err()
		}
		_, err := io.Copy(io.Discard, c.Request().Body)
		if err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
	e.GET("/api/body-fixture/read", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	stream := func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(c.Response(), "data: synthetic\n\n"); err != nil {
			return err
		}
		c.Response().Flush()
		<-c.Request().Context().Done()
		return nil
	}
	e.GET("/api/events", stream)
	e.GET("/api/file-archive/download", stream)
	e.POST("/api/documents", func(c echo.Context) error {
		if err := c.Request().ParseMultipartForm(32 << 10); err != nil {
			return echo.ErrBadRequest
		}
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}
		c.Response().Header().Set("X-Fixture-Upload-Bytes", fmt.Sprint(file.Size))
		return c.NoContent(http.StatusNoContent)
	})
	e.POST("/api/login", func(c echo.Context) error {
		if _, err := io.Copy(io.Discard, c.Request().Body); err != nil {
			return echo.ErrBadRequest
		}
		return c.NoContent(http.StatusNoContent)
	}, SmallAuthBodyBudget)
	listener, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: e, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 15 * time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()
	defer func() {
		finishHeld()
		_ = server.Close()
		if err := <-serverDone; !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("fixture server: %v", err)
		}
	}()
	// Synthetic certificates belong to this disposable fixture only.
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // disposable local fixture
		ForceAttemptHTTP2: true,
		MaxConnsPerHost:   1,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second}
	var firstConn net.Conn
	var connMu sync.Mutex
	var changedConn atomic.Bool
	do := func(method, path string, body io.Reader, contentType string) (*http.Response, error) {
		req, err := http.NewRequest(method, base+path, body)
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				connMu.Lock()
				defer connMu.Unlock()
				if firstConn == nil {
					firstConn = info.Conn
				} else if firstConn != info.Conn {
					changedConn.Store(true)
				}
			},
		}))
		return client.Do(req)
	}
	check := func(response *http.Response, err error, want int, upstream bool) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		if response.ProtoMajor != 2 || response.TLS == nil || response.TLS.NegotiatedProtocol != "h2" || response.StatusCode != want {
			t.Fatalf("edge: proto=%s status=%d want=%d", response.Proto, response.StatusCode, want)
		}
		if upstream && response.Header.Get("X-Fixture-Upstream-Proto") != "HTTP/1.1" {
			t.Fatalf("upstream was not HTTP/1.1: %v", response.Header)
		}
	}
	results := make(chan error, 8)
	writers := make([]*io.PipeWriter, 0, 8)
	defer func() {
		for _, writer := range writers {
			_ = writer.Close()
		}
	}()
	for i := 0; i < 8; i++ {
		reader, writer := io.Pipe()
		writers = append(writers, writer)
		go func() { _, _ = io.WriteString(writer, "x") }()
		go func() {
			response, err := do("DELETE", "/api/body-fixture/hold", reader, "")
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode != http.StatusNoContent {
					err = fmt.Errorf("held request status=%d", response.StatusCode)
				}
			}
			results <- err
		}()
	}
	for i := 0; i < 8; i++ {
		select {
		case <-held:
		case err := <-results:
			t.Fatalf("request returned before holding body slot: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("nginx did not stream eight unknown-length bodies to upstream")
		}
	}
	response, err := do("DELETE", "/api/body-fixture/hold", io.LimitReader(strings.NewReader("x"), 1), "")
	check(response, err, http.StatusServiceUnavailable, true)
	// Concurrent bodyless UI reads must pass while every body slot is occupied.
	reads := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() {
			response, err := do("GET", "/api/body-fixture/read", nil, "")
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode != http.StatusNoContent || response.ProtoMajor != 2 || response.Header.Get("X-Fixture-Upstream-Proto") != "HTTP/1.1" {
					err = fmt.Errorf("bodyless GET: status=%d proto=%s", response.StatusCode, response.Proto)
				}
			}
			reads <- err
		}()
	}
	for i := 0; i < 20; i++ {
		if err := <-reads; err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/api/events", "/api/file-archive/download"} {
		response, err := do("GET", path, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		var prefix [5]byte
		_, readErr := io.ReadFull(response.Body, prefix[:])
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK || string(prefix[:]) != "data:" || readErr != nil {
			t.Fatalf("stream %s failed: status=%d err=%v", path, response.StatusCode, readErr)
		}
	}
	for _, writer := range writers {
		_ = writer.Close()
	}
	finishHeld()
	for i := 0; i < 8; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	var form strings.Builder
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "synthetic.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(part, &repeatedByteReader{remaining: 2 << 20})
	_ = writer.Close()
	response, err = do("POST", "/api/documents", strings.NewReader(form.String()), writer.FormDataContentType())
	if err == nil && response.Header.Get("X-Fixture-Upload-Bytes") != fmt.Sprint(2<<20) {
		_ = response.Body.Close()
		t.Fatalf("normal upload size changed: %v", response.Header)
	}
	check(response, err, http.StatusNoContent, true)
	response, err = do("POST", "/api/login", &repeatedByteReader{remaining: SmallAuthBodyLimit}, "application/octet-stream")
	check(response, err, http.StatusNoContent, true)
	response, err = do("POST", "/api/login", &repeatedByteReader{remaining: SmallAuthBodyLimit + 1}, "application/octet-stream")
	check(response, err, http.StatusRequestEntityTooLarge, true)
	response, err = do("POST", "/api/login", strings.NewReader(strings.Repeat("x", (1<<20)+1)), "application/octet-stream")
	if err == nil && response.Header.Get("X-Fixture-Upstream-Proto") != "" {
		_ = response.Body.Close()
		t.Fatal("production exact login 1MiB limit no longer enforced by nginx")
	}
	check(response, err, http.StatusRequestEntityTooLarge, false)
	response, err = do("GET", "/api/body-fixture/read", nil, "")
	check(response, err, http.StatusNoContent, true)
	if changedConn.Load() {
		t.Fatal("body rejection terminated the shared external HTTP/2 session")
	}
	assertBodySlotFree(t, gate)
}
