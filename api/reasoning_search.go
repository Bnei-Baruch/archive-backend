package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/text/language/display"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ReasoningSearchRequest struct {
	SessionID       *string `json:"session_id" form:"session_id"`
	CancelSessionID *string `json:"cancel_session_id" form:"cancel_session_id"`
	Query           string  `json:"q" form:"q"`
	Deb             bool    `json:"deb" form:"deb" binding:"omitempty"`
	UILanguage      string  `json:"ui_language" form:"ui_language" binding:"omitempty,len=2"`
	IsRapid         bool    `json:"is_rapid" form:"is_rapid" binding:"omitempty"`
}

type ReasoningSearchCancelRequest struct {
	SessionID string `json:"session_id" form:"session_id"`
}

const (
	reasoningSearchDisplayHighlightMaxItems = 3
	reasoningSearchDisplayHighlightMaxRunes = 200
)

type rapidGatherResponse struct {
	llm.ReasoningSearchResponse `json:"-"`
	Done                        bool     `json:"done"`
	SelectedUIDs                []string `json:"selected_uids"`
}

var (
	errReasoningSearchAlreadyRunning  = errors.New("reasoning search is already running for this session")
	errReasoningSearchResultsNotReady = errors.New("reasoning search results are not ready yet")
	errReasoningSearchFailed          = errors.New("reasoning search failed")
	errReasoningSearchCanceled        = errors.New("reasoning search was canceled")
	errReasoningSearchQueryMismatch   = errors.New("reasoning search query mismatch")
)

const reasoningSearchDraftMinInterval = 15 * time.Second
const reasoningSearchDraftMinResults = 8
const reasoningSearchRapidSelectedUIDLimit = 12

const reasoningSearchDraftInstruction = `Draft mode: prepare a partial archive search response from Elasticsearch results that were already collected while the main reasoning search is still running.
Use only the supplied Elasticsearch results. Do not invent results, IDs, titles, dates, or content types.
Use search_highlights as Elasticsearch evidence for the result itself.
Some results may include lookup_evidence from AI source/transcript lookup tools. Use this evidence only to judge and explain relevance of the supplied Elasticsearch result; do not treat evidence as a separate result.
Keep mdb_uid values exactly as provided.
The summary must say that the user asked for fast results and the answer is based on results gathered so far.`

var reasoningSearchRapidGatherInstruction = fmt.Sprintf(`Rapid search gather mode: use archive tools to collect strong Elasticsearch candidates for the user query.
Do not create final search results for the user. Return only the JSON object required by the schema.
Return selected_uids with the best candidate mdb_uid values collected from Elasticsearch. Select up to %d IDs, ordered best first.
Prefer Elasticsearch searches that collect varied candidate types. Use AI source/transcript tools only when they can add useful evidence to candidates already found.`, reasoningSearchRapidSelectedUIDLimit)

const reasoningSearchRapidFinalizerInstruction = `Rapid search finalizer mode: create the final user-facing archive search response from the supplied gathered Elasticsearch candidates.
Use only supplied candidates. Do not invent results, IDs, titles, dates, or content types.
Use search_highlights as Elasticsearch evidence for the candidate itself.
Some candidates may include lookup_evidence from AI source/transcript lookup tools. Use this evidence only to judge and explain relevance of the supplied candidate; do not treat evidence as a separate result.
If previous_response is provided, use it only to understand the follow-up query and conversation context.
Keep mdb_uid values exactly as provided.`

// Async flow entrypoint: start work in background and return a workflow
// session_id immediately. The client then polls status and later fetches the
// stored response snapshot for that session.
func ReasoningSearchStartHandler(c *gin.Context) {
	r := ReasoningSearchRequest{}
	if c.Bind(&r) != nil {
		return
	}
	if err := normalizeReasoningSearchRequest(&r); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Tools == nil || runtime.Progress == nil || runtime.Workflow == nil || runtime.Cancellations == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)

	if r.CancelSessionID != nil {
		cancelReasoningSearchSession(runtime, *r.CancelSessionID)
	}

	sessionID, err := prepareReasoningSearchSession(c.Request.Context(), runtime, &r)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		var maxFollowupsErr *llm.MaxReasoningFollowupsError
		if errors.As(err, &maxFollowupsErr) {
			NewHttpError(http.StatusUnprocessableEntity, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		if errors.Is(err, errReasoningSearchAlreadyRunning) {
			NewHttpError(http.StatusConflict, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}

	requestCopy := r
	// Async work is intentionally detached from the request context; that context
	// is canceled when this handler returns after sending 202 Accepted.
	bgCtx, cancel := context.WithCancel(context.Background())
	runtime.Cancellations.Set(sessionID, cancel)
	go executeReasoningSearchInBackground(bgCtx, runtime, db, requestCopy, sessionID)

	c.JSON(http.StatusAccepted, gin.H{
		"session_id":   sessionID,
		"result_ready": false,
	})
}

func ReasoningSearchCancelHandler(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		r := ReasoningSearchCancelRequest{}
		if c.Bind(&r) != nil {
			return
		}
		sessionID = strings.TrimSpace(r.SessionID)
	}
	if sessionID == "" {
		NewBadRequestError(errors.New("session_id is required")).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Progress == nil || runtime.Cancellations == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}

	canceled := cancelReasoningSearchSession(runtime, sessionID)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"canceled":   canceled,
	})
}

func ReasoningSearchFinishNowHandler(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		r := ReasoningSearchCancelRequest{}
		if c.Bind(&r) != nil {
			return
		}
		sessionID = strings.TrimSpace(r.SessionID)
	}
	if sessionID == "" {
		NewBadRequestError(errors.New("session_id is required")).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	if err := refreshReasoningSearchSession(runtime, sessionID); err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	if err := runtime.Workflow.RequestFinishNow(sessionID); err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}

	response, err := runtime.Workflow.FinalizeWithDraft(sessionID)
	if err != nil {
		// Roll back the intent flag when no draft could actually be returned.
		if clearErr := runtime.Workflow.ClearFinishNowRequest(sessionID); clearErr != nil && !errors.Is(clearErr, llm.ErrReasoningSessionNotFoundOrExpired) {
			log.Warnf("Reasoning Search failed clearing finish-now request session=%s: %v", sessionID, clearErr)
		}
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		if errors.Is(err, llm.ErrReasoningDraftNotReady) {
			NewHttpError(http.StatusConflict, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	log.Infof("Reasoning Search finish-now returned stored draft session=%s results=%d", sessionID, len(response.Results))
	if runtime.Cancellations != nil {
		canceled := runtime.Cancellations.Cancel(sessionID)
		log.Infof("Reasoning Search finish-now canceled active background run session=%s canceled=%t", sessionID, canceled)
	}
	runtime.Progress.Complete(sessionID, 0)

	c.JSON(http.StatusOK, gin.H{
		"session_id":   sessionID,
		"draft_used":   true,
		"result_ready": true,
	})
}

func ReasoningSearchCacheHandler(c *gin.Context) {
	r := ReasoningSearchRequest{}
	if c.Bind(&r) != nil {
		return
	}
	if err := normalizeReasoningSearchRequest(&r); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}
	r.SessionID = nil
	r.CancelSessionID = nil

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.ReasoningCache == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}

	cacheKey, cacheEligible := llm.ReasoningSearchCacheKeyForQuery(r.Query)
	if !cacheEligible {
		c.JSON(http.StatusOK, gin.H{"cache_hit": false})
		return
	}
	cachedEntry, ok := runtime.ReasoningCache.Get(cacheKey)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"cache_hit": false})
		return
	}

	sessionID, err := prepareReasoningSearchSession(c.Request.Context(), runtime, &r)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)
	workflowSession, err := runtime.Workflow.Get(sessionID)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	reasoningStage := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
	if err := storeReasoningSearchCachedResponse(runtime, db, r, sessionID, cachedEntry, reasoningStage.MaxFollowups); err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cache_hit":  true,
		"session_id": sessionID,
	})
}

func ReasoningSearchStatusHandler(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		NewBadRequestError(errors.New("session_id is required")).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("LLM_REASONING_PROGRESS is not initialized")).Abort(c)
		return
	}

	if err := refreshReasoningSearchSession(runtime, sessionID); err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}

	status, err := runtime.Progress.Get(sessionID)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningProgressNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusOK, status)
}

