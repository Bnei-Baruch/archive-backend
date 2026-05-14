package llm

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const ReasoningWorkflowStageReasoning = "reasoning"
const ReasoningWorkflowStagePlanning = "planning"
const ReasoningWorkflowStageVerification = "verification"

var ErrReasoningDraftNotReady = errors.New("reasoning draft is not ready")

const maxDraftEvidencePerResult = 5

// ReasoningWorkflowSessionStore keeps client-facing workflow sessions in local
// process memory. It works only on a single machine; multi-instance deployments
// need sticky routing or a shared backing store.
type ReasoningWorkflowStageSession struct {
	Provider                      string
	Model                         string
	ReasoningEffort               string
	MaxTokens                     int
	MaxInputTokensForVerification int
	MaxIterations                 int
	RerunMaxIterations            int
	MaxFollowups                  int
	// ProviderSessionID may be empty for one-shot stages such as verification.
	// Example: the reasoning stage stores OpenAI previous_response_id or a
	// provider-side session/history id, while the verification stage is a single
	// structured-output call with no continuation state to persist.
	ProviderSessionID string
}

type ReasoningWorkflowSession struct {
	ID                      string
	Query                   string
	Stages                  map[string]ReasoningWorkflowStageSession
	InitialRequestCompleted bool
	FollowupCount           int
	CachedInitialResponse   *ReasoningSearchCacheEntry
	// ResponseSnapshotJSON stores the exact final API response for this workflow
	// session so the fetch endpoint can return it after the background run ends.
	// This is session-scoped state, not the shared query-based ReasoningCache.
	ResponseSnapshotJSON []byte
	// PartialResults stores full ES result data collected during the run. A cheap
	// draft model periodically turns this into a response that finish-now can
	// return immediately.
	PartialResults         []ReasoningSearchResult
	PartialResultsRevision int
	PartialResultEvidence  map[string][]ReasoningSearchResultEvidence
	DraftResponseJSON      []byte
	DraftRevision          int
	DraftInProgress        bool
	LastDraftAt            time.Time
	// Set only after the client explicitly asks to stop early and take the draft.
	FinishNowRequested bool
	FinalizedFromDraft bool
	DraftFollowupSeed  *ReasoningSearchResponse
	DraftModelRuns     []ReasoningSearchUsageBreakdown
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExpiresAt          time.Time
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
	copySession.CachedInitialResponse = cloneReasoningSearchCacheEntry(session.CachedInitialResponse)
	copySession.ResponseSnapshotJSON = append([]byte(nil), session.ResponseSnapshotJSON...)
	copySession.PartialResults = cloneReasoningSearchResults(session.PartialResults)
	if session.PartialResultEvidence != nil {
		copySession.PartialResultEvidence = make(map[string][]ReasoningSearchResultEvidence, len(session.PartialResultEvidence))
		for key, value := range session.PartialResultEvidence {
			copySession.PartialResultEvidence[key] = append([]ReasoningSearchResultEvidence(nil), value...)
		}
	}
	copySession.DraftResponseJSON = append([]byte(nil), session.DraftResponseJSON...)
	copySession.DraftFollowupSeed = cloneReasoningSearchResponse(session.DraftFollowupSeed)
	copySession.DraftModelRuns = append([]ReasoningSearchUsageBreakdown(nil), session.DraftModelRuns...)
	return &copySession, nil
}

