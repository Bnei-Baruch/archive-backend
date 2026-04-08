package api

import (
	"database/sql"
	"encoding/json"
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
	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Tools == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	service := runtime.Services[reasoningConfig.Provider]
	if service == nil {
		NewInternalError(errors.New("reasoning llm service is not initialized")).Abort(c)
		return
	}

	providerSessionID, err := service.ReserveReasoningSession(reasoningConfig.Model, &reasoningConfig.Effort)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	sessionID, err := runtime.Workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:           reasoningConfig.Provider,
		Model:              reasoningConfig.Model,
		ReasoningEffort:    reasoningConfig.Effort,
		MaxTokens:          reasoningConfig.MaxTokens,
		MaxIterations:      reasoningConfig.MaxIterations,
		RerunMaxIterations: reasoningConfig.RerunMaxIterations,
		ProviderSessionID:  providerSessionID,
	})
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	if reasoningConfig.Verification != nil {
		// Verification is a one-shot structured call today, so only stage metadata is stored.
		if err := runtime.Workflow.SetStage(sessionID, llm.ReasoningWorkflowStageVerification, llm.ReasoningWorkflowStageSession{
			Provider:        reasoningConfig.Verification.Provider,
			Model:           reasoningConfig.Verification.Model,
			ReasoningEffort: reasoningConfig.Verification.Effort,
			MaxTokens:       reasoningConfig.Verification.MaxTokens,
		}); err != nil {
			NewInternalError(err).Abort(c)
			return
		}
	}
	runtime.Progress.Reserve(sessionID)

	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
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

	runtime, _ := c.MustGet("LLM_RUNTIME").(*llm.Runtime)
	if runtime == nil || runtime.Tools == nil || runtime.Progress == nil || runtime.Workflow == nil {
		NewInternalError(errors.New("reasoning workflow is not initialized")).Abort(c)
		return
	}
	manager := runtime.Tools
	if manager == nil {
		NewInternalError(errors.New("LLM_TOOLS is not initialized")).Abort(c)
		return
	}

	db := c.MustGet("MDB_DB").(*sql.DB)
	workflowStore := runtime.Workflow
	progressStore := runtime.Progress
	response := llm.ReasoningSearchResponse{}
	responseSessionID := ""
	var reasoningStage llm.ReasoningWorkflowStageSession
	var verificationStage *llm.ReasoningWorkflowStageSession
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
		reasoningStage = stage
		if stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageVerification]; ok {
			stageCopy := stage
			verificationStage = &stageCopy
		}
		responseSessionID = workflowSession.ID
		providerID := strings.TrimSpace(stage.ProviderSessionID)
		providerSessionID = &providerID
		progressID := workflowSession.ID
		progressSessionID = &progressID
	} else {
		reasoningConfig, err := llm.ReasoningSearchConfigFromConfig()
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		service := runtime.Services[reasoningConfig.Provider]
		if service == nil {
			NewInternalError(errors.New("reasoning llm service is not initialized")).Abort(c)
			return
		}
		reservedProviderSessionID, err := service.ReserveReasoningSession(reasoningConfig.Model, &reasoningConfig.Effort)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		reasoningStage = llm.ReasoningWorkflowStageSession{
			Provider:           reasoningConfig.Provider,
			Model:              reasoningConfig.Model,
			ReasoningEffort:    reasoningConfig.Effort,
			MaxTokens:          reasoningConfig.MaxTokens,
			MaxIterations:      reasoningConfig.MaxIterations,
			RerunMaxIterations: reasoningConfig.RerunMaxIterations,
			ProviderSessionID:  reservedProviderSessionID,
		}
		responseSessionID, err = workflowStore.Create(llm.ReasoningWorkflowStageReasoning, reasoningStage)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		if reasoningConfig.Verification != nil {
			stage := llm.ReasoningWorkflowStageSession{
				Provider:        reasoningConfig.Verification.Provider,
				Model:           reasoningConfig.Verification.Model,
				ReasoningEffort: reasoningConfig.Verification.Effort,
				MaxTokens:       reasoningConfig.Verification.MaxTokens,
			}
			if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageVerification, stage); err != nil {
				NewInternalError(err).Abort(c)
				return
			}
			verificationStage = &stage
		}
		progressStore.Reserve(responseSessionID)
		providerID := reservedProviderSessionID
		providerSessionID = &providerID
		progressID := responseSessionID
		progressSessionID = &progressID
	}

	service := runtime.Services[reasoningStage.Provider]
	if service == nil {
		NewInternalError(errors.New("reasoning llm service is not initialized")).Abort(c)
		return
	}

	systemMessage := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), reasoningStage.MaxIterations)
	messages := []llm.LLMBotMessage{
		{
			Role:    "system",
			Content: systemMessage,
		},
		{
			Role:    "user",
			Content: r.Query,
		},
	}

	promptCacheKey := fmt.Sprintf(
		"reasoning-search:m=%s:e=%s",
		reasoningStage.Model,
		reasoningStage.ReasoningEffort,
	)

	log.Infof("Reasoning Search Query: [%s]", r.Query)

	resolvedProviderSessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		providerSessionID,
		progressSessionID,
		llm.GenerateReasoningSearchResponseJSONSchema(),
		reasoningStage.Model,
		&reasoningStage.MaxTokens,
		messages,
		manager.ToolCalls(),
		manager.ToolHandlers(),
		&promptCacheKey,
		&reasoningStage.ReasoningEffort,
		r.Deb,
		reasoningStage.MaxIterations,
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
	reasoningStage.ProviderSessionID = resolvedProviderSessionID
	if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, reasoningStage); err != nil {
		progressStore.Fail(responseSessionID, response.ReasoningIterations)
		NewInternalError(err).Abort(c)
		return
	}
	response.SetSessionID(responseSessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		progressStore.Fail(responseSessionID, response.ReasoningIterations)
		NewInternalError(err).Abort(c)
		return
	}
	if verificationStage != nil {
		verificationService := runtime.Services[verificationStage.Provider]
		if verificationService == nil {
			log.Warnf("Reasoning Search verification skipped: service for provider %q is not initialized", verificationStage.Provider)
		} else {
			progressStore.Verifying(responseSessionID, response.ReasoningIterations)

			verificationPromptCacheKey := fmt.Sprintf(
				"reasoning-search-verification:m=%s:e=%s",
				verificationStage.Model,
				verificationStage.ReasoningEffort,
			)
			verificationInput, err := json.Marshal(struct {
				Query   string                      `json:"query"`
				Summary string                      `json:"summary"`
				Results []llm.ReasoningSearchResult `json:"results"`
			}{
				Query:   r.Query,
				Summary: response.Summary,
				Results: response.Results,
			})
			if err != nil {
				log.Warnf("Reasoning Search verification failed to build input: %v", err)
			} else {
				verificationMessages := []llm.LLMBotMessage{
					{
						Role:    "system",
						Content: llm.ReasoningSearchVerificationInstruction,
					},
					{
						Role:    "user",
						Content: string(verificationInput),
					},
				}
				verificationResponse := llm.ReasoningSearchVerificationResponse{}
				verificationDebug, err := verificationService.GetStructuredOutputWithDebug(
					llm.GenerateReasoningSearchVerificationResponseJSONSchema(),
					verificationStage.Model,
					&verificationStage.MaxTokens,
					verificationMessages,
					&verificationPromptCacheKey,
					&verificationStage.ReasoningEffort,
					&verificationResponse,
				)
				if err != nil {
					log.Warnf("Reasoning Search verification failed: %v", err)
				} else {
					if r.Deb {
						verificationOutput := verificationResponse
						response.VerificationOutput = &verificationOutput
					}
					initialUsedTokens := response.UsedTokens
					initialReasoningIterations := response.ReasoningIterations
					initialUsedTools := append([]string(nil), response.UsedTools...)
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
									Content: "Verification feedback:\n" + recommendation,
								},
							}
							nextProviderSessionID := resolvedProviderSessionID
							rerunResponse := llm.ReasoningSearchResponse{}
							resolvedProviderSessionID, err = service.GetReasoningStructuredOutputWithToolsForSession(
								&nextProviderSessionID,
								progressSessionID,
								llm.GenerateReasoningSearchResponseJSONSchema(),
								reasoningStage.Model,
								&reasoningStage.MaxTokens,
								rerunMessages,
								manager.ToolCalls(),
								manager.ToolHandlers(),
								&promptCacheKey,
								&reasoningStage.ReasoningEffort,
								r.Deb,
								reasoningStage.RerunMaxIterations,
								&rerunResponse,
							)
							if err != nil {
								log.Warnf("Reasoning Search rerun after verification failed: %v", err)
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
								reasoningStage.ProviderSessionID = resolvedProviderSessionID
								if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStageReasoning, reasoningStage); err != nil {
									log.Warnf("Reasoning Search failed to persist rerun session state: %v", err)
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
									rerunResponse.SetSessionID(responseSessionID)
									if err := enrichReasoningSearchResults(db, r.UILanguage, rerunResponse.Results); err != nil {
										log.Warnf("Reasoning Search failed to enrich rerun results: %v", err)
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
										if rerunResponse.Debug != nil {
											if initialDebug != nil {
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

	progressStore.Complete(responseSessionID, response.ReasoningIterations)

	c.JSON(http.StatusOK, response)
}
