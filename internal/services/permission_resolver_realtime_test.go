package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/realtime"
	"systemburo/internal/services"
)

// recvEvent читает событие из канала подписчика или падает по таймауту.
func recvEvent(t *testing.T, ch <-chan realtime.Event) (realtime.Event, bool) {
	t.Helper()
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(time.Second):
		return realtime.Event{}, false
	}
}

// TestPermissionResolver_Invalidate_PublishesToUser: смена роли/группы/override
// конкретному юзеру (Invalidate(userID)) шлёт ЕМУ user.permissions (scope user:<id>) -
// App.vue перезапросит права мгновенно (#840). Адресно: другой юзер сигнал не
// получает. DB не нужна - Invalidate только чистит кэш и публикует.
func TestPermissionResolver_Invalidate_PublishesToUser(t *testing.T) {
	hub := realtime.NewHub()
	resolver := services.NewPermissionResolver(nil)
	resolver.SetRealtimePublisher(hub)

	ch5, unsub5 := hub.Subscribe(5)
	defer unsub5()
	ch9, unsub9 := hub.Subscribe(9)
	defer unsub9()

	resolver.Invalidate(5)

	ev, ok := recvEvent(t, ch5)
	require.True(t, ok, "юзер 5 должен получить сигнал смены прав")
	assert.Equal(t, "user.permissions", ev.Type)
	assert.Equal(t, "user:5", ev.Scope)

	_, gotOther := recvEvent(t, ch9)
	assert.False(t, gotOther, "невовлечённый юзер сигнал не получает (адресно)")
}

// TestPermissionResolver_InvalidateAll_BroadcastsToConnected: смена грантов
// роли/группы (InvalidateAll) затрагивает произвольный набор носителей, поэтому
// шлёт КАЖДОМУ подключённому сигнал перезапросить своё - у каждого свой scope
// user:<id>, чтобы сработала его подписка App.vue (#840).
func TestPermissionResolver_InvalidateAll_BroadcastsToConnected(t *testing.T) {
	hub := realtime.NewHub()
	resolver := services.NewPermissionResolver(nil)
	resolver.SetRealtimePublisher(hub)

	ch5, unsub5 := hub.Subscribe(5)
	defer unsub5()
	ch9, unsub9 := hub.Subscribe(9)
	defer unsub9()

	resolver.InvalidateAll()

	ev5, ok5 := recvEvent(t, ch5)
	require.True(t, ok5, "подключённый юзер 5 получает сигнал")
	assert.Equal(t, "user.permissions", ev5.Type)
	assert.Equal(t, "user:5", ev5.Scope)

	ev9, ok9 := recvEvent(t, ch9)
	require.True(t, ok9, "подключённый юзер 9 получает сигнал со своим scope")
	assert.Equal(t, "user:9", ev9.Scope)
}

// TestPermissionResolver_NilPublisherSafe: без паблишера инвалидация не паникует.
func TestPermissionResolver_NilPublisherSafe(t *testing.T) {
	resolver := services.NewPermissionResolver(nil)
	require.NotPanics(t, func() {
		resolver.Invalidate(5)
		resolver.InvalidateAll()
	})
}