// Synchronous flow entrypoint: run the full reasoning search now and return the
// final response directly. This is mainly useful for simple clients and manual
// testing, while the async flow uses /search/reasoning/start + status + result.
func ReasoningSearchHandler(c *gin.Context) {
	r := ReasoningSearchRequest{}
	if c.Bind(&r) != nil {
		return
	}
	if err := normalizeReasoningSearchRequest(&r); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)
	sessionID, err := prepareReasoningSearchSession(c.Request.Context(), runtime, &r)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		var maxFollowupsErr *llm.MaxReasoningFollowupsError
		if errors.As(err, &maxFollowupsErr) {
			NewHttpError(http.StatusUnprocessableEntity, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		if errors.Is(err, errReasoningSearchAlreadyRunning) {
			NewHttpError(http.StatusConflict, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	if err := executeReasoningSearchForSessionByMode(c.Request.Context(), runtime, db, r, sessionID, false); err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		if isReasoningSearchCancellation(err) {
			NewHttpError(499, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		var maxIterationsErr *llm.MaxReasoningIterationsError
		if errors.As(err, &maxIterationsErr) {
			NewHttpError(http.StatusUnprocessableEntity, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}

	workflowSession, err := runtime.Workflow.Get(sessionID)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	if len(workflowSession.ResponseSnapshotJSON) != 0 {
		c.Data(http.StatusOK, "application/json; charset=utf-8", workflowSession.ResponseSnapshotJSON)
		return
	}

	status, err := runtime.Progress.Get(sessionID)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningProgressNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	switch status.State {
	case llm.ReasoningProgressStateCanceled:
		NewHttpError(http.StatusUnprocessableEntity, errReasoningSearchCanceled, gin.ErrorTypePublic).Abort(c)
		return
	case llm.ReasoningProgressStateFailed:
		NewHttpError(http.StatusUnprocessableEntity, errReasoningSearchFailed, gin.ErrorTypePublic).Abort(c)
		return
	case llm.ReasoningProgressStateCompleted:
		NewInternalError(errors.New("reasoning search completed without stored response snapshot")).Abort(c)
		return
	default:
		NewHttpError(http.StatusConflict, errReasoningSearchResultsNotReady, gin.ErrorTypePublic).Abort(c)
		return
	}
}

// Async flow result endpoint: fetch the stored response snapshot for a
// background reasoning run by workflow session_id.
func ReasoningSearchResultHandler(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		NewBadRequestError(errors.New("session_id is required")).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}

	if err := refreshReasoningSearchSession(runtime, sessionID); err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}

	workflowSession, err := runtime.Workflow.Get(sessionID)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	if len(workflowSession.ResponseSnapshotJSON) != 0 {
		c.Data(http.StatusOK, "application/json; charset=utf-8", workflowSession.ResponseSnapshotJSON)
		return
	}

	status, err := runtime.Progress.Get(sessionID)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningProgressNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
			return
		}
		NewInternalError(err).Abort(c)
		return
	}
	switch status.State {
	case llm.ReasoningProgressStateCanceled:
		NewHttpError(http.StatusUnprocessableEntity, errReasoningSearchCanceled, gin.ErrorTypePublic).Abort(c)
		return
	case llm.ReasoningProgressStateFailed:
		NewHttpError(http.StatusUnprocessableEntity, errReasoningSearchFailed, gin.ErrorTypePublic).Abort(c)
		return
	case llm.ReasoningProgressStateCompleted:
		NewInternalError(errors.New("reasoning search completed without stored response snapshot")).Abort(c)
		return
	default:
		NewHttpError(http.StatusConflict, errReasoningSearchResultsNotReady, gin.ErrorTypePublic).Abort(c)
		return
	}
}

func normalizeReasoningSearchRequest(r *ReasoningSearchRequest) error {
	r.Query = strings.TrimSpace(r.Query)
	if r.Query == "" {
		return errors.New("q is required")
	}
	r.Query = rewriteReasoningSearchQuery(r.Query)
	r.UILanguage = strings.ToLower(strings.TrimSpace(r.UILanguage))
	if r.SessionID != nil {
		trimmedSessionID := strings.TrimSpace(*r.SessionID)
		if trimmedSessionID == "" {
			return errors.New("session_id cannot be empty")
		}
		r.SessionID = &trimmedSessionID
	}
	if r.CancelSessionID != nil {
		trimmedSessionID := strings.TrimSpace(*r.CancelSessionID)
		if trimmedSessionID == "" {
			r.CancelSessionID = nil
		} else {
			r.CancelSessionID = &trimmedSessionID
		}
	}
	return nil
}

// Both flows share the same preparation rules so follow-up limits, progress
// reset, and previous response snapshot cleanup stay consistent.
func prepareReasoningSearchSession(ctx context.Context, runtime *llm.Runtime, r *ReasoningSearchRequest) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if runtime == nil || runtime.Progress == nil || runtime.Workflow == nil {
		return "", errors.New("reasoning workflow is not initialized")
	}

	workflowStore := runtime.Workflow
	progressStore := runtime.Progress

	if r.SessionID != nil {
		sessionID := *r.SessionID
		workflowSession, err := workflowStore.Get(sessionID)
		if err != nil {
			return "", err
		}
		reasoningStage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
		if !ok || strings.TrimSpace(reasoningStage.ProviderSessionID) == "" {
			return "", llm.ErrReasoningSessionNotFoundOrExpired
		}
		if !workflowSession.FinalizedFromDraft && !workflowSession.Rapid {
			refreshProviderReasoningSession(runtime, reasoningStage)
		}
		if status, err := progressStore.Get(sessionID); err == nil {
			if !status.Done && len(workflowSession.ResponseSnapshotJSON) == 0 {
				if len(workflowSession.DraftResponseJSON) == 0 {
					return "", errReasoningSearchAlreadyRunning
				}
				// A ready draft alone is not enough: only allow fast follow-up after
				// an explicit finish-now request from the client.
				if !workflowSession.FinishNowRequested {
					return "", errReasoningSearchAlreadyRunning
				}
				// The user already asked to stop early; promote the ready draft to the
				// official response so the follow-up can continue from visible results.
				if _, err := workflowStore.FinalizeWithDraft(sessionID); err != nil {
					return "", err
				}
				progressStore.Complete(sessionID, status.Iteration)
				workflowSession, err = workflowStore.Get(sessionID)
				if err != nil {
					return "", err
				}
				reasoningStage = workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
			}
		} else if !errors.Is(err, llm.ErrReasoningProgressNotFoundOrExpired) {
			return "", err
		}
		isFollowup := workflowSession.InitialRequestCompleted || workflowSession.FinalizedFromDraft
		if isFollowup {
			if workflowSession.FollowupCount >= reasoningStage.MaxFollowups {
				return "", &llm.MaxReasoningFollowupsError{MaxFollowups: reasoningStage.MaxFollowups}
			}
			nextFollowupCount := workflowSession.FollowupCount + 1
			if workflowSession.Rapid {
				rapidConfig := runtime.RapidConfig
				if rapidConfig == nil {
					return "", errors.New("rapid reasoning search is not initialized")
				}
				service := runtime.Services[rapidConfig.Gather.Provider]
				if service == nil {
					return "", errors.New("rapid gather llm service is not initialized")
				}
				providerSessionID, err := service.ReserveReasoningSession(ctx, rapidConfig.Gather.Model, &rapidConfig.Gather.Effort)
				if err != nil {
					return "", err
				}
				gatherStage := llm.ReasoningWorkflowStageSession{
					Provider:          rapidConfig.Gather.Provider,
					Model:             rapidConfig.Gather.Model,
					ReasoningEffort:   rapidConfig.Gather.Effort,
					MaxTokens:         rapidConfig.Gather.MaxTokens,
					MaxIterations:     rapidConfig.Gather.MaxIterations,
					MaxFollowups:      reasoningStage.MaxFollowups,
					ProviderSessionID: providerSessionID,
				}
				if err := workflowStore.StartRapidFollowup(sessionID, r.Query, nextFollowupCount, gatherStage); err != nil {
					return "", err
				}
				progressStore.Reserve(sessionID)
				progressStore.SetIterationOffset(sessionID, 0)
				return sessionID, nil
			}
			if workflowSession.FinalizedFromDraft {
				service := runtime.Services[reasoningStage.Provider]
				if service == nil {
					return "", errors.New("reasoning llm service is not initialized")
				}
				providerSessionID, err := service.ReserveReasoningSession(ctx, reasoningStage.Model, &reasoningStage.ReasoningEffort)
				if err != nil {
					return "", err
				}
				reasoningStage.ProviderSessionID = providerSessionID
				if err := workflowStore.StartDraftFollowup(sessionID, r.Query, nextFollowupCount, reasoningStage); err != nil {
					return "", err
				}
				progressStore.Reserve(sessionID)
				progressStore.SetIterationOffset(sessionID, 0)
				return sessionID, nil
			}
			if err := workflowStore.SetFollowupState(sessionID, true, nextFollowupCount); err != nil {
				return "", err
			}
		}
		if err := workflowStore.SetResponseSnapshot(sessionID, nil); err != nil {
			return "", err
		}
		if err := workflowStore.SetQuery(sessionID, r.Query); err != nil {
			return "", err
		}
		progressStore.Reserve(sessionID)
		progressStore.SetIterationOffset(sessionID, 0)
		return sessionID, nil
	}

	reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		return "", err
	}
	if r.IsRapid {
		if runtime.RapidConfig == nil {
			return "", errors.New("rapid reasoning search is not initialized")
		}
		reasoningConfig.Provider = runtime.RapidConfig.Gather.Provider
		reasoningConfig.Model = runtime.RapidConfig.Gather.Model
		reasoningConfig.Effort = runtime.RapidConfig.Gather.Effort
		reasoningConfig.MaxTokens = runtime.RapidConfig.Gather.MaxTokens
		reasoningConfig.MaxIterations = runtime.RapidConfig.Gather.MaxIterations
		reasoningConfig.RerunMaxIterations = 0
		reasoningConfig.Planning = nil
		reasoningConfig.Verification = nil
	}
	service := runtime.Services[reasoningConfig.Provider]
	if service == nil {
		return "", errors.New("reasoning llm service is not initialized")
	}

	providerSessionID, err := service.ReserveReasoningSession(ctx, reasoningConfig.Model, &reasoningConfig.Effort)
	if err != nil {
		return "", err
	}

	sessionID, err := workflowStore.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:           reasoningConfig.Provider,
		Model:              reasoningConfig.Model,
		ReasoningEffort:    reasoningConfig.Effort,
		MaxTokens:          reasoningConfig.MaxTokens,
		MaxIterations:      reasoningConfig.MaxIterations,
		RerunMaxIterations: reasoningConfig.RerunMaxIterations,
		MaxFollowups:       reasoningConfig.MaxFollowups,
		ProviderSessionID:  providerSessionID,
	})
	if err != nil {
		return "", err
	}
	if err := workflowStore.SetQuery(sessionID, r.Query); err != nil {
		return "", err
	}
	if r.IsRapid {
		if err := workflowStore.SetRapid(sessionID, true); err != nil {
			return "", err
		}
		if runtime.RapidConfig.FinalizerEnabled {
			if err := workflowStore.SetStage(sessionID, llm.ReasoningWorkflowStageRapidFinalizer, llm.ReasoningWorkflowStageSession{
				Provider:        runtime.RapidConfig.Finalizer.Provider,
				Model:           runtime.RapidConfig.Finalizer.Model,
				ReasoningEffort: runtime.RapidConfig.Finalizer.Effort,
				MaxTokens:       runtime.RapidConfig.Finalizer.MaxTokens,
			}); err != nil {
				return "", err
			}
		}
	}
	if reasoningConfig.Verification != nil {
		if err := workflowStore.SetStage(sessionID, llm.ReasoningWorkflowStageVerification, llm.ReasoningWorkflowStageSession{
			Provider:                      reasoningConfig.Verification.Provider,
			Model:                         reasoningConfig.Verification.Model,
			ReasoningEffort:               reasoningConfig.Verification.Effort,
			MaxTokens:                     reasoningConfig.Verification.MaxTokens,
			MaxInputTokensForVerification: reasoningConfig.Verification.MaxInputTokens,
		}); err != nil {
			return "", err
		}
	}
	if reasoningConfig.Planning != nil {
		if err := workflowStore.SetStage(sessionID, llm.ReasoningWorkflowStagePlanning, llm.ReasoningWorkflowStageSession{
			Provider:        reasoningConfig.Planning.Provider,
			Model:           reasoningConfig.Planning.Model,
			ReasoningEffort: reasoningConfig.Planning.Effort,
			MaxTokens:       reasoningConfig.Planning.MaxTokens,
		}); err != nil {
			return "", err
		}
	}
	if err := workflowStore.SetResponseSnapshot(sessionID, nil); err != nil {
		return "", err
	}
	progressStore.Reserve(sessionID)
	progressStore.SetIterationOffset(sessionID, 0)
	return sessionID, nil
}

func storeReasoningSearchCachedResponse(runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, sessionID string, cachedEntry *llm.ReasoningSearchCacheEntry, maxFollowups int) error {
	if runtime == nil || runtime.Workflow == nil || runtime.Progress == nil {
		return errors.New("reasoning workflow is not initialized")
	}
	if cachedEntry == nil {
		return errors.New("reasoning search cache entry is nil")
	}

	cachedEntry.Query = r.Query
	if err := runtime.Workflow.SetCachedInitialResponse(sessionID, cachedEntry); err != nil {
		return err
	}
	if err := runtime.Workflow.SetFollowupState(sessionID, true, 0); err != nil {
		return err
	}

	response := llm.ReasoningSearchResponse{
		Query:    r.Query,
		Summary:  cachedEntry.Summary,
		CacheHit: true,
		Results:  append([]llm.ReasoningSearchResult(nil), cachedEntry.Results...),
	}
	response.SetUsedTools([]string{})
	response.SetSessionID(sessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		return err
	}
	response.SetFollowupBudget(maxFollowups, 0, maxFollowups)
	if err := runtime.Workflow.SetResponseSnapshot(sessionID, &response); err != nil {
		return err
	}
	runtime.Progress.Complete(sessionID, 0)
	return nil
}

func maybeStartReasoningSearchDraft(runtime *llm.Runtime, db *sql.DB, uiLanguage string, sessionID string, deb bool) {
	if runtime == nil || runtime.Workflow == nil || runtime.Progress == nil || runtime.DraftConfig == nil || runtime.Services == nil {
		return
	}
	service := runtime.Services[runtime.DraftConfig.Provider]
	if service == nil {
		return
	}

	results, revision, query, ok, err := runtime.Workflow.TryStartDraft(sessionID, reasoningSearchDraftMinInterval, reasoningSearchDraftMinResults)
	if err != nil {
		if !errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			log.Warnf("Reasoning Search draft start failed session=%s: %v", sessionID, err)
		}
		return
	}
	if !ok {
		return
	}

	go func() {
		if err := buildAndStoreReasoningSearchDraft(context.Background(), runtime, db, service, runtime.DraftConfig, uiLanguage, sessionID, query, revision, results, deb); err != nil {
			runtime.Workflow.FailDraft(sessionID)
			if !errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
				log.Warnf("Reasoning Search draft failed session=%s revision=%d: %v", sessionID, revision, err)
			}
		}
	}()
}

