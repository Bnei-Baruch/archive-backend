package api

import (
	"errors"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

func TestRewriteReasoningSearchQueryAddsApostropheToStandaloneHebrewLetters(t *testing.T) {
	cases := map[string]string{
		"ד בחינות דאור ישר":   "ד' בחינות דאור ישר",
		"א ב ג ד ה ו ז ח ט י": "א' ב' ג' ד' ה' ו' ז' ח' ט' י'",
		"פרק ב":               "פרק ב'",
		"אות י.":              "אות י'.",
	}

	for input, expected := range cases {
		if got := rewriteReasoningSearchQuery(input); got != expected {
			t.Fatalf("rewriteReasoningSearchQuery(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestRewriteReasoningSearchQueryKeepsExistingGereshAndWords(t *testing.T) {
	cases := map[string]string{
		"ד' בחינות דאור ישר": "ד' בחינות דאור ישר",
		"ד׳ בחינות דאור ישר": "ד׳ בחינות דאור ישר",
		"יא בחינות":          "יא בחינות",
		"דאור ישר":           "דאור ישר",
	}

	for input, expected := range cases {
		if got := rewriteReasoningSearchQuery(input); got != expected {
			t.Fatalf("rewriteReasoningSearchQuery(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestValidateReasoningSearchResponseQueryAllowsQuotePunctuationVariants(t *testing.T) {
	response := &llm.ReasoningSearchResponse{Query: "ציטוטים על ט''ו בשבט"}
	err := validateReasoningSearchResponseQuery("ציטוטים על ט'\"ו' בשבט", response)
	if err != nil {
		t.Fatalf("expected quote punctuation variant to pass, got %v", err)
	}
}

func TestValidateReasoningSearchResponseQueryRejectsTopicDrift(t *testing.T) {
	response := &llm.ReasoningSearchResponse{Query: "מחשבת הבריאה"}
	err := validateReasoningSearchResponseQuery("נס", response)
	if !errors.Is(err, errReasoningSearchQueryMismatch) {
		t.Fatalf("expected query mismatch error, got %v", err)
	}
}

func TestPrepareReasoningSearchSessionAllowsFollowupWhenSnapshotExistsDespiteRunningProgress(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected follow-up state error: %v", err)
	}
	if err := workflow.SetResponseSnapshot(sessionID, &llm.ReasoningSearchResponse{
		Query:   "משה",
		Summary: "done",
	}); err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	_, err = prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if err != nil {
		t.Fatalf("expected stale running progress not to block follow-up, got %v", err)
	}
}

func TestPrepareReasoningSearchSessionFinalizesReadyDraftForFastFollowup(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()
	service := llm.NewStubLLMService(nil)

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "stub-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "משה"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := workflow.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", Title: "draft result"},
	}); err != nil {
		t.Fatalf("unexpected partial results error: %v", err)
	}
	_, revision, _, ok, err := workflow.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start: ok=%t err=%v", ok, err)
	}
	draft := &llm.ReasoningSearchResponse{
		Query:   "משה",
		Summary: "draft",
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "uid-1", Title: "draft result"},
		},
	}
	draft.SetSessionID(sessionID)
	if err := workflow.FinishDraft(sessionID, revision, draft); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}
	if err := workflow.RequestFinishNow(sessionID); err != nil {
		t.Fatalf("unexpected finish-now request error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	gotSessionID, err := prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
		Services: map[string]llm.Service{"stub": service},
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if err != nil {
		t.Fatalf("expected ready draft to be finalized for fast follow-up, got %v", err)
	}
	if gotSessionID != sessionID {
		t.Fatalf("unexpected session id: %q", gotSessionID)
	}
	session, err := workflow.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected workflow get error: %v", err)
	}
	if session.FinalizedFromDraft {
		t.Fatalf("expected follow-up preparation to reset finalized-from-draft")
	}
	if session.DraftFollowupSeed == nil || session.DraftFollowupSeed.Summary != "draft" {
		t.Fatalf("expected draft follow-up seed, got %#v", session.DraftFollowupSeed)
	}
}

func TestPrepareReasoningSearchSessionBlocksFollowupWhenDraftReadyButFinishNowWasNotRequested(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "stub-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "משה"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := workflow.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}); err != nil {
		t.Fatalf("unexpected partial results error: %v", err)
	}
	_, revision, _, ok, err := workflow.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start: ok=%t err=%v", ok, err)
	}
	draft := &llm.ReasoningSearchResponse{Query: "משה", Summary: "draft", Results: []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}}
	draft.SetSessionID(sessionID)
	if err := workflow.FinishDraft(sessionID, revision, draft); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	_, err = prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if !errors.Is(err, errReasoningSearchAlreadyRunning) {
		t.Fatalf("expected running error without finish-now request, got %v", err)
	}
}

func TestPrepareReasoningSearchSessionBlocksFollowupAfterFailedFinishNowAttempt(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "stub-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "משה"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if err := workflow.RequestFinishNow(sessionID); err != nil {
		t.Fatalf("unexpected finish-now request error: %v", err)
	}
	if err := workflow.ClearFinishNowRequest(sessionID); err != nil {
		t.Fatalf("unexpected clear finish-now request error: %v", err)
	}
	if _, _, err := workflow.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}); err != nil {
		t.Fatalf("unexpected partial results error: %v", err)
	}
	_, revision, _, ok, err := workflow.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start: ok=%t err=%v", ok, err)
	}
	draft := &llm.ReasoningSearchResponse{Query: "משה", Summary: "draft", Results: []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}}
	draft.SetSessionID(sessionID)
	if err := workflow.FinishDraft(sessionID, revision, draft); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	_, err = prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if !errors.Is(err, errReasoningSearchAlreadyRunning) {
		t.Fatalf("expected running error after failed finish-now attempt, got %v", err)
	}
}
