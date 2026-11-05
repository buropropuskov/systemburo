package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/workbudget"
)

// ResourcePreparationContext bounds DB/transaction work without assigning a
// deadline to the subsequent HTTP file/ZIP stream. A shorter parent wins.
func ResourcePreparationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 120*time.Second)
}

// ResourceAdmissionError maps only admission/cancellation failures to neutral
// public errors, preserving errors.Is for callers and background workers.
func ResourceAdmissionError(err error) error {
	switch {
	case errors.Is(err, workbudget.ErrBusy):
		return apperr.New(http.StatusServiceUnavailable,
			"Сервис временно занят. Повтори запрос позже.", err).WithHeader("Retry-After", "2")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return apperr.New(http.StatusRequestTimeout, "Операция прервана. Повтори запрос.", err)
	default:
		return err
	}
}

// AcquireResourceBudget must run before archive coordination/DB transactions.
// Pass the returned context to nested services and defer release after success.
func AcquireResourceBudget(ctx context.Context) (context.Context, func(), error) {
	ctx, release, err := workbudget.Acquire(ctx)
	return ctx, release, ResourceAdmissionError(err)
}

func admitApplicationCommand(ctx context.Context) (context.Context, func(), error) {
	return AcquireResourceBudget(ctx)
}