func buildAndStoreReasoningSearchDraft(ctx context.Context, runtime *llm.Runtime, db *sql.DB, service llm.Service, config *llm.ReasoningSearchDraftConfig, uiLanguage string, sessionID string, query string, revision int, candidates []llm.ReasoningSearchResult, deb bool) error {
	if len(candidates) == 0 {
		runtime.Workflow.FailDraft(sessionID)
		return nil
	}
	if len(candidates) > 30 {
		candidates = candidates[:30]
	}
	evidenceResultCount := 0
	evidenceItemCount := 0
	for _, candidate := range candidates {
		if len(candidate.LookupEvidence) == 0 {
			continue
		}
		evidenceResultCount++
		evidenceItemCount += len(candidate.LookupEvidence)
	}
	if evidenceItemCount > 0 {
		log.Infof("Reasoning Search draft evidence attached session=%s results=%d evidence_items=%d", sessionID, evidenceResultCount, evidenceItemCount)
	}

	workflowSession, err := runtime.Workflow.Get(sessionID)
	if err != nil {
		return err
	}
	reasoningStage := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
	followupsUsed := workflowSession.FollowupCount
	followupsRemaining := reasoningStage.MaxFollowups - followupsUsed
	if followupsRemaining < 0 {
		followupsRemaining = 0
	}
	outputLanguageName := reasoningSearchOutputLanguageName(uiLanguage, query)
	schema, err := llm.GenerateReasoningSearchResponseJSONSchemaForLanguage(outputLanguageName)
	if err != nil {
		return err
	}
	systemMessage := llm.GenerateSystemMessageForReasoningSearch(runtime.Tools.Tools(), reasoningStage.MaxIterations, followupsRemaining)
	systemMessage = llm.AppendReasoningSearchOutputLanguage(systemMessage, outputLanguageName)
	systemMessage = systemMessage + "\n\n" + reasoningSearchDraftInstruction

	input, err := json.Marshal(struct {
		Query   string                          `json:"query"`
		Results []reasoningSearchCandidateInput `json:"results"`
	}{
		Query:   query,
		Results: reasoningSearchCandidateInputs(candidates),
	})
	if err != nil {
		return err
	}

	messages := []llm.LLMBotMessage{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}
	if workflowSession.DraftFollowupSeed != nil {
		messages = append(messages,
			llm.LLMBotMessage{
				Role:    "user",
				Content: workflowSession.DraftFollowupSeed.Query,
			},
			llm.LLMBotMessage{
				Role:    "assistant",
				Content: buildReasoningSearchDraftFollowupSeedAssistantContent(workflowSession.DraftFollowupSeed),
			},
		)
	}
	messages = append(messages, llm.LLMBotMessage{
		Role:    "user",
		Content: string(input),
	})

	response := llm.ReasoningSearchResponse{}
	promptCacheKey := fmt.Sprintf("reasoning-search-draft:m=%s:e=%s", config.Model, config.Effort)
	debug, err := service.GetStructuredOutputWithDebugInfo(ctx, schema, config.Model, &config.MaxTokens, messages, &promptCacheKey, &config.Effort, deb, &response)
	if err != nil {
		return err
	}

	response.Query = query
	response.SetSessionID(sessionID)
	response.SetUsedTools([]string{"elasticsearch_search"})
	response.SetReasoningSteps([]llm.ReasoningSearchReasoningStep{})
	if debug != nil {
		response.UsedTokens = debug.TotalTokens
		if usage := debug.UsageBreakdown(); usage != nil {
			debug.DraftModelUsage = usage
			debug.DraftModelRuns = []llm.ReasoningSearchUsageBreakdown{*usage}
		}
		if deb {
			response.Debug = debug
		}
	}
	mergeReasoningSearchDraftMetadata(&response, candidates)
	if len(response.Results) == 0 {
		runtime.Workflow.FailDraft(sessionID)
		return nil
	}
	if err := enrichReasoningSearchResults(db, uiLanguage, response.Results); err != nil {
		return err
	}
	response.SetFollowupBudget(reasoningStage.MaxFollowups, followupsUsed, followupsRemaining)

	if err := runtime.Workflow.FinishDraft(sessionID, revision, &response); err != nil {
		return err
	}
	runtime.Progress.ReportDraftAvailability(sessionID)
	log.Infof("Reasoning Search draft ready session=%s revision=%d results=%d", sessionID, revision, len(response.Results))
	return nil
}

func mergeReasoningSearchDraftMetadata(response *llm.ReasoningSearchResponse, candidates []llm.ReasoningSearchResult) {
	if response == nil || len(response.Results) == 0 {
		return
	}

	byUID := map[string]llm.ReasoningSearchResult{}
	for _, candidate := range candidates {
		uid := strings.TrimSpace(candidate.MDBUID)
		if uid == "" {
			continue
		}
		byUID[uid] = candidate
	}

	merged := make([]llm.ReasoningSearchResult, 0, len(response.Results))
	seen := map[string]bool{}
	for _, selected := range response.Results {
		uid := strings.TrimSpace(selected.MDBUID)
		if uid == "" || seen[uid] {
			continue
		}
		candidate, ok := byUID[uid]
		if !ok {
			continue
		}
		candidate.Reason = strings.TrimSpace(selected.Reason)
		candidate.IsGroupingResult = candidate.IsGroupingResult || selected.IsGroupingResult
		candidate.Origin = llm.ReasoningSearchResultOriginOriginal
		merged = append(merged, candidate)
		seen[uid] = true
	}
	response.Results = merged
	populateReasoningSearchHighlightsFromEvidence(response, candidates, nil)
}

func populateReasoningSearchHighlightsFromEvidence(response *llm.ReasoningSearchResponse, candidates []llm.ReasoningSearchResult, evidenceByUID map[string][]llm.ReasoningSearchResultEvidence) {
	if response == nil || len(response.Results) == 0 {
		return
	}
	byUID := map[string]llm.ReasoningSearchResult{}
	for _, candidate := range candidates {
		uid := strings.TrimSpace(candidate.MDBUID)
		if uid != "" {
			byUID[uid] = candidate
		}
	}
	for i := range response.Results {
		uid := strings.TrimSpace(response.Results[i].MDBUID)
		response.Results[i].Highlights = nil
		if candidate, ok := byUID[uid]; ok {
			if len(candidate.LookupEvidence) == 0 && len(evidenceByUID[uid]) > 0 {
				candidate.LookupEvidence = append([]llm.ReasoningSearchResultEvidence(nil), evidenceByUID[uid]...)
			}
			response.Results[i].Highlights = compactReasoningSearchHighlightsForDisplay(buildReasoningSearchHighlightsFromEvidence(candidate))
		} else if len(evidenceByUID[uid]) > 0 {
			response.Results[i].Highlights = compactReasoningSearchHighlightsForDisplay(buildReasoningSearchHighlightsFromEvidence(llm.ReasoningSearchResult{
				LookupEvidence: evidenceByUID[uid],
			}))
		}
		response.Results[i].LookupEvidence = nil
	}
}

func buildReasoningSearchHighlightsFromEvidence(result llm.ReasoningSearchResult) []string {
	highlights := []string{}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		highlights = append(highlights, value)
	}
	for _, highlight := range result.Highlights {
		add(highlight)
	}
	// Keep ES highlights first, then append AI-tool evidence as extra proof text.
	for _, evidence := range result.LookupEvidence {
		if strings.TrimSpace(evidence.SupportingSnippet) != "" {
			add(evidence.SupportingSnippet)
			continue
		}
		add(evidence.Content)
	}
	return highlights
}

func compactReasoningSearchHighlightsForDisplay(highlights []string) []string {
	if len(highlights) == 0 {
		return nil
	}
	items := append([]string(nil), highlights...)
	if len(items) > reasoningSearchDisplayHighlightMaxItems {
		items = items[:reasoningSearchDisplayHighlightMaxItems]
	}
	for i := range items {
		items[i] = truncateHighlightForDisplay(items[i], reasoningSearchDisplayHighlightMaxRunes)
	}
	return items
}

func visibleHighlightRuneCount(value string) int {
	count := 0
	inTag := false
	for _, r := range value {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			count++
		}
	}
	return count
}

func truncateHighlightForDisplay(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxRunes <= 0 || visibleHighlightRuneCount(value) <= maxRunes {
		return value
	}

	var b strings.Builder
	visible := 0
	openEm := 0
	truncated := false

	for i := 0; i < len(value); {
		if value[i] == '<' {
			end := strings.IndexByte(value[i:], '>')
			if end < 0 {
				break
			}
			tag := value[i : i+end+1]
			b.WriteString(tag)
			switch tag {
			case "<em>":
				openEm++
			case "</em>":
				if openEm > 0 {
					openEm--
				}
			}
			i += end + 1
			continue
		}

		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		if visible >= maxRunes {
			truncated = true
			break
		}
		b.WriteRune(r)
		visible++
		i += size
	}

	result := strings.TrimSpace(b.String())
	if truncated {
		result = strings.TrimRight(result, " ,;:.!?")
		result += "..."
	}
	for openEm > 0 {
		result += "</em>"
		openEm--
	}
	return result
}

type reasoningSearchCandidateInput struct {
	MDBUID           string                              `json:"mdb_uid"`
	ResultType       string                              `json:"result_type"`
	Title            string                              `json:"title"`
	Description      string                              `json:"description,omitempty"`
	ContentType      string                              `json:"content_type"`
	ProgramName      string                              `json:"program_name,omitempty"`
	Date             string                              `json:"date,omitempty"`
	Reason           string                              `json:"reason,omitempty"`
	SearchHighlights []string                            `json:"search_highlights,omitempty"`
	IsGroupingResult bool                                `json:"is_grouping_result"`
	LookupEvidence   []llm.ReasoningSearchResultEvidence `json:"lookup_evidence,omitempty"`
}

func reasoningSearchCandidateInputs(results []llm.ReasoningSearchResult) []reasoningSearchCandidateInput {
	items := make([]reasoningSearchCandidateInput, 0, len(results))
	for _, result := range results {
		items = append(items, reasoningSearchCandidateInput{
			MDBUID:           result.MDBUID,
			ResultType:       result.ResultType,
			Title:            result.Title,
			Description:      result.Description,
			ContentType:      result.ContentType,
			ProgramName:      result.ProgramName,
			Date:             result.Date,
			Reason:           result.Reason,
			SearchHighlights: append([]string(nil), result.Highlights...),
			IsGroupingResult: result.IsGroupingResult,
			LookupEvidence:   append([]llm.ReasoningSearchResultEvidence(nil), result.LookupEvidence...),
		})
	}
	return items
}

func rapidCollectedResults(session *llm.ReasoningWorkflowSession) []llm.ReasoningSearchResult {
	if session == nil {
		return nil
	}
	results := append([]llm.ReasoningSearchResult(nil), session.PartialResults...)
	if session.PartialLookupEvidence == nil {
		return results
	}
	for i := range results {
		uid := strings.TrimSpace(results[i].MDBUID)
		if uid == "" {
			continue
		}
		results[i].LookupEvidence = append([]llm.ReasoningSearchResultEvidence(nil), session.PartialLookupEvidence[uid]...)
	}
	return results
}

type reasoningSearchPreviousResponseContext struct {
	Query   string                              `json:"query"`
	Summary string                              `json:"summary"`
	Results []reasoningSearchPreviousResultItem `json:"results"`
}

