package api

import (
	"errors"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

type ReasoningSearchRequest struct {
	SessionID *string `json:"session_id" form:"session_id"`
	Query     string  `json:"q" form:"q" binding:"required"`
	Deb       bool    `json:"deb" form:"deb" binding:"omitempty"`
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
	reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	messages := []llm.LLMBotMessage{
		{
			Role:    "system",
			Content: llm.GenerateSystemMessageForReasoningSearch(manager.Tools()),
		},
		{
			Role:    "user",
			Content: r.Query,
		},
	}

	user := "archive-backend-reasoning-search"
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
		&user,
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
		NewInternalError(err).Abort(c)
		return
	}
	response.SetSessionID(sessionID)

	c.JSON(http.StatusOK, response)
}
