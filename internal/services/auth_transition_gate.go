package services

import "context"

// 64 полосы — небольшой фиксированный компромисс памяти и ложных пересечений.
// Это синхронизация DB transition + RAM effect, а не admission cap: ожидание
// отменяется context, но число ожидающих этим механизмом не ограничивается.
var authTransitionStripes = func() [64]chan struct{} {
	var stripes [64]chan struct{}
	for i := range stripes {
		stripes[i] = make(chan struct{}, 1)
	}
	return stripes
}()

// withAuthTransitionContext не охватывает Argon2 и внешние уведомления.
// Один userID всегда попадает в ту же полосу, в том числе у разных authService.
func withAuthTransitionContext(ctx context.Context, userID int, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stripe := authTransitionStripes[uint64(userID)%uint64(len(authTransitionStripes))]
	select {
	case stripe <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-stripe }()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn()
}
