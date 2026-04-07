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

	log.Infof("Reasoning Search Query: [%s]", r.Query)

	sessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		r.SessionID,
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
	response.SetSessionID(sessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusOK, response)
}
