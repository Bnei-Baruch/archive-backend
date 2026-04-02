package llm

import (
	"context"
)

type reasoningToolStateContextKey struct{}

type reasoningToolState struct {
	// Tool execution is sequential today, so this state intentionally stays lock-free.
	// If tool execution becomes parallel in the future, add synchronization here.
	sourceLookupChunkRequestCounts map[string]int
}

func ContextWithReasoningToolState(ctx context.Context) context.Context {
	return context.WithValue(ctx, reasoningToolStateContextKey{}, &reasoningToolState{
		sourceLookupChunkRequestCounts: map[string]int{},
	})
}

func IncrementSourceLookupChunkRequestCount(ctx context.Context, sourceKey string) int {
	if ctx == nil {
		return 0
	}
	state, ok := ctx.Value(reasoningToolStateContextKey{}).(*reasoningToolState)
	if !ok || state == nil {
		return 0
	}

	state.sourceLookupChunkRequestCounts[sourceKey]++
	return state.sourceLookupChunkRequestCounts[sourceKey]
}
