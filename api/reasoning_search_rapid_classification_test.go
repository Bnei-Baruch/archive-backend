package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/stretchr/testify/require"
	"gopkg.in/gin-gonic/gin.v1"
)

type previewGatherService struct {
	llm.Service
	started chan context.Context
	result  chan error
}

func (s *previewGatherService) GetReasoningStructuredOutputWithToolsForSession(ctx context.Context, _ *string, _ *string, _ string, _ string, _ *int, _ []llm.LLMBotMessage, _ []llm.ToolCall, _ map[string]llm.ToolHandler, _ []llm.ToolCall, _ map[string]llm.ToolHandler, _ *string, _ *string, _ bool, _ int, _ interface{}) (string, error) {
	candidates := make([]llm.ReasoningSearchResult, reasoningSearchRapidClassificationBatchSize+1)
	for i := range candidates {
		candidates[i] = llm.ReasoningSearchResult{MDBUID: fmt.Sprintf("uid-%d", i), Title: "Candidate"}
	}
	llm.ReportReasoningPartialResults(ctx, candidates)
	s.started <- ctx
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-s.result:
		return "", err
	}
}

type controlledRapidClassifier struct {
	llm.Service
	calls   chan []reasoningSearchCandidateInput
	release chan struct{}
}

func (s *controlledRapidClassifier) GetStructuredOutputWithDebugInfo(ctx context.Context, _ string, _ string, _ *int, messages []llm.LLMBotMessage, _ *string, _ *string, _ bool, output interface{}) (*llm.ReasoningSearchDebugInfo, error) {
	var payload struct {
		Results []reasoningSearchCandidateInput `json:"results"`
	}
	if err := json.Unmarshal([]byte(messages[len(messages)-1].Content), &payload); err != nil {
		return nil, err
	}
	s.calls <- payload.Results
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
	}
	response := output.(*rapidClassificationResponse)
	for _, candidate := range payload.Results {
		response.Results = append(response.Results, llm.ReasoningSearchRapidClassification{
			MDBUID: candidate.MDBUID, Relevance: "highly_relevant", Reason: "Preview reason",
		})
	}
	return &llm.ReasoningSearchDebugInfo{TotalTokens: 1}, nil
}

func TestRapidPreviewsDuringGatheringDoNotClassifyOnStatusPolls(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Minute)
	defer progress.Close()
	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{Provider: "gather", Model: "gather-model"})
	require.NoError(t, err)
	require.NoError(t, workflow.SetRapid(sessionID, true))
	progress.Reserve(sessionID)
	manager, err := llm.NewReasoningToolManager()
	require.NoError(t, err)
	gather := &previewGatherService{started: make(chan context.Context, 1)}
	classifier := &controlledRapidClassifier{calls: make(chan []reasoningSearchCandidateInput, 10), release: make(chan struct{}, 1)}
	runtime := &llm.Runtime{
		Workflow: workflow, Progress: progress, Tools: manager,
		RapidConfig: &llm.ReasoningSearchRapidConfig{Classifier: llm.ReasoningSearchRapidStageConfig{Provider: "classifier", Model: "classifier-model"}},
		Services:    map[string]llm.Service{"gather": gather, "classifier": classifier},
	}
	runDone := make(chan error, 1)
	go func() {
		runDone <- executeRapidReasoningSearchForSession(ctx, runtime, &sql.DB{}, ReasoningSearchRequest{Query: "query", UILanguage: "en"}, sessionID)
	}()
	// On assertion failure, cancel and join before closing the session stores.
	defer func() {
		cancel()
		select {
		case err := <-runDone:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected canceled rapid run, got %v", err)
			}
		case <-time.After(time.Second):
			t.Error("rapid run did not stop after cancellation")
		}
	}()
	var gatherCtx context.Context
	select {
	case gatherCtx = <-gather.started:
	case <-ctx.Done():
		t.Fatal("gathering did not start")
	}
	select {
	case batch := <-classifier.calls:
		require.Len(t, batch, reasoningSearchRapidClassificationBatchSize)
	case <-ctx.Done():
		t.Fatal("preview classification did not start during gathering")
	}
	classifier.release <- struct{}{}
	select {
	case batch := <-classifier.calls:
		require.Len(t, batch, 1)
	case <-ctx.Done():
		t.Fatal("second classification batch did not start")
	}
	// The second batch is blocked, but the first batch must already be visible.
	session, err := workflow.Get(sessionID)
	require.NoError(t, err)
	require.Len(t, session.RapidClassifications, reasoningSearchRapidClassificationBatchSize)
	require.Len(t, session.RapidClassifierModelRuns, 1)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("LLM_RUNTIME", runtime)
		c.Set("MDB_DB", (*sql.DB)(nil))
	})
	router.GET("/search/reasoning/status", ReasoningSearchStatusHandler)
	var polls sync.WaitGroup
	for i := 0; i < 8; i++ {
		polls.Add(1)
		go func() {
			defer polls.Done()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/search/reasoning/status?session_id="+sessionID, nil).WithContext(ctx)
			router.ServeHTTP(recorder, request)
			var response reasoningSearchStatusResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Error(err)
				return
			}
			if recorder.Code != http.StatusOK || !response.RapidResultsAvailable || len(response.RapidResults) != reasoningSearchRapidClassificationBatchSize {
				t.Errorf("missing preview while gathering/classification is running: %s", recorder.Body.String())
			}
		}()
	}
	polls.Wait()
	select {
	case <-classifier.calls:
		t.Fatal("status polling started a duplicate classifier call")
	default:
	}
	// Evidence arriving during an active batch is queued for the same worker.
	llm.ReportReasoningResultEvidence(gatherCtx, "uid-0", []llm.ReasoningSearchResultEvidence{{Content: "New evidence"}})
	classifier.release <- struct{}{}
	select {
	case batch := <-classifier.calls:
		require.Len(t, batch, 1)
		require.Equal(t, "uid-0", batch[0].MDBUID)
		require.Len(t, batch[0].LookupEvidence, 1)
	case <-ctx.Done():
		t.Fatal("new lookup evidence was not classified")
	}
	classifier.release <- struct{}{}
	require.Eventually(t, func() bool {
		session, err := workflow.Get(sessionID)
		return err == nil && len(session.RapidClassifications) == reasoningSearchRapidClassificationBatchSize+1 &&
			session.RapidClassifications["uid-0"].EvidenceRevision == 1 && len(session.RapidClassifierModelRuns) == 3
	}, time.Second, time.Millisecond)
}

