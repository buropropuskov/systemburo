package services

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Проверка адреса службы доставки уведомлений (#2466): без неё подписка с адресом
// внутри сети превращала сервер в средство прощупывать соседей. Тесты чистые - DNS
// подменён, база и сеть не нужны.

// stubResolver подменяет разрешение имён: карта "имя -> адреса", неизвестное имя
// отвечает ошибкой, как настоящий DNS на несуществующей записи.
func stubResolver(table map[string][]string) func(context.Context, string) ([]netip.Addr, error) {
	return func(_ context.Context, host string) ([]netip.Addr, error) {
		raw, ok := table[host]
		if !ok {
			return nil, fmt.Errorf("no such host: %s", host)
		}
		addrs := make([]netip.Addr, 0, len(raw))
		for _, r := range raw {
			addrs = append(addrs, netip.MustParseAddr(r))
		}
		return addrs, nil
	}
}

// testPushGuard -- страж с подменённым DNS: наружные имена разрешаются в наружные
// адреса, "внутреннее" имя - во внутренний (так выглядит попытка обойти проверку по
// имени), остальные не разрешаются вовсе.
func testPushGuard(t *testing.T, allowedHosts []string) *pushEndpointGuard {
	t.Helper()
	g := newPushEndpointGuard(allowedHosts)
	g.resolve = stubResolver(map[string][]string{
		"fcm.googleapis.com":   {"142.250.185.234"},
		"web.push.apple.com":   {"17.188.166.1"},
		"push.example.com":     {"93.184.216.34"},
		"inside.example.com":   {"10.0.0.5"},
		"mixed.example.com":    {"93.184.216.34", "192.168.1.7"},
		"evilpush.apple.com":   {"93.184.216.34"},
		"metadata.example.com": {"169.254.169.254"},
	})
	return g
}

func TestPushEndpointGuard_AcceptsRealPushService(t *testing.T) {
	g := testPushGuard(t, nil)
	require.NoError(t, g.check(context.Background(), "https://fcm.googleapis.com/fcm/send/abc-123"))
	require.NoError(t, g.check(context.Background(), "https://push.example.com/ep-1"))
}

// TestPushEndpointGuard_RejectsInternalAddresses -- ядро #2466: адрес внутрь сети не
// должен сохраняться и не должен использоваться для исходящего запроса.
func TestPushEndpointGuard_RejectsInternalAddresses(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		wantText string
	}{
		{"частная сеть по числовому адресу", "https://192.168.0.15:8080/ep", "частная сеть"},
		{"десятка", "https://10.0.0.5/ep", "частная сеть"},
		{"диапазон 172.16/12", "https://172.16.3.9/ep", "частная сеть"},
		{"петля", "https://127.0.0.1/ep", "петля самой машины"},
		{"петля IPv6", "https://[::1]/ep", "петля самой машины"},
		{"петля в обёртке IPv4-mapped", "https://[::ffff:127.0.0.1]/ep", "петля самой машины"},
		{"служба метаданных", "https://169.254.169.254/latest/meta-data/", "метаданных"},
		{"уникальный локальный IPv6", "https://[fd00::1]/ep", "частная сеть"},
		{"канальный IPv6", "https://[fe80::1]/ep", "канального уровня"},
		{"неопределённый адрес", "https://0.0.0.0/ep", "неопределённый адрес"},
		{"имя самой машины", "https://localhost/ep", "имя самой машины"},
		{"имя соседа по docker-сети", "https://backend/ep", "имя без домена"},
		{"внутренняя зона", "https://db.internal/ep", "зона internal"},
		{"зона mDNS", "https://printer.local/ep", "зона local"},
		{"имя, разрешающееся внутрь сети", "https://inside.example.com/ep", "10.0.0.5"},
		{"имя с наружным и внутренним адресом вперемешку", "https://mixed.example.com/ep", "192.168.1.7"},
		{"имя службы метаданных", "https://metadata.example.com/ep", "метаданных"},
	}
	g := testPushGuard(t, nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := g.check(context.Background(), tc.endpoint)
			require.Error(t, err, "адрес должен быть отвергнут")
			assert.Contains(t, err.Error(), tc.wantText)
			assert.NotErrorIs(t, err, errPushEndpointUnresolved, "это отказ по существу, а не молчание DNS")
		})
	}
}

