package llm

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const ReasoningWorkflowStageReasoning = "reasoning"
const ReasoningWorkflowStageVerification = "verification"

// ReasoningWorkflowSessionStore keeps client-facing workflow sessions in local
// process memory. It works only on a single machine; multi-instance deployments
// need sticky routing or a shared backing store.
type ReasoningWorkflowStageSession struct {
	Provider           string
	Model              string
	ReasoningEffort    string
	MaxTokens          int
	MaxIterations      int
	RerunMaxIterations int
	MaxFollowups       int
	// ProviderSessionID may be empty for one-shot stages such as verification.
	// Example: the reasoning stage stores OpenAI previous_response_id or a
	// provider-side session/history id, while the verification stage is a single
	// structured-output call with no continuation state to persist.
	ProviderSessionID string
}

type ReasoningWorkflowSession struct {
	ID                      string
	Stages                  map[string]ReasoningWorkflowStageSession
	InitialRequestCompleted bool
	FollowupCount           int
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ExpiresAt               time.Time
}

type ReasoningWorkflowSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*ReasoningWorkflowSession
	ttl      time.Duration
	stop     chan struct{}
	done     chan struct{}
}

func NewReasoningWorkflowSessionStore(ttl time.Duration) *ReasoningWorkflowSessionStore {
	if ttl <= 0 {
		ttl = defaultOpenAIReasoningSessionTTL
	}

	store := &ReasoningWorkflowSessionStore{
		sessions: map[string]*ReasoningWorkflowSession{},
		ttl:      ttl,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go store.cleanupLoop()

	return store
}

func (s *ReasoningWorkflowSessionStore) Create(stageName string, stage ReasoningWorkflowStageSession) (string, error) {
	sessionID, err := newReasoningSessionID()
	if err != nil {
		return "", err
	}

	now := time.Now()
	session := &ReasoningWorkflowSession{
		ID:        sessionID,
		Stages:    map[string]ReasoningWorkflowStageSession{stageName: stage},
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return sessionID, nil
}

func (s *ReasoningWorkflowSessionStore) Get(sessionID string) (*ReasoningWorkflowSession, error) {
	now := time.Now()

	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil, ErrReasoningSessionNotFoundOrExpired
	}

	copySession := *session
	copySession.Stages = make(map[string]ReasoningWorkflowStageSession, len(session.Stages))
	for key, value := range session.Stages {
		copySession.Stages[key] = value
	}
	return &copySession, nil
}

func (s *ReasoningWorkflowSessionStore) SetStage(sessionID string, stageName string, stage ReasoningWorkflowStageSession) error {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return ErrReasoningSessionNotFoundOrExpired
	}

	if session.Stages == nil {
		session.Stages = map[string]ReasoningWorkflowStageSession{}
	}
	session.Stages[stageName] = stage
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) SetFollowupState(sessionID string, initialRequestCompleted bool, followupCount int) error {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return ErrReasoningSessionNotFoundOrExpired
	}

	session.InitialRequestCompleted = initialRequestCompleted
	session.FollowupCount = followupCount
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) Close() error {
	close(s.stop)
	<-s.done
	return nil
}

func (s *ReasoningWorkflowSessionStore) cleanupLoop() {
	ticker := time.NewTicker(openAIReasoningSessionCleanupInterval(s.ttl))
	defer func() {
		ticker.Stop()
		close(s.done)
	}()

	for {
		select {
		case <-ticker.C:
			s.deleteExpired()
		case <-s.stop:
			return
		}
	}
}

func (s *ReasoningWorkflowSessionStore) deleteExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}

func newReasoningSessionID() (string, error) {
	// Generate a random 16-byte session ID and encode it as a hex string.
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
