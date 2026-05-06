package llm

import (
	"context"
	"sync"
)

type reasoningToolStateContextKey struct{}

type reasoningToolState struct {
	mu            sync.Mutex
	toolDebugInfo *ReasoningSearchDebugInfo
	progress      *ReasoningProgressStore
	progressID    string
}

func ContextWithReasoningToolState(ctx context.Context, progress *ReasoningProgressStore, progressID string) context.Context {
	return context.WithValue(ctx, reasoningToolStateContextKey{}, &reasoningToolState{
		progress:   progress,
		progressID: progressID,
	})
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

func ReportReasoningProgressResults(ctx context.Context, hasPotentiallyGoodResults bool) {
	if ctx == nil {
		return
	}
	state, ok := ctx.Value(reasoningToolStateContextKey{}).(*reasoningToolState)
	if !ok || state == nil || state.progress == nil || state.progressID == "" {
		return
	}
	state.progress.ReportResultAvailability(state.progressID, hasPotentiallyGoodResults)
}