// TestPushEndpointGuard_RejectsBadShape: схема, учётные данные и мусор вместо ссылки.
func TestPushEndpointGuard_RejectsBadShape(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		wantText string
	}{
		{"http вместо https", "http://fcm.googleapis.com/fcm/send/abc", "https://"},
		{"чужая схема", "gopher://fcm.googleapis.com/ep", "https://"},
		{"без схемы", "fcm.googleapis.com/ep", "https://"},
		{"учётные данные в ссылке", "https://user:pass@fcm.googleapis.com/ep", "имя и пароль"},
		{"пустой узел", "https:///ep", "не указан узел"},
		{"пустая строка", "   ", "пуст"},
	}
	g := testPushGuard(t, nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := g.check(context.Background(), tc.endpoint)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantText)
		})
	}
}

// TestPushEndpointGuard_UnresolvedIsSeparateOutcome: неразрешимое имя - отдельная
// ошибка, а не отказ по существу. Установка внутри сети заказчика может ходить наружу
// через прокси и не иметь наружного DNS вовсе, и подписку там отвергать не за что.
func TestPushEndpointGuard_UnresolvedIsSeparateOutcome(t *testing.T) {
	g := testPushGuard(t, nil)
	err := g.check(context.Background(), "https://unknown.example.org/ep")
	require.Error(t, err)
	assert.ErrorIs(t, err, errPushEndpointUnresolved)
}

// TestPushEndpointGuard_AllowlistNarrows: заполненный белый список сужает круг узлов,
// пустой - пропускает любой наружный.
func TestPushEndpointGuard_AllowlistNarrows(t *testing.T) {
	open := testPushGuard(t, nil)
	require.NoError(t, open.check(context.Background(), "https://push.example.com/ep"),
		"пустой список не должен резать наружные службы")

	narrow := testPushGuard(t, []string{pushKnownHostsKeyword})
	require.NoError(t, narrow.check(context.Background(), "https://fcm.googleapis.com/fcm/send/abc"))
	require.NoError(t, narrow.check(context.Background(), "https://web.push.apple.com/ep"),
		"поддомен записи списка разрешён")

	err := narrow.check(context.Background(), "https://push.example.com/ep")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "не входит в список")
}

// TestPushEndpointGuard_AllowlistMatchesByLabel: совпадение считается по метке, иначе
// узел "evilpush.apple.com" прошёл бы как поддомен "push.apple.com".
func TestPushEndpointGuard_AllowlistMatchesByLabel(t *testing.T) {
	g := testPushGuard(t, []string{"push.apple.com"})
	err := g.check(context.Background(), "https://evilpush.apple.com/ep")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "не входит в список")
}

// TestPushEndpointGuard_LocalEndpointsOnlyForTests: опция WithPushLocalEndpoints
// открывает петлю и http - на этом стоят тесты доставки с httptest-сервером.
func TestPushEndpointGuard_LocalEndpointsOnlyForTests(t *testing.T) {
	g := testPushGuard(t, nil)
	require.Error(t, g.check(context.Background(), "http://127.0.0.1:41234/ep"))

	g.allowLocal = true
	require.NoError(t, g.check(context.Background(), "http://127.0.0.1:41234/ep"))
	require.NoError(t, g.check(context.Background(), "https://inside.example.com/ep"))
}

// TestNormalizePushAllowedHosts: раскрытие known, регистр, пробелы, пустые записи и
// завершающая точка в имени.
func TestNormalizePushAllowedHosts(t *testing.T) {
	got := normalizePushAllowedHosts([]string{" KNOWN ", "", "Push.Corp.Example.", "push.corp.example"})
	assert.Subset(t, got, pushKnownServiceHosts, "known должен раскрыться в список известных служб")
	assert.Contains(t, got, "push.corp.example")
	assert.Len(t, got, len(pushKnownServiceHosts)+1, "повтор и пустая запись не должны попадать в список")
}

