package llm

import (
	"context"
	"sync"
)

type reasoningToolStateContextKey struct{}

type reasoningToolState struct {
	mu            sync.Mutex
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
	state.mu.Lock()
	defer state.mu.Unlock()
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
	if !ok || state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.toolDebugInfo == nil {
		return nil
	}
	copy := *state.toolDebugInfo
	return &copy
}
