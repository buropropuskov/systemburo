package services

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

type pushProxySelector func(*http.Request) (*url.URL, error)
type pushProxyContextKey struct{}

// Раздельные пулы сохраняют проверку прямого соединения независимо от прокси.
// Выбранный прокси хранится в запросе, а не в изменяемом состоянии транспорта.
type pushRoutingTransport struct {
	direct, proxied *http.Transport
	selectProxy     pushProxySelector
	guard           *pushEndpointGuard
}

func (s *pushService) proxyForRequest(req *http.Request) (*url.URL, error) {
	selector := s.proxySelector
	if selector == nil {
		selector = http.ProxyFromEnvironment
	}
	proxy, err := selector(req)
	if err != nil {
		// Ошибка разбора прокси может содержать учётные данные из настроек установки.
		return nil, errors.New("не удалось определить маршрут службы уведомлений")
	}
	return proxy, nil
}

func (g *pushEndpointGuard) checkProxyURL(u *url.URL) error {
	if u.User != nil || (u.Scheme != "https" && !(g.allowLocal && u.Scheme == "http")) {
		return errors.New("недопустимый адрес службы уведомлений")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	allowed := g.allowedHosts
	if len(allowed) == 0 {
		allowed = pushKnownServiceHosts
	}
	if host == "" || !hostMatchesPushAllowlist(host, allowed) {
		return errors.New("служба уведомлений не разрешена для отправки через прокси (PUSH_ALLOWED_HOSTS)")
	}
	return nil
}

func (s *pushService) checkEndpoint(ctx context.Context, endpoint string) error {
	err := s.guard().check(ctx, endpoint)
	if err != nil && !errors.Is(err, errPushEndpointUnresolved) {
		return err
	}
	req, parseErr := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(endpoint), nil)
	if parseErr != nil {
		return errors.New("недопустимый адрес службы уведомлений")
	}
	proxy, routeErr := s.proxyForRequest(req)
	if routeErr != nil {
		return routeErr
	}
	if proxy != nil {
		if policyErr := s.guard().checkProxyURL(req.URL); policyErr != nil {
			return policyErr
		}
	}
	return err
}

func (s *pushService) newGuardedHTTPClient() *http.Client {
	direct := http.DefaultTransport.(*http.Transport).Clone()
	direct.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	dialer.Control = func(_, address string, _ syscall.RawConn) error {
		return s.guard().checkDialAddress(address)
	}
	direct.DialContext = dialer.DialContext
	proxied := http.DefaultTransport.(*http.Transport).Clone()
	proxied.Proxy = func(req *http.Request) (*url.URL, error) {
		proxy, ok := req.Context().Value(pushProxyContextKey{}).(*url.URL)
		if !ok || proxy == nil {
			return nil, errors.New("не определён маршрут службы уведомлений")
		}
		return proxy, nil
	}
	return &http.Client{
		Transport:     &pushRoutingTransport{direct: direct, proxied: proxied, selectProxy: s.proxyForRequest, guard: s.guard()},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (t *pushRoutingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	proxy, err := t.selectProxy(req)
	if err == nil && proxy != nil {
		err = t.guard.checkProxyURL(req.URL)
	}
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	if proxy == nil {
		return t.direct.RoundTrip(req)
	}
	forward := req.Clone(context.WithValue(req.Context(), pushProxyContextKey{}, proxy))
	return t.proxied.RoundTrip(forward)
}

func (t *pushRoutingTransport) CloseIdleConnections() {
	t.direct.CloseIdleConnections()
	t.proxied.CloseIdleConnections()
}

func pushEndpointHost(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func safePushDeliveryError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "отправка уведомления отменена"
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return "истекло время отправки уведомления"
	}
	return "не удалось отправить уведомление: ошибка соединения или адреса службы"
}
