package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// ClientIPExtractor accepts X-Real-IP only from explicitly configured proxies.
// The edge proxy must overwrite that header with its connection's remote IP.
// Without trusted proxies, all forwarding headers are ignored.
func ClientIPExtractor(trustedCIDRs []string) (echo.IPExtractor, error) {
	options := []echo.TrustOption{
		echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false),
	}
	for _, cidr := range trustedCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", cidr, err)
		}
		options = append(options, echo.TrustIPRange(network))
	}
	extract := echo.ExtractIPDirect()
	if len(trustedCIDRs) > 0 {
		extract = echo.ExtractIPFromRealIPHeader(options...)
	}
	return func(r *http.Request) string {
		if ip := net.ParseIP(extract(r)); ip != nil {
			return ip.String()
		}
		return "" // Invalid addresses share one budget rather than arbitrary keys.
	}, nil
}
