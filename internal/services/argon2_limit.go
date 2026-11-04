package services

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"systemburo/internal/admission"
	"systemburo/internal/apperr"
)

const passwordQueueWait = 2 * time.Second

type passwordLimiter struct {
	slots chan struct{}
	gate  *admission.Gate
}

var passwordLimit atomic.Pointer[passwordLimiter]

type passwordSlotKey struct{}

func init() {
	SetArgon2Concurrency(0)
}

// defaultArgon2Concurrency - по числу доступных процессору ядер. Больше смысла не
// имеет: вычисление упирается в процессор при p=1, и лишние одновременные проверки
// делят между собой то же время, добавляя память.
func defaultArgon2Concurrency() int {
	if n := runtime.GOMAXPROCS(0); n > 0 {
		return n
	}
	return 1
}

// SetArgon2Concurrency перенастраивает предел одновременных вычислений Argon2id.
// n<=0 - по числу ядер. Вызывать на старте, до приёма трафика: вычисления, уже
// занявшие слот, освобождают прежний семафор и нового предела не видят.
func SetArgon2Concurrency(n int) {
	if n <= 0 {
		n = defaultArgon2Concurrency()
	}
	slots := make(chan struct{}, n)
	passwordLimit.Store(&passwordLimiter{
		slots: slots,
		gate:  admission.NewWithSlots(slots, 2*n, passwordQueueWait),
	})
}

// argon2Concurrency сообщает действующий предел.
func argon2Concurrency() int {
	return cap(passwordLimit.Load().slots)
}

// withArgon2Slot обслуживает синхронные CLI и fixtures с прежним контрактом.
func withArgon2Slot(fn func()) {
	slots := passwordLimit.Load().slots
	slots <- struct{}{}
	defer func() { <-slots }()
	fn()
}

// admitPasswordOperation вызывается до транзакции. Производный контекст
// используется только внутри синхронной операции, пока её слот занят.
func admitPasswordOperation(ctx context.Context) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if ctx.Value(passwordSlotKey{}) != nil {
		return ctx, func() {}, nil
	}
	release, err := passwordLimit.Load().gate.Acquire(ctx)
	if errors.Is(err, admission.ErrBusy) {
		err = apperr.New(http.StatusServiceUnavailable,
			"Сервис временно занят. Повтори попытку чуть позже.", err).
			WithHeader("Retry-After", "2")
	}
	if err != nil {
		return ctx, nil, err
	}
	return context.WithValue(ctx, passwordSlotKey{}, true), release, nil
}

// Начатый IDKey завершается синхронно и удерживает слот до окончания.
func withArgon2SlotContext(ctx context.Context, fn func()) error {
	ctx, release, err := admitPasswordOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	fn()
	return ctx.Err()
}
