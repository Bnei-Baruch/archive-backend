package tests

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

func TestOpenAIReasoningSessionStoreCreateGetUpdate(t *testing.T) {
	store := llm.NewOpenAIReasoningSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.Create("resp_1", "gpt-5.4", "high")
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if session.LastResponseID != "resp_1" {
		t.Fatalf("unexpected response id: %s", session.LastResponseID)
	}
	if err := store.Update(sessionID, "resp_2"); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error after update: %v", err)
	}
	if session.LastResponseID != "resp_2" {
		t.Fatalf("unexpected response id after update: %s", session.LastResponseID)
	}
}

func TestOpenAIReasoningSessionStoreReturnsErrorForExpiredSession(t *testing.T) {
	store := llm.NewOpenAIReasoningSessionStore(10 * time.Millisecond)
	defer store.Close()

	sessionID, err := store.Create("resp_1", "gpt-5.4", "high")
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	_, err = store.Get(sessionID)
	if !errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
		t.Fatalf("expected not found or expired error, got: %v", err)
	}
}

func TestOpenAIReasoningSessionStoreCreateReservedThenUpdate(t *testing.T) {
	store := llm.NewOpenAIReasoningSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.CreateReserved("gpt-5.4", "high")
	if err != nil {
		t.Fatalf("unexpected create reserved error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if session.LastResponseID != "" {
		t.Fatalf("expected empty response id for reserved session, got %q", session.LastResponseID)
	}

	if err := store.Update(sessionID, "resp_1"); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error after update: %v", err)
	}
	if session.LastResponseID != "resp_1" {
		t.Fatalf("unexpected response id after update: %s", session.LastResponseID)
	}
}

func TestChatReasoningSessionStoreCreateReservedThenUpdate(t *testing.T) {
	store := llm.NewChatReasoningSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.CreateReserved("gemma4:31b", "high")
	if err != nil {
		t.Fatalf("unexpected create reserved error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if len(session.History) != 0 {
		t.Fatalf("expected empty history for reserved session, got %d items", len(session.History))
	}

	history := []llm.LLMBotMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	if err := store.Update(sessionID, history); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error after update: %v", err)
	}
	if len(session.History) != len(history) {
		t.Fatalf("expected %d history items, got %d", len(history), len(session.History))
	}
}

func TestReasoningProgressStoreLifecycle(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Thinking("session-1", 1, false)
	store.RunningTool("session-1", 1, "elasticsearch_search")
	store.Verifying("session-1", 2)
	store.Complete("session-1", 2)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.State != llm.ReasoningProgressStateCompleted {
		t.Fatalf("unexpected state: %s", status.State)
	}
	if status.Phase != llm.ReasoningProgressPhaseDone {
		t.Fatalf("unexpected phase: %s", status.Phase)
	}
	if !status.Done {
		t.Fatalf("expected done status")
	}
	if status.Iteration != 2 {
		t.Fatalf("unexpected iteration: %d", status.Iteration)
	}
}

func TestReasoningProgressStoreFinalizingPhase(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Finalizing("session-1", 3)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.State != llm.ReasoningProgressStateRunning {
		t.Fatalf("unexpected state: %s", status.State)
	}
	if status.Phase != llm.ReasoningProgressPhaseFinalizing {
		t.Fatalf("unexpected phase: %s", status.Phase)
	}
	if status.Message != "Finalizing results..." {
		t.Fatalf("unexpected message: %s", status.Message)
	}
	if status.Iteration != 3 {
		t.Fatalf("unexpected iteration: %d", status.Iteration)
	}
}

func TestReasoningProgressStoreResultFlags(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.ReportResultAvailability("session-1", false)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !status.HasAnyResults {
		t.Fatalf("expected has_any_results")
	}
	if status.HasPotentiallyGoodResults {
		t.Fatalf("did not expect has_potentially_good_results")
	}

	store.ReportResultAvailability("session-1", true)
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if !status.HasPotentiallyGoodResults {
		t.Fatalf("expected has_potentially_good_results")
	}

	store.Reserve("session-1")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected final get error: %v", err)
	}
	if status.HasAnyResults || status.HasPotentiallyGoodResults {
		t.Fatalf("reserve should reset result flags")
	}
}

