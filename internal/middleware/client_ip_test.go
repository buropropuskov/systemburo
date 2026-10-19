package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestClientIPExtractor_TrustBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, peer, realIP, expected string
		trusted                      []string
	}{
		{"direct", "198.51.100.2:1234", "203.0.113.8", "198.51.100.2", nil},
		{"private untrusted", "172.20.0.2:1234", "203.0.113.8", "172.20.0.2", []string{"172.20.0.3/32"}},
		{"loopback untrusted", "127.0.0.1:1234", "203.0.113.8", "127.0.0.1", []string{"172.20.0.3/32"}},
		{"trusted proxy", "172.20.0.3:1234", "203.0.113.8", "203.0.113.8", []string{"172.20.0.3/32"}},
		{"invalid header", "172.20.0.3:1234", "invalid", "172.20.0.3", []string{"172.20.0.3/32"}},
		{"missing header", "172.20.0.3:1234", "", "172.20.0.3", []string{"172.20.0.3/32"}},
		{"IPv6 proxy", "[fd00::3]:1234", "2001:0db8:0:0:0:0:0:5", "2001:db8::5", []string{"fd00::3/128"}},
		{"IPv6 direct", "[2001:0db8::5]:1234", "203.0.113.8", "2001:db8::5", nil},
		{"invalid peer", "invalid", "203.0.113.8", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extract, err := ClientIPExtractor(tc.trusted)
			require.NoError(t, err)
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set(echo.HeaderXRealIP, tc.realIP)
			r.Header.Set(echo.HeaderXForwardedFor, "192.0.2.99")
			require.Equal(t, tc.expected, extract(r))
		})
	}
	_, err := ClientIPExtractor([]string{"not-a-cidr"})
	require.Error(t, err)
}
