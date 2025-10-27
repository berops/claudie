package processlimit

import (
	"context"

	"golang.org/x/sync/semaphore"
)

type ctxKey struct{}

var key ctxKey

func With(ctx context.Context, semaphore *semaphore.Weighted) context.Context {
	return context.WithValue(ctx, key, semaphore)
}

func Value(ctx context.Context) (*semaphore.Weighted, bool) {
	val := ctx.Value(ctx)
	limit, ok := val.(*semaphore.Weighted)
	return limit, ok
}