type reasoningSearchPreviousResultItem struct {
	MDBUID           string `json:"mdb_uid"`
	ResultType       string `json:"result_type"`
	Title            string `json:"title"`
	ContentType      string `json:"content_type"`
	Reason           string `json:"reason"`
	IsGroupingResult bool   `json:"is_grouping_result"`
}

func buildReasoningSearchPreviousResponseContext(response *llm.ReasoningSearchResponse) *reasoningSearchPreviousResponseContext {
	if response == nil {
		return nil
	}
	context := &reasoningSearchPreviousResponseContext{
		Query:   response.Query,
		Summary: response.Summary,
		Results: make([]reasoningSearchPreviousResultItem, 0, len(response.Results)),
	}
	for _, result := range response.Results {
		context.Results = append(context.Results, reasoningSearchPreviousResultItem{
			MDBUID:           result.MDBUID,
			ResultType:       result.ResultType,
			Title:            result.Title,
			ContentType:      result.ContentType,
			Reason:           result.Reason,
			IsGroupingResult: result.IsGroupingResult,
		})
	}
	return context
}

func rapidSelectedResults(db *sql.DB, uiLanguage string, results []llm.ReasoningSearchResult, evidenceByUID map[string][]llm.ReasoningSearchResultEvidence, selectedUIDs []string) ([]llm.ReasoningSearchResult, error) {
	if len(selectedUIDs) == 0 {
		return nil, nil
	}
	byUID := map[string]llm.ReasoningSearchResult{}
	for _, result := range results {
		uid := strings.TrimSpace(result.MDBUID)
		if uid != "" {
			byUID[uid] = result
		}
	}
	selected := make([]llm.ReasoningSearchResult, 0, len(selectedUIDs))
	missing := make([]llm.ReasoningSearchResult, 0, len(selectedUIDs))
	seen := map[string]bool{}
	for _, rawUID := range selectedUIDs {
		uid := strings.TrimSpace(rawUID)
		if uid == "" || seen[uid] {
			continue
		}
		result, ok := byUID[uid]
		if !ok {
			missing = append(missing, llm.ReasoningSearchResult{MDBUID: uid})
			seen[uid] = true
			if len(selected)+len(missing) == reasoningSearchRapidSelectedUIDLimit {
				break
			}
			continue
		}
		selected = append(selected, result)
		seen[uid] = true
		if len(selected)+len(missing) == reasoningSearchRapidSelectedUIDLimit {
			break
		}
	}
	if len(missing) != 0 {
		if err := enrichReasoningSearchResults(db, uiLanguage, missing); err != nil {
			return nil, err
		}
		for i := range missing {
			if strings.TrimSpace(missing[i].ResultType) == "" {
				continue
			}
			if len(evidenceByUID[missing[i].MDBUID]) > 0 {
				missing[i].LookupEvidence = append([]llm.ReasoningSearchResultEvidence(nil), evidenceByUID[missing[i].MDBUID]...)
			}
			missing[i].IsGroupingResult = missing[i].ResultType == consts.ES_RESULT_TYPE_COLLECTIONS || missing[i].ResultType == consts.ES_RESULT_TYPE_TAGS
			missing[i].Origin = llm.ReasoningSearchResultOriginOriginal
			byUID[missing[i].MDBUID] = missing[i]
		}
	}
	selected = selected[:0]
	seen = map[string]bool{}
	for _, rawUID := range selectedUIDs {
		uid := strings.TrimSpace(rawUID)
		if uid == "" || seen[uid] {
			continue
		}
		result, ok := byUID[uid]
		if !ok {
			continue
		}
		selected = append(selected, result)
		seen[uid] = true
		if len(selected) == reasoningSearchRapidSelectedUIDLimit {
			break
		}
	}
	if err := enrichReasoningSearchResults(db, uiLanguage, selected); err != nil {
		return nil, err
	}
	return selected, nil
}

func rapidMainModelUsage(debug *llm.ReasoningSearchDebugInfo) *llm.ReasoningSearchUsageBreakdown {
	if debug == nil {
		return nil
	}
	if debug.MainModelUsage != nil {
		usage := *debug.MainModelUsage
		return &usage
	}
	return debug.UsageBreakdown()
}

func buildReasoningSearchPreviousResponseSeedAssistantContent(response *llm.ReasoningSearchResponse) string {
	if response == nil {
		return ""
	}
	raw, err := json.Marshal(struct {
		Query   string                              `json:"query"`
		Summary string                              `json:"summary"`
		Results []reasoningSearchPreviousResultItem `json:"results"`
	}{
		Query:   response.Query,
		Summary: response.Summary,
		Results: buildReasoningSearchPreviousResponseContext(response).Results,
	})
	if err != nil {
		return response.Summary
	}
	return "Previous visible response. Use it as conversation context for the user's follow-up, but verify and improve with current tools/results when needed:\n" + string(raw)
}

func buildReasoningSearchDraftFollowupSeedAssistantContent(response *llm.ReasoningSearchResponse) string {
	if response == nil {
		return ""
	}
	raw, err := json.Marshal(buildReasoningSearchPreviousResponseContext(response))
	if err != nil {
		return response.Summary
	}
	return "Previous visible response was an early draft based on partial Elasticsearch results. Use it as conversation context for the user's follow-up, but verify and improve it with tools when needed:\n" + string(raw)
}

func executeReasoningSearchInBackground(ctx context.Context, runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, responseSessionID string) {
	defer func() {
		if runtime != nil && runtime.Cancellations != nil {
			runtime.Cancellations.Delete(responseSessionID)
		}
		if recovered := recover(); recovered != nil {
			log.Errorf("Reasoning Search background panic session=%s: %v", responseSessionID, recovered)
			if runtime != nil && runtime.Workflow != nil {
				if finalized, err := runtime.Workflow.IsFinalizedFromDraft(responseSessionID); err == nil && finalized {
					log.Infof("Reasoning Search panic ignored because draft was already returned session=%s", responseSessionID)
					return
				}
				if err := runtime.Workflow.SetResponseSnapshot(responseSessionID, nil); err != nil && !errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
					log.Warnf("Reasoning Search failed clearing response snapshot after panic: %v", err)
				}
			}
			if runtime != nil && runtime.Progress != nil {
				runtime.Progress.Fail(responseSessionID, 0)
			}
		}
	}()
	if err := executeReasoningSearchForSessionByMode(ctx, runtime, db, r, responseSessionID, true); err != nil {
		if isReasoningSearchCancellation(err) {
			log.Infof("Reasoning Search background canceled session=%s", responseSessionID)
			return
		}
		if runtime != nil && runtime.Workflow != nil {
			if finalized, finalErr := runtime.Workflow.IsFinalizedFromDraft(responseSessionID); finalErr == nil && finalized {
				log.Infof("Reasoning Search background error ignored because draft was already returned session=%s err=%v", responseSessionID, err)
				return
			}
		}
		log.Warnf("Reasoning Search background execution failed session=%s: %v", responseSessionID, err)
	}
}

func validateReasoningSearchResponseQuery(expected string, response *llm.ReasoningSearchResponse) error {
	expected = strings.TrimSpace(expected)
	actual := ""
	if response != nil {
		actual = strings.TrimSpace(response.Query)
	}
	if normalizeReasoningSearchQueryForComparison(expected) == normalizeReasoningSearchQueryForComparison(actual) {
		return nil
	}
	return fmt.Errorf("%w: returned query %q but expected %q", errReasoningSearchQueryMismatch, actual, expected)
}

func normalizeReasoningSearchQueryForComparison(query string) string {
	query = strings.TrimSpace(query)
	var builder strings.Builder
	lastWasSpace := false
	for _, r := range query {
		if isReasoningSearchQuoteMark(r) {
			continue
		}
		if unicode.IsSpace(r) {
			if builder.Len() > 0 && !lastWasSpace {
				builder.WriteByte(' ')
				lastWasSpace = true
			}
			continue
		}
		builder.WriteRune(r)
		lastWasSpace = false
	}
	return strings.TrimSpace(builder.String())
}

func mergeReasoningSearchUsageBreakdown(dst **llm.ReasoningSearchUsageBreakdown, src *llm.ReasoningSearchUsageBreakdown) {
	if src == nil {
		return
	}
	if *dst == nil {
		copy := *src
		*dst = &copy
		return
	}
	(*dst).TotalTokens += src.TotalTokens
	(*dst).InputTokens += src.InputTokens
	(*dst).CachedInputTokens += src.CachedInputTokens
	(*dst).UncachedInputTokens += src.UncachedInputTokens
	(*dst).OutputTokens += src.OutputTokens
	(*dst).ReasoningTokens += src.ReasoningTokens
	(*dst).EstimatedInputCostUSD += src.EstimatedInputCostUSD
	(*dst).EstimatedCachedInputCostUSD += src.EstimatedCachedInputCostUSD
	(*dst).EstimatedOutputCostUSD += src.EstimatedOutputCostUSD
	(*dst).EstimatedCostUSD += src.EstimatedCostUSD
	if !src.PricingConfigured {
		(*dst).PricingConfigured = false
	}
}

func mergeReasoningSearchAttemptStats(dst *llm.ReasoningSearchResponse, src *llm.ReasoningSearchResponse) {
	if dst == nil || src == nil {
		return
	}
	dst.UsedTokens += src.UsedTokens
	dst.ReasoningIterations += src.ReasoningIterations
	if len(src.UsedTools) != 0 {
		mergedTools := make([]string, 0, len(dst.UsedTools)+len(src.UsedTools))
		seen := map[string]bool{}
		for _, tool := range dst.UsedTools {
			if !seen[tool] {
				seen[tool] = true
				mergedTools = append(mergedTools, tool)
			}
		}
		for _, tool := range src.UsedTools {
			if !seen[tool] {
				seen[tool] = true
				mergedTools = append(mergedTools, tool)
			}
		}
		dst.SetUsedTools(mergedTools)
	}
	if src.Debug == nil {
		return
	}
	if dst.Debug == nil {
		debugCopy := *src.Debug
		if src.Debug.MainModelUsage != nil {
			mainCopy := *src.Debug.MainModelUsage
			debugCopy.MainModelUsage = &mainCopy
		}
		if src.Debug.AIToolsUsage != nil {
			aiCopy := *src.Debug.AIToolsUsage
			debugCopy.AIToolsUsage = &aiCopy
		}
		if src.Debug.DraftModelUsage != nil {
			draftCopy := *src.Debug.DraftModelUsage
			debugCopy.DraftModelUsage = &draftCopy
		}
		if len(src.Debug.DraftModelRuns) != 0 {
			debugCopy.DraftModelRuns = append([]llm.ReasoningSearchUsageBreakdown(nil), src.Debug.DraftModelRuns...)
		}
		if len(src.Debug.AIToolsCalls) != 0 {
			debugCopy.AIToolsCalls = append([]llm.ReasoningSearchAIToolCallDebug(nil), src.Debug.AIToolsCalls...)
		}
		dst.Debug = &debugCopy
		return
	}
	mergeReasoningSearchUsageBreakdown(&dst.Debug.MainModelUsage, src.Debug.MainModelUsage)
	mergeReasoningSearchUsageBreakdown(&dst.Debug.AIToolsUsage, src.Debug.AIToolsUsage)
	mergeReasoningSearchUsageBreakdown(&dst.Debug.DraftModelUsage, src.Debug.DraftModelUsage)
	dst.Debug.Add(src.Debug)
}