func (s *ReasoningWorkflowSessionStore) Refresh(sessionID string) error {
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

	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
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

func (s *ReasoningWorkflowSessionStore) SetQuery(sessionID string, query string) error {
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

	session.Query = query
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

func (s *ReasoningWorkflowSessionStore) SetCachedInitialResponse(sessionID string, entry *ReasoningSearchCacheEntry) error {
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

	session.CachedInitialResponse = cloneReasoningSearchCacheEntry(entry)
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) SetResponseSnapshot(sessionID string, response *ReasoningSearchResponse) error {
	var responseJSON []byte
	if response != nil {
		raw, err := marshalJSON(response)
		if err != nil {
			return err
		}
		responseJSON = []byte(raw)
	}

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

	session.ResponseSnapshotJSON = append([]byte(nil), responseJSON...)
	session.DraftFollowupSeed = nil
	if response == nil {
		session.PartialResults = nil
		session.PartialResultsRevision = 0
		session.PartialResultEvidence = nil
		session.DraftResponseJSON = nil
		session.DraftRevision = 0
		session.DraftInProgress = false
		session.LastDraftAt = time.Time{}
		session.FinishNowRequested = false
		session.FinalizedFromDraft = false
		session.DraftModelRuns = nil
	}
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) AddPartialResults(sessionID string, results []ReasoningSearchResult) (int, int, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return 0, 0, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return 0, 0, ErrReasoningSessionNotFoundOrExpired
	}
	if session.FinalizedFromDraft {
		return session.PartialResultsRevision, 0, nil
	}

	seen := make(map[string]bool, len(session.PartialResults))
	for _, result := range session.PartialResults {
		if result.MDBUID != "" {
			seen[result.MDBUID] = true
		}
	}

	added := 0
	for _, result := range results {
		result = cloneReasoningSearchResult(result)
		if result.MDBUID == "" || seen[result.MDBUID] {
			continue
		}
		seen[result.MDBUID] = true
		session.PartialResults = append(session.PartialResults, result)
		added++
	}
	if added > 0 {
		session.PartialResultsRevision++
		session.UpdatedAt = now
		session.ExpiresAt = now.Add(s.ttl)
	}

	return session.PartialResultsRevision, added, nil
}

// AddPartialResultEvidence stores AI-reader excerpts by result UID. Drafts still
// use only ES results; evidence is attached only when a matching ES result exists.
func (s *ReasoningWorkflowSessionStore) AddPartialResultEvidence(sessionID string, documentID string, evidence []ReasoningSearchResultEvidence) (int, int, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" || len(evidence) == 0 {
		return 0, 0, nil
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return 0, 0, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return 0, 0, ErrReasoningSessionNotFoundOrExpired
	}
	if session.FinalizedFromDraft {
		return session.PartialResultsRevision, 0, nil
	}
	if session.PartialResultEvidence == nil {
		session.PartialResultEvidence = map[string][]ReasoningSearchResultEvidence{}
	}

	existing := session.PartialResultEvidence[documentID]
	added := 0
	for _, item := range evidence {
		item = normalizeReasoningSearchResultEvidence(item, documentID)
		if item.Content == "" || reasoningSearchEvidenceExists(existing, item) {
			continue
		}
		if len(existing) >= maxDraftEvidencePerResult {
			copy(existing, existing[1:])
			existing = existing[:len(existing)-1]
		}
		existing = append(existing, item)
		added++
	}
	if added == 0 {
		return session.PartialResultsRevision, 0, nil
	}
	session.PartialResultEvidence[documentID] = existing
	for _, result := range session.PartialResults {
		if result.MDBUID == documentID {
			session.PartialResultsRevision++
			break
		}
	}
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return session.PartialResultsRevision, added, nil
}

func (s *ReasoningWorkflowSessionStore) TryStartDraft(sessionID string, minInterval time.Duration, minResults int) ([]ReasoningSearchResult, int, string, bool, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, 0, "", false, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return nil, 0, "", false, ErrReasoningSessionNotFoundOrExpired
	}
	if session.FinalizedFromDraft || session.DraftInProgress || len(session.PartialResults) < minResults || session.DraftRevision == session.PartialResultsRevision {
		return nil, session.PartialResultsRevision, session.Query, false, nil
	}
	if minInterval > 0 && !session.LastDraftAt.IsZero() && now.Sub(session.LastDraftAt) < minInterval {
		return nil, session.PartialResultsRevision, session.Query, false, nil
	}

	session.DraftInProgress = true
	session.LastDraftAt = now
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)

	results := cloneReasoningSearchResults(session.PartialResults)
	for i := range results {
		uid := strings.TrimSpace(results[i].MDBUID)
		if uid != "" && len(session.PartialResultEvidence[uid]) > 0 {
			results[i].DraftEvidence = append([]ReasoningSearchResultEvidence(nil), session.PartialResultEvidence[uid]...)
		}
	}
	return results, session.PartialResultsRevision, session.Query, true, nil
}

