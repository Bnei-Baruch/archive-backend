package llm

import (
	"sync"
	"time"
)

// ChatReasoningSessionStore keeps full replayable conversation history for chat-style
// reasoning providers such as OpenRouter and Ollama. It is not used by OpenAI,
// which continues reasoning sessions via previous_response_id instead of history replay.
// The store lives only in local process memory, so it works on a single machine;
// multi-instance deployments need sticky routing or shared storage.
type ChatReasoningSession struct {
	ID              string
	Model           string
	ReasoningEffort string
	History         []LLMBotMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExpiresAt       time.Time
}

type ChatReasoningSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*ChatReasoningSession
	ttl      time.Duration
	stop     chan struct{}
	done     chan struct{}
}

func NewChatReasoningSessionStore(ttl time.Duration) *ChatReasoningSessionStore {
	if ttl <= 0 {
		ttl = defaultOpenAIReasoningSessionTTL
	}

	store := &ChatReasoningSessionStore{
		sessions: map[string]*ChatReasoningSession{},
		ttl:      ttl,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go store.cleanupLoop()

	return store
}

func (s *ChatReasoningSessionStore) Create(history []LLMBotMessage, model string, reasoningEffort string) (string, error) {
	sessionID, err := newOpenAIReasoningSessionID()
	if err != nil {
		return "", err
	}
	return s.createWithID(sessionID, history, model, reasoningEffort)
}

func (s *ChatReasoningSessionStore) CreateReserved(model string, reasoningEffort string) (string, error) {
	sessionID, err := newOpenAIReasoningSessionID()
	if err != nil {
		return "", err
	}
	return s.createWithID(sessionID, nil, model, reasoningEffort)
}

func (s *ChatReasoningSessionStore) createWithID(sessionID string, history []LLMBotMessage, model string, reasoningEffort string) (string, error) {
	now := time.Now()
	session := &ChatReasoningSession{
		ID:              sessionID,
		Model:           model,
		ReasoningEffort: reasoningEffort,
		History:         append([]LLMBotMessage(nil), history...),
		CreatedAt:       now,
		UpdatedAt:       now,
		ExpiresAt:       now.Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return sessionID, nil
}

func (s *ChatReasoningSessionStore) Get(sessionID string) (*ChatReasoningSession, error) {
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
	sessionCopy.History = append([]LLMBotMessage(nil), session.History...)
	return &sessionCopy, nil
}

func (s *ChatReasoningSessionStore) Update(sessionID string, history []LLMBotMessage) error {
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

	session.History = append([]LLMBotMessage(nil), history...)
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ChatReasoningSessionStore) Close() error {
	close(s.stop)
	<-s.done
	return nil
}

func (s *ChatReasoningSessionStore) cleanupLoop() {
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

func (s *ChatReasoningSessionStore) deleteExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}