// TestPushEndpointGuard_DefaultResolverIsWiredIn -- страж по умолчанию ходит в
// настоящий DNS, а не остаётся с nil-резолвером: без этой строки check падал бы
// паникой на первом же имени в проде, а все тесты выше остались бы зелёными.
func TestPushEndpointGuard_DefaultResolverIsWiredIn(t *testing.T) {
	g := newPushEndpointGuard(nil)
	require.NotNil(t, g.resolve)
	err := g.check(context.Background(), "https://192.168.0.15/ep")
	require.Error(t, err)
	assert.False(t, errors.Is(err, errPushEndpointUnresolved))
}

// TestPushService_NilGuard_FallsBackToStrict: pushService, собранный литералом
// структуры (так делают соседние тесты этого пакета), не имеет стража - и должен
// получить строгого, а не отправлять без проверки.
func TestPushService_NilGuard_FallsBackToStrict(t *testing.T) {
	s := &pushService{}
	require.NotNil(t, s.guard())
	require.Error(t, s.guard().check(context.Background(), "https://192.168.0.15/ep"))
}

// clearPushProxyEnv снимает переменные прокси на время теста: при заданном прокси
// проверка адреса в момент соединения намеренно выключена (сервер соединяется с
// прокси, а тот стоит внутри сети), и тесты этой проверки нужно ставить в известные
// условия, а не полагаться на окружение сборки.
func clearPushProxyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(name, "")
	}
}

// TestPushGuardedClient_RefusesInternalDialTarget (#2466): даже если адрес прошёл
// проверку подписки, соединение с внутренним адресом не состоится - это защита от
// подмены DNS между проверкой и запросом.
func TestPushGuardedClient_RefusesInternalDialTarget(t *testing.T) {
	clearPushProxyEnv(t)
	s := &pushService{}

	// Порт 9 (discard) выбран нарочно: до соединения дело не дойдёт, проверка
	// сработает раньше, и тест не зависит от того, слушает ли кто-нибудь порт.
	_, err := s.newGuardedHTTPClient().Get("http://127.0.0.1:9/ep")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "отклонено")
}

// TestPushGuardedClient_DoesNotFollowRedirects (#2466): перенаправление на внутренний
// адрес - способ обойти проверку самого адреса подписки, поэтому клиент по
// перенаправлениям не ходит, а отдаёт 3xx как обычный неуспешный ответ.
func TestPushGuardedClient_DoesNotFollowRedirects(t *testing.T) {
	clearPushProxyEnv(t)
	var internalHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal" {
			internalHits++
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/internal", http.StatusFound)
	}))
	defer srv.Close()

	// allowLocal - подставная служба поднята httptest-ом на 127.0.0.1, иначе до неё не
	// дошло бы и первое соединение.
	s := &pushService{endpointGuard: &pushEndpointGuard{allowLocal: true, resolve: resolvePushHost}}
	resp, err := s.newGuardedHTTPClient().Get(srv.URL + "/ep")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode, "перенаправление должно вернуться как есть")
	assert.Equal(t, 0, internalHits, "клиент не должен был пойти по перенаправлению")
}

// TestPushLocalEndpoints_NotWiredIntoServer -- замок против недосмотра (#2466).
// WithPushLocalEndpoints - единственное место, где проверка адреса снимается целиком,
// и нужна она только тестам: подставная служба поднимается httptest-ом на 127.0.0.1.
// Попади она в сборку сервера, защита исчезла бы вся, причём молча: уведомления
// продолжили бы работать, и заметить это на глаз в ревью нечем. Поэтому проверяем
// сборку сервера текстом, а не надеемся на внимательность.
func TestPushLocalEndpoints_NotWiredIntoServer(t *testing.T) {
	cmdRoot := filepath.Join("..", "..", "cmd")
	var found []string
	err := filepath.WalkDir(cmdRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "WithPushLocalEndpoints") {
			found = append(found, path)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, found, "опция снимает проверку адреса службы уведомлений целиком - в сборке сервера её быть не должно")
}
