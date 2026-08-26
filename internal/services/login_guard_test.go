package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoginGuard_CounterDecrementsThenBlocks(t *testing.T) {
	g := newLoginGuard(10, 5*time.Minute, time.Minute)

	// Попытки 1..9 - остаток убывает 9..1, без блокировки.
	for i := 1; i <= 9; i++ {
		remaining, _, blocked := g.recordFailure("1.1.1.1", "u")
		assert.False(t, blocked, "попытка %d не должна блокировать", i)
		assert.Equal(t, 10-i, remaining, "остаток на попытке %d", i)
	}

	// 10-я попытка исчерпывает лимит - сразу блокировка (без "осталось 0").
	remaining, blockedSec, blocked := g.recordFailure("1.1.1.1", "u")
	assert.True(t, blocked, "10-я попытка блокирует")
	assert.Equal(t, 0, remaining)
	assert.Greater(t, blockedSec, 0)
}

func TestLoginGuard_BlockedSecondsReportsRemainder(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	g.recordFailure("2.2.2.2", "u")
	g.recordFailure("2.2.2.2", "u") // блокирует

	sec, blocked := g.blockedSeconds("2.2.2.2", "u")
	assert.True(t, blocked)
	assert.Greater(t, sec, 0)
	assert.LessOrEqual(t, sec, 60)
}

func TestLoginGuard_FreshCycleAfterBlockExpires(t *testing.T) {
	// Короткая блокировка, чтобы проверить истечение без ожидания минуты.
	g := newLoginGuard(2, 5*time.Minute, 20*time.Millisecond)
	g.recordFailure("3.3.3.3", "u")
	_, _, blocked := g.recordFailure("3.3.3.3", "u")
	assert.True(t, blocked)

	time.Sleep(30 * time.Millisecond) // блокировка истекла

	_, ok := g.blockedSeconds("3.3.3.3", "u")
	assert.False(t, ok, "после истечения не заблокирован")

	// Свежий цикл: первая новая неудача даёт остаток max-1, а не ре-лок.
	remaining, _, reblocked := g.recordFailure("3.3.3.3", "u")
	assert.False(t, reblocked, "не лочит мгновенно после истечения")
	assert.Equal(t, 1, remaining, "свежий цикл: 2 - 1 = 1")
}

func TestLoginGuard_ResetOnSuccess(t *testing.T) {
	g := newLoginGuard(3, 5*time.Minute, time.Minute)
	g.recordFailure("4.4.4.4", "u")
	g.recordFailure("4.4.4.4", "u")
	g.reset("4.4.4.4")

	remaining, _, blocked := g.recordFailure("4.4.4.4", "u")
	assert.False(t, blocked)
	assert.Equal(t, 2, remaining, "после reset счётчик с нуля: 3 - 1 = 2")
}

// TestLoginGuard_PairLadderEscalates - счётчик пары «адрес + логин» растёт по той
// же лестнице, что блокировка учётной записи. Это и делает сроки одинаковыми для
// существующего и выдуманного логина.
func TestLoginGuard_PairLadderEscalates(t *testing.T) {
	base := 20 * time.Millisecond
	g := newLoginGuard(2, 5*time.Minute, base)

	for round, mult := range []int{1, 5, 15, 30, 60, 60} {
		// Отбываем прошлую блокировку, чтобы начался следующий круг.
		if round > 0 {
			g.mu.Lock()
			past := time.Now().Add(-time.Millisecond)
			g.entries["7.7.7.7"].blockedUntil = past
			g.entries["7.7.7.7"].users["ghost"].blockedUntil = past
			g.mu.Unlock()
		}
		var got time.Duration
		for i := 0; i < 2; i++ {
			g.recordFailure("7.7.7.7", "ghost")
		}
		g.mu.Lock()
		got = time.Until(g.entries["7.7.7.7"].users["ghost"].blockedUntil)
		g.mu.Unlock()
		want := base * time.Duration(mult)
		assert.InDelta(t, want.Milliseconds(), got.Milliseconds(), 15,
			"круг %d: ожидалась ступень x%d", round+1, mult)
	}
}

// TestLoginGuard_PairIsolatedPerUsername - лестница у каждого логина своя: сосед
// по адресу не получает чужую ступень.
func TestLoginGuard_PairIsolatedPerUsername(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, 20*time.Millisecond)
	g.recordFailure("8.8.8.8", "petrov")
	g.recordFailure("8.8.8.8", "petrov") // ступень petrov поднялась

	g.mu.Lock()
	assert.Equal(t, 1, g.entries["8.8.8.8"].users["petrov"].level)
	assert.Nil(t, g.entries["8.8.8.8"].users["sidorov"], "у соседа записи ещё нет")
	g.mu.Unlock()
}

func TestLoginGuard_ResetUserClearsHisIPs(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	// Один логин падал с двух адресов, с одного из них падал и другой логин.
	g.recordFailure("10.0.0.1", "petrov")
	g.recordFailure("10.0.0.1", "petrov")
	g.recordFailure("10.0.0.2", "petrov")
	g.recordFailure("10.0.0.2", "sidorov")
	g.recordFailure("10.0.0.3", "sidorov")
	g.recordFailure("10.0.0.3", "sidorov")

	assert.Equal(t, 2, g.resetUser("petrov"), "сняты оба адреса, где падал petrov")

	_, blocked := g.blockedSeconds("10.0.0.1", "u")
	assert.False(t, blocked, "адрес petrov разблокирован")
	_, blocked = g.blockedSeconds("10.0.0.3", "u")
	assert.True(t, blocked, "чужой адрес не тронут")
}

func TestLoginGuard_ResetUnknownUserIsNoop(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	g.recordFailure("11.0.0.1", "petrov")
	g.recordFailure("11.0.0.1", "petrov")

	assert.Equal(t, 0, g.resetUser("никого"))
	assert.Equal(t, 0, g.resetUser(""), "пустой логин ничего не чистит")
	_, blocked := g.blockedSeconds("11.0.0.1", "u")
	assert.True(t, blocked, "блокировка на месте")
}

func TestLoginGuard_PerIPIsolation(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	g.recordFailure("5.5.5.5", "u")
	_, _, blocked := g.recordFailure("5.5.5.5", "u") // IP1 заблокирован
	assert.True(t, blocked)

	// Другой IP - свой счётчик.
	_, blk := g.blockedSeconds("6.6.6.6", "u")
	assert.False(t, blk, "другой IP не заблокирован")
	remaining, _, blocked2 := g.recordFailure("6.6.6.6", "u")
	assert.False(t, blocked2)
	assert.Equal(t, 1, remaining)
}

func TestLoginGuard_EmptyIPNotTracked(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	remaining, _, blocked := g.recordFailure("", "u")
	assert.False(t, blocked)
	assert.Equal(t, 2, remaining, "пустой IP не учитывается")
	_, ok := g.blockedSeconds("", "u")
	assert.False(t, ok)
}

func TestLoginGuard_StaleWindowResetsCounter(t *testing.T) {
	// Короткое окно: неудачи старше окна не копятся.
	g := newLoginGuard(3, 20*time.Millisecond, time.Minute)
	g.recordFailure("7.7.7.7", "u")
	g.recordFailure("7.7.7.7", "u")

	time.Sleep(30 * time.Millisecond) // окно истекло

	// Счётчик сброшен - снова полный остаток.
	remaining, _, blocked := g.recordFailure("7.7.7.7", "u")
	assert.False(t, blocked)
	assert.Equal(t, 2, remaining, "после окна счётчик с нуля: 3 - 1 = 2")
}
