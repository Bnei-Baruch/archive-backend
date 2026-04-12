package llm

import (
	"context"
)

type reasoningToolStateContextKey struct{}

type reasoningToolState struct {
	// Tool execution is sequential today, so this state intentionally stays lock-free.
	// If tool execution becomes parallel in the future, add synchronization here.
	toolDebugInfo *ReasoningSearchDebugInfo
}

func ContextWithReasoningToolState(ctx context.Context) context.Context {
	return context.WithValue(ctx, reasoningToolStateContextKey{}, &reasoningToolState{})
}

func AddToolDebugInfo(ctx context.Context, info *ReasoningSearchDebugInfo) {
	if ctx == nil || info == nil {
		return
	}
	state, ok := ctx.Value(reasoningToolStateContextKey{}).(*reasoningToolState)
	if !ok || state == nil {
		return
	}
	if state.toolDebugInfo == nil {
		copy := *info
		state.toolDebugInfo = &copy
		return
	}
	state.toolDebugInfo.Add(info)
}

func ToolDebugInfoFromContext(ctx context.Context) *ReasoningSearchDebugInfo {
	if ctx == nil {
		return nil
	}
	state, ok := ctx.Value(reasoningToolStateContextKey{}).(*reasoningToolState)
	if !ok || state == nil || state.toolDebugInfo == nil {
		return nil
	}
	copy := *state.toolDebugInfo
	return &copy
}
