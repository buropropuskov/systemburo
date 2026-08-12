package httpx

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyServerTimeouts_ReachTheServer - заданные сроки доезжают до http.Server,
// а не теряются по дороге от конфигурации.
func TestApplyServerTimeouts_ReachTheServer(t *testing.T) {
	t.Parallel()

	srv := &http.Server{}
	ApplyServerTimeouts(srv, ServerTimeouts{
		ReadHeader: 10 * time.Second,
		Read:       120 * time.Second,
		Write:      120 * time.Second,
		Idle:       120 * time.Second,
	})

	assert.Equal(t, 10*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 120*time.Second, srv.ReadTimeout)
	assert.Equal(t, 120*time.Second, srv.WriteTimeout)
	assert.Equal(t, 120*time.Second, srv.IdleTimeout)
}

// TestApplyServerTimeouts_ZeroMeansNoLimit: ноль обязан доезжать нулём. Это
// семантика net/http ("без срока") и единственный способ администратору выключить
// таймаут параметром окружения, не правя код.
func TestApplyServerTimeouts_ZeroMeansNoLimit(t *testing.T) {
	t.Parallel()

	srv := &http.Server{
		ReadHeaderTimeout: time.Minute,
		ReadTimeout:       time.Minute,
		WriteTimeout:      time.Minute,
		IdleTimeout:       time.Minute,
	}
	ApplyServerTimeouts(srv, ServerTimeouts{})

	assert.Zero(t, srv.ReadHeaderTimeout)
	assert.Zero(t, srv.ReadTimeout)
	assert.Zero(t, srv.WriteTimeout)
	assert.Zero(t, srv.IdleTimeout)
}

// TestAllowLongResponse_SurvivesWriteTimeout - главная проверка: обработчик, который
// пишет дольше общего WriteTimeout, доходит до конца, сняв срок с себя.
//
// Ответ отдаётся кусками с паузами, как SSE-поток или потоковый ZIP файлового
// архива, а WriteTimeout сервера заведомо короче суммарного времени записи.
func TestAllowLongResponse_SurvivesWriteTimeout(t *testing.T) {
	t.Parallel()

	const (
		writeTimeout = 150 * time.Millisecond
		chunks       = 6
		chunkPause   = 50 * time.Millisecond
	)

	e := echo.New()
	e.GET("/long", func(c echo.Context) error {
		require.NoError(t, AllowLongResponse(c))

		res := c.Response()
		res.WriteHeader(http.StatusOK)
		for i := range chunks {
			if _, err := fmt.Fprintf(res, "chunk-%d\n", i); err != nil {
				return err
			}
			res.Flush()
			time.Sleep(chunkPause)
		}
		return nil
	})

	srv := httptest.NewUnstartedServer(e)
	ApplyServerTimeouts(srv.Config, ServerTimeouts{Write: writeTimeout})
	srv.Start()
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/long")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "поток оборвался - срок записи снят не был")

	// Запись шла заведомо дольше WriteTimeout, и все куски обязаны доехать целиком.
	require.Greater(t, chunks*chunkPause, writeTimeout, "тест обязан писать дольше таймаута")
	for i := range chunks {
		assert.Contains(t, string(body), fmt.Sprintf("chunk-%d", i))
	}
}

// TestAllowLongResponse_WriteTimeoutCutsWithoutIt - тот же обработчик без снятия
// срока обрывается. Без этой проверки предыдущий тест был бы зелёным и при
// AllowLongResponse, не делающем ничего.
func TestAllowLongResponse_WriteTimeoutCutsWithoutIt(t *testing.T) {
	t.Parallel()

	const (
		writeTimeout = 150 * time.Millisecond
		chunks       = 6
		chunkPause   = 50 * time.Millisecond
	)

	e := echo.New()
	e.GET("/long", func(c echo.Context) error {
		res := c.Response()
		res.WriteHeader(http.StatusOK)
		for i := range chunks {
			// Ошибку записи глотаем намеренно: проверяем, что увидит клиент.
			fmt.Fprintf(res, "chunk-%d\n", i)
			res.Flush()
			time.Sleep(chunkPause)
		}
		return nil
	})

	srv := httptest.NewUnstartedServer(e)
	ApplyServerTimeouts(srv.Config, ServerTimeouts{Write: writeTimeout})
	srv.Start()
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/long")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	// Сервер закрывает соединение посреди ответа: клиент видит либо ошибку чтения,
	// либо усечённое тело. Целым оно быть не может - иначе таймаут не сработал.
	if readErr == nil {
		assert.NotContains(t, string(body), fmt.Sprintf("chunk-%d", chunks-1),
			"ответ доехал целиком - WriteTimeout не сработал, и первый тест ничего не доказывает")
	}
}

// TestAllowLongResponse_ReportsUnsupportedWriter - на писателе без доступа к
// соединению возвращается ошибка, а не тихий успех. Тихий успех означал бы, что
// длинный ответ оборвётся по таймауту, а в логе об этом не будет ни строки.
func TestAllowLongResponse_ReportsUnsupportedWriter(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/long", nil)
	c := e.NewContext(req, httptest.NewRecorder())

	err := AllowLongResponse(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write deadline")
}

// TestApplyServerTimeouts_SurviveEchoStart - сроки, проставленные в e.Server до
// запуска, действуют на поднятом сервере Echo.
//
// Проверка стережёт именно связку с Echo, а не net/http: настройка делается в main
// над e.Server, и достаточно смены версии Echo, где Start пересоздаёт сервер или
// сбрасывает поля, чтобы вся защита молча перестала существовать - при зелёных
// остальных тестах и без единой строки в логе.
func TestApplyServerTimeouts_SurviveEchoStart(t *testing.T) {
	t.Parallel()

	const writeTimeout = 200 * time.Millisecond

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Долгий ответ без снятия срока: сервер обязан его оборвать.
	e.GET("/slow", func(c echo.Context) error {
		time.Sleep(2 * writeTimeout)
		return c.String(http.StatusOK, "late")
	})
	// Тот же долгий ответ, но со снятием срока - обязан дойти целиком.
	e.GET("/slow-allowed", func(c echo.Context) error {
		require.NoError(t, AllowLongResponse(c))
		time.Sleep(2 * writeTimeout)
		return c.String(http.StatusOK, "late")
	})

	ApplyServerTimeouts(e.Server, ServerTimeouts{Write: writeTimeout})

	go func() { _ = e.Start("127.0.0.1:0") }()
	t.Cleanup(func() { _ = e.Close() })

	var addr net.Addr
	require.Eventually(t, func() bool {
		addr = e.ListenerAddr()
		return addr != nil
	}, 5*time.Second, 10*time.Millisecond, "сервер не поднялся")

	base := "http://" + addr.String()
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(base + "/slow")
	if err == nil {
		defer resp.Body.Close()
		_, readErr := io.ReadAll(resp.Body)
		assert.Error(t, readErr, "ответ дольше WriteTimeout доехал целиком - таймаут до сервера не доехал")
	}

	allowed, err := client.Get(base + "/slow-allowed")
	require.NoError(t, err)
	defer allowed.Body.Close()
	body, err := io.ReadAll(allowed.Body)
	require.NoError(t, err, "длинный обработчик оборвался, хотя снял срок записи")
	assert.Equal(t, "late", string(body))
}