func TestReasoningProgressStoreDraftAvailabilityFlag(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.ReportDraftAvailability("session-1")

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !status.HasDraftResults {
		t.Fatalf("expected has_draft_results")
	}

	store.Reserve("session-1")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if status.HasDraftResults {
		t.Fatalf("reserve should reset has_draft_results")
	}
}

func TestReasoningProgressSearchResultsAccumulateAcrossReports(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	ctx := llm.ContextWithReasoningToolState(context.Background(), store, "session-1")

	llm.ReportReasoningProgressSearchResults(ctx, []llm.ReasoningProgressResultSignal{
		{Key: "source-1", Category: "source"},
		{Key: "source-2", Category: "source"},
	})

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !status.HasAnyResults {
		t.Fatalf("expected has_any_results")
	}
	if status.HasPotentiallyGoodResults {
		t.Fatalf("did not expect has_potentially_good_results after one category")
	}

	llm.ReportReasoningProgressSearchResults(ctx, []llm.ReasoningProgressResultSignal{
		{Key: "source-2", Category: "source"},
		{Key: "lesson-1", Category: "lesson"},
		{Key: "lesson-2", Category: "lesson"},
	})

	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if !status.HasPotentiallyGoodResults {
		t.Fatalf("expected accumulated unique results across searches to be potentially good")
	}
}

func TestReasoningProgressSearchResultsEmptySignalsStillMarkAnyResults(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	ctx := llm.ContextWithReasoningToolState(context.Background(), store, "session-1")

	llm.ReportReasoningProgressSearchResults(ctx, nil)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !status.HasAnyResults {
		t.Fatalf("expected has_any_results")
	}
	if status.HasPotentiallyGoodResults {
		t.Fatalf("did not expect has_potentially_good_results")
	}
}

func TestReasoningProgressStoreQueryAnalyzedFlag(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Thinking("session-1", 1, false)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.QueryAnalyzed {
		t.Fatalf("did not expect query_analyzed before first tool")
	}

	store.RunningTool("session-1", 1, "elasticsearch_search")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if !status.QueryAnalyzed {
		t.Fatalf("expected query_analyzed after first tool")
	}

	store.Reserve("session-1")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected final get error: %v", err)
	}
	if status.QueryAnalyzed {
		t.Fatalf("reserve should reset query_analyzed")
	}
}

func TestReasoningProgressStoreMayTakeLongerFlag(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.RunningTool("session-1", 1, "elasticsearch_search")

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.MayTakeLonger {
		t.Fatalf("did not expect may_take_longer before AI tool")
	}

	store.RunningTool("session-1", 2, "query_source_ai")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if !status.MayTakeLonger {
		t.Fatalf("expected may_take_longer after AI tool")
	}

	store.Thinking("session-1", 3, false)
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected third get error: %v", err)
	}
	if !status.MayTakeLonger {
		t.Fatalf("expected may_take_longer to stay true")
	}

	store.Reserve("session-1")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected final get error: %v", err)
	}
	if status.MayTakeLonger {
		t.Fatalf("reserve should reset may_take_longer")
	}
}

func TestReasoningProgressStoreNearFinishFlag(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Thinking("session-1", 1, false)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.NearFinish {
		t.Fatalf("did not expect near_finish before the halfway iteration")
	}

	store.Thinking("session-1", 2, true)
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if !status.NearFinish {
		t.Fatalf("expected near_finish on the halfway iteration")
	}

	store.RunningTool("session-1", 2, "elasticsearch_search")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected third get error: %v", err)
	}
	if !status.NearFinish {
		t.Fatalf("expected near_finish to stay true during final iteration tools")
	}

	store.Reserve("session-1")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected final get error: %v", err)
	}
	if status.NearFinish {
		t.Fatalf("reserve should reset near_finish")
	}
}