func (s *ReasoningWorkflowSessionStore) FinishDraft(sessionID string, revision int, response *ReasoningSearchResponse) error {
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
	if session.FinalizedFromDraft {
		session.DraftInProgress = false
		return nil
	}

	if response != nil && response.Debug != nil && len(response.Debug.DraftModelRuns) != 0 {
		session.DraftModelRuns = append(session.DraftModelRuns, response.Debug.DraftModelRuns...)
		response.Debug.DraftModelRuns = append([]ReasoningSearchUsageBreakdown(nil), session.DraftModelRuns...)
		if usage := aggregateReasoningSearchUsageBreakdowns(session.DraftModelRuns); usage != nil {
			response.Debug.DraftModelUsage = usage
			response.UsedTokens = usage.TotalTokens
			response.Debug.TotalTokens = usage.TotalTokens
			response.Debug.InputTokens = usage.InputTokens
			response.Debug.CachedInputTokens = usage.CachedInputTokens
			response.Debug.UncachedInputTokens = usage.UncachedInputTokens
			response.Debug.OutputTokens = usage.OutputTokens
			response.Debug.ReasoningTokens = usage.ReasoningTokens
			response.Debug.PricingConfigured = usage.PricingConfigured
			response.Debug.InputPer1MTokensUSD = usage.InputPer1MTokensUSD
			response.Debug.CachedInputPer1MTokensUSD = usage.CachedInputPer1MTokensUSD
			response.Debug.OutputPer1MTokensUSD = usage.OutputPer1MTokensUSD
			response.Debug.EstimatedInputCostUSD = usage.EstimatedInputCostUSD
			response.Debug.EstimatedCachedInputCostUSD = usage.EstimatedCachedInputCostUSD
			response.Debug.EstimatedOutputCostUSD = usage.EstimatedOutputCostUSD
			response.Debug.EstimatedCostUSD = usage.EstimatedCostUSD
		}
	}

	var responseJSON []byte
	if response != nil {
		raw, err := marshalJSON(response)
		if err != nil {
			return err
		}
		responseJSON = []byte(raw)
	}

	session.DraftResponseJSON = append([]byte(nil), responseJSON...)
	session.DraftRevision = revision
	session.DraftInProgress = false
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) DraftModelUsage(sessionID string) ([]ReasoningSearchUsageBreakdown, *ReasoningSearchUsageBreakdown, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return nil, nil, ErrReasoningSessionNotFoundOrExpired
	}

	runs := append([]ReasoningSearchUsageBreakdown(nil), session.DraftModelRuns...)
	return runs, aggregateReasoningSearchUsageBreakdowns(runs), nil
}

func (s *ReasoningWorkflowSessionStore) FailDraft(sessionID string) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok || now.After(session.ExpiresAt) {
		return
	}
	session.DraftInProgress = false
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
}

func (s *ReasoningWorkflowSessionStore) RequestFinishNow(sessionID string) error {
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
	session.FinishNowRequested = true
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) ClearFinishNowRequest(sessionID string) error {
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
	session.FinishNowRequested = false
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) FinalizeWithDraft(sessionID string) (*ReasoningSearchResponse, error) {
	now := time.Now()

	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return nil, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil, ErrReasoningSessionNotFoundOrExpired
	}
	if len(session.DraftResponseJSON) == 0 {
		s.mu.Unlock()
		return nil, ErrReasoningDraftNotReady
	}
	raw := append([]byte(nil), session.DraftResponseJSON...)
	session.ResponseSnapshotJSON = append([]byte(nil), raw...)
	// Keep this true so a near-simultaneous follow-up can distinguish a real
	// finish-now flow from an ordinary running search with background drafts.
	session.FinishNowRequested = true
	session.FinalizedFromDraft = true
	session.InitialRequestCompleted = true
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	s.mu.Unlock()

	response := ReasoningSearchResponse{}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *ReasoningWorkflowSessionStore) StartDraftFollowup(sessionID string, query string, followupCount int, reasoningStage ReasoningWorkflowStageSession) error {
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
	if !session.FinalizedFromDraft || len(session.ResponseSnapshotJSON) == 0 {
		return ErrReasoningDraftNotReady
	}

	seed := ReasoningSearchResponse{}
	if err := json.Unmarshal(session.ResponseSnapshotJSON, &seed); err != nil {
		return err
	}
	if session.DraftFollowupSeed != nil {
		previous := session.DraftFollowupSeed
		if strings.TrimSpace(previous.Summary) != "" {
			seed.Summary = strings.TrimSpace(seed.Summary + "\nPrevious draft context: " + previous.Summary)
		}
		// If a follow-up draft is followed by another question, keep the earlier
		// visible draft in the seed too. Latest results stay first because they
		// are the response the user most recently saw.
		seen := map[string]bool{}
		for _, result := range seed.Results {
			if uid := strings.TrimSpace(result.MDBUID); uid != "" {
				seen[uid] = true
			}
		}
		for _, result := range previous.Results {
			uid := strings.TrimSpace(result.MDBUID)
			if uid != "" && seen[uid] {
				continue
			}
			seed.Results = append(seed.Results, cloneReasoningSearchResult(result))
			if uid != "" {
				seen[uid] = true
			}
		}
	}

	if session.Stages == nil {
		session.Stages = map[string]ReasoningWorkflowStageSession{}
	}
	session.Stages[ReasoningWorkflowStageReasoning] = reasoningStage
	session.Query = query
	session.InitialRequestCompleted = true
	session.FollowupCount = followupCount
	session.CachedInitialResponse = nil
	session.ResponseSnapshotJSON = nil
	session.PartialResults = nil
	session.PartialResultsRevision = 0
	session.PartialResultEvidence = nil
	session.DraftResponseJSON = nil
	session.DraftRevision = 0
	session.DraftInProgress = false
	session.LastDraftAt = time.Time{}
	session.FinishNowRequested = false
	session.FinalizedFromDraft = false
	session.DraftFollowupSeed = &seed
	session.DraftModelRuns = nil
	session.UpdatedAt = now
	session.ExpiresAt = now.Add(s.ttl)
	return nil
}

