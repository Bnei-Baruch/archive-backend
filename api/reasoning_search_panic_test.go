package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

type panickingDraftService struct {
	llm.Service
	called chan struct{}
}

func (s *panickingDraftService) GetStructuredOutputWithDebugInfo(context.Context, string, string, *int, []llm.LLMBotMessage, *string, *string, bool, interface{}) (*llm.ReasoningSearchDebugInfo, error) {
	close(s.called)
	panic("broken draft")
}

func TestReasoningSearchDraftPanicClearsInProgress(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Minute)
	defer progress.Close()
	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{MaxIterations: 2})
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]llm.ReasoningSearchResult, reasoningSearchDraftMinResults)
	for i := range candidates {
		candidates[i] = llm.ReasoningSearchResult{MDBUID: fmt.Sprintf("uid-%d", i)}
	}
	if _, _, err := workflow.AddPartialResults(sessionID, candidates); err != nil {
		t.Fatal(err)
	}
	manager, err := llm.NewReasoningToolManager()
	if err != nil {
		t.Fatal(err)
	}
	service := &panickingDraftService{called: make(chan struct{})}
	maybeStartReasoningSearchDraft(&llm.Runtime{
		Workflow:    workflow,
		Progress:    progress,
		Tools:       manager,
		DraftConfig: &llm.ReasoningSearchDraftConfig{Provider: "stub"},
		Services:    map[string]llm.Service{"stub": service},
	}, nil, "", sessionID, false)

	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	select {
	case <-service.called:
	case <-timeout.C:
		t.Fatal("draft service was not called")
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		session, err := workflow.Get(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if !session.DraftInProgress {
			return
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatal("draft remained in progress after panic")
		}
	}
}
