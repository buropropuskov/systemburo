package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// Проверка адреса службы доставки уведомлений (#2466). Адрес push-сервиса приходит от
// браузера вместе с подпиской (#974) и уходит в исходящий запрос сервера как есть -
// ни схема, ни узел раньше не проверялись. Любой пользователь системы мог сохранить
// подписку с адресом внутри сети (https://192.168.0.15:8080/...) и заставить сервер
// обращаться к соседям: ответ до него не доходит (тело зашифровано ключами подписки,
// сервер молчит о результате), но пилот разворачивается ВНУТРИ сети заказчика, где
// сервером можно прощупывать чужую инфраструктуру - есть порт или нет, отвечает узел
// или нет.

// errPushEndpointUnresolved -- имя узла не удалось разложить в адреса. Отдельная
// ошибка, а не общий отказ: "не знаю адрес" - это не "адрес внутренний". Стенд внутри
// сети заказчика может не иметь наружного DNS вовсе (выход через прокси), и отказ
// подписки в таком месте выглядел бы как поломка push на ровном месте. См. решение о
// поведении в pushService.Subscribe и pushService.deliver.
var errPushEndpointUnresolved = errors.New("имя узла службы уведомлений не разрешается в адрес")

// pushKnownServiceHosts -- узлы служб доставки, которыми пользуются браузеры. Список
// подставляется вместо ключевого слова known в PUSH_ALLOWED_HOSTS, чтобы белый список
// не пришлось выписывать руками и держать в актуальном состоянии в .env каждой
// установки. Совпадение считается по суффиксу: у Mozilla и WNS адрес выдаётся на
// конкретном узле кластера (autopush-1.push.services.mozilla.com, db5p.notify.windows.com).
var pushKnownServiceHosts = []string{
	"fcm.googleapis.com",        // Chrome, Edge, Яндекс.Браузер и прочие сборки Chromium
	"android.googleapis.com",    // старые сборки Chrome, адрес вида .../gcm/send
	"push.services.mozilla.com", // Firefox
	"notify.windows.com",        // WNS: Edge на Windows
	"push.apple.com",            // Safari, web.push.apple.com
}

// pushKnownHostsKeyword -- ключевое слово в PUSH_ALLOWED_HOSTS, раскрывающееся в
// pushKnownServiceHosts.
const pushKnownHostsKeyword = "known"

// pushLocalHostSuffixes -- имена, которые по определению указывают внутрь машины или
// локальной сети, независимо от того, во что их разложит DNS. Проверяются до
// разрешения имени: на машине без наружного DNS разрешение упало бы, и без этого
// списка внутреннее имя прошло бы как "не смогли проверить".
var pushLocalHostSuffixes = []string{
	".localhost",
	".localdomain",
	".local",     // mDNS/Bonjour
	".internal",  // внутренние зоны, в том числе в облаках
	".home.arpa", // RFC 8375, домашние и мелкие офисные сети
}

// pushBlockedPrefixes -- диапазоны, которые не покрываются готовыми проверками
// netip.Addr (IsPrivate, IsLoopback, IsLinkLocalUnicast и прочими), но наружу вести
// не могут.
var pushBlockedPrefixes = []struct {
	prefix netip.Prefix
	reason string
}{
	{netip.MustParsePrefix("0.0.0.0/8"), "адрес этой же сети"},
	{netip.MustParsePrefix("100.64.0.0/10"), "сеть оператора связи между NAT"},
	{netip.MustParsePrefix("192.0.0.0/24"), "служебный диапазон протоколов"},
	{netip.MustParsePrefix("198.18.0.0/15"), "диапазон для измерений сетевого оборудования"},
	{netip.MustParsePrefix("255.255.255.255/32"), "широковещательный адрес"},
}

// pushEndpointGuard решает, можно ли отправлять запрос по адресу подписки.
type pushEndpointGuard struct {
	// allowedHosts -- белый список узлов; пустой означает "проверяем только
	// диапазоны адресов". Пустой он не случайно, см. WithPushAllowedHosts.
	allowedHosts []string
	// allowLocal снимает требование https и разрешает петлю и частные сети. Нужен
	// тестам: httptest поднимает подставной push-сервис на http://127.0.0.1.
	allowLocal bool
	// resolve подменяется в тестах, чтобы проверять разбор имён без настоящего DNS.
	resolve func(ctx context.Context, host string) ([]netip.Addr, error)
}

func newPushEndpointGuard(allowedHosts []string) *pushEndpointGuard {
	return &pushEndpointGuard{
		allowedHosts: normalizePushAllowedHosts(allowedHosts),
		resolve:      resolvePushHost,
	}
}

func resolvePushHost(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// normalizePushAllowedHosts приводит список к нижнему регистру, выбрасывает пустые
// записи и раскрывает ключевое слово known в список известных служб.
func normalizePushAllowedHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	seen := make(map[string]bool, len(hosts))
	add := func(h string) {
		h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
		if h == "" || seen[h] {
			return
		}
		seen[h] = true
		out = append(out, h)
	}
	for _, h := range hosts {
		if strings.EqualFold(strings.TrimSpace(h), pushKnownHostsKeyword) {
			for _, known := range pushKnownServiceHosts {
				add(known)
			}
			continue
		}
		add(h)
	}
	return out
}