func TestReasoningProgressStoreNearFinishThreshold(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	if store.IsNearFinish("session-1", 3, 8) {
		t.Fatalf("did not expect near_finish before 50%% of max iterations")
	}
	if !store.IsNearFinish("session-1", 4, 8) {
		t.Fatalf("expected near_finish at 50%% of max iterations")
	}

	store.RunningTool("session-1", 4, "query_source_ai")
	if store.IsNearFinish("session-1", 5, 8) {
		t.Fatalf("did not expect near_finish before 65%% of max iterations when may_take_longer is true")
	}
	if !store.IsNearFinish("session-1", 6, 8) {
		t.Fatalf("expected near_finish at 65%% of max iterations when may_take_longer is true")
	}
}

func TestReasoningProgressStoreCancel(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Thinking("session-1", 2, false)
	store.Cancel("session-1", 2)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.State != llm.ReasoningProgressStateCanceled {
		t.Fatalf("unexpected state: %s", status.State)
	}
	if status.Phase != llm.ReasoningProgressPhaseCanceled {
		t.Fatalf("unexpected phase: %s", status.Phase)
	}
	if !status.Done {
		t.Fatalf("expected done status")
	}

	store.Reserve("session-2")
	store.SetIterationOffset("session-2", 3)
	store.Thinking("session-2", 1, false)
	store.Cancel("session-2", 0)
	status, err = store.Get("session-2")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if status.Iteration != 4 {
		t.Fatalf("cancel should not move iteration forward or backward, got %d", status.Iteration)
	}
}

func TestReasoningCancellationStoreCancel(t *testing.T) {
	store := llm.NewReasoningCancellationStore()
	ctx, cancel := context.WithCancel(context.Background())

	store.Set("session-1", cancel)
	if !store.Cancel("session-1") {
		t.Fatalf("expected cancel to return true")
	}
	if err := ctx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if store.Cancel("session-1") {
		t.Fatalf("expected second cancel to return false")
	}
}

func TestReasoningProgressStoreIterationOffset(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Thinking("session-1", 2, false)
	store.SetIterationOffset("session-1", 2)
	store.Thinking("session-1", 1, false)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.Iteration != 3 {
		t.Fatalf("unexpected offset thinking iteration: %d", status.Iteration)
	}

	store.RunningTool("session-1", 2, "elasticsearch_search")
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if status.Iteration != 4 {
		t.Fatalf("unexpected offset tool iteration: %d", status.Iteration)
	}

	store.SetIterationOffset("session-1", 0)
	store.Complete("session-1", 4)
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected final get error: %v", err)
	}
	if status.Iteration != 4 {
		t.Fatalf("unexpected complete iteration: %d", status.Iteration)
	}
}

func TestReasoningProgressStoreStageIterationsDoNotRepeat(t *testing.T) {
	store := llm.NewReasoningProgressStore(5 * time.Minute)
	defer store.Close()

	store.Reserve("session-1")
	store.Planning("session-1", 1)
	store.SetIterationOffset("session-1", 1)
	store.Thinking("session-1", 1, false)

	status, err := store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if status.Iteration != 2 {
		t.Fatalf("unexpected first reasoning iteration: %d", status.Iteration)
	}

	store.Verifying("session-1", 3)
	store.SetIterationOffset("session-1", 3)
	store.Thinking("session-1", 1, false)

	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if status.Iteration != 4 {
		t.Fatalf("unexpected rerun reasoning iteration: %d", status.Iteration)
	}

	store.SetIterationOffset("session-1", 0)
	store.Complete("session-1", 3)
	status, err = store.Get("session-1")
	if err != nil {
		t.Fatalf("unexpected final get error: %v", err)
	}
	if status.Iteration != 4 {
		t.Fatalf("complete should not move iteration backward: %d", status.Iteration)
	}
}