func mergeReasoningSearchDraftUsage(response *llm.ReasoningSearchResponse, runs []llm.ReasoningSearchUsageBreakdown, usage *llm.ReasoningSearchUsageBreakdown, deb bool) {
	if response == nil || usage == nil {
		return
	}
	response.UsedTokens += usage.TotalTokens
	if !deb && response.Debug == nil {
		return
	}
	if response.Debug == nil {
		response.Debug = &llm.ReasoningSearchDebugInfo{Enabled: true}
	}
	response.Debug.Add(&llm.ReasoningSearchDebugInfo{
		Enabled:                     true,
		PricingConfigured:           usage.PricingConfigured,
		TotalTokens:                 usage.TotalTokens,
		InputTokens:                 usage.InputTokens,
		CachedInputTokens:           usage.CachedInputTokens,
		UncachedInputTokens:         usage.UncachedInputTokens,
		OutputTokens:                usage.OutputTokens,
		ReasoningTokens:             usage.ReasoningTokens,
		EstimatedInputCostUSD:       usage.EstimatedInputCostUSD,
		EstimatedCachedInputCostUSD: usage.EstimatedCachedInputCostUSD,
		EstimatedOutputCostUSD:      usage.EstimatedOutputCostUSD,
		EstimatedCostUSD:            usage.EstimatedCostUSD,
		DraftModelUsage:             usage,
		DraftModelRuns:              append([]llm.ReasoningSearchUsageBreakdown(nil), runs...),
	})
}

// Runs the full reasoning workflow and stores the final per-session response
// snapshot. The shared query cache may seed an initial result, but the final
// API payload is always stored per workflow session.
func executeReasoningSearchForSessionByMode(ctx context.Context, runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, responseSessionID string, enableDraft bool) error {
	if runtime == nil || runtime.Workflow == nil {
		return errors.New("reasoning workflow is not initialized")
	}
	session, err := runtime.Workflow.Get(responseSessionID)
	if err != nil {
		return err
	}
	if session.Rapid {
		return executeRapidReasoningSearchForSession(ctx, runtime, db, r, responseSessionID)
	}
	return executeReasoningSearchForSession(ctx, runtime, db, r, responseSessionID, enableDraft)
}

func executeRapidReasoningSearchForSession(ctx context.Context, runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, responseSessionID string) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtime == nil || runtime.Tools == nil || runtime.Workflow == nil || runtime.Progress == nil || runtime.RapidConfig == nil {
		return errors.New("rapid reasoning workflow is not initialized")
	}
	if db == nil {
		return errors.New("MDB_DB is not initialized")
	}

	workflowStore := runtime.Workflow
	progressStore := runtime.Progress
	progressIteration := 0
	defer func() {
		if err == nil {
			return
		}
		if workflowStore != nil {
			if clearErr := workflowStore.SetResponseSnapshot(responseSessionID, nil); clearErr != nil && !errors.Is(clearErr, llm.ErrReasoningSessionNotFoundOrExpired) {
				log.Warnf("Rapid Reasoning Search failed clearing response snapshot: %v", clearErr)
			}
		}
		if progressStore != nil {
			// The LLM service may have already advanced progress; preserve the latest
			// visible iteration when converting a rapid failure into terminal status.
			if status, statusErr := progressStore.Get(responseSessionID); statusErr == nil && status.Iteration > progressIteration {
				progressIteration = status.Iteration
			}
			if isReasoningSearchCancellation(err) {
				progressStore.Cancel(responseSessionID, progressIteration)
				return
			}
			progressStore.Fail(responseSessionID, progressIteration)
		}
	}()

	session, err := workflowStore.Get(responseSessionID)
	if err != nil {
		return err
	}
	gatherStage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	gatherService := runtime.Services[gatherStage.Provider]
	if gatherService == nil {
		return errors.New("rapid reasoning llm service is not initialized")
	}

	outputLanguageName := reasoningSearchOutputLanguageName(r.UILanguage, r.Query)
	systemMessage := llm.GenerateSystemMessageForReasoningSearch(runtime.Tools.Tools(), gatherStage.MaxIterations, gatherStage.MaxFollowups-session.FollowupCount)
	systemMessage = llm.AppendReasoningSearchOutputLanguage(systemMessage, outputLanguageName)
	systemMessage += "\n\n" + reasoningSearchRapidGatherInstruction
	messages := []llm.LLMBotMessage{{Role: "system", Content: systemMessage}}
	if session.RapidFollowupSeed != nil {
		messages = append(messages,
			llm.LLMBotMessage{Role: "user", Content: session.RapidFollowupSeed.Query},
			llm.LLMBotMessage{Role: "assistant", Content: buildReasoningSearchPreviousResponseSeedAssistantContent(session.RapidFollowupSeed)},
		)
	}
	messages = append(messages, llm.LLMBotMessage{Role: "user", Content: r.Query})

	providerID := strings.TrimSpace(gatherStage.ProviderSessionID)
	providerSessionID := &providerID
	gather := rapidGatherResponse{}
	gatherCtx := llm.ContextWithReasoningDraftState(ctx, workflowStore, responseSessionID, nil)
	rapidGatherSchema := llm.ReasoningSearchRapidGatherResponseJSONSchema
	gatherStarted := time.Now()
	resolvedProviderSessionID, err := gatherService.GetReasoningStructuredOutputWithToolsForSession(
		gatherCtx,
		providerSessionID,
		&responseSessionID,
		rapidGatherSchema,
		gatherStage.Model,
		&gatherStage.MaxTokens,
		messages,
		runtime.Tools.ToolCalls(),
		runtime.Tools.ToolHandlers(),
		nil,
		nil,
		nil,
		&gatherStage.ReasoningEffort,
		r.Deb,
		gatherStage.MaxIterations,
		&gather,
	)
	gatherLatencyMS := time.Since(gatherStarted).Milliseconds()
	if err != nil {
		return err
	}
	progressIteration = gather.ReasoningIterations
	gatherStage.ProviderSessionID = resolvedProviderSessionID
	if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, gatherStage); err != nil {
		return err
	}

	session, err = workflowStore.Get(responseSessionID)
	if err != nil {
		return err
	}
	candidates := rapidCollectedResults(session)
	if len(candidates) == 0 {
		return errors.New("rapid reasoning search gathered no results")
	}
	gatheredCandidateCount := len(candidates)
	candidates, err = rapidSelectedResults(db, r.UILanguage, candidates, session.PartialLookupEvidence, gather.SelectedUIDs)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return errors.New("rapid reasoning search selected no gathered results")
	}
	// Debug should describe the cleaned finalizer input, not the raw UID list the
	// gather model returned before duplicate/unknown IDs were removed.
	selectedUIDsForDebug := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		uid := strings.TrimSpace(candidate.MDBUID)
		if uid != "" {
			selectedUIDsForDebug = append(selectedUIDsForDebug, uid)
		}
	}

	followupsRemaining := gatherStage.MaxFollowups - session.FollowupCount
	if followupsRemaining < 0 {
		followupsRemaining = 0
	}
	if !runtime.RapidConfig.FinalizerEnabled {
		response := llm.ReasoningSearchResponse{
			Query:   r.Query,
			Results: append([]llm.ReasoningSearchResult(nil), candidates...),
		}
		response.SetSessionID(responseSessionID)
		response.SetUsedTools(gather.UsedTools)
		response.SetReasoningSteps(gather.ReasoningSummary)
		response.SetReasoningProcessStats(gather.UsedTokens, gather.ReasoningIterations)
		for i := range response.Results {
			response.Results[i].Reason = ""
			response.Results[i].Origin = llm.ReasoningSearchResultOriginOriginal
		}
		populateReasoningSearchHighlightsFromEvidence(&response, candidates, session.PartialLookupEvidence)
		response.SetFollowupBudget(gatherStage.MaxFollowups, session.FollowupCount, followupsRemaining)
		if r.Deb {
			debug := &llm.ReasoningSearchDebugInfo{}
			debug.RapidGatherModelUsage = rapidMainModelUsage(gather.Debug)
			debug.RapidGatherLatencyMS = gatherLatencyMS
			debug.RapidGatheredCandidateCount = gatheredCandidateCount
			debug.RapidSelectedUIDCount = len(candidates)
			debug.RapidSelectedUIDs = selectedUIDsForDebug
			debug.Add(gather.Debug)
			response.Debug = debug
		}
		if err := workflowStore.SetResponseSnapshot(responseSessionID, &response); err != nil {
			return err
		}
		if !session.InitialRequestCompleted {
			if err := workflowStore.SetFollowupState(responseSessionID, true, session.FollowupCount); err != nil {
				return err
			}
		}
		progressStore.Complete(responseSessionID, gather.ReasoningIterations)
		return nil
	}

	progressStore.Finalizing(responseSessionID, gather.ReasoningIterations+1)
	progressIteration = gather.ReasoningIterations + 1
	finalizerStage, ok := session.Stages[llm.ReasoningWorkflowStageRapidFinalizer]
	if !ok {
		finalizerStage = llm.ReasoningWorkflowStageSession{
			Provider:        runtime.RapidConfig.Finalizer.Provider,
			Model:           runtime.RapidConfig.Finalizer.Model,
			ReasoningEffort: runtime.RapidConfig.Finalizer.Effort,
			MaxTokens:       runtime.RapidConfig.Finalizer.MaxTokens,
		}
	}
	finalizerService := runtime.Services[finalizerStage.Provider]
	if finalizerService == nil {
		return errors.New("rapid reasoning finalizer llm service is not initialized")
	}
	responseSchema, err := llm.GenerateReasoningSearchResponseJSONSchemaForLanguage(outputLanguageName)
	if err != nil {
		return err
	}
	finalizerInputPayload := struct {
		Query            string                                  `json:"query"`
		PreviousResponse *reasoningSearchPreviousResponseContext `json:"previous_response,omitempty"`
		Results          []reasoningSearchCandidateInput         `json:"results"`
	}{
		Query:            r.Query,
		PreviousResponse: buildReasoningSearchPreviousResponseContext(session.RapidFollowupSeed),
		Results:          reasoningSearchCandidateInputs(candidates),
	}
	var finalizerInputBuffer bytes.Buffer
	finalizerInputEncoder := json.NewEncoder(&finalizerInputBuffer)
	finalizerInputEncoder.SetEscapeHTML(false)
	if err := finalizerInputEncoder.Encode(finalizerInputPayload); err != nil {
		return err
	}
	// Rapid finalization is a no-tools formatting/ranking pass over already
	// selected candidates, so keep the prompt small and avoid tool instructions.
	finalizerSystemMessage := reasoningSearchRapidFinalizerInstruction
	finalizerSystemMessage = llm.AppendReasoningSearchOutputLanguage(finalizerSystemMessage, outputLanguageName)
	finalizerMessages := []llm.LLMBotMessage{
		{Role: "system", Content: finalizerSystemMessage},
		{Role: "user", Content: strings.TrimSpace(finalizerInputBuffer.String())},
	}

	response := llm.ReasoningSearchResponse{}
	promptCacheKey := fmt.Sprintf("reasoning-search-rapid-finalizer:m=%s:e=%s", finalizerStage.Model, finalizerStage.ReasoningEffort)
	finalizerStarted := time.Now()
	finalizerDebug, err := finalizerService.GetStructuredOutputWithDebugInfo(ctx, responseSchema, finalizerStage.Model, &finalizerStage.MaxTokens, finalizerMessages, &promptCacheKey, &finalizerStage.ReasoningEffort, r.Deb, &response)
	finalizerLatencyMS := time.Since(finalizerStarted).Milliseconds()
	if err != nil {
		return err
	}
	response.Query = r.Query
	response.SetSessionID(responseSessionID)
	response.SetUsedTools(gather.UsedTools)
	response.SetReasoningSteps(gather.ReasoningSummary)
	response.SetReasoningProcessStats(gather.UsedTokens, gather.ReasoningIterations)
	if finalizerDebug != nil {
		response.UsedTokens += finalizerDebug.TotalTokens
	}
	if r.Deb {
		debug := finalizerDebug
		if debug == nil {
			debug = &llm.ReasoningSearchDebugInfo{}
		}
		debug.RapidGatherModelUsage = rapidMainModelUsage(gather.Debug)
		debug.RapidFinalizerModelUsage = rapidMainModelUsage(finalizerDebug)
		debug.RapidGatherLatencyMS = gatherLatencyMS
		debug.RapidFinalizerLatencyMS = finalizerLatencyMS
		debug.RapidGatheredCandidateCount = gatheredCandidateCount
		debug.RapidSelectedUIDCount = len(candidates)
		debug.RapidFinalizerResultCount = len(candidates)
		debug.RapidSelectedUIDs = selectedUIDsForDebug
		debug.Add(gather.Debug)
		response.Debug = debug
	}
	mergeReasoningSearchDraftMetadata(&response, candidates)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		return err
	}
	for i := range response.Results {
		response.Results[i].Origin = llm.ReasoningSearchResultOriginOriginal
	}
	response.SetFollowupBudget(gatherStage.MaxFollowups, session.FollowupCount, followupsRemaining)
	if err := workflowStore.SetResponseSnapshot(responseSessionID, &response); err != nil {
		return err
	}
	if !session.InitialRequestCompleted {
		if err := workflowStore.SetFollowupState(responseSessionID, true, session.FollowupCount); err != nil {
			return err
		}
	}
	progressStore.Complete(responseSessionID, gather.ReasoningIterations+1)
	return nil
}

