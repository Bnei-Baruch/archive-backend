package tests

import (
	"errors"
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
	store.Thinking("session-1", 1)
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
