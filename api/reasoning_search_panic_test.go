package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/stretchr/testify/require"
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
	maybeStartReasoningSearchDraft(context.Background(), &llm.Runtime{
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

type blockingDraftService struct {
	llm.Service
	started chan context.Context
	stopped chan error
}

func (s *blockingDraftService) GetStructuredOutputWithDebugInfo(ctx context.Context, _ string, _ string, _ *int, _ []llm.LLMBotMessage, _ *string, _ *string, _ bool, _ interface{}) (*llm.ReasoningSearchDebugInfo, error) {
	s.started <- ctx
	<-ctx.Done()
	s.stopped <- ctx.Err()
	return nil, ctx.Err()
}

func TestReasoningSearchDraftUsesRunContext(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline", "already canceled"} {
		t.Run(mode, func(t *testing.T) {
			workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
			defer workflow.Close()
			progress := llm.NewReasoningProgressStore(time.Minute)
			defer progress.Close()
			id, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{MaxIterations: 2})
			require.NoError(t, err)
			candidates := make([]llm.ReasoningSearchResult, reasoningSearchDraftMinResults)
			for i := range candidates {
				candidates[i] = llm.ReasoningSearchResult{MDBUID: fmt.Sprintf("uid-%d", i)}
			}
			_, _, err = workflow.AddPartialResults(id, candidates)
			require.NoError(t, err)
			manager, err := llm.NewReasoningToolManager()
			require.NoError(t, err)
			service := &blockingDraftService{started: make(chan context.Context, 1), stopped: make(chan error, 1)}
			runtime := &llm.Runtime{
				Workflow: workflow, Progress: progress, Tools: manager,
				DraftConfig: &llm.ReasoningSearchDraftConfig{Provider: "stub"},
				Services:    map[string]llm.Service{"stub": service},
			}
			var ctx context.Context
			var cancel context.CancelFunc
			if mode == "deadline" {
				ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()
			if mode == "already canceled" {
				cancel()
			}
			maybeStartReasoningSearchDraft(ctx, runtime, nil, "", id, false)
			if mode == "already canceled" {
				select {
				case <-service.started:
					t.Fatal("draft started for an already-canceled run")
				default:
				}
			} else {
				select {
				case received := <-service.started:
					require.Equal(t, ctx, received)
				case <-time.After(time.Second):
					t.Fatal("draft LLM call did not start")
				}
				if mode == "cancel" {
					cancel()
				}
				select {
				case err := <-service.stopped:
					require.True(t, errors.Is(err, ctx.Err()))
				case <-time.After(time.Second):
					t.Fatal("draft LLM call did not observe run cancellation")
				}
			}
			require.Eventually(t, func() bool {
				session, err := workflow.Get(id)
				return err == nil && !session.DraftInProgress && len(session.DraftResponseJSON) == 0
			}, time.Second, time.Millisecond)
		})
	}
}