func executeReasoningSearchForSession(ctx context.Context, runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, responseSessionID string, enableDraft bool) (err error) {
	const maxQueryMismatchValidationAttempts = 2

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtime == nil || runtime.Tools == nil || runtime.Progress == nil || runtime.Workflow == nil {
		return errors.New("reasoning workflow is not initialized")
	}
	if db == nil {
		return errors.New("MDB_DB is not initialized")
	}

	manager := runtime.Tools
	if manager == nil {
		return errors.New("LLM_TOOLS is not initialized")
	}

	workflowStore := runtime.Workflow
	progressStore := runtime.Progress
	response := llm.ReasoningSearchResponse{}
	resolvedProviderSessionID := ""
	reasoningIterations := 0
	runProviderSessionID := ""
	defer func() {
		if err == nil {
			return
		}
		if workflowStore != nil {
			if finalized, finalErr := workflowStore.IsFinalizedFromDraft(responseSessionID); finalErr == nil && finalized {
				log.Infof("Reasoning Search background error ignored because draft was already returned session=%s err=%v", responseSessionID, err)
				return
			}
			if runProviderSessionID != "" {
				if current, currentErr := reasoningStageProviderSessionCurrent(workflowStore, responseSessionID, runProviderSessionID); currentErr == nil && !current {
					log.Infof("Reasoning Search background error ignored because a newer follow-up run is active session=%s err=%v", responseSessionID, err)
					return
				}
			}
		}
		if workflowStore != nil {
			if clearErr := workflowStore.SetResponseSnapshot(responseSessionID, nil); clearErr != nil && !errors.Is(clearErr, llm.ErrReasoningSessionNotFoundOrExpired) {
				log.Warnf("Reasoning Search failed clearing response snapshot: %v", clearErr)
			}
		}
		if progressStore != nil {
			if isReasoningSearchCancellation(err) {
				progressStore.Cancel(responseSessionID, reasoningIterations)
				return
			}
			progressStore.Fail(responseSessionID, reasoningIterations)
		}
	}()

	workflowSession, err := workflowStore.Get(responseSessionID)
	if err != nil {
		return err
	}
	reasoningStage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
	if !ok || strings.TrimSpace(reasoningStage.ProviderSessionID) == "" {
		return llm.ErrReasoningSessionNotFoundOrExpired
	}
	var planningStage *llm.ReasoningWorkflowStageSession
	if stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStagePlanning]; ok {
		stageCopy := stage
		planningStage = &stageCopy
	}
	var verificationStage *llm.ReasoningWorkflowStageSession
	if stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageVerification]; ok {
		stageCopy := stage
		verificationStage = &stageCopy
	}
	initialRequestCompleted := workflowSession.InitialRequestCompleted
	cachedInitialResponse := workflowSession.CachedInitialResponse
	draftFollowupSeed := workflowSession.DraftFollowupSeed
	providerID := strings.TrimSpace(reasoningStage.ProviderSessionID)
	providerSessionID := &providerID
	runProviderSessionID = providerID
	progressSessionID := responseSessionID
	followupsUsed := workflowSession.FollowupCount
	followupsRemaining := reasoningStage.MaxFollowups - followupsUsed
	if followupsRemaining < 0 {
		followupsRemaining = 0
	}

	cacheKey := ""
	cacheEligible := false
	if runtime.ReasoningCache != nil && !r.Deb {
		cacheKey, cacheEligible = llm.ReasoningSearchCacheKeyForQuery(r.Query)
	}
	if cacheEligible && !initialRequestCompleted {
		if cachedEntry, ok := runtime.ReasoningCache.Get(cacheKey); ok {
			if err := storeReasoningSearchCachedResponse(runtime, db, r, responseSessionID, cachedEntry, reasoningStage.MaxFollowups); err != nil {
				return err
			}
			log.Infof("Reasoning Search Cache Hit: [%s]", cacheKey)
			return nil
		}
	}

	service := runtime.Services[reasoningStage.Provider]
	if service == nil {
		return errors.New("reasoning llm service is not initialized")
	}

	outputLanguageName := reasoningSearchOutputLanguageName(r.UILanguage, r.Query)
	systemMessage := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), reasoningStage.MaxIterations, followupsRemaining)
	var planningDebug *llm.ReasoningSearchDebugInfo
	var firstIterationTools []llm.ToolCall
	var firstIterationToolHandlers map[string]llm.ToolHandler
	progressIterationOffset := 0
	if planningStage != nil && !initialRequestCompleted {
		if err := ctx.Err(); err != nil {
			return err
		}
		planningService := runtime.Services[planningStage.Provider]
		if planningService == nil {
			log.Warnf("Reasoning Search planning skipped: service for provider %q is not initialized", planningStage.Provider)
		} else {
			progressIterationOffset++
			progressStore.Planning(responseSessionID, progressIterationOffset)
			planningPromptCacheKey := fmt.Sprintf(
				"reasoning-search-planning:m=%s:e=%s",
				planningStage.Model,
				planningStage.ReasoningEffort,
			)
			planningMessages := []llm.LLMBotMessage{
				{
					Role:    "system",
					Content: llm.GenerateSystemMessageForReasoningSearchPlanning(manager.Tools()),
				},
				{
					Role:    "user",
					Content: r.Query,
				},
			}
			planningResponse := llm.ReasoningSearchPlanningResponse{}
			debugInfo, err := planningService.GetStructuredOutputWithDebugInfo(ctx,
				llm.GenerateReasoningSearchPlanningResponseJSONSchema(manager.Tools()),
				planningStage.Model,
				&planningStage.MaxTokens,
				planningMessages,
				&planningPromptCacheKey,
				&planningStage.ReasoningEffort,
				r.Deb,
				&planningResponse,
			)
			if err != nil {
				if isReasoningSearchCancellation(err) {
					return err
				}
				log.Warnf("Reasoning Search planning failed: %v", err)
			} else {
				planningDebug = debugInfo
				if r.Deb {
					planningOutput := planningResponse
					if planningResponse.FirstIterationTools != nil {
						planningOutput.FirstIterationTools = append([]llm.ReasoningSearchPlanningToolSpec(nil), planningResponse.FirstIterationTools...)
					}
					response.PlanningOutput = &planningOutput
					if debugInfo != nil {
						response.PlanningReasoningSummary = strings.TrimSpace(debugInfo.ReasoningSummary)
					}
				}
				planningText := strings.TrimSpace(planningResponse.InstructionText)
				if planningText != "" {
					systemMessage = llm.AppendReasoningSearchPlanning(systemMessage, planningText)
				}
				firstIterationTools, firstIterationToolHandlers, err = buildFirstIterationPlannedTools(manager, &planningResponse)
				if err != nil {
					log.Warnf("Reasoning Search planning tool restriction skipped: %v", err)
					firstIterationTools = nil
					firstIterationToolHandlers = nil
				}
			}
		}
	}
	progressStore.SetIterationOffset(responseSessionID, progressIterationOffset)
	systemMessage = llm.AppendReasoningSearchOutputLanguage(systemMessage, outputLanguageName)
	responseSchema, err := llm.GenerateReasoningSearchResponseJSONSchemaForLanguage(outputLanguageName)
	if err != nil {
		return err
	}
	messages := []llm.LLMBotMessage{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}
	usedCachedInitialResponse := cachedInitialResponse != nil
	if cachedInitialResponse != nil {
		messages = append(messages,
			llm.LLMBotMessage{
				Role:    "user",
				Content: cachedInitialResponse.Query,
			},
			llm.LLMBotMessage{
				Role:    "assistant",
				Content: llm.BuildReasoningSearchCacheSeedAssistantContent(cachedInitialResponse),
			},
		)
	}
	if draftFollowupSeed != nil {
		messages = append(messages,
			llm.LLMBotMessage{
				Role:    "user",
				Content: draftFollowupSeed.Query,
			},
			llm.LLMBotMessage{
				Role:    "assistant",
				Content: buildReasoningSearchDraftFollowupSeedAssistantContent(draftFollowupSeed),
			},
		)
	}
	messages = append(messages, llm.LLMBotMessage{
		Role:    "user",
		Content: r.Query,
	})

	promptCacheKey := fmt.Sprintf(
		"reasoning-search:m=%s:e=%s",
		reasoningStage.Model,
		reasoningStage.ReasoningEffort,
	)

	log.Infof("Reasoning Search Query: [%s]", r.Query)
	if err := ctx.Err(); err != nil {
		return err
	}

	expectedQuery := strings.TrimSpace(r.Query)
	validateResponseQuery := !initialRequestCompleted
	currentProviderSessionID := providerSessionID
	var draftScheduler func(string)
	if enableDraft {
		draftScheduler = func(sessionID string) {
			maybeStartReasoningSearchDraft(runtime, db, r.UILanguage, sessionID, r.Deb)
		}
	}
	serviceCtx := llm.ContextWithReasoningDraftState(ctx, workflowStore, responseSessionID, draftScheduler)
	var previousReasoningAttempt *llm.ReasoningSearchResponse
	for attempt := 1; attempt <= maxQueryMismatchValidationAttempts; attempt++ {
		resolvedProviderSessionID, err = service.GetReasoningStructuredOutputWithToolsForSession(serviceCtx,
			currentProviderSessionID,
			&progressSessionID,
			responseSchema,
			reasoningStage.Model,
			&reasoningStage.MaxTokens,
			messages,
			manager.ToolCalls(),
			manager.ToolHandlers(),
			firstIterationTools,
			firstIterationToolHandlers,
			&promptCacheKey,
			&reasoningStage.ReasoningEffort,
			r.Deb,
			reasoningStage.MaxIterations,
			&response,
		)
		if err != nil {
			return err
		}
		if validateResponseQuery {
			err := validateReasoningSearchResponseQuery(expectedQuery, &response)
			if err != nil {
				if !errors.Is(err, errReasoningSearchQueryMismatch) {
					return err
				}
				if attempt == maxQueryMismatchValidationAttempts {
					return fmt.Errorf("reasoning search query mismatch after retry: %w", err)
				}
				log.Warnf("Reasoning Search query mismatch after reasoning stage: expected %q, got %q. Retrying with a fresh provider session.", expectedQuery, strings.TrimSpace(response.Query))
				previousAttempt := response
				previousReasoningAttempt = &previousAttempt
				currentProviderSessionID = nil
				continue
			}
		}
		if previousReasoningAttempt != nil {
			mergeReasoningSearchAttemptStats(&response, previousReasoningAttempt)
			response.QueryMismatchRetry = true
		}
		break
	}
	reasoningIterations = response.ReasoningIterations
	if current, err := reasoningStageProviderSessionCurrent(workflowStore, responseSessionID, providerID); err != nil {
		return err
	} else if !current {
		log.Infof("Reasoning Search result ignored because a newer follow-up run is active session=%s", responseSessionID)
		return nil
	}
	reasoningStage.ProviderSessionID = resolvedProviderSessionID
	if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, reasoningStage); err != nil {
		return err
	}
	runProviderSessionID = reasoningStage.ProviderSessionID
	if usedCachedInitialResponse {
		if err := workflowStore.SetCachedInitialResponse(responseSessionID, nil); err != nil {
			return err
		}
	}
	response.SetSessionID(responseSessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		return err
	}
	if currentSession, err := workflowStore.Get(responseSessionID); err != nil {
		return err
	} else {
		populateReasoningSearchHighlightsFromEvidence(&response, currentSession.PartialResults, currentSession.PartialLookupEvidence)
	}
	for i := range response.Results {
		response.Results[i].Origin = llm.ReasoningSearchResultOriginOriginal
	}
	if planningDebug != nil {
		response.UsedTokens += planningDebug.TotalTokens
		if response.Debug != nil {
			response.Debug.PlanningModelUsage = planningDebug.UsageBreakdown()
			response.Debug.Add(planningDebug)
		}
	}
	if finalized, err := workflowStore.IsFinalizedFromDraft(responseSessionID); err != nil {
		return err
	} else if finalized {
		log.Infof("Reasoning Search reasoning stage finished after draft was already returned session=%s; skipping verification", responseSessionID)
		progressStore.Complete(responseSessionID, progressIterationOffset+response.ReasoningIterations)
		return nil
	}
	progressCompleteIteration := progressIterationOffset + response.ReasoningIterations
	if verificationStage != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		if verificationStage.MaxInputTokensForVerification > 0 && response.UsedTokens >= verificationStage.MaxInputTokensForVerification {
			log.Infof("Reasoning Search verification skipped: total tokens %d reached threshold %d", response.UsedTokens, verificationStage.MaxInputTokensForVerification)
		} else if verificationService := runtime.Services[verificationStage.Provider]; verificationService == nil {
			log.Warnf("Reasoning Search verification skipped: service for provider %q is not initialized", verificationStage.Provider)
		} else {
			progressCompleteIteration++
			progressStore.Verifying(responseSessionID, progressCompleteIteration)

			verificationPromptCacheKey := fmt.Sprintf(
				"reasoning-search-verification:m=%s:e=%s",
				verificationStage.Model,
				verificationStage.ReasoningEffort,
			)
			verificationInput, err := json.Marshal(struct {
				Query            string                             `json:"query"`
				Summary          string                             `json:"summary"`
				ReasoningSummary []llm.ReasoningSearchReasoningStep `json:"reasoning_summary,omitempty"`
				Results          []llm.ReasoningSearchResult        `json:"results"`
			}{
				Query:            r.Query,
				Summary:          response.Summary,
				ReasoningSummary: response.ReasoningSummary,
				Results:          response.Results,
			})
			if err != nil {
				log.Warnf("Reasoning Search verification failed to build input: %v", err)
			} else {
				verificationMessages := []llm.LLMBotMessage{
					{
						Role:    "system",
						Content: fmt.Sprintf(llm.ReasoningSearchVerificationInstructionMask, llm.GeneralReasoningSearchInstruction),
					},
					{
						Role:    "user",
						Content: string(verificationInput),
					},
				}
				verificationResponse := llm.ReasoningSearchVerificationResponse{}
				verificationDebug, err := verificationService.GetStructuredOutputWithDebugInfo(ctx,
					llm.GenerateReasoningSearchVerificationResponseJSONSchema(),
					verificationStage.Model,
					&verificationStage.MaxTokens,
					verificationMessages,
					&verificationPromptCacheKey,
					&verificationStage.ReasoningEffort,
					true,
					&verificationResponse,
				)
				if err != nil {
					if isReasoningSearchCancellation(err) {
						return err
					}
					log.Warnf("Reasoning Search verification failed: %v", err)
				} else {
					originalResponse := response
					if response.UsedTools != nil {
						originalResponse.UsedTools = append([]string(nil), response.UsedTools...)
					}
					if response.Results != nil {
						originalResponse.Results = append([]llm.ReasoningSearchResult(nil), response.Results...)
					}
					if response.Debug != nil {
						debugCopy := *response.Debug
						originalResponse.Debug = &debugCopy
					}

					if r.Deb {
						verificationOutput := verificationResponse
						response.VerificationOutput = &verificationOutput
					}
					initialUsedTokens := response.UsedTokens
					initialReasoningIterations := response.ReasoningIterations
					initialUsedTools := append([]string(nil), response.UsedTools...)
					var initialPlanningOutput *llm.ReasoningSearchPlanningResponse
					if response.PlanningOutput != nil {
						planningCopy := *response.PlanningOutput
						if response.PlanningOutput.FirstIterationTools != nil {
							planningCopy.FirstIterationTools = append([]llm.ReasoningSearchPlanningToolSpec(nil), response.PlanningOutput.FirstIterationTools...)
						}
						initialPlanningOutput = &planningCopy
					}
					initialPlanningReasoningSummary := response.PlanningReasoningSummary
					initialDebug := response.Debug

					if verificationResponse.NeedsAnotherIteration {
						recommendation := strings.TrimSpace(verificationResponse.Recommendation)
						if recommendation == "" {
							log.Warn("Reasoning Search verification requested another iteration without recommendation")
							if verificationDebug != nil {
								response.UsedTokens += verificationDebug.TotalTokens
								if response.Debug != nil {
									response.Debug.VerificationModel = verificationDebug.Model
									response.Debug.VerificationReasoningEffort = verificationDebug.ReasoningEffort
									response.Debug.VerificationTotalTokens = verificationDebug.TotalTokens
									response.Debug.VerificationEstimatedCostUSD = verificationDebug.EstimatedCostUSD
									response.Debug.Add(verificationDebug)
								}
							}
						} else {
							rerunMessages := []llm.LLMBotMessage{
								{
									Role:    "system",
									Content: systemMessage,
								},
								{
									Role:    "user",
									Content: "INTERNAL VERIFICATION FEEDBACK:\n" + recommendation,
								},
							}
							nextProviderSessionID := resolvedProviderSessionID
							rerunProgressOffset := progressCompleteIteration
							progressStore.SetIterationOffset(responseSessionID, rerunProgressOffset)
							rerunStage := reasoningStage
							rerunStage.MaxIterations = reasoningStage.RerunMaxIterations
							rerunResponse := llm.ReasoningSearchResponse{}
							rerunResolvedProviderSessionID := ""
							currentRerunProviderSessionID := &nextProviderSessionID
							var previousRerunAttempt *llm.ReasoningSearchResponse
							for attempt := 1; attempt <= maxQueryMismatchValidationAttempts; attempt++ {
								rerunResolvedProviderSessionID, err = service.GetReasoningStructuredOutputWithToolsForSession(ctx,
									currentRerunProviderSessionID,
									&progressSessionID,
									responseSchema,
									rerunStage.Model,
									&rerunStage.MaxTokens,
									rerunMessages,
									manager.ToolCalls(),
									manager.ToolHandlers(),
									nil,
									nil,
									&promptCacheKey,
									&rerunStage.ReasoningEffort,
									r.Deb,
									rerunStage.MaxIterations,
									&rerunResponse,
								)
								if err != nil {
									break
								}
								if validateResponseQuery {
									if err := validateReasoningSearchResponseQuery(expectedQuery, &rerunResponse); err != nil {
										if !errors.Is(err, errReasoningSearchQueryMismatch) {
											break
										}
										if attempt == maxQueryMismatchValidationAttempts {
											err = fmt.Errorf("reasoning search query mismatch after retry: %w", err)
											break
										}
										log.Warnf("Reasoning Search query mismatch after rerun stage: expected %q, got %q. Retrying with a fresh provider session.", expectedQuery, strings.TrimSpace(rerunResponse.Query))
										previousAttempt := rerunResponse
										previousRerunAttempt = &previousAttempt
										currentRerunProviderSessionID = nil
										continue
									}
								}
								if previousRerunAttempt != nil {
									mergeReasoningSearchAttemptStats(&rerunResponse, previousRerunAttempt)
									rerunResponse.QueryMismatchRetry = true
								}
								break
							}
							progressStore.SetIterationOffset(responseSessionID, 0)
							if err != nil {
								if isReasoningSearchCancellation(err) {
									return err
								}
								log.Warnf("Reasoning Search rerun after verification failed: %v", err)
								response = originalResponse
							} else {
								if current, currentErr := reasoningStageProviderSessionCurrent(workflowStore, responseSessionID, reasoningStage.ProviderSessionID); currentErr != nil {
									return currentErr
								} else if !current {
									log.Infof("Reasoning Search verification rerun ignored because a newer follow-up run is active session=%s", responseSessionID)
									return nil
								}
								resolvedProviderSessionID = rerunResolvedProviderSessionID
								reasoningStage.ProviderSessionID = resolvedProviderSessionID
								if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, reasoningStage); err != nil {
									log.Warnf("Reasoning Search failed to persist rerun session state: %v", err)
									response = originalResponse
								} else {
									runProviderSessionID = reasoningStage.ProviderSessionID
									rerunResponse.SetSessionID(responseSessionID)
									if err := enrichReasoningSearchResults(db, r.UILanguage, rerunResponse.Results); err != nil {
										log.Warnf("Reasoning Search failed to enrich rerun results: %v", err)
										response = originalResponse
									} else {
										if currentSession, currentErr := workflowStore.Get(responseSessionID); currentErr == nil {
											populateReasoningSearchHighlightsFromEvidence(&rerunResponse, currentSession.PartialResults, currentSession.PartialLookupEvidence)
										}
										rerunResponse.QueryMismatchRetry = rerunResponse.QueryMismatchRetry || originalResponse.QueryMismatchRetry
										for i := range rerunResponse.Results {
											rerunResponse.Results[i].Origin = llm.ReasoningSearchResultOriginRerun
										}
										if len(originalResponse.Results) != 0 {
											mergedResults := make([]llm.ReasoningSearchResult, 0, len(rerunResponse.Results)+len(originalResponse.Results))
											seenResultUIDs := map[string]bool{}
											for _, result := range rerunResponse.Results {
												uid := strings.TrimSpace(result.MDBUID)
												if uid != "" {
													if seenResultUIDs[uid] {
														continue
													}
													seenResultUIDs[uid] = true
												}
												mergedResults = append(mergedResults, result)
											}
											for _, result := range originalResponse.Results {
												uid := strings.TrimSpace(result.MDBUID)
												if uid != "" {
													if seenResultUIDs[uid] {
														continue
													}
													seenResultUIDs[uid] = true
												}
												mergedResults = append(mergedResults, result)
											}
											rerunResponse.Results = mergedResults
										}
										rerunReasoningIterations := rerunResponse.ReasoningIterations
										progressCompleteIteration = rerunProgressOffset + rerunReasoningIterations
										rerunResponse.UsedTokens += initialUsedTokens
										rerunResponse.ReasoningIterations += initialReasoningIterations
										if verificationDebug != nil {
											rerunResponse.UsedTokens += verificationDebug.TotalTokens
										}
										mergedTools := make([]string, 0, len(initialUsedTools)+len(rerunResponse.UsedTools))
										seenTools := map[string]bool{}
										for _, tool := range initialUsedTools {
											if !seenTools[tool] {
												seenTools[tool] = true
												mergedTools = append(mergedTools, tool)
											}
										}
										for _, tool := range rerunResponse.UsedTools {
											if !seenTools[tool] {
												seenTools[tool] = true
												mergedTools = append(mergedTools, tool)
											}
										}
										rerunResponse.SetUsedTools(mergedTools)
										rerunResponse.PlanningOutput = initialPlanningOutput
										rerunResponse.PlanningReasoningSummary = initialPlanningReasoningSummary
										if rerunResponse.Debug != nil {
											if initialDebug != nil {
												if initialDebug.PlanningModelUsage != nil {
													planningUsageCopy := *initialDebug.PlanningModelUsage
													rerunResponse.Debug.PlanningModelUsage = &planningUsageCopy
												}
												rerunResponse.Debug.Add(initialDebug)
											}
											if verificationDebug != nil {
												rerunResponse.Debug.VerificationModel = verificationDebug.Model
												rerunResponse.Debug.VerificationReasoningEffort = verificationDebug.ReasoningEffort
												rerunResponse.Debug.VerificationTotalTokens = verificationDebug.TotalTokens
												rerunResponse.Debug.VerificationEstimatedCostUSD = verificationDebug.EstimatedCostUSD
												rerunResponse.Debug.Add(verificationDebug)
											}
										}
										if r.Deb {
											verificationOutput := verificationResponse
											rerunResponse.VerificationOutput = &verificationOutput
										}
										response = rerunResponse
									}
								}
							}
						}
					} else {
						if verificationDebug != nil {
							response.UsedTokens += verificationDebug.TotalTokens
							if response.Debug != nil {
								response.Debug.VerificationModel = verificationDebug.Model
								response.Debug.VerificationReasoningEffort = verificationDebug.ReasoningEffort
								response.Debug.VerificationTotalTokens = verificationDebug.TotalTokens
								response.Debug.VerificationEstimatedCostUSD = verificationDebug.EstimatedCostUSD
								response.Debug.Add(verificationDebug)
							}
						}
					}
				}
			}
		}
	}
	if !initialRequestCompleted {
		if err := workflowStore.SetFollowupState(responseSessionID, true, followupsUsed); err != nil {
			return err
		}
	}

	response.SetFollowupBudget(reasoningStage.MaxFollowups, followupsUsed, followupsRemaining)
	reasoningIterations = response.ReasoningIterations
	if finalized, err := workflowStore.IsFinalizedFromDraft(responseSessionID); err != nil {
		return err
	} else if finalized {
		log.Infof("Reasoning Search finished after draft was already returned session=%s; keeping draft snapshot", responseSessionID)
		progressStore.Complete(responseSessionID, progressCompleteIteration)
		return nil
	}
	if current, err := reasoningStageProviderSessionCurrent(workflowStore, responseSessionID, reasoningStage.ProviderSessionID); err != nil {
		return err
	} else if !current {
		log.Infof("Reasoning Search final response ignored because a newer follow-up run is active session=%s", responseSessionID)
		return nil
	}
	if runs, usage, err := workflowStore.DraftModelUsage(responseSessionID); err != nil {
		return err
	} else {
		mergeReasoningSearchDraftUsage(&response, runs, usage, r.Deb)
	}
	if err := workflowStore.SetResponseSnapshot(responseSessionID, &response); err != nil {
		return err
	}
	progressStore.Complete(responseSessionID, progressCompleteIteration)
	if cacheEligible && !initialRequestCompleted && runtime.ReasoningCache != nil {
		runtime.ReasoningCache.Set(cacheKey, llm.BuildReasoningSearchCacheEntryFromResponse(&response))
	}

	return nil
}

func isReasoningSearchCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func cancelReasoningSearchSession(runtime *llm.Runtime, sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if runtime == nil || runtime.Cancellations == nil || sessionID == "" {
		return false
	}
	canceled := runtime.Cancellations.Cancel(sessionID)
	if !canceled {
		return false
	}
	if runtime.Workflow != nil {
		if err := runtime.Workflow.SetResponseSnapshot(sessionID, nil); err != nil && !errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			log.Warnf("Reasoning Search failed clearing response snapshot after cancel: %v", err)
		}
	}
	if runtime.Progress != nil {
		runtime.Progress.Cancel(sessionID, 0)
	}
	return true
}

func refreshReasoningSearchSession(runtime *llm.Runtime, sessionID string) error {
	if runtime == nil || runtime.Workflow == nil {
		return errors.New("reasoning workflow is not initialized")
	}
	workflowSession, err := runtime.Workflow.Get(sessionID)
	if err != nil {
		return err
	}
	if reasoningStage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]; ok {
		refreshProviderReasoningSession(runtime, reasoningStage)
	}
	if err := runtime.Workflow.Refresh(sessionID); err != nil {
		return err
	}
	if runtime.Progress != nil {
		if err := runtime.Progress.Refresh(sessionID); err != nil && !errors.Is(err, llm.ErrReasoningProgressNotFoundOrExpired) {
			return err
		}
	}
	return nil
}

