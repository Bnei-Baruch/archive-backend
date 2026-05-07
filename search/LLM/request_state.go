package llm

import (
	"context"
	"sync"
)

type reasoningToolStateContextKey struct{}

type reasoningToolState struct {
	mu                  sync.Mutex
	toolDebugInfo       *ReasoningSearchDebugInfo
	progress            *ReasoningProgressStore
	progressID          string
	resultKeys          map[string]bool
	resultCategories    map[string]bool
	concreteResultCount int
}

type ReasoningProgressResultSignal struct {
	Key      string
	Category string
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

func ReportReasoningProgressSearchResults(ctx context.Context, results []ReasoningProgressResultSignal) {
	if ctx == nil || len(results) == 0 {
		return
	}
	state, ok := ctx.Value(reasoningToolStateContextKey{}).(*reasoningToolState)
	if !ok || state == nil || state.progress == nil || state.progressID == "" {
		return
	}

	state.mu.Lock()
	if state.resultKeys == nil {
		state.resultKeys = map[string]bool{}
	}
	if state.resultCategories == nil {
		state.resultCategories = map[string]bool{}
	}
	for _, result := range results {
		if result.Key == "" || result.Category == "" || state.resultKeys[result.Key] {
			continue
		}
		state.resultKeys[result.Key] = true
		state.resultCategories[result.Category] = true
		state.concreteResultCount++
	}
	hasPotentiallyGoodResults := state.concreteResultCount >= 4 && len(state.resultCategories) >= 2
	state.mu.Unlock()

	state.progress.ReportResultAvailability(state.progressID, hasPotentiallyGoodResults)
}
