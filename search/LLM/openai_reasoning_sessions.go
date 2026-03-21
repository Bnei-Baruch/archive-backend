package llm

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const defaultOpenAIReasoningSessionTTL = 30 * time.Minute

var ErrReasoningSessionNotFoundOrExpired = errors.New("reasoning session not found or expired")

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
	sessionID, err := newOpenAIReasoningSessionID()
	if err != nil {
		return "", err
	}

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

func newOpenAIReasoningSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