func (s *ReasoningWorkflowSessionStore) IsFinalizedFromDraft(sessionID string) (bool, error) {
	now := time.Now()

	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return false, ErrReasoningSessionNotFoundOrExpired
	}
	if now.After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return false, ErrReasoningSessionNotFoundOrExpired
	}
	return session.FinalizedFromDraft, nil
}

func (s *ReasoningWorkflowSessionStore) Close() error {
	close(s.stop)
	<-s.done
	return nil
}

func cloneReasoningSearchResults(results []ReasoningSearchResult) []ReasoningSearchResult {
	if results == nil {
		return nil
	}
	cloned := make([]ReasoningSearchResult, len(results))
	for i, result := range results {
		cloned[i] = cloneReasoningSearchResult(result)
	}
	return cloned
}

func cloneReasoningSearchResult(result ReasoningSearchResult) ReasoningSearchResult {
	result.Highlights = append([]string(nil), result.Highlights...)
	result.DraftEvidence = append([]ReasoningSearchResultEvidence(nil), result.DraftEvidence...)
	return result
}

func normalizeReasoningSearchResultEvidence(item ReasoningSearchResultEvidence, documentID string) ReasoningSearchResultEvidence {
	item.ToolName = strings.TrimSpace(item.ToolName)
	item.DocumentType = strings.TrimSpace(item.DocumentType)
	item.DocumentID = strings.TrimSpace(item.DocumentID)
	if item.DocumentID == "" {
		item.DocumentID = documentID
	}
	item.Query = strings.TrimSpace(item.Query)
	item.Content = strings.TrimSpace(item.Content)
	item.Reason = strings.TrimSpace(item.Reason)
	item.SupportingSnippet = strings.TrimSpace(item.SupportingSnippet)
	return item
}

func reasoningSearchEvidenceExists(existing []ReasoningSearchResultEvidence, candidate ReasoningSearchResultEvidence) bool {
	for _, item := range existing {
		if item.ToolName == candidate.ToolName && item.DocumentID == candidate.DocumentID && item.Query == candidate.Query && item.ChunkNumber == candidate.ChunkNumber && item.Content == candidate.Content {
			return true
		}
	}
	return false
}

func cloneReasoningSearchResponse(response *ReasoningSearchResponse) *ReasoningSearchResponse {
	if response == nil {
		return nil
	}
	raw, err := marshalJSON(response)
	if err != nil {
		return nil
	}
	cloned := ReasoningSearchResponse{}
	if err := json.Unmarshal([]byte(raw), &cloned); err != nil {
		return nil
	}
	return &cloned
}

func aggregateReasoningSearchUsageBreakdowns(usages []ReasoningSearchUsageBreakdown) *ReasoningSearchUsageBreakdown {
	if len(usages) == 0 {
		return nil
	}
	total := usages[0]
	for _, usage := range usages[1:] {
		total.TotalTokens += usage.TotalTokens
		total.InputTokens += usage.InputTokens
		total.CachedInputTokens += usage.CachedInputTokens
		total.UncachedInputTokens += usage.UncachedInputTokens
		total.OutputTokens += usage.OutputTokens
		total.ReasoningTokens += usage.ReasoningTokens
		total.EstimatedInputCostUSD += usage.EstimatedInputCostUSD
		total.EstimatedCachedInputCostUSD += usage.EstimatedCachedInputCostUSD
		total.EstimatedOutputCostUSD += usage.EstimatedOutputCostUSD
		total.EstimatedCostUSD += usage.EstimatedCostUSD
		total.PricingConfigured = total.PricingConfigured && usage.PricingConfigured
	}
	return &total
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
