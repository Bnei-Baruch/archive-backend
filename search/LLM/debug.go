package llm

import (
	"context"

	log "github.com/Sirupsen/logrus"
)

type reasoningDebugContextKey struct{}

func ContextWithDeb(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, reasoningDebugContextKey{}, enabled)
}

func DebFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, ok := ctx.Value(reasoningDebugContextKey{}).(bool)
	return ok && enabled
}

func LogIfDeb(ctx context.Context, format string, args ...interface{}) {
	if DebFromContext(ctx) {
		log.Infof(format, args...)
	}
}
