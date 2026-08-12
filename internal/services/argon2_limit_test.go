package services

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArgon2Slot_HoldsConcurrencyLimit - одновременных вычислений не больше предела.
// Это и есть защита от того, что утренний вход смены умножит 19 МБ рабочей памяти
// Argon2id на число одновременных попыток.
func TestArgon2Slot_HoldsConcurrencyLimit(t *testing.T) {
	const limit = 2
	SetArgon2Concurrency(limit)
	defer SetArgon2Concurrency(0)

	var inFlight, maxSeen int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			withArgon2Slot(func() {
				current := atomic.AddInt64(&inFlight, 1)
				for {
					peak := atomic.LoadInt64(&maxSeen)
					if current <= peak || atomic.CompareAndSwapInt64(&maxSeen, peak, current) {
						break
					}
				}
				// Слот держится ощутимое время, иначе вычисления разойдутся во
				// времени сами и тест не увидит превышения даже без семафора.
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt64(&inFlight, -1)
			})
		}()
	}
	wg.Wait()

	assert.LessOrEqual(t, atomic.LoadInt64(&maxSeen), int64(limit),
		"одновременных вычислений Argon2 больше предела")
}

// TestArgon2Slot_ExtraCallersWaitNotRejected - лишний вызов ждёт очереди, а не
// получает отказ. Разница принципиальная: вход, отклонённый из-за нагрузки, человек
// повторит вручную, и очередь станет длиннее, а не короче.
func TestArgon2Slot_ExtraCallersWaitNotRejected(t *testing.T) {
	const limit = 1
	SetArgon2Concurrency(limit)
	defer SetArgon2Concurrency(0)

	occupied := make(chan struct{})
	release := make(chan struct{})
	go func() {
		withArgon2Slot(func() {
			close(occupied)
			<-release
		})
	}()
	<-occupied

	// Второй вызов при занятом единственном слоте: он обязан именно ждать.
	secondDone := make(chan struct{})
	var secondRan atomic.Bool
	go func() {
		defer close(secondDone)
		withArgon2Slot(func() { secondRan.Store(true) })
	}()

	select {
	case <-secondDone:
		t.Fatal("второй вызов прошёл, не дождавшись освобождения слота")
	case <-time.After(50 * time.Millisecond):
		// Ждёт - как и задумано.
	}
	assert.False(t, secondRan.Load(), "тело второго вызова не должно было выполниться")

	close(release)
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("второй вызов не дождался слота - очередь вместо ожидания превратилась в отказ")
	}
	assert.True(t, secondRan.Load(), "после освобождения слота вызов обязан выполниться")
}

// TestSetArgon2Concurrency_NonPositiveFallsBackToCores: неположительное значение -
// это "по числу ядер", а не нулевой семафор. Нулевой канал заблокировал бы вход
// навсегда, то есть параметр, оставленный пустым, запирал бы систему целиком.
func TestSetArgon2Concurrency_NonPositiveFallsBackToCores(t *testing.T) {
	for _, n := range []int{0, -1} {
		SetArgon2Concurrency(n)
		require.Positive(t, argon2Concurrency())
		assert.Equal(t, defaultArgon2Concurrency(), argon2Concurrency())
	}
	SetArgon2Concurrency(0)
}

// TestArgon2Slot_LimitDoesNotBreakHashing - под пределом проверка пароля продолжает
// работать: хеш совпадает сам с собой и не совпадает с чужим паролем.
func TestArgon2Slot_LimitDoesNotBreakHashing(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)

	hash := hashPassword("correct-horse-battery-staple")
	match, err := verifyPassword(hash, "correct-horse-battery-staple")
	require.NoError(t, err)
	assert.True(t, match)
	match, err = verifyPassword(hash, "wrong-password")
	require.NoError(t, err)
	assert.False(t, match)
}
