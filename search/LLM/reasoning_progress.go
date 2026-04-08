package llm

import (
	"errors"
	"sync"
	"time"
)

const (
	ReasoningProgressStatePending   = "pending"
	ReasoningProgressStateRunning   = "running"
	ReasoningProgressStateCompleted = "completed"
	ReasoningProgressStateFailed    = "failed"

	ReasoningProgressPhasePending     = "pending"
	ReasoningProgressPhaseThinking    = "thinking"
	ReasoningProgressPhaseRunningTool = "running_tool"
	ReasoningProgressPhaseVerifying   = "verifying"
	ReasoningProgressPhaseDone        = "done"
	ReasoningProgressPhaseError       = "error"
)

var ErrReasoningProgressNotFoundOrExpired = errors.New("reasoning progress not found or expired")

// ReasoningProgressStore keeps transient per-session progress in local process memory.
// It is suitable only for a single backend instance or sticky routing to one machine.
type ReasoningProgressStatus struct {
	SessionID string    `json:"session_id"`
	State     string    `json:"state"`
	Phase     string    `json:"phase"`
	Iteration int       `json:"iteration"`
	ToolName  string    `json:"tool_name,omitempty"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
	Done      bool      `json:"done"`
	Seq       int64     `json:"seq"`
	ExpiresAt time.Time `json:"-"`
}

type ReasoningProgressStore struct {
	mu       sync.RWMutex
	statuses map[string]*ReasoningProgressStatus
	ttl      time.Duration
	nextSeq  int64
	stop     chan struct{}
	done     chan struct{}
}

func NewReasoningProgressStore(ttl time.Duration) *ReasoningProgressStore {
	if ttl <= 0 {
		ttl = defaultOpenAIReasoningSessionTTL
	}

	store := &ReasoningProgressStore{
		statuses: map[string]*ReasoningProgressStatus{},
		ttl:      ttl,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go store.cleanupLoop()

	return store
}

func (s *ReasoningProgressStore) Reserve(sessionID string) {
	s.update(sessionID, func(status *ReasoningProgressStatus) {
		status.State = ReasoningProgressStatePending
		status.Phase = ReasoningProgressPhasePending
		status.Iteration = 0
		status.ToolName = ""
		status.Message = "Waiting to start..."
		status.Done = false
	})
}

func (s *ReasoningProgressStore) Thinking(sessionID string, iteration int) {
	s.update(sessionID, func(status *ReasoningProgressStatus) {
		status.State = ReasoningProgressStateRunning
		status.Phase = ReasoningProgressPhaseThinking
		status.Iteration = iteration
		status.ToolName = ""
		status.Message = "Thinking..."
		status.Done = false
	})
}

func (s *ReasoningProgressStore) RunningTool(sessionID string, iteration int, toolName string) {
	s.update(sessionID, func(status *ReasoningProgressStatus) {
		status.State = ReasoningProgressStateRunning
		status.Phase = ReasoningProgressPhaseRunningTool
		status.Iteration = iteration
		status.ToolName = toolName
		status.Message = reasoningProgressToolMessage(toolName)
		status.Done = false
	})
}

func (s *ReasoningProgressStore) Verifying(sessionID string, iteration int) {
	s.update(sessionID, func(status *ReasoningProgressStatus) {
		status.State = ReasoningProgressStateRunning
		status.Phase = ReasoningProgressPhaseVerifying
		status.Iteration = iteration
		status.ToolName = ""
		status.Message = "Verifing results..."
		status.Done = false
	})
}

func (s *ReasoningProgressStore) Complete(sessionID string, iteration int) {
	s.update(sessionID, func(status *ReasoningProgressStatus) {
		status.State = ReasoningProgressStateCompleted
		status.Phase = ReasoningProgressPhaseDone
		status.Iteration = iteration
		status.ToolName = ""
		status.Message = "Done."
		status.Done = true
	})
}

func (s *ReasoningProgressStore) Fail(sessionID string, iteration int) {
	s.update(sessionID, func(status *ReasoningProgressStatus) {
		status.State = ReasoningProgressStateFailed
		status.Phase = ReasoningProgressPhaseError
		status.Iteration = iteration
		status.ToolName = ""
		status.Message = "Failed."
		status.Done = true
	})
}

func (s *ReasoningProgressStore) Get(sessionID string) (*ReasoningProgressStatus, error) {
	now := time.Now()

	s.mu.RLock()
	status, ok := s.statuses[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrReasoningProgressNotFoundOrExpired
	}
	if now.After(status.ExpiresAt) {
		s.mu.Lock()
		delete(s.statuses, sessionID)
		s.mu.Unlock()
		return nil, ErrReasoningProgressNotFoundOrExpired
	}

	statusCopy := *status
	return &statusCopy, nil
}

func (s *ReasoningProgressStore) Close() error {
	close(s.stop)
	<-s.done
	return nil
}

func (s *ReasoningProgressStore) cleanupLoop() {
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

func (s *ReasoningProgressStore) deleteExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, status := range s.statuses {
		if now.After(status.ExpiresAt) {
			delete(s.statuses, sessionID)
		}
	}
}

func (s *ReasoningProgressStore) update(sessionID string, mutate func(*ReasoningProgressStatus)) {
	if s == nil || sessionID == "" {
		return
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.statuses[sessionID]
	if !ok {
		status = &ReasoningProgressStatus{SessionID: sessionID}
		s.statuses[sessionID] = status
	}
	mutate(status)
	s.nextSeq++
	status.Seq = s.nextSeq
	status.UpdatedAt = now
	status.ExpiresAt = now.Add(s.ttl)
}

func reasoningProgressToolMessage(toolName string) string {
	switch toolName {
	case "elasticsearch_search":
		return "Searching archive..."
	case "source_lookup":
		return "Checking source text..."
	case "transcript_lookup":
		return "Checking transcript..."
	case "get_available_books":
		return "Looking up books..."
	case "get_sources_by_author":
		return "Looking up author sources..."
	case "get_sources_by_source":
		return "Looking up source structure..."
	case "get_collections":
		return "Looking up collections..."
	case "get_content_units_by_collection":
		return "Looking up collection items..."
	default:
		return "Using tool..."
	}
}
