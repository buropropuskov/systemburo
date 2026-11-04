package httpx

import "context"

type requestIDKey struct{}

// WithRequestID carries the validated correlation ID into service/DB contexts.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
