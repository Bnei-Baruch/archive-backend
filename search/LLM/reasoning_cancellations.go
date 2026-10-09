package llm

import (
	"context"
	"sync"
)

// ReasoningCancellationStore keeps cancel functions for background reasoning runs.
// It is process-local, like the workflow/progress stores.
type ReasoningCancellationStore struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewReasoningCancellationStore() *ReasoningCancellationStore {
	return &ReasoningCancellationStore{
		cancels: map[string]context.CancelFunc{},
	}
}

func (s *ReasoningCancellationStore) Set(sessionID string, cancel context.CancelFunc) {
	if s == nil || sessionID == "" || cancel == nil {
		return
	}
	s.mu.Lock()
	s.cancels[sessionID] = cancel
	s.mu.Unlock()
}

func (s *ReasoningCancellationStore) Cancel(sessionID string) bool {
	if s == nil || sessionID == "" {
		return false
	}
	s.mu.Lock()
	cancel, ok := s.cancels[sessionID]
	if ok {
		delete(s.cancels, sessionID)
	}
	s.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (s *ReasoningCancellationStore) Delete(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.mu.Lock()
	delete(s.cancels, sessionID)
	s.mu.Unlock()
}