func TestReasoningWorkflowSessionStoreFollowupLifecycle(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:        "openai",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		MaxFollowups:    2,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if session.InitialRequestCompleted {
		t.Fatalf("did not expect initial request to be completed")
	}
	if session.FollowupCount != 0 {
		t.Fatalf("unexpected initial follow-up count: %d", session.FollowupCount)
	}

	if err := store.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected state update error after first request: %v", err)
	}

	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error after first request update: %v", err)
	}
	if !session.InitialRequestCompleted {
		t.Fatalf("expected initial request to be completed")
	}
	if session.FollowupCount != 0 {
		t.Fatalf("unexpected follow-up count after first request: %d", session.FollowupCount)
	}

	if err := store.SetFollowupState(sessionID, true, 1); err != nil {
		t.Fatalf("unexpected state update error for first follow-up: %v", err)
	}
	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error after first follow-up: %v", err)
	}
	if session.FollowupCount != 1 {
		t.Fatalf("unexpected follow-up count after first follow-up: %d", session.FollowupCount)
	}

	if err := store.SetFollowupState(sessionID, true, 2); err != nil {
		t.Fatalf("unexpected state update error for second follow-up: %v", err)
	}
	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error after second follow-up: %v", err)
	}
	if session.FollowupCount != 2 {
		t.Fatalf("unexpected follow-up count after second follow-up: %d", session.FollowupCount)
	}
}

func TestReasoningWorkflowSessionStoreDraftLifecycle(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetQuery(sessionID, "אהבה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}

	revision, added, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", ResultType: "sources", Title: "Source", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: "one"}}},
		{MDBUID: "uid-2", ResultType: "units", Title: "Unit", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: "two"}}},
	})
	if err != nil {
		t.Fatalf("unexpected add partial results error: %v", err)
	}
	if revision != 1 || added != 2 {
		t.Fatalf("unexpected revision/added: %d/%d", revision, added)
	}

	results, draftRevision, query, ok, err := store.TryStartDraft(sessionID, time.Minute, 2)
	if err != nil {
		t.Fatalf("unexpected try start draft error: %v", err)
	}
	if !ok || draftRevision != revision || query != "אהבה" || len(results) != 2 {
		t.Fatalf("unexpected draft start: ok=%t revision=%d query=%q results=%d", ok, draftRevision, query, len(results))
	}

	summary := "draft"
	response := &llm.ReasoningSearchResponse{
		Query:   query,
		Summary: &summary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "uid-1", Reason: "reason"},
		},
	}
	response.SetSessionID(sessionID)
	if err := store.FinishDraft(sessionID, draftRevision, response); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}

	finalized, err := store.IsFinalizedFromDraft(sessionID)
	if err != nil {
		t.Fatalf("unexpected finalized check error: %v", err)
	}
	if finalized {
		t.Fatalf("did not expect finalized before finish-now")
	}

	draft, err := store.FinalizeWithDraft(sessionID)
	if err != nil {
		t.Fatalf("unexpected finalize draft error: %v", err)
	}
	if draft.SessionID != sessionID || len(draft.Results) != 1 || draft.Results[0].MDBUID != "uid-1" {
		t.Fatalf("unexpected draft response: %#v", draft)
	}
	finalized, err = store.IsFinalizedFromDraft(sessionID)
	if err != nil {
		t.Fatalf("unexpected finalized check after finalize error: %v", err)
	}
	if !finalized {
		t.Fatalf("expected finalized from draft")
	}

	updatedStage := llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "fresh-provider-session",
	}
	if err := store.StartDraftFollowup(sessionID, "follow-up query", 1, updatedStage); err != nil {
		t.Fatalf("unexpected start draft follow-up error: %v", err)
	}
	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get after draft follow-up start: %v", err)
	}
	if session.FinalizedFromDraft {
		t.Fatalf("expected draft finalized flag to be reset")
	}
	if !session.InitialRequestCompleted || session.FollowupCount != 1 {
		t.Fatalf("unexpected follow-up state: completed=%t count=%d", session.InitialRequestCompleted, session.FollowupCount)
	}
	if session.Query != "follow-up query" {
		t.Fatalf("unexpected follow-up query: %q", session.Query)
	}
	stage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	if stage.ProviderSessionID != "fresh-provider-session" {
		t.Fatalf("unexpected provider session id: %q", stage.ProviderSessionID)
	}
	if session.DraftFollowupSeed == nil || session.DraftFollowupSeed.Summary == nil || *session.DraftFollowupSeed.Summary != "draft" {
		t.Fatalf("expected draft follow-up seed, got %#v", session.DraftFollowupSeed)
	}
}