func refreshProviderReasoningSession(runtime *llm.Runtime, stage llm.ReasoningWorkflowStageSession) {
	if runtime == nil || runtime.Services == nil {
		return
	}
	providerSessionID := strings.TrimSpace(stage.ProviderSessionID)
	if providerSessionID == "" {
		return
	}
	service, ok := runtime.Services[stage.Provider].(llm.ReasoningSessionRefresher)
	if !ok || service == nil {
		return
	}
	if err := service.RefreshReasoningSession(providerSessionID); err != nil {
		log.Warnf("Reasoning Search failed refreshing provider session: provider=%q err=%v", stage.Provider, err)
	}
}

func reasoningStageProviderSessionCurrent(workflowStore *llm.ReasoningWorkflowSessionStore, sessionID string, expectedProviderSessionID string) (bool, error) {
	if workflowStore == nil {
		return false, errors.New("reasoning workflow is not initialized")
	}
	// A workflow session can outlive one provider-side reasoning session. For
	// example, a draft follow-up starts a fresh provider session while the old
	// background run may still finish later. Only the provider session currently
	// stored on the workflow is allowed to persist results or mark failures.
	workflowSession, err := workflowStore.Get(sessionID)
	if err != nil {
		return false, err
	}
	stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
	if !ok {
		return false, llm.ErrReasoningSessionNotFoundOrExpired
	}
	return strings.TrimSpace(stage.ProviderSessionID) == strings.TrimSpace(expectedProviderSessionID), nil
}

func buildFirstIterationPlannedTools(manager *llm.ReasoningToolManager, plan *llm.ReasoningSearchPlanningResponse) ([]llm.ToolCall, map[string]llm.ToolHandler, error) {
	if manager == nil || plan == nil || len(plan.FirstIterationTools) == 0 {
		return nil, nil, nil
	}

	definitionsByName := map[string]llm.ReasoningToolDefinition{}
	for _, definition := range manager.Definitions() {
		definitionsByName[definition.Name] = definition
	}
	baseHandlers := manager.ToolHandlers()

	plannedTools := []llm.ToolCall{}
	plannedHandlers := map[string]llm.ToolHandler{}
	nextIndex := 1

	for _, planned := range plan.FirstIterationTools {
		baseName := strings.TrimSpace(planned.ToolName)
		if baseName == "" {
			continue
		}
		definition, ok := definitionsByName[baseName]
		if !ok {
			return nil, nil, fmt.Errorf("unknown planned tool %q", baseName)
		}
		handler, ok := baseHandlers[baseName]
		if !ok {
			return nil, nil, fmt.Errorf("missing handler for planned tool %q", baseName)
		}

		baseParams, err := normalizePlannedToolParams(planned.ParamsJSON)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid params for planned tool %q: %w", baseName, err)
		}
		paramVariants := []json.RawMessage{baseParams}
		if baseName == "elasticsearch_search" {
			for _, altQuery := range planned.AlternativeQueries {
				altQuery = strings.TrimSpace(altQuery)
				if altQuery == "" {
					continue
				}
				altParams, err := withPlannedElasticsearchQuery(baseParams, altQuery)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid alternative query for planned tool %q: %w", baseName, err)
				}
				paramVariants = append(paramVariants, altParams)
			}
		}

		for _, params := range paramVariants {
			plannedName := llm.MakePlannedReasoningToolName(baseName, nextIndex)
			nextIndex++
			paramsCopy := append(json.RawMessage(nil), params...)
			plannedTools = append(plannedTools, llm.ToolCall{
				Type: "function",
				Function: map[string]interface{}{
					"name":        plannedName,
					"description": buildPlannedToolDescription(definition.Description, paramsCopy),
					"parameters": map[string]interface{}{
						"type":                 "object",
						"properties":           map[string]interface{}{},
						"additionalProperties": false,
					},
				},
			})
			plannedHandlers[plannedName] = func(baseHandler llm.ToolHandler, fixedArgs json.RawMessage) llm.ToolHandler {
				return func(ctx context.Context, _ json.RawMessage) (string, error) {
					return baseHandler(ctx, fixedArgs)
				}
			}(handler, paramsCopy)
		}
	}

	if len(plannedTools) == 0 {
		return nil, nil, nil
	}
	return plannedTools, plannedHandlers, nil
}

func normalizePlannedToolParams(raw string) (json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return json.RawMessage("{}"), nil
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("params must be valid JSON")
	}
	if raw == "null" {
		return json.RawMessage("{}"), nil
	}
	if !strings.HasPrefix(raw, "{") {
		return nil, fmt.Errorf("params must be a JSON object")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func withPlannedElasticsearchQuery(raw json.RawMessage, query string) (json.RawMessage, error) {
	params := map[string]interface{}{}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	params["query"] = query
	updated, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func buildPlannedToolDescription(baseDescription string, params json.RawMessage) string {
	baseDescription = strings.TrimSpace(baseDescription)
	if baseDescription == "" {
		baseDescription = "Run the planned first-step tool with fixed arguments."
	}
	return fmt.Sprintf("%s Predefined fixed arguments: %s", baseDescription, strings.TrimSpace(string(params)))
}

func reasoningSearchOutputLanguageName(uiLanguage string, query string) string {
	languageCode := strings.TrimSpace(uiLanguage)
	if languageCode == "" {
		order := utils.DetectLanguage(query, consts.DEFAULT_UI_LANGUAGE, "", nil)
		if len(order) > 0 {
			languageCode = order[0]
		}
	}

	if tag, ok := utils.MDB_TO_GO[languageCode]; ok {
		return display.English.Tags().Name(tag)
	}
	return display.English.Tags().Name(utils.MDB_TO_GO[consts.DEFAULT_UI_LANGUAGE])
}

func rewriteReasoningSearchQuery(query string) string {
	runes := []rune(query)
	if len(runes) == 0 {
		return query
	}

	var builder strings.Builder
	for i, r := range runes {
		builder.WriteRune(r)
		if !isReasoningSearchRewriteLetter(r) {
			continue
		}
		if i > 0 && isReasoningSearchQueryWordRune(runes[i-1]) {
			continue
		}
		if i+1 < len(runes) && (isReasoningSearchQueryWordRune(runes[i+1]) || isReasoningSearchGeresh(runes[i+1])) {
			continue
		}
		builder.WriteByte('\'')
	}
	return builder.String()
}

func isReasoningSearchRewriteLetter(r rune) bool {
	return r >= 'א' && r <= 'י'
}

func isReasoningSearchQueryWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isReasoningSearchGeresh(r rune) bool {
	return r == '\'' || r == '׳'
}

func isReasoningSearchQuoteMark(r rune) bool {
	return r == '\'' || r == '"' || r == '׳' || r == '״'
}
