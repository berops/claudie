package loggerutils

import (
	"context"

	"github.com/rs/zerolog"
)

type ctxKey struct{}

var key ctxKey

func With(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, key, logger)
}

func Value(ctx context.Context) (zerolog.Logger, bool) {
	v := ctx.Value(key)
	logger, ok := v.(zerolog.Logger)
	return logger, ok
}
