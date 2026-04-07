package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

type ReasoningSearchRequest struct {
	SessionID  *string `json:"session_id" form:"session_id"`
	Query      string  `json:"q" form:"q" binding:"required"`
	Deb        bool    `json:"deb" form:"deb" binding:"omitempty"`
	UILanguage string  `json:"ui_language" form:"ui_language" binding:"omitempty,len=2"`
}

func ReasoningSearchStartHandler(c *gin.Context) {
	service := c.MustGet("LLM_SERVICE").(llm.Service)
	progressStore, _ := c.MustGet("LLM_REASONING_PROGRESS").(*llm.ReasoningProgressStore)
	workflowStore, _ := c.MustGet("LLM_REASONING_WORKFLOW").(*llm.ReasoningWorkflowSessionStore)
	if progressStore == nil || workflowStore == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	providerSessionID, err := service.ReserveReasoningSession(reasoningConfig.Model, &reasoningConfig.Effort)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	sessionID, err := workflowStore.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          llm.ProviderFromConfig(),
		Model:             reasoningConfig.Model,
		ReasoningEffort:   reasoningConfig.Effort,
		ProviderSessionID: providerSessionID,
	})
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	progressStore.Reserve(sessionID)

	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}

func ReasoningSearchStatusHandler(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		NewBadRequestError(errors.New("session_id is required")).Abort(c)
		return
	}

	progressStore, _ := c.MustGet("LLM_REASONING_PROGRESS").(*llm.ReasoningProgressStore)
	if progressStore == nil {
		NewInternalError(errors.New("LLM_REASONING_PROGRESS is not initialized")).Abort(c)
		return
	}

	status, err := progressStore.Get(sessionID)
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

func ReasoningSearchHandler(c *gin.Context) {
	r := ReasoningSearchRequest{}
	if c.Bind(&r) != nil {
		return
	}

	r.Query = strings.TrimSpace(r.Query)
	if r.Query == "" {
		NewBadRequestError(errors.New("q is required")).Abort(c)
		return
	}
	r.UILanguage = strings.ToLower(strings.TrimSpace(r.UILanguage))
	if r.SessionID != nil {
		trimmedSessionID := strings.TrimSpace(*r.SessionID)
		if trimmedSessionID == "" {
			NewBadRequestError(errors.New("session_id cannot be empty")).Abort(c)
			return
		}
		r.SessionID = &trimmedSessionID
	}

	manager := c.MustGet("LLM_TOOLS").(*llm.ReasoningToolManager)
	if manager == nil {
		NewInternalError(errors.New("LLM_TOOLS is not initialized")).Abort(c)
		return
	}

	service := c.MustGet("LLM_SERVICE").(llm.Service)
	db := c.MustGet("MDB_DB").(*sql.DB)
	workflowStore, _ := c.MustGet("LLM_REASONING_WORKFLOW").(*llm.ReasoningWorkflowSessionStore)
	progressStore, _ := c.MustGet("LLM_REASONING_PROGRESS").(*llm.ReasoningProgressStore)
	if workflowStore == nil || progressStore == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	messages := []llm.LLMBotMessage{
		{
			Role:    "system",
			Content: llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), reasoningConfig.MaxIterations),
		},
		{
			Role:    "user",
			Content: r.Query,
		},
	}

	promptCacheKey := fmt.Sprintf(
		"reasoning-search:m=%s:e=%s",
		reasoningConfig.Model,
		reasoningConfig.Effort,
	)
	response := llm.ReasoningSearchResponse{}
	responseSessionID := ""
	var providerSessionID *string
	var progressSessionID *string

	if r.SessionID != nil {
		workflowSession, err := workflowStore.Get(*r.SessionID)
		if err != nil {
			if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
				NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
				return
			}
			NewInternalError(err).Abort(c)
			return
		}
		stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageReasoning]
		if !ok || strings.TrimSpace(stage.ProviderSessionID) == "" {
			NewHttpError(http.StatusNotFound, llm.ErrReasoningSessionNotFoundOrExpired, gin.ErrorTypePublic).Abort(c)
			return
		}
		if stage.Provider != llm.ProviderFromConfig() {
			NewHttpError(http.StatusConflict, errors.New("reasoning session provider does not match configured provider"), gin.ErrorTypePublic).Abort(c)
			return
		}
		responseSessionID = workflowSession.ID
		providerID := strings.TrimSpace(stage.ProviderSessionID)
		providerSessionID = &providerID
		progressID := workflowSession.ID
		progressSessionID = &progressID
	} else {
		reservedProviderSessionID, err := service.ReserveReasoningSession(reasoningConfig.Model, &reasoningConfig.Effort)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		responseSessionID, err = workflowStore.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
			Provider:          llm.ProviderFromConfig(),
			Model:             reasoningConfig.Model,
			ReasoningEffort:   reasoningConfig.Effort,
			ProviderSessionID: reservedProviderSessionID,
		})
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		progressStore.Reserve(responseSessionID)
		providerID := reservedProviderSessionID
		providerSessionID = &providerID
		progressID := responseSessionID
		progressSessionID = &progressID
	}

	log.Infof("Reasoning Search Query: [%s]", r.Query)

	resolvedProviderSessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		providerSessionID,
		progressSessionID,
		llm.GenerateReasoningSearchResponseJSONSchema(),
		reasoningConfig.Model,
		&reasoningConfig.MaxTokens,
		messages,
		manager.ToolCalls(),
		manager.ToolHandlers(),
		&promptCacheKey,
		&reasoningConfig.Effort,
		r.Deb,
		reasoningConfig.MaxIterations,
		&response,
	)
	if err != nil {
		if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
			NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
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
	if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          llm.ProviderFromConfig(),
		Model:             reasoningConfig.Model,
		ReasoningEffort:   reasoningConfig.Effort,
		ProviderSessionID: resolvedProviderSessionID,
	}); err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	response.SetSessionID(responseSessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusOK, response)
}
