// Package archivecoord согласует файловую выгрузку и уничтожение в одном процессе.
package archivecoord

import (
	"context"
	"sync"
)

// Единственный активный писатель: размер не зависит от числа заявок. Ожидающие
// HTTP-запросы этим gate не ограничиваются; для них нужен отдельный допуск.
var gate = make(chan struct{}, 1)

// Acquire удерживается до завершения файловой и связанной DB-части операции.
// Вызывающий не должен входить повторно, удерживая gate, или ждать его из TX,
// блокирующей строки, нужные текущему писателю.
func Acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-gate
			return nil, err
		}
		var once sync.Once
		return func() { once.Do(func() { <-gate }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