func TestReasoningWorkflowDraftFollowupKeepsPreviousDraftContext(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      3,
		ProviderSessionID: "provider-session-1",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetQuery(sessionID, "initial query", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{MDBUID: "initial-uid", Title: "Initial"}}); err != nil {
		t.Fatalf("unexpected add partial results error: %v", err)
	}
	_, revision, _, ok, err := store.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected first draft start: ok=%t err=%v", ok, err)
	}
	firstSummary := "first draft"
	firstDraft := &llm.ReasoningSearchResponse{
		Query:   "initial query",
		Summary: &firstSummary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "initial-uid", Title: "Initial"},
		},
	}
	firstDraft.SetSessionID(sessionID)
	if err := store.FinishDraft(sessionID, revision, firstDraft); err != nil {
		t.Fatalf("unexpected first finish draft error: %v", err)
	}
	if _, err := store.FinalizeWithDraft(sessionID); err != nil {
		t.Fatalf("unexpected first finalize draft error: %v", err)
	}
	if err := store.StartDraftFollowup(sessionID, "first follow-up", 1, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      3,
		ProviderSessionID: "provider-session-2",
	}); err != nil {
		t.Fatalf("unexpected first start follow-up error: %v", err)
	}
	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{MDBUID: "followup-uid", Title: "Follow-up"}}); err != nil {
		t.Fatalf("unexpected follow-up partial results error: %v", err)
	}
	_, revision, _, ok, err = store.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected second draft start: ok=%t err=%v", ok, err)
	}
	secondSummary := "second draft"
	secondDraft := &llm.ReasoningSearchResponse{
		Query:   "first follow-up",
		Summary: &secondSummary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "followup-uid", Title: "Follow-up"},
		},
	}
	secondDraft.SetSessionID(sessionID)
	if err := store.FinishDraft(sessionID, revision, secondDraft); err != nil {
		t.Fatalf("unexpected second finish draft error: %v", err)
	}
	if _, err := store.FinalizeWithDraft(sessionID); err != nil {
		t.Fatalf("unexpected second finalize draft error: %v", err)
	}
	if err := store.StartDraftFollowup(sessionID, "second follow-up", 2, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      3,
		ProviderSessionID: "provider-session-3",
	}); err != nil {
		t.Fatalf("unexpected second start follow-up error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if session.DraftFollowupSeed == nil {
		t.Fatalf("expected draft follow-up seed")
	}
	if session.DraftFollowupSeed.Summary == nil || !strings.Contains(*session.DraftFollowupSeed.Summary, "second draft") || !strings.Contains(*session.DraftFollowupSeed.Summary, "first draft") {
		t.Fatalf("expected both draft summaries in seed, got %#v", session.DraftFollowupSeed.Summary)
	}
	if len(session.DraftFollowupSeed.Results) != 2 {
		t.Fatalf("expected latest and previous draft results, got %#v", session.DraftFollowupSeed.Results)
	}
	if session.DraftFollowupSeed.Results[0].MDBUID != "followup-uid" || session.DraftFollowupSeed.Results[1].MDBUID != "initial-uid" {
		t.Fatalf("expected latest result first and previous context second, got %#v", session.DraftFollowupSeed.Results)
	}
}

func TestReasoningWorkflowStartRapidFollowupUsesPreviousSnapshotAsSeed(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4-nano",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session-1",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetRapid(sessionID, true); err != nil {
		t.Fatalf("unexpected rapid state error: %v", err)
	}
	if err := store.SetQuery(sessionID, "initial", "he"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	summary := "initial summary"
	response := &llm.ReasoningSearchResponse{
		Query:   "initial",
		Summary: &summary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "uid-1", Title: "Initial result"},
		},
	}
	response.SetSessionID(sessionID)
	if err := store.SetResponseSnapshot(sessionID, response); err != nil {
		t.Fatalf("unexpected set snapshot error: %v", err)
	}
	if err := store.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected follow-up state error: %v", err)
	}

	if err := store.StartRapidFollowup(sessionID, "follow-up", "he", 1, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4-nano",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session-2",
	}); err != nil {
		t.Fatalf("unexpected rapid follow-up start error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !session.Rapid || session.Query != "follow-up" || session.UILanguage != "he" || session.FollowupCount != 1 {
		t.Fatalf("unexpected rapid follow-up state: rapid=%t query=%q ui_language=%q count=%d", session.Rapid, session.Query, session.UILanguage, session.FollowupCount)
	}
	if len(session.ResponseSnapshotJSON) != 0 {
		t.Fatalf("expected response snapshot to be cleared for rapid follow-up")
	}
	if session.RapidFollowupSeed == nil || session.RapidFollowupSeed.Summary == nil || *session.RapidFollowupSeed.Summary != "initial summary" {
		t.Fatalf("expected previous response seed, got %#v", session.RapidFollowupSeed)
	}
	stage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	if stage.ProviderSessionID != "provider-session-2" {
		t.Fatalf("expected fresh provider session, got %q", stage.ProviderSessionID)
	}
}

