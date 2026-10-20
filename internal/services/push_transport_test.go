package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noPushProxy(*http.Request) (*url.URL, error) { return nil, nil }

func TestPushRouting_Environment(t *testing.T) {
	cases := map[string]struct {
		env            []string
		endpoint, want string
	}{
		"https":              {[]string{"HTTPS_PROXY=http://proxy.invalid:3128"}, "https://push.example.com/ep", "proxy.invalid:3128"},
		"lowercase":          {[]string{"https_proxy=http://proxy.invalid:3128"}, "https://push.example.com/ep", "proxy.invalid:3128"},
		"http":               {[]string{"HTTP_PROXY=http://proxy.invalid:3128"}, "http://push.example.com/ep", "proxy.invalid:3128"},
		"no_proxy":           {[]string{"HTTPS_PROXY=http://proxy.invalid:3128", "NO_PROXY=example.com"}, "https://push.example.com/ep", ""},
		"lowercase_no_proxy": {[]string{"https_proxy=http://proxy.invalid:3128", "no_proxy=example.com"}, "https://push.example.com/ep", ""},
		"all_proxy_only":     {[]string{"ALL_PROXY=http://proxy.invalid:3128"}, "https://push.example.com/ep", ""},
		"loopback":           {[]string{"HTTPS_PROXY=http://proxy.invalid:3128"}, "https://127.0.0.1/ep", ""},
	}
	if name := os.Getenv("SYSTEMBURO_PUSH_ENV_CASE"); name != "" {
		tc, ok := cases[name]
		require.True(t, ok)
		req, err := http.NewRequest(http.MethodPost, tc.endpoint, nil)
		require.NoError(t, err)
		proxy, err := (&pushService{}).proxyForRequest(req)
		require.NoError(t, err)
		if tc.want == "" {
			assert.Nil(t, proxy)
		} else {
			require.NotNil(t, proxy)
			assert.Equal(t, tc.want, proxy.Host)
		}
		if name == "no_proxy" || name == "all_proxy_only" {
			assertPushDirectDialChecked(t, &pushService{}, tc.endpoint)
		}
		return
	}
	binary, err := os.Executable()
	require.NoError(t, err)
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(binary, "-test.run=^TestPushRouting_Environment$", "-test.count=1")
			for _, entry := range os.Environ() {
				key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
				if key != "HTTP_PROXY" && key != "HTTPS_PROXY" && key != "ALL_PROXY" && key != "NO_PROXY" && key != "REQUEST_METHOD" && key != "SYSTEMBURO_PUSH_ENV_CASE" {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, tc.env...)
			cmd.Env = append(cmd.Env, "SYSTEMBURO_PUSH_ENV_CASE="+name)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func TestPushRouting_Policy(t *testing.T) {
	proxy, err := url.Parse("http://proxy.invalid:3128")
	require.NoError(t, err)
	for _, tc := range []struct {
		name, endpoint     string
		allowed            []string
		proxied, wantError bool
	}{
		{"direct_custom", "https://push.example.com/ep", nil, false, false},
		{"proxy_known", "https://fcm.googleapis.com/ep", nil, true, false},
		{"proxy_subdomain", "https://web.push.apple.com/ep", nil, true, false},
		{"proxy_similar_name", "https://evilpush.apple.com/ep", nil, true, true},
		{"proxy_custom_default", "https://push.example.com/ep", nil, true, true},
		{"proxy_custom_explicit", "https://push.example.com/ep", []string{"push.example.com"}, true, false},
		{"explicit_still_restricts_direct", "https://push.example.com/ep", []string{"known"}, false, true},
		{"unresolved_proxy_known", "https://notify.windows.com/ep", nil, true, false},
		{"unresolved_proxy_custom", "https://unresolved.example.com/ep", nil, true, true},
		{"unresolved_direct", "https://unresolved.example.com/ep", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &pushService{endpointGuard: testPushGuard(t, tc.allowed), proxySelector: noPushProxy}
			if tc.proxied {
				s.proxySelector = func(*http.Request) (*url.URL, error) { return proxy, nil }
			}
			err := s.checkEndpointOnSubscribe(context.Background(), 1, tc.endpoint)
			if tc.wantError {
				require.Error(t, err)
				assert.NotErrorIs(t, err, errPushEndpointUnresolved)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPushRouting_DirectDialCheckedWithProxyConfiguration(t *testing.T) {
	assertPushDirectDialChecked(t, &pushService{proxySelector: noPushProxy}, "https://push.example.com/ep")
}

func assertPushDirectDialChecked(t *testing.T, s *pushService, endpoint string) {
	t.Helper()
	// Моделируем выбор локального адреса после определения прямого маршрута.
	// Проверяющий dialer должен отказать до обращения к контрольному listener.
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(201) }))
	defer server.Close()
	client := s.newGuardedHTTPClient()
	defer client.CloseIdleConnections()
	transport := client.Transport.(*pushRoutingTransport)
	dial := transport.direct.DialContext
	transport.direct.DialContext = func(ctx context.Context, network, _ string) (netConn net.Conn, err error) {
		return dial(ctx, network, server.Listener.Addr().String())
	}
	_, err := client.Get(endpoint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "отклонено")
	assert.Zero(t, hits.Load())
}

func TestPushRouting_ConcurrentRoutes(t *testing.T) {
	var directHits, proxyHits atomic.Int32
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { directHits.Add(1); w.WriteHeader(201) }))
	defer direct.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { proxyHits.Add(1); w.WriteHeader(201) }))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	s := &pushService{endpointGuard: &pushEndpointGuard{allowLocal: true, allowedHosts: []string{"push.example.com"}}, proxySelector: func(req *http.Request) (*url.URL, error) {
		if req.URL.Hostname() == "push.example.com" {
			return proxyURL, nil
		}
		return nil, nil
	}}
	client := s.newGuardedHTTPClient()
	defer client.CloseIdleConnections()
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		endpoint := direct.URL + "/ep"
		if i%2 == 0 {
			endpoint = "http://push.example.com/ep"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.Get(endpoint)
			if assert.NoError(t, err) {
				assert.Equal(t, 201, response.StatusCode)
				response.Body.Close()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(12), directHits.Load())
	assert.Equal(t, int32(12), proxyHits.Load())
}

