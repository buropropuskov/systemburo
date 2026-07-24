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
		remaining, _, blocked := g.recordFailure("1.1.1.1")
		assert.False(t, blocked, "попытка %d не должна блокировать", i)
		assert.Equal(t, 10-i, remaining, "остаток на попытке %d", i)
	}

	// 10-я попытка исчерпывает лимит - сразу блокировка (без "осталось 0").
	remaining, blockedSec, blocked := g.recordFailure("1.1.1.1")
	assert.True(t, blocked, "10-я попытка блокирует")
	assert.Equal(t, 0, remaining)
	assert.Greater(t, blockedSec, 0)
}

func TestLoginGuard_BlockedSecondsReportsRemainder(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	g.recordFailure("2.2.2.2")
	g.recordFailure("2.2.2.2") // блокирует

	sec, blocked := g.blockedSeconds("2.2.2.2")
	assert.True(t, blocked)
	assert.Greater(t, sec, 0)
	assert.LessOrEqual(t, sec, 60)
}

func TestLoginGuard_FreshCycleAfterBlockExpires(t *testing.T) {
	// Короткая блокировка, чтобы проверить истечение без ожидания минуты.
	g := newLoginGuard(2, 5*time.Minute, 20*time.Millisecond)
	g.recordFailure("3.3.3.3")
	_, _, blocked := g.recordFailure("3.3.3.3")
	assert.True(t, blocked)

	time.Sleep(30 * time.Millisecond) // блокировка истекла

	_, ok := g.blockedSeconds("3.3.3.3")
	assert.False(t, ok, "после истечения не заблокирован")

	// Свежий цикл: первая новая неудача даёт остаток max-1, а не ре-лок.
	remaining, _, reblocked := g.recordFailure("3.3.3.3")
	assert.False(t, reblocked, "не лочит мгновенно после истечения")
	assert.Equal(t, 1, remaining, "свежий цикл: 2 - 1 = 1")
}

func TestLoginGuard_ResetOnSuccess(t *testing.T) {
	g := newLoginGuard(3, 5*time.Minute, time.Minute)
	g.recordFailure("4.4.4.4")
	g.recordFailure("4.4.4.4")
	g.reset("4.4.4.4")

	remaining, _, blocked := g.recordFailure("4.4.4.4")
	assert.False(t, blocked)
	assert.Equal(t, 2, remaining, "после reset счётчик с нуля: 3 - 1 = 2")
}

func TestLoginGuard_PerIPIsolation(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	g.recordFailure("5.5.5.5")
	_, _, blocked := g.recordFailure("5.5.5.5") // IP1 заблокирован
	assert.True(t, blocked)

	// Другой IP - свой счётчик.
	_, blk := g.blockedSeconds("6.6.6.6")
	assert.False(t, blk, "другой IP не заблокирован")
	remaining, _, blocked2 := g.recordFailure("6.6.6.6")
	assert.False(t, blocked2)
	assert.Equal(t, 1, remaining)
}

func TestLoginGuard_EmptyIPNotTracked(t *testing.T) {
	g := newLoginGuard(2, 5*time.Minute, time.Minute)
	remaining, _, blocked := g.recordFailure("")
	assert.False(t, blocked)
	assert.Equal(t, 2, remaining, "пустой IP не учитывается")
	_, ok := g.blockedSeconds("")
	assert.False(t, ok)
}

func TestLoginGuard_StaleWindowResetsCounter(t *testing.T) {
	// Короткое окно: неудачи старше окна не копятся.
	g := newLoginGuard(3, 20*time.Millisecond, time.Minute)
	g.recordFailure("7.7.7.7")
	g.recordFailure("7.7.7.7")

	time.Sleep(30 * time.Millisecond) // окно истекло

	// Счётчик сброшен - снова полный остаток.
	remaining, _, blocked := g.recordFailure("7.7.7.7")
	assert.False(t, blocked)
	assert.Equal(t, 2, remaining, "после окна счётчик с нуля: 3 - 1 = 2")
}