func TestReasoningWorkflowDraftRequiresMinimumResults(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:        "openai",
		Model:           "gpt-5.4",
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetQuery(sessionID, "אהבה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", ResultType: "sources", Title: "Source"},
	}); err != nil {
		t.Fatalf("unexpected add partial results error: %v", err)
	}

	_, _, _, ok, err := store.TryStartDraft(sessionID, time.Minute, 2)
	if err != nil {
		t.Fatalf("unexpected try start draft error: %v", err)
	}
	if ok {
		t.Fatalf("did not expect draft to start before minimum result count")
	}

	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-2", ResultType: "units", Title: "Unit"},
	}); err != nil {
		t.Fatalf("unexpected second add partial results error: %v", err)
	}

	results, _, _, ok, err := store.TryStartDraft(sessionID, time.Minute, 2)
	if err != nil {
		t.Fatalf("unexpected try start draft after minimum error: %v", err)
	}
	if !ok || len(results) != 2 {
		t.Fatalf("expected draft to start after minimum result count, ok=%t results=%d", ok, len(results))
	}
}

func TestReasoningWorkflowDraftAttachesEvidenceToPartialResults(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:        "openai",
		Model:           "gpt-5.4",
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetQuery(sessionID, "אהבה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", ResultType: "sources", Title: "Source"},
		{MDBUID: "uid-2", ResultType: "units", Title: "Unit"},
	}); err != nil {
		t.Fatalf("unexpected add partial results error: %v", err)
	}
	revision, added, err := store.AddPartialLookupEvidence(sessionID, "uid-1", []llm.ReasoningSearchResultEvidence{
		{ToolName: "query_source_ai", DocumentType: "source", DocumentID: "uid-1", Query: "meaning", ChunkNumber: 3, Content: "supporting source text"},
	})
	if err != nil {
		t.Fatalf("unexpected add evidence error: %v", err)
	}
	if revision != 2 || added != 1 {
		t.Fatalf("unexpected evidence revision/added: %d/%d", revision, added)
	}

	results, draftRevision, _, ok, err := store.TryStartDraft(sessionID, 0, 2)
	if err != nil {
		t.Fatalf("unexpected try start draft error: %v", err)
	}
	if !ok || draftRevision != revision {
		t.Fatalf("expected draft to start with evidence revision, ok=%t revision=%d", ok, draftRevision)
	}
	if len(results) != 2 || len(results[0].LookupEvidence) != 1 || results[0].LookupEvidence[0].Content != "supporting source text" {
		t.Fatalf("expected evidence on first draft result, got %#v", results)
	}
	if len(results[1].LookupEvidence) != 0 {
		t.Fatalf("did not expect evidence on second draft result: %#v", results[1].LookupEvidence)
	}
}

