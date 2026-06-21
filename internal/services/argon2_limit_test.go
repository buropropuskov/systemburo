package services

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestArgon2SlotLimitsConcurrency проверяет, что withArgon2Slot не пускает
// больше заданного числа вычислений одновременно - это и есть защита от
// OOM при login-storm, которую используют hashPassword/verifyPassword.
func TestArgon2SlotLimitsConcurrency(t *testing.T) {
	SetArgon2Concurrency(2)
	defer SetArgon2Concurrency(0) // вернуть дефолт по числу ядер

	var inFlight, maxSeen int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			withArgon2Slot(func() {
				cur := atomic.AddInt32(&inFlight, 1)
				for {
					m := atomic.LoadInt32(&maxSeen)
					if cur <= m || atomic.CompareAndSwapInt32(&maxSeen, m, cur) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt32(&inFlight, -1)
			})
		}()
	}
	wg.Wait()

	require.LessOrEqual(t, maxSeen, int32(2), "одновременных Argon2-вычислений не должно быть больше лимита")
}

// TestSetArgon2ConcurrencyDefaults проверяет, что неположительное n возвращает
// семафор по числу ядер, а не нулевой (нулевой канал заблокировал бы логины).
func TestSetArgon2ConcurrencyDefaults(t *testing.T) {
	SetArgon2Concurrency(-1)
	defer SetArgon2Concurrency(0)
	require.Equal(t, defaultArgon2Concurrency(), cap(argon2Sem))
	require.Positive(t, cap(argon2Sem))
}
