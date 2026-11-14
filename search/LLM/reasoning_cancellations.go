package llm

import (
	"context"
	"sync"
)

// ReasoningCancellationStore keeps cancel functions for background reasoning runs.
// It is process-local, like the workflow/progress stores.
type ReasoningCancellationStore struct {
	mu        sync.Mutex
	cancels   map[string]reasoningCancellation
	nextToken uint64
}

type reasoningCancellation struct {
	cancel context.CancelFunc
	token  uint64
}

func NewReasoningCancellationStore() *ReasoningCancellationStore {
	return &ReasoningCancellationStore{
		cancels: map[string]reasoningCancellation{},
	}
}

func (s *ReasoningCancellationStore) Set(sessionID string, cancel context.CancelFunc) uint64 {
	if s == nil || sessionID == "" || cancel == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextToken++
	s.cancels[sessionID] = reasoningCancellation{cancel: cancel, token: s.nextToken}
	return s.nextToken
}

func (s *ReasoningCancellationStore) Cancel(sessionID string) bool {
	if s == nil || sessionID == "" {
		return false
	}
	s.mu.Lock()
	entry, ok := s.cancels[sessionID]
	if ok {
		delete(s.cancels, sessionID)
	}
	s.mu.Unlock()
	if ok {
		entry.cancel()
	}
	return ok
}

// Delete removes only the caller's registration; follow-ups reuse the session ID.
func (s *ReasoningCancellationStore) Delete(sessionID string, token uint64) {
	if s == nil || sessionID == "" {
		return
	}
	s.mu.Lock()
	if entry, ok := s.cancels[sessionID]; ok && entry.token == token {
		delete(s.cancels, sessionID)
	}
	s.mu.Unlock()
}