func TestReasoningWorkflowLookupEvidenceKeepsLatestItems(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:        "openai",
		Model:           "gpt-5.4",
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetQuery(sessionID, "אהבה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", ResultType: "sources", Title: "Source"},
	}); err != nil {
		t.Fatalf("unexpected add partial results error: %v", err)
	}

	evidence := []llm.ReasoningSearchResultEvidence{
		{ToolName: "query_source_ai", DocumentID: "uid-1", Query: "q", ChunkNumber: 1, Content: "old"},
		{ToolName: "query_source_ai", DocumentID: "uid-1", Query: "q", ChunkNumber: 2, Content: "older"},
		{ToolName: "query_source_ai", DocumentID: "uid-1", Query: "q", ChunkNumber: 3, Content: "middle"},
		{ToolName: "query_source_ai", DocumentID: "uid-1", Query: "q", ChunkNumber: 4, Content: "recent"},
		{ToolName: "query_source_ai", DocumentID: "uid-1", Query: "q", ChunkNumber: 5, Content: "newer"},
		{ToolName: "query_source_ai", DocumentID: "uid-1", Query: "q", ChunkNumber: 6, Content: "newest"},
	}
	if _, added, err := store.AddPartialLookupEvidence(sessionID, "uid-1", evidence); err != nil || added != 6 {
		t.Fatalf("unexpected add evidence result: added=%d err=%v", added, err)
	}
	results, _, _, ok, err := store.TryStartDraft(sessionID, 0, 1)
	if err != nil {
		t.Fatalf("unexpected try start draft error: %v", err)
	}
	if !ok || len(results) != 1 {
		t.Fatalf("expected draft to start, ok=%t results=%d", ok, len(results))
	}
	got := results[0].LookupEvidence
	if len(got) != 5 || got[0].Content != "older" || got[1].Content != "middle" || got[2].Content != "recent" || got[3].Content != "newer" || got[4].Content != "newest" {
		t.Fatalf("expected latest evidence items, got %#v", got)
	}
}

func TestReasoningWorkflowDraftPartialResultsAreSessionIsolated(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer store.Close()

	createSession := func(query string) string {
		sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
			Provider:        "openai",
			Model:           "gpt-5.4",
			ReasoningEffort: "low",
		})
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if err := store.SetQuery(sessionID, query, ""); err != nil {
			t.Fatalf("unexpected set query error: %v", err)
		}
		return sessionID
	}

	sessionA := createSession("query-a")
	sessionB := createSession("query-b")

	var wg sync.WaitGroup
	add := func(sessionID string, prefix string) {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{
				MDBUID: prefix + "-" + string(rune('a'+i)),
				Title:  prefix,
			}})
			if err != nil {
				t.Errorf("unexpected add partial results error: %v", err)
			}
		}
	}
	wg.Add(2)
	go add(sessionA, "a")
	go add(sessionB, "b")
	wg.Wait()

	resultsA, _, queryA, ok, err := store.TryStartDraft(sessionA, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start for session A: ok=%t err=%v", ok, err)
	}
	resultsB, _, queryB, ok, err := store.TryStartDraft(sessionB, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start for session B: ok=%t err=%v", ok, err)
	}
	if queryA != "query-a" || queryB != "query-b" {
		t.Fatalf("unexpected draft queries: %q %q", queryA, queryB)
	}
	for _, result := range resultsA {
		if len(result.MDBUID) == 0 || result.MDBUID[0] != 'a' {
			t.Fatalf("session A got result from another session: %#v", result)
		}
	}
	for _, result := range resultsB {
		if len(result.MDBUID) == 0 || result.MDBUID[0] != 'b' {
			t.Fatalf("session B got result from another session: %#v", result)
		}
	}
}