// A canceled HTTP call may take time to return; cleanup must not assume it
// finishes before a retry can reserve the same workflow session.
type delayedCancelRapidClassifier struct{ *controlledRapidClassifier }

func (s *delayedCancelRapidClassifier) GetStructuredOutputWithDebugInfo(_ context.Context, schema string, model string, tokens *int, messages []llm.LLMBotMessage, key *string, effort *string, deb bool, output interface{}) (*llm.ReasoningSearchDebugInfo, error) {
	return s.controlledRapidClassifier.GetStructuredOutputWithDebugInfo(context.Background(), schema, model, tokens, messages, key, effort, deb, output)
}

func TestRapidOldRunCleanupDoesNotClearRetry(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			workflow := llm.NewReasoningWorkflowSessionStore(time.Minute)
			defer workflow.Close()
			progress := llm.NewReasoningProgressStore(time.Minute)
			defer progress.Close()
			id, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{Provider: "gather", Model: "gather", ProviderSessionID: "provider-session"})
			require.NoError(t, err)
			require.NoError(t, workflow.SetRapid(id, true))
			progress.Reserve(id)
			tools, err := llm.NewReasoningToolManager()
			require.NoError(t, err)
			gather := &previewGatherService{started: make(chan context.Context, 1), result: make(chan error, 1)}
			classifier := &delayedCancelRapidClassifier{&controlledRapidClassifier{calls: make(chan []reasoningSearchCandidateInput, 10), release: make(chan struct{})}}
			runtime := &llm.Runtime{
				Workflow: workflow, Progress: progress, Tools: tools, Cancellations: llm.NewReasoningCancellationStore(),
				RapidConfig: &llm.ReasoningSearchRapidConfig{Classifier: llm.ReasoningSearchRapidStageConfig{Provider: "classifier", Model: "classifier"}},
				Services:    map[string]llm.Service{"gather": gather, "classifier": classifier},
			}
			runtime.Cancellations.Set(id, cancel)
			done := make(chan struct{})
			var runErr error
			go func() {
				defer close(done)
				runErr = executeRapidReasoningSearchForSession(ctx, runtime, &sql.DB{}, ReasoningSearchRequest{Query: "old", IsRapid: true}, id)
			}()
			var release sync.Once
			defer func() {
				cancel()
				release.Do(func() { close(classifier.release) })
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("old rapid run did not exit")
				}
			}()
			select {
			case <-classifier.calls:
			case <-time.After(time.Second):
				t.Fatal("classifier did not start")
			}
			failure := errors.New("gather failed")
			if canceled {
				require.True(t, cancelReasoningSearchSession(runtime, id))
			} else {
				// Provider services can expose failure before the run finishes cleanup.
				progress.Fail(id, 1, failure)
				gather.result <- failure
			}
			_, err = prepareReasoningSearchSession(context.Background(), runtime, &ReasoningSearchRequest{Query: "retry", SessionID: &id, IsRapid: true})
			require.NoError(t, err)
			_, _, err = workflow.AddPartialResults(id, []llm.ReasoningSearchResult{{MDBUID: "new-run"}})
			require.NoError(t, err)
			progress.Thinking(id, 1, false)
			release.Do(func() { close(classifier.release) })
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("old rapid run did not exit")
			}
			if canceled {
				require.ErrorIs(t, runErr, context.Canceled)
			} else {
				require.ErrorIs(t, runErr, failure)
			}
			current, err := workflow.Get(id)
			require.NoError(t, err)
			require.Len(t, current.PartialResults, 1)
			require.Equal(t, "new-run", current.PartialResults[0].MDBUID)
			status, err := progress.Get(id)
			require.NoError(t, err)
			require.Equal(t, llm.ReasoningProgressStateRunning, status.State)
			require.False(t, status.Done)
			require.Empty(t, status.Error)
		})
	}
}
