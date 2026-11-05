package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/workbudget"
)

func TestResourceAdmissionPublicErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{workbudget.ErrBusy, http.StatusServiceUnavailable},
		{context.Canceled, http.StatusRequestTimeout},
		{context.DeadlineExceeded, http.StatusRequestTimeout},
	} {
		mapped := ResourceAdmissionError(tc.err)
		var public *apperr.Error
		if !errors.As(mapped, &public) || public.Code != tc.status || !errors.Is(mapped, tc.err) {
			t.Fatalf("mapped error: %v", mapped)
		}
		if tc.err == workbudget.ErrBusy && public.Headers["Retry-After"] != "2" {
			t.Fatal("busy response is missing Retry-After")
		}
	}
	ordinary := errors.New("ordinary failure")
	if ResourceAdmissionError(ordinary) != ordinary || ResourceAdmissionError(nil) != nil {
		t.Fatal("unrelated result changed")
	}
}

func TestApplicationCommandSharesHTTPServiceLease(t *testing.T) {
	ctx, release, err := workbudget.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, nested, err := admitApplicationCommand(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nested()
	_, next, err := AcquireResourceBudget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next()
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, free, err := admitApplicationCommand(ctx); free != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled command: %v", err)
	}
}