func TestReasoningWorkflowDraftAccumulatesModelRuns(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:        "openai",
		Model:           "gpt-5.4",
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := store.SetQuery(sessionID, "אהבה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", ResultType: "sources", Title: "Source"},
		{MDBUID: "uid-2", ResultType: "units", Title: "Unit"},
	}); err != nil {
		t.Fatalf("unexpected add partial results error: %v", err)
	}

	_, revision, _, ok, err := store.TryStartDraft(sessionID, 0, 2)
	if err != nil || !ok {
		t.Fatalf("unexpected first draft start: ok=%t err=%v", ok, err)
	}
	firstSummary := "first"
	first := &llm.ReasoningSearchResponse{
		Query:   "אהבה",
		Summary: &firstSummary,
		Results: []llm.ReasoningSearchResult{{MDBUID: "uid-1"}},
		Debug: &llm.ReasoningSearchDebugInfo{
			Enabled:           true,
			Model:             "gpt-5.4-nano",
			PricingConfigured: true,
			TotalTokens:       10,
			InputTokens:       7,
			OutputTokens:      3,
			EstimatedCostUSD:  0.1,
			DraftModelRuns: []llm.ReasoningSearchUsageBreakdown{{
				Model:             "gpt-5.4-nano",
				PricingConfigured: true,
				TotalTokens:       10,
				InputTokens:       7,
				OutputTokens:      3,
				EstimatedCostUSD:  0.1,
			}},
		},
	}
	if err := store.FinishDraft(sessionID, revision, first); err != nil {
		t.Fatalf("unexpected first finish draft error: %v", err)
	}

	if _, _, err := store.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-3", ResultType: "units", Title: "Unit 2"},
	}); err != nil {
		t.Fatalf("unexpected second add partial results error: %v", err)
	}
	_, revision, _, ok, err = store.TryStartDraft(sessionID, 0, 2)
	if err != nil || !ok {
		t.Fatalf("unexpected second draft start: ok=%t err=%v", ok, err)
	}
	secondSummary := "second"
	second := &llm.ReasoningSearchResponse{
		Query:   "אהבה",
		Summary: &secondSummary,
		Results: []llm.ReasoningSearchResult{{MDBUID: "uid-2"}},
		Debug: &llm.ReasoningSearchDebugInfo{
			Enabled:           true,
			Model:             "gpt-5.4-nano",
			PricingConfigured: true,
			TotalTokens:       20,
			InputTokens:       15,
			OutputTokens:      5,
			EstimatedCostUSD:  0.2,
			DraftModelRuns: []llm.ReasoningSearchUsageBreakdown{{
				Model:             "gpt-5.4-nano",
				PricingConfigured: true,
				TotalTokens:       20,
				InputTokens:       15,
				OutputTokens:      5,
				EstimatedCostUSD:  0.2,
			}},
		},
	}
	if err := store.FinishDraft(sessionID, revision, second); err != nil {
		t.Fatalf("unexpected second finish draft error: %v", err)
	}

	draft, err := store.FinalizeWithDraft(sessionID)
	if err != nil {
		t.Fatalf("unexpected finalize draft error: %v", err)
	}
	if draft.Debug == nil {
		t.Fatalf("expected draft debug")
	}
	if len(draft.Debug.DraftModelRuns) != 2 {
		t.Fatalf("expected two draft model runs, got %d", len(draft.Debug.DraftModelRuns))
	}
	if draft.UsedTokens != 30 || draft.Debug.TotalTokens != 30 || draft.Debug.DraftModelUsage.TotalTokens != 30 {
		t.Fatalf("unexpected accumulated tokens: used=%d debug=%d draft=%#v", draft.UsedTokens, draft.Debug.TotalTokens, draft.Debug.DraftModelUsage)
	}
	if !draft.Debug.PricingConfigured || draft.Debug.EstimatedCostUSD < 0.299 || draft.Debug.EstimatedCostUSD > 0.301 {
		t.Fatalf("unexpected accumulated pricing: configured=%t cost=%f", draft.Debug.PricingConfigured, draft.Debug.EstimatedCostUSD)
	}
}

func TestReasoningDebugInfoAddPreservesConfiguredPricing(t *testing.T) {
	total := &llm.ReasoningSearchDebugInfo{}
	total.Add(&llm.ReasoningSearchDebugInfo{
		PricingConfigured: true,
		TotalTokens:       10,
		EstimatedCostUSD:  0.1,
	})
	if !total.PricingConfigured {
		t.Fatalf("expected aggregate pricing to stay configured")
	}
	total.Add(&llm.ReasoningSearchDebugInfo{
		PricingConfigured: true,
		TotalTokens:       20,
		EstimatedCostUSD:  0.2,
	})
	if !total.PricingConfigured {
		t.Fatalf("expected aggregate pricing to stay configured after second priced usage")
	}
	total.Add(&llm.ReasoningSearchDebugInfo{
		PricingConfigured: false,
		TotalTokens:       5,
	})
	if total.PricingConfigured {
		t.Fatalf("expected aggregate pricing to become unconfigured when one priced usage is missing")
	}
}
