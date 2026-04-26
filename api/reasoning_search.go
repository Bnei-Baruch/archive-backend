package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

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
}

type ReasoningSearchCancelRequest struct {
	SessionID string `json:"session_id" form:"session_id"`
}

var (
	errReasoningSearchAlreadyRunning  = errors.New("reasoning search is already running for this session")
	errReasoningSearchResultsNotReady = errors.New("reasoning search results are not ready yet")
	errReasoningSearchFailed          = errors.New("reasoning search failed")
	errReasoningSearchCanceled        = errors.New("reasoning search was canceled")
)

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

	c.JSON(http.StatusAccepted, gin.H{"session_id": sessionID})
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

func ReasoningSearchStatusHandler(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		NewBadRequestError(errors.New("session_id is required")).Abort(c)
		return
	}

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Progress == nil {
		NewInternalError(errors.New("LLM_REASONING_PROGRESS is not initialized")).Abort(c)
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
	if err := executeReasoningSearchForSession(c.Request.Context(), runtime, db, r, sessionID); err != nil {
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
		if status, err := progressStore.Get(sessionID); err == nil {
			if !status.Done {
				return "", errReasoningSearchAlreadyRunning
			}
		} else if !errors.Is(err, llm.ErrReasoningProgressNotFoundOrExpired) {
			return "", err
		}
		if workflowSession.InitialRequestCompleted {
			if workflowSession.FollowupCount >= reasoningStage.MaxFollowups {
				return "", &llm.MaxReasoningFollowupsError{MaxFollowups: reasoningStage.MaxFollowups}
			}
			if err := workflowStore.SetFollowupState(sessionID, true, workflowSession.FollowupCount+1); err != nil {
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

	reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		return "", err
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

func executeReasoningSearchInBackground(ctx context.Context, runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, responseSessionID string) {
	defer func() {
		if runtime != nil && runtime.Cancellations != nil {
			runtime.Cancellations.Delete(responseSessionID)
		}
		if recovered := recover(); recovered != nil {
			log.Errorf("Reasoning Search background panic session=%s: %v", responseSessionID, recovered)
			if runtime != nil && runtime.Workflow != nil {
				if err := runtime.Workflow.SetResponseSnapshot(responseSessionID, nil); err != nil && !errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
					log.Warnf("Reasoning Search failed clearing response snapshot after panic: %v", err)
				}
			}
			if runtime != nil && runtime.Progress != nil {
				runtime.Progress.Fail(responseSessionID, 0)
			}
		}
	}()
	if err := executeReasoningSearchForSession(ctx, runtime, db, r, responseSessionID); err != nil {
		if isReasoningSearchCancellation(err) {
			log.Infof("Reasoning Search background canceled session=%s", responseSessionID)
			return
		}
		log.Warnf("Reasoning Search background execution failed session=%s: %v", responseSessionID, err)
	}
}

// Runs the full reasoning workflow and stores the final per-session response
// snapshot. The shared query cache may seed an initial result, but the final
// API payload is always stored per workflow session.
func executeReasoningSearchForSession(ctx context.Context, runtime *llm.Runtime, db *sql.DB, r ReasoningSearchRequest, responseSessionID string) (err error) {
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
	reasoningIterations := 0
	defer func() {
		if err == nil {
			return
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
	providerID := strings.TrimSpace(reasoningStage.ProviderSessionID)
	providerSessionID := &providerID
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
			cachedEntry.Query = r.Query
			if err := workflowStore.SetCachedInitialResponse(responseSessionID, cachedEntry); err != nil {
				return err
			}
			if err := workflowStore.SetFollowupState(responseSessionID, true, 0); err != nil {
				return err
			}

			response.Query = r.Query
			response.Summary = cachedEntry.Summary
			response.CacheHit = true
			response.SetUsedTools([]string{})
			response.Results = append([]llm.ReasoningSearchResult(nil), cachedEntry.Results...)
			response.SetSessionID(responseSessionID)
			if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
				return err
			}
			response.SetFollowupBudget(reasoningStage.MaxFollowups, 0, reasoningStage.MaxFollowups)
			if err := workflowStore.SetResponseSnapshot(responseSessionID, &response); err != nil {
				return err
			}
			progressStore.Complete(responseSessionID, 0)
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

	resolvedProviderSessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(ctx,
		providerSessionID,
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
	reasoningIterations = response.ReasoningIterations
	reasoningStage.ProviderSessionID = resolvedProviderSessionID
	if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, reasoningStage); err != nil {
		return err
	}
	if usedCachedInitialResponse {
		if err := workflowStore.SetCachedInitialResponse(responseSessionID, nil); err != nil {
			return err
		}
	}
	response.SetSessionID(responseSessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		return err
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
				Query            string                      `json:"query"`
				Summary          string                      `json:"summary"`
				ReasoningSummary string                      `json:"reasoning_summary,omitempty"`
				Results          []llm.ReasoningSearchResult `json:"results"`
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
							rerunResponse := llm.ReasoningSearchResponse{}
							rerunProgressOffset := progressCompleteIteration
							progressStore.SetIterationOffset(responseSessionID, rerunProgressOffset)
							resolvedProviderSessionID, err = service.GetReasoningStructuredOutputWithToolsForSession(ctx,
								&nextProviderSessionID,
								&progressSessionID,
								responseSchema,
								reasoningStage.Model,
								&reasoningStage.MaxTokens,
								rerunMessages,
								manager.ToolCalls(),
								manager.ToolHandlers(),
								nil,
								nil,
								&promptCacheKey,
								&reasoningStage.ReasoningEffort,
								r.Deb,
								reasoningStage.RerunMaxIterations,
								&rerunResponse,
							)
							progressStore.SetIterationOffset(responseSessionID, 0)
							if err != nil {
								if isReasoningSearchCancellation(err) {
									return err
								}
								log.Warnf("Reasoning Search rerun after verification failed: %v", err)
								response = originalResponse
							} else {
								reasoningStage.ProviderSessionID = resolvedProviderSessionID
								if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, reasoningStage); err != nil {
									log.Warnf("Reasoning Search failed to persist rerun session state: %v", err)
									response = originalResponse
								} else {
									rerunResponse.SetSessionID(responseSessionID)
									if err := enrichReasoningSearchResults(db, r.UILanguage, rerunResponse.Results); err != nil {
										log.Warnf("Reasoning Search failed to enrich rerun results: %v", err)
										response = originalResponse
									} else {
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