// check проверяет адрес подписки целиком: схема, учётные данные, узел, белый список и
// адреса, в которые узел разрешается. Сообщение об ошибке уходит человеку в интерфейс
// (см. pushService.Subscribe), поэтому оно связной русской фразой, а не кодом.
func (g *pushEndpointGuard) check(ctx context.Context, endpoint string) error {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return errors.New("адрес службы уведомлений пуст")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("адрес службы уведомлений не разбирается как ссылка: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && !(g.allowLocal && scheme == "http") {
		return fmt.Errorf("адрес службы уведомлений должен начинаться с https:// (получено %q)", u.Scheme)
	}
	// Имя и пароль в ссылке службам доставки не нужны ни одной: их появление означает
	// либо попытку подсунуть серверу чужие учётные данные, либо мусор в подписке.
	if u.User != nil {
		return errors.New("адрес службы уведомлений не должен содержать имя и пароль")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return errors.New("в адресе службы уведомлений не указан узел")
	}
	if len(g.allowedHosts) > 0 && !hostMatchesPushAllowlist(host, g.allowedHosts) {
		return fmt.Errorf("узел %q не входит в список разрешённых служб уведомлений (PUSH_ALLOWED_HOSTS)", host)
	}
	// Адрес записан числом - разрешать нечего, проверяем как есть.
	if addr, err := netip.ParseAddr(host); err == nil {
		return g.checkAddr(host, addr)
	}
	if reason := localPushHostReason(host); reason != "" {
		return fmt.Errorf("узел %q ведёт внутрь сети (%s), а служба уведомлений браузера находится снаружи", host, reason)
	}
	addrs, err := g.resolve(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: %s (%v)", errPushEndpointUnresolved, host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: %s", errPushEndpointUnresolved, host)
	}
	// Отвергаем, если ХОТЯ БЫ один из адресов внутренний: имя может отдавать
	// вперемешку наружный и внутренний адрес, и выбор между ними делает не сервер.
	for _, addr := range addrs {
		if err := g.checkAddr(host, addr); err != nil {
			return err
		}
	}
	return nil
}

// checkAddr проверяет один разрешённый адрес.
func (g *pushEndpointGuard) checkAddr(host string, addr netip.Addr) error {
	// Unmap: ::ffff:127.0.0.1 -- та же самая петля, записанная по-другому, и без
	// снятия обёртки ни одна из проверок ниже её бы не увидела.
	a := addr.Unmap()
	if g.allowLocal && (a.IsLoopback() || a.IsPrivate()) {
		return nil
	}
	if reason := blockedPushAddrReason(a); reason != "" {
		return fmt.Errorf("узел %s ведёт на адрес %s (%s), а служба уведомлений браузера находится снаружи", host, a, reason)
	}
	return nil
}

// checkDialAddress -- та же проверка, но в момент установления соединения, когда адрес
// уже выбран разрешателем имён (см. pushService.newGuardedHTTPClient). Закрывает
// подмену DNS между проверкой и запросом: имя отдаёт наружный адрес на проверке и
// внутренний через секунду, когда до него доходит дело. Заодно ловит перенаправление
// на внутренний адрес, если бы клиент по нему пошёл.
func (g *pushEndpointGuard) checkDialAddress(address string) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("адрес соединения %q не разобран: %w", address, err)
	}
	addr := ap.Addr().Unmap()
	if g.allowLocal && (addr.IsLoopback() || addr.IsPrivate()) {
		return nil
	}
	if reason := blockedPushAddrReason(addr); reason != "" {
		return fmt.Errorf("соединение со службой уведомлений по адресу %s отклонено (%s)", addr, reason)
	}
	return nil
}

// blockedPushAddrReason возвращает причину отказа для внутреннего или служебного
// адреса и пустую строку для обычного наружного.
func blockedPushAddrReason(addr netip.Addr) string {
	switch {
	case !addr.IsValid():
		return "адрес не разобран"
	case addr.IsUnspecified():
		return "неопределённый адрес"
	case addr.IsLoopback():
		return "петля самой машины"
	case addr.IsPrivate():
		return "частная сеть"
	case addr.IsLinkLocalUnicast():
		// 169.254.0.0/16 и fe80::/10. Сюда же попадает 169.254.169.254 - служба
		// метаданных облачных площадок, самая ценная цель такого запроса.
		return "адрес канального уровня, в облаках - служба метаданных"
	case addr.IsInterfaceLocalMulticast(), addr.IsLinkLocalMulticast(), addr.IsMulticast():
		return "групповая рассылка"
	}
	for _, blocked := range pushBlockedPrefixes {
		if blocked.prefix.Contains(addr) {
			return blocked.reason
		}
	}
	return ""
}

// localPushHostReason ловит имена, заведомо указывающие внутрь, до разрешения в адрес.
func localPushHostReason(host string) string {
	if host == "localhost" {
		return "это имя самой машины"
	}
	for _, suffix := range pushLocalHostSuffixes {
		if strings.HasSuffix(host, suffix) {
			return "зона " + strings.TrimPrefix(suffix, ".") + " не выходит за пределы локальной сети"
		}
	}
	// Имя без единой точки - это имя соседа по локальной сети или службы в docker
	// (backend, db, pgadmin). У служб доставки адрес всегда полностью определённый.
	if !strings.Contains(host, ".") {
		return "имя без домена принадлежит локальной сети"
	}
	return ""
}

// hostMatchesPushAllowlist -- совпадение по узлу целиком либо по поддомену записи
// белого списка. Суффиксное сравнение делается по метке ".запись", а не по голому
// strings.HasSuffix: иначе злоумышленник прошёл бы с именем вида
// "злойpush.apple.com" без точки перед записью.
func hostMatchesPushAllowlist(host string, allowed []string) bool {
	for _, a := range allowed {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}
