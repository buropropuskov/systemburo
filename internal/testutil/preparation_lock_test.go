package testutil

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// errFailedWork - провал работы под замком, свой чтобы не путать с ошибками базы.
var errFailedWork = errors.New("работа под замком провалилась")

// Замок подготовки базы (#1974). Тест лежит рядом с замком, а не в handlers, куда
// правило отправляет тесты на базе: он не трогает ни схему, ни данные - только
// advisory-lock, то есть та самая гонка за общую базу, ради которой правило и
// заведено, ему не грозит.

// TestPreparationLock_SerializesConcurrentWork: под замком одновременно работает
// не больше одного. Ради этого он и стоит: два бинаря, одновременно создающие одну
// таблицу, роняли сборку конфликтом системного индекса каталога.
func TestPreparationLock_SerializesConcurrentWork(t *testing.T) {
	db := initTestDB()

	var inside, maxInside int32
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := withPreparationLock(db, func() error {
				n := atomic.AddInt32(&inside, 1)
				for {
					old := atomic.LoadInt32(&maxInside)
					if n <= old || atomic.CompareAndSwapInt32(&maxInside, old, n) {
						break
					}
				}
				// Пауза даёт остальным реальный шанс войти, если замок не держит.
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&inside, -1)
				return nil
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	require.EqualValues(t, 1, atomic.LoadInt32(&maxInside),
		"под замком подготовки одновременно должен работать ровно один")
}

// TestPreparationLock_ReleasedAfterFailure: замок снимается, даже когда работа под
// ним провалилась. Иначе первая же неудачная миграция подвесила бы все остальные
// бинари сборки, и вместо понятной ошибки получилось бы зависание.
func TestPreparationLock_ReleasedAfterFailure(t *testing.T) {
	db := initTestDB()

	boom := errFailedWork
	require.ErrorIs(t, withPreparationLock(db, func() error { return boom }), boom)

	done := make(chan struct{})
	go func() {
		defer close(done)
		require.NoError(t, withPreparationLock(db, func() error { return nil }))
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("замок не снят после ошибки: следующий захват завис")
	}
}
