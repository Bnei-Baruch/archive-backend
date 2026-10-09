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
	workflow            *ReasoningWorkflowSessionStore
	draftSessionID      string
	draftScheduler      func(string)
	resultKeys          map[string]bool
	resultCategories    map[string]bool
	concreteResultCount int
}

type ReasoningProgressResultSignal struct {
	Key      string
	Category string
}

func ContextWithReasoningToolState(ctx context.Context, progress *ReasoningProgressStore, progressID string) context.Context {
	state := reasoningToolStateFromContext(ctx)
	if state == nil {
		state = &reasoningToolState{}
	}
	state.progress = progress
	state.progressID = progressID
	return context.WithValue(ctx, reasoningToolStateContextKey{}, state)
}

func ContextWithReasoningDraftState(ctx context.Context, workflow *ReasoningWorkflowSessionStore, sessionID string, scheduler func(string)) context.Context {
	state := reasoningToolStateFromContext(ctx)
	if state == nil {
		state = &reasoningToolState{}
	}
	state.workflow = workflow
	state.draftSessionID = sessionID
	state.draftScheduler = scheduler
	return context.WithValue(ctx, reasoningToolStateContextKey{}, state)
}

func reasoningToolStateFromContext(ctx context.Context) *reasoningToolState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(reasoningToolStateContextKey{}).(*reasoningToolState)
	return state
}

func ReportReasoningPartialResults(ctx context.Context, results []ReasoningSearchResult) {
	if ctx == nil || len(results) == 0 {
		return
	}
	state := reasoningToolStateFromContext(ctx)
	if state == nil || state.workflow == nil || state.draftSessionID == "" {
		return
	}

	revision, added, err := state.workflow.AddPartialResults(state.draftSessionID, results)
	if err != nil || added == 0 || revision <= 0 || state.draftScheduler == nil {
		return
	}
	state.draftScheduler(state.draftSessionID)
}

func ReportReasoningResultEvidence(ctx context.Context, documentID string, evidence []ReasoningSearchResultEvidence) {
	if ctx == nil || len(evidence) == 0 {
		return
	}
	state := reasoningToolStateFromContext(ctx)
	if state == nil || state.workflow == nil || state.draftSessionID == "" {
		return
	}

	revision, added, err := state.workflow.AddPartialLookupEvidence(state.draftSessionID, documentID, evidence)
	if err != nil || added == 0 || revision <= 0 || state.draftScheduler == nil {
		return
	}
	state.draftScheduler(state.draftSessionID)
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
	if ctx == nil {
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
