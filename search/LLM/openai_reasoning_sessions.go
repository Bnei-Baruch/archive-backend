package llm

import (
	"errors"
	"sync"
	"time"
)

const defaultOpenAIReasoningSessionTTL = 30 * time.Minute

var ErrReasoningSessionNotFoundOrExpired = errors.New("reasoning session not found or expired")

// OpenAIReasoningSessionStore keeps reasoning continuation state in local process
// memory only. It works on a single machine; multi-instance deployments need sticky
// routing or a shared backing store to keep a session available across instances.
type OpenAIReasoningSession struct {
	ID              string
	LastResponseID  string
	Model           string
	ReasoningEffort string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExpiresAt       time.Time
}

type OpenAIReasoningSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*OpenAIReasoningSession
	ttl      time.Duration
	stop     chan struct{}
	done     chan struct{}
}

func NewOpenAIReasoningSessionStore(ttl time.Duration) *OpenAIReasoningSessionStore {
	if ttl <= 0 {
		ttl = defaultOpenAIReasoningSessionTTL
	}

	store := &OpenAIReasoningSessionStore{
		sessions: map[string]*OpenAIReasoningSession{},
		ttl:      ttl,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go store.cleanupLoop()

	return store
}

func (s *OpenAIReasoningSessionStore) Create(lastResponseID string, model string, reasoningEffort string) (string, error) {
	sessionID, err := newReasoningSessionID()
	if err != nil {
		return "", err
	}
	return s.createWithID(sessionID, lastResponseID, model, reasoningEffort)
}

func (s *OpenAIReasoningSessionStore) CreateReserved(model string, reasoningEffort string) (string, error) {
	sessionID, err := newReasoningSessionID()
	if err != nil {
		return "", err
	}
	return s.createWithID(sessionID, "", model, reasoningEffort)
}

func (s *OpenAIReasoningSessionStore) createWithID(sessionID string, lastResponseID string, model string, reasoningEffort string) (string, error) {
	now := time.Now()
	session := &OpenAIReasoningSession{
		ID:              sessionID,
		LastResponseID:  lastResponseID,
		Model:           model,
		ReasoningEffort: reasoningEffort,
		CreatedAt:       now,
		UpdatedAt:       now,
		ExpiresAt:       now.Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return sessionID, nil
}

func (s *OpenAIReasoningSessionStore) Get(sessionID string) (*OpenAIReasoningSession, error) {
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

	sessionCopy := *session
	return &sessionCopy, nil
}

func (s *OpenAIReasoningSessionStore) Update(sessionID string, lastResponseID string) error {
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

	session.LastResponseID = lastResponseID
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *OpenAIReasoningSessionStore) Close() error {
	close(s.stop)
	<-s.done
	return nil
}

func (s *OpenAIReasoningSessionStore) cleanupLoop() {
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

func (s *OpenAIReasoningSessionStore) deleteExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}

func openAIReasoningSessionCleanupInterval(ttl time.Duration) time.Duration {
	if ttl <= time.Minute {
		return ttl
	}
	if ttl <= 10*time.Minute {
		return time.Minute
	}
	return 5 * time.Minute
}
