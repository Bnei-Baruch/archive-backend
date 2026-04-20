package api

import (
	"context"
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
		MaxFollowups:       reasoningConfig.MaxFollowups,
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
	if reasoningConfig.Planning != nil {
		if err := runtime.Workflow.SetStage(sessionID, llm.ReasoningWorkflowStagePlanning, llm.ReasoningWorkflowStageSession{
			Provider:        reasoningConfig.Planning.Provider,
			Model:           reasoningConfig.Planning.Model,
			ReasoningEffort: reasoningConfig.Planning.Effort,
			MaxTokens:       reasoningConfig.Planning.MaxTokens,
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
	var planningStage *llm.ReasoningWorkflowStageSession
	var verificationStage *llm.ReasoningWorkflowStageSession
	var providerSessionID *string
	var progressSessionID *string
	followupsUsed := 0
	followupsRemaining := 0
	initialRequestCompleted := false
	var cachedInitialResponse *llm.ReasoningSearchCacheEntry
	cacheKey := ""
	cacheEligible := false

	if r.SessionID == nil && runtime.ReasoningCache != nil && !r.Deb {
		cacheKey, cacheEligible = llm.ReasoningSearchCacheKeyForQuery(r.Query)
	}

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
		if stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStagePlanning]; ok {
			stageCopy := stage
			planningStage = &stageCopy
		}
		if stage, ok := workflowSession.Stages[llm.ReasoningWorkflowStageVerification]; ok {
			stageCopy := stage
			verificationStage = &stageCopy
		}
		initialRequestCompleted = workflowSession.InitialRequestCompleted
		cachedInitialResponse = workflowSession.CachedInitialResponse
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
			MaxFollowups:       reasoningConfig.MaxFollowups,
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
		if reasoningConfig.Planning != nil {
			stage := llm.ReasoningWorkflowStageSession{
				Provider:        reasoningConfig.Planning.Provider,
				Model:           reasoningConfig.Planning.Model,
				ReasoningEffort: reasoningConfig.Planning.Effort,
				MaxTokens:       reasoningConfig.Planning.MaxTokens,
			}
			if err := workflowStore.SetStage(responseSessionID, llm.ReasoningWorkflowStagePlanning, stage); err != nil {
				NewInternalError(err).Abort(c)
				return
			}
			planningStage = &stage
		}
		progressStore.Reserve(responseSessionID)
		providerID := reservedProviderSessionID
		providerSessionID = &providerID
		progressID := responseSessionID
		progressSessionID = &progressID

		if cacheEligible {
			if cachedEntry, ok := runtime.ReasoningCache.Get(cacheKey); ok {
				cachedEntry.Query = r.Query
				if err := workflowStore.SetCachedInitialResponse(responseSessionID, cachedEntry); err != nil {
					NewInternalError(err).Abort(c)
					return
				}
				if err := workflowStore.SetFollowupState(responseSessionID, true, 0); err != nil {
					NewInternalError(err).Abort(c)
					return
				}

				response.Query = r.Query
				response.Summary = cachedEntry.Summary
				response.CacheHit = true
				response.SetUsedTools([]string{})
				response.Results = append([]llm.ReasoningSearchResult(nil), cachedEntry.Results...)
				response.SetSessionID(responseSessionID)
				if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
					progressStore.Fail(responseSessionID, response.ReasoningIterations)
					NewInternalError(err).Abort(c)
					return
				}
				response.SetFollowupBudget(reasoningStage.MaxFollowups, 0, reasoningStage.MaxFollowups)
				progressStore.Complete(responseSessionID, 0)
				log.Infof("Reasoning Search Cache Hit: [%s]", cacheKey)
				c.JSON(http.StatusOK, response)
				return
			}
		}
	}

	if r.SessionID != nil {
		if initialRequestCompleted {
			if workflowSession, err := workflowStore.Get(responseSessionID); err != nil {
				if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
					NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
					return
				}
				NewInternalError(err).Abort(c)
				return
			} else {
				if workflowSession.FollowupCount >= reasoningStage.MaxFollowups {
					NewHttpError(http.StatusUnprocessableEntity, &llm.MaxReasoningFollowupsError{MaxFollowups: reasoningStage.MaxFollowups}, gin.ErrorTypePublic).Abort(c)
					return
				}
				followupsUsed = workflowSession.FollowupCount + 1
				followupsRemaining = reasoningStage.MaxFollowups - followupsUsed
				if err := workflowStore.SetFollowupState(responseSessionID, true, followupsUsed); err != nil {
					if errors.Is(err, llm.ErrReasoningSessionNotFoundOrExpired) {
						NewHttpError(http.StatusNotFound, err, gin.ErrorTypePublic).Abort(c)
						return
					}
					NewInternalError(err).Abort(c)
					return
				}
			}
		} else {
			followupsUsed = 0
			followupsRemaining = reasoningStage.MaxFollowups
		}
	} else {
		followupsUsed = 0
		followupsRemaining = reasoningStage.MaxFollowups
	}

	service := runtime.Services[reasoningStage.Provider]
	if service == nil {
		NewInternalError(errors.New("reasoning llm service is not initialized")).Abort(c)
		return
	}

	systemMessage := llm.GenerateSystemMessageForReasoningSearch(manager.Tools(), reasoningStage.MaxIterations, followupsRemaining)
	var planningDebug *llm.ReasoningSearchDebugInfo
	var firstIterationTools []llm.ToolCall
	var firstIterationToolHandlers map[string]llm.ToolHandler
	if planningStage != nil && !initialRequestCompleted {
		planningService := runtime.Services[planningStage.Provider]
		if planningService == nil {
			log.Warnf("Reasoning Search planning skipped: service for provider %q is not initialized", planningStage.Provider)
		} else {
			progressStore.Planning(responseSessionID)
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
			debugInfo, err := planningService.GetStructuredOutputWithDebugInfo(
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

	resolvedProviderSessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		providerSessionID,
		progressSessionID,
		llm.GenerateReasoningSearchResponseJSONSchema(),
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
	if usedCachedInitialResponse {
		if err := workflowStore.SetCachedInitialResponse(responseSessionID, nil); err != nil {
			progressStore.Fail(responseSessionID, response.ReasoningIterations)
			NewInternalError(err).Abort(c)
			return
		}
	}
	response.SetSessionID(responseSessionID)
	if err := enrichReasoningSearchResults(db, r.UILanguage, response.Results); err != nil {
		progressStore.Fail(responseSessionID, response.ReasoningIterations)
		NewInternalError(err).Abort(c)
		return
	}
	if planningDebug != nil {
		response.UsedTokens += planningDebug.TotalTokens
		if response.Debug != nil {
			response.Debug.PlanningModelUsage = planningDebug.UsageBreakdown()
			response.Debug.Add(planningDebug)
		}
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
						Content: fmt.Sprintf(llm.ReasoningSearchVerificationInstructionMask, llm.GeneralReasoningSearchInstruction),
					},
					{
						Role:    "user",
						Content: string(verificationInput),
					},
				}
				verificationResponse := llm.ReasoningSearchVerificationResponse{}
				verificationDebug, err := verificationService.GetStructuredOutputWithDebugInfo(
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
					log.Warnf("Reasoning Search verification failed: %v", err)
				} else {
					// Keep a clean fallback in case verification asks for a rerun and
					// that repair pass fails. In that case we want to return the
					// original first-pass results, not a partially annotated response.
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
								nil,
								nil,
								&promptCacheKey,
								&reasoningStage.ReasoningEffort,
								r.Deb,
								reasoningStage.RerunMaxIterations,
								&rerunResponse,
							)
							if err != nil {
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
		// We set the followup state when the initial request is completed,
		// the initial request itself is not counted as a followup.
		if err := workflowStore.SetFollowupState(responseSessionID, true, followupsUsed); err != nil {
			progressStore.Fail(responseSessionID, response.ReasoningIterations)
			NewInternalError(err).Abort(c)
			return
		}
	}

	response.SetFollowupBudget(reasoningStage.MaxFollowups, followupsUsed, followupsRemaining)
	progressStore.Complete(responseSessionID, response.ReasoningIterations)
	if cacheEligible && runtime.ReasoningCache != nil {
		runtime.ReasoningCache.Set(cacheKey, llm.BuildReasoningSearchCacheEntryFromResponse(&response))
	}

	c.JSON(http.StatusOK, response)
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