func TestPushRouting_HTTPSProxyConnect(t *testing.T) {
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodConnect, r.Method)
		assert.Equal(t, "fcm.googleapis.com:443", r.Host)
		w.WriteHeader(http.StatusBadGateway) // Туннель и внешнее соединение не открываются.
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	s := &pushService{proxySelector: func(*http.Request) (*url.URL, error) { return proxyURL, nil }}
	client := s.newGuardedHTTPClient()
	defer client.CloseIdleConnections()
	_, err = client.Get("https://fcm.googleapis.com/ep")
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
}

func TestPushRouting_ProxyDoesNotFollowRedirect(t *testing.T) {
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Location", "http://127.0.0.1/unused")
		w.WriteHeader(http.StatusFound)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	s := &pushService{endpointGuard: &pushEndpointGuard{allowLocal: true, allowedHosts: []string{"push.example.com"}}, proxySelector: func(*http.Request) (*url.URL, error) { return proxyURL, nil }}
	client := s.newGuardedHTTPClient()
	defer client.CloseIdleConnections()
	response, err := client.Get("http://push.example.com/ep")
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusFound, response.StatusCode)
	assert.Equal(t, int32(1), calls.Load())
}

type pushCloseProbe struct {
	io.Reader
	closed bool
}

func (p *pushCloseProbe) Close() error { p.closed = true; return nil }

func TestPushRouting_RejectClosesBody(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.invalid:3128")
	require.NoError(t, err)
	s := &pushService{proxySelector: func(*http.Request) (*url.URL, error) { return proxyURL, nil }}
	client := s.newGuardedHTTPClient()
	defer client.CloseIdleConnections()
	body := &pushCloseProbe{Reader: strings.NewReader("synthetic")}
	req, err := http.NewRequest(http.MethodPost, "https://unlisted.example.com/ep", body)
	require.NoError(t, err)
	_, err = client.Transport.RoundTrip(req)
	require.Error(t, err)
	assert.True(t, body.closed)
}

func TestPushRouting_RedactsErrorsAndLogs(t *testing.T) {
	const secret = "synthetic-secret-never-log"
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	s := &pushService{endpointGuard: testPushGuard(t, nil), proxySelector: noPushProxy}
	for _, endpoint := range []string{"https://user:" + secret + "@push.example.com/ep", "https://push.example.com/%zz?token=" + secret} {
		require.Error(t, s.checkEndpointOnSubscribe(context.Background(), 1, endpoint))
	}
	assert.NotContains(t, logs.String(), secret)
	s.proxySelector = func(*http.Request) (*url.URL, error) { return nil, errors.New("proxy password=" + secret) }
	req, err := http.NewRequest(http.MethodPost, "https://fcm.googleapis.com/ep", nil)
	require.NoError(t, err)
	_, err = s.proxyForRequest(req)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	networkError := &url.Error{Op: "Post", URL: "https://push.example.com/" + secret, Err: errors.New("proxy password=" + secret)}
	assert.NotContains(t, safePushDeliveryError(networkError), secret)
	assert.Contains(t, safePushDeliveryError(context.DeadlineExceeded), "время")
	assert.Contains(t, safePushDeliveryError(context.Canceled), "отменена")
}
