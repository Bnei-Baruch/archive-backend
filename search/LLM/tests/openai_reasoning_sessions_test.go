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
