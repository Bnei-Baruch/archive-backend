package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const defaultOpenRouterAPIBaseURL = "https://openrouter.ai/api/v1"
const openRouterResultsReadyToolName = "results_ready"

type OpenRouterService struct {
	*OpenAICompatibleAPIService
	sessions                                 *ChatReasoningSessionStore
	providerPreferences                      *ResponsesProvider
	reasoningSearchProviderPreferences       *ResponsesProvider
	reasoningSearchPlanningProviderPrefs     *ResponsesProvider
	reasoningSearchVerificationProviderPrefs *ResponsesProvider
	aiToolsProviderPreferences               *ResponsesProvider
	requiredToolIterations                   int
}

var _ Service = (*OpenRouterService)(nil)

func NewOpenRouterService(token string) *OpenRouterService {
	return NewOpenRouterServiceWithOptions(token, nil, nil, "")
}

func NewOpenRouterServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *OpenRouterService {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = defaultOpenRouterAPIBaseURL
	}

	return &OpenRouterService{
		OpenAICompatibleAPIService: newOpenAICompatibleAPIServiceWithOptions(token, pricing, nil, apiBaseURL),
		sessions:                   sessions,
		requiredToolIterations:     1,
	}
}

func (s *OpenRouterService) reasoningSearchProviderPrefs() *ResponsesProvider {
	if s.reasoningSearchProviderPreferences != nil {
		return s.reasoningSearchProviderPreferences
	}
	return s.providerPreferences
}

func (s *OpenRouterService) structuredOutputProviderPreferences(promptCacheKey *string) *ResponsesProvider {
	key := ""
	if promptCacheKey != nil {
		key = strings.TrimSpace(*promptCacheKey)
	}
	switch {
	case strings.HasPrefix(key, "reasoning-search-planning:"):
		if s.reasoningSearchPlanningProviderPrefs != nil {
			return s.reasoningSearchPlanningProviderPrefs
		}
	case strings.HasPrefix(key, "reasoning-search-verification:"):
		if s.reasoningSearchVerificationProviderPrefs != nil {
			return s.reasoningSearchVerificationProviderPrefs
		}
	default:
		if s.aiToolsProviderPreferences != nil {
			return s.aiToolsProviderPreferences
		}
	}
	return s.providerPreferences
}

func openRouterWithResultsReadyTool(tools []ToolCall, handlers map[string]ToolHandler) ([]ToolCall, map[string]ToolHandler) {
	retTools := append([]ToolCall(nil), tools...)
	if !openRouterHasTool(retTools, openRouterResultsReadyToolName) {
		retTools = append(retTools, openRouterResultsReadyToolCall())
	}

	retHandlers := make(map[string]ToolHandler, len(handlers)+1)
	for name, handler := range handlers {
		retHandlers[name] = handler
	}
	retHandlers[openRouterResultsReadyToolName] = openRouterResultsReadyHandler
	return retTools, retHandlers
}

func openRouterHasTool(tools []ToolCall, name string) bool {
	for _, tool := range tools {
		function, ok := tool.Function.(map[string]interface{})
		if !ok {
			continue
		}
		if toolCallStringField(function, "name") == name {
			return true
		}
	}
	return false
}

func openRouterResultsReadyToolCall() ToolCall {
	return ToolCall{
		Type: "function",
		Function: map[string]interface{}{
			"name":        openRouterResultsReadyToolName,
			"description": "Call this internal tool only when you already have enough information and are ready to return the final structured response. It performs no search.",
			"parameters": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Short reason why no more search tools are needed.",
					},
				},
				"required": []string{"reason"},
			},
		},
	}
}

func openRouterResultsReadyHandler(ctx context.Context, arguments json.RawMessage) (string, error) {
	return `{"ok":true,"message":"No more tool calls are needed. Return the final structured response now."}`, nil
}

func (s *OpenRouterService) GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, output interface{}) error {
	msg, _, usageTotals, err := s.getStructuredOutputWithUsage(model, maxTokens, messages, promptCacheKey, jsonSchema, reasoningEffort, s.structuredOutputProviderPreferences(promptCacheKey), false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenRouter GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	return nil
}

func (s *OpenRouterService) GetStructuredOutputWithDebugInfo(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, reasoningSummary, usageTotals, err := s.getStructuredOutputWithUsage(model, maxTokens, messages, promptCacheKey, jsonSchema, reasoningEffort, s.structuredOutputProviderPreferences(promptCacheKey), debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenRouter GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	debugInfo := s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals)
	debugInfo.ReasoningSummary = reasoningSummary
	return debugInfo, nil
}

func (s *OpenRouterService) GetReasoningResponseWithTools(
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	promptCacheKey *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
) (*LLMBotMessage, error) {
	msg, _, _, _, _, _, _, err := s.getReasoningResponseWithTools("GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, nil, nil, promptCacheKey, reasoningEffort, deb, maxIterations, "")
	return msg, err
}

func (s *OpenRouterService) GetReasoningStructuredOutputWithTools(
	jsonSchema string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	promptCacheKey *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	output interface{},
) error {
	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, _, toolDebug, err := s.getReasoningResponseWithTools("GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, nil, nil, promptCacheKey, reasoningEffort, deb, maxIterations, "")
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	if deb && reasoningSummary != "" {
		if setter, ok := output.(reasoningSummarySetter); ok {
			setter.SetReasoningSummary(reasoningSummary)
		}
	}
	totalTokens := usageTotals.TotalTokens
	if toolDebug != nil {
		totalTokens += toolDebug.TotalTokens
	}
	if setter, ok := output.(reasoningProcessStatsSetter); ok {
		setter.SetReasoningProcessStats(totalTokens, reasoningIterations)
	}
	if setter, ok := output.(reasoningUsedToolsSetter); ok {
		setter.SetUsedTools(usedTools)
	}
	if deb {
		if setter, ok := output.(reasoningDebugInfoSetter); ok {
			debugInfo := s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals)
			debugInfo.MainModelUsage = debugInfo.UsageBreakdown()
			debugInfo.AIToolsUsage = toolDebug.UsageBreakdown()
			debugInfo.Add(toolDebug)
			setter.SetReasoningDebugInfo(debugInfo)
		}
	}
	return nil
}

func (s *OpenRouterService) GetReasoningStructuredOutputWithToolsForSession(
	sessionID *string,
	progressSessionID *string,
	jsonSchema string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	firstIterationTools []ToolCall,
	firstIterationToolHandlers map[string]ToolHandler,
	promptCacheKey *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	output interface{},
) (string, error) {
	if s.sessions == nil {
		return "", errors.New("reasoning sessions are not enabled")
	}

	effectiveSessionID := ""
	effectiveProgressSessionID := ""
	effectiveModel := model
	effectiveReasoningEffort := reasoningEffort
	effectiveMessages := append([]LLMBotMessage(nil), messages...)
	priorHistory := []LLMBotMessage{}

	if sessionID != nil && strings.TrimSpace(*sessionID) != "" {
		session, err := s.sessions.Get(strings.TrimSpace(*sessionID))
		if err != nil {
			return "", err
		}
		instructions, currentConversation, err := splitInstructionsAndConversation(messages)
		if err != nil {
			return "", err
		}
		effectiveSessionID = session.ID
		effectiveModel = session.Model
		effectiveReasoningEffort = nil
		if session.ReasoningEffort != "" {
			effort := session.ReasoningEffort
			effectiveReasoningEffort = &effort
		}
		effectiveMessages = []LLMBotMessage{{Role: "system", Content: instructions}}
		if len(session.History) > 0 {
			effectiveMessages = append(effectiveMessages, session.History...)
			priorHistory = append(priorHistory, session.History...)
		}
		effectiveMessages = append(effectiveMessages, currentConversation...)
	}
	if progressSessionID != nil && strings.TrimSpace(*progressSessionID) != "" {
		effectiveProgressSessionID = strings.TrimSpace(*progressSessionID)
	}

	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, _, toolDebug, err := s.getReasoningResponseWithTools(
		"GetReasoningStructuredOutputWithToolsForSession",
		&jsonSchema,
		effectiveModel,
		maxTokens,
		effectiveMessages,
		tools,
		toolHandlers,
		firstIterationTools,
		firstIterationToolHandlers,
		promptCacheKey,
		effectiveReasoningEffort,
		deb,
		maxIterations,
		effectiveProgressSessionID,
	)
	if err != nil {
		if s.progress != nil && effectiveProgressSessionID != "" {
			s.progress.Fail(effectiveProgressSessionID, reasoningIterations)
		}
		return "", err
	}

	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		if s.progress != nil && effectiveProgressSessionID != "" {
			s.progress.Fail(effectiveProgressSessionID, reasoningIterations)
		}
		return "", err
	}

	if deb && reasoningSummary != "" {
		if setter, ok := output.(reasoningSummarySetter); ok {
			setter.SetReasoningSummary(reasoningSummary)
		}
	}
	totalTokens := usageTotals.TotalTokens
	if toolDebug != nil {
		totalTokens += toolDebug.TotalTokens
	}
	if setter, ok := output.(reasoningProcessStatsSetter); ok {
		setter.SetReasoningProcessStats(totalTokens, reasoningIterations)
	}
	if setter, ok := output.(reasoningUsedToolsSetter); ok {
		setter.SetUsedTools(usedTools)
	}
	if deb {
		if setter, ok := output.(reasoningDebugInfoSetter); ok {
			debugInfo := s.buildReasoningDebugInfo(effectiveModel, effectiveReasoningEffort, usageTotals)
			debugInfo.MainModelUsage = debugInfo.UsageBreakdown()
			debugInfo.AIToolsUsage = toolDebug.UsageBreakdown()
			debugInfo.Add(toolDebug)
			setter.SetReasoningDebugInfo(debugInfo)
		}
	}

	_, currentConversation, err := splitInstructionsAndConversation(messages)
	if err != nil {
		return "", err
	}
	newHistory := append(priorHistory, sessionConversationMessages(currentConversation)...)
	newHistory = append(newHistory, *msg)

	if effectiveSessionID == "" {
		storedReasoningEffort := ""
		if effectiveReasoningEffort != nil {
			storedReasoningEffort = *effectiveReasoningEffort
		}
		effectiveSessionID, err = s.sessions.Create(newHistory, effectiveModel, storedReasoningEffort)
		if err != nil {
			return "", err
		}
	} else {
		if err := s.sessions.Update(effectiveSessionID, newHistory); err != nil {
			if s.progress != nil && effectiveProgressSessionID != "" {
				s.progress.Fail(effectiveProgressSessionID, reasoningIterations)
			}
			return "", err
		}
	}
	return effectiveSessionID, nil
}

func (s *OpenRouterService) ReserveReasoningSession(model string, reasoningEffort *string) (string, error) {
	if s.sessions == nil {
		return "", errors.New("reasoning sessions are not enabled")
	}

	storedReasoningEffort := ""
	if reasoningEffort != nil {
		storedReasoningEffort = *reasoningEffort
	}
	sessionID, err := s.sessions.CreateReserved(model, storedReasoningEffort)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func (s *OpenRouterService) getReasoningResponseWithTools(
	methodName string,
	jsonSchema *string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	firstIterationTools []ToolCall,
	firstIterationToolHandlers map[string]ToolHandler,
	promptCacheKey *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	progressSessionID string,
) (*LLMBotMessage, string, LLMUsageTotals, int, []string, string, *ReasoningSearchDebugInfo, error) {
	if reasoningEffort != nil {
		switch *reasoningEffort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("reasoning effort %q is not supported for OpenRouter models; supported values are minimal, low, medium, high", *reasoningEffort)
		}
	}
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("OpenRouter %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf(
				"OpenRouter %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t",
				methodName,
				usageTotals.InputTokens,
				usageTotals.CachedInputTokens,
				usageTotals.OutputTokens,
				usageTotals.ReasoningTokens,
				usageTotals.TotalTokens,
				cost.EstimatedCostUSD,
				cost.PricingConfigured,
			)
		}
	}()
	reasoningSummaries := []string{}

	if len(tools) == 0 {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New("tools must contain at least one tool definition")
	}
	if len(toolHandlers) == 0 {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New("toolHandlers must contain at least one handler")
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}
	reasoningCtx := ContextWithReasoningToolState(ContextWithDeb(context.Background(), deb))
	if s.requiredToolIterations > 0 {
		tools, toolHandlers = openRouterWithResultsReadyTool(tools, toolHandlers)
		if len(firstIterationTools) > 0 {
			firstIterationTools, firstIterationToolHandlers = openRouterWithResultsReadyTool(firstIterationTools, firstIterationToolHandlers)
		}
	}

	instructions, conversation, err := splitInstructionsAndConversation(messages)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
	}
	firstIterationInstructions := instructions
	if len(firstIterationTools) > 0 {
		firstIterationInstructions = BuildFirstIterationReasoningSearchSystemMessage(instructions, firstIterationTools)
	}
	conversationInput, err := buildOpenRouterInput(conversation)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
	}

	normalizedTools, err := normalizeResponseTools(tools)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
	}
	firstIterationNormalizedTools := normalizedTools
	if len(firstIterationTools) > 0 {
		firstIterationNormalizedTools, err = normalizeResponseTools(firstIterationTools)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
		}
		if len(firstIterationToolHandlers) == 0 {
			return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New("firstIterationToolHandlers must contain at least one handler when firstIterationTools are provided")
		}
	}

	text, err := buildResponsesText(jsonSchema)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
	}

	resultsReadyCalled := false
	for i := 0; i < maxIterations; i++ {
		if s.progress != nil && progressSessionID != "" {
			s.progress.Thinking(progressSessionID, i+1)
		}
		currentTools := normalizedTools
		currentToolHandlers := toolHandlers
		if i == 0 && len(firstIterationTools) > 0 {
			currentTools = firstIterationNormalizedTools
			currentToolHandlers = firstIterationToolHandlers
		}
		toolChoice := "auto"
		if i < s.requiredToolIterations && !resultsReadyCalled {
			toolChoice = "required"
		}
		instructionsForRequest := &instructions
		if i == 0 && len(firstIterationTools) > 0 {
			instructionsForRequest = &firstIterationInstructions
		}
		req := ResponsesRequest{
			Model:           model,
			Input:           conversationInput,
			Instructions:    instructionsForRequest,
			MaxOutputTokens: maxTokens,
			PromptCacheKey:  promptCacheKey,
			Text:            text,
			Tools:           currentTools,
			ToolChoice:      toolChoice,
			Provider:        s.reasoningSearchProviderPrefs(),
		}
		if reasoningEffort != nil {
			req.Reasoning = &ResponsesReasoning{Effort: *reasoningEffort}
		}

		var responsesResp ResponsesResponse
		if err := callLLMAPI(s.client, s.token, req, s.apiBaseURL+"/responses", &responsesResp, deb); err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
		}
		iterations = i + 1
		iterationSummary := strings.Join(extractReasoningSummaryText(responsesResp.Output), "\n\n")
		if strings.TrimSpace(iterationSummary) != "" {
			reasoningSummaries = append(reasoningSummaries, iterationSummary)
		}
		printReasoningOutputIfDeb(deb, i+1, responsesResp.Output)
		usageTotals.Add(responsesResp.Usage)

		if err := responsesCompletionError(&responsesResp); err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, "", nil, err
		}

		if len(responsesResp.Output) == 0 {
			log.Printf(
				"OpenRouter %s empty output: response_status=%q output_items=%s",
				methodName,
				responsesResp.Status,
				describeResponsesOutputItems(responsesResp.Output),
			)
			return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New("responses API returned no output")
		}

		functionCalls := []ResponsesOutputItem{}
		for _, item := range responsesResp.Output {
			if item.Type == "function_call" {
				functionCalls = append(functionCalls, item)
			}
		}

		if len(functionCalls) == 0 {
			content := extractAssistantOutputText(responsesResp.Output)
			if content == "" {
				log.Printf(
					"OpenRouter %s empty assistant output: response_status=%q output_items=%s",
					methodName,
					responsesResp.Status,
					describeResponsesOutputItems(responsesResp.Output),
				)
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New(ResponsesAPIEmptyAssistantOutputError)
			}
			return &LLMBotMessage{
				Role:    "assistant",
				Content: content,
			}, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations, usedTools, responsesResp.ID, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		nextConversationInput := append([]interface{}{}, conversationInput...)
		toolCallLogs := []string{}
		for _, item := range responsesResp.Output {
			if item.Type == "function_call" || (item.Type == "message" && item.Role == "assistant") {
				nextConversationInput = append(nextConversationInput, item)
			}
		}
		for _, toolCall := range functionCalls {
			if toolCall.CallID == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("tool call for '%s' is missing call_id", toolCall.Name)
			}
			canonicalToolName := CanonicalReasoningToolName(toolCall.Name)
			isResultsReadyTool := toolCall.Name == openRouterResultsReadyToolName
			if !usedToolsSet[canonicalToolName] {
				usedTools = append(usedTools, canonicalToolName)
				usedToolsSet[canonicalToolName] = true
			}

			handler, ok := ResolveReasoningToolHandler(toolCall.Name, currentToolHandlers, firstIterationToolHandlers)
			if !ok {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("missing handler for tool '%s'", toolCall.Name)
			}

			rawArgs := json.RawMessage(toolCall.Arguments)
			if len(rawArgs) == 0 {
				rawArgs = json.RawMessage("{}")
			}
			if !json.Valid(rawArgs) {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("invalid arguments for tool '%s': %s", toolCall.Name, toolCall.Arguments)
			}
			if deb {
				toolCallLogs = append(toolCallLogs, fmt.Sprintf("- %s args: %s", toolCall.Name, compactToolCallArguments(rawArgs)))
			}

			if s.progress != nil && progressSessionID != "" && !isResultsReadyTool {
				s.progress.RunningTool(progressSessionID, i+1, canonicalToolName)
			}
			result, err := handler(reasoningCtx, rawArgs)
			if err != nil {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("tool '%s' execution failed: %w", toolCall.Name, err)
			}

			nextConversationInput = append(nextConversationInput, map[string]string{
				"type":    "function_call_output",
				"call_id": toolCall.CallID,
				"output":  result,
			})
			if isResultsReadyTool {
				resultsReadyCalled = true
			}
		}
		if deb && len(toolCallLogs) > 0 {
			reasoningSummaries = append(reasoningSummaries, "Tool calls:\n"+strings.Join(toolCallLogs, "\n"))
		}
		conversationInput = nextConversationInput
	}

	return nil, "", LLMUsageTotals{}, 0, nil, "", ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func splitInstructionsAndConversation(messages []LLMBotMessage) (string, []LLMBotMessage, error) {
	sysMsgCount := 0
	instructions := ""
	conversation := []LLMBotMessage{}

	for _, message := range messages {
		if message.Role == "system" || message.Role == "developer" {
			sysMsgCount++
			instructions = message.Content
			continue
		}
		conversation = append(conversation, message)
	}
	if sysMsgCount != 1 {
		return "", nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}
	return instructions, conversation, nil
}

func buildOpenRouterInput(messages []LLMBotMessage) ([]interface{}, error) {
	input := make([]interface{}, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case "user":
			input = append(input, map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []map[string]string{{
					"type": "input_text",
					"text": message.Content,
				}},
			})
		case "assistant":
			input = append(input, map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": []map[string]string{{
					"type": "output_text",
					"text": message.Content,
				}},
			})
		case "tool":
			if message.ToolCallID == "" {
				return nil, errors.New("tool message is missing tool_call_id")
			}
			input = append(input, map[string]string{
				"type":    "function_call_output",
				"call_id": message.ToolCallID,
				"output":  message.Content,
			})
		default:
			return nil, fmt.Errorf("unsupported message role: %s", message.Role)
		}
	}
	return input, nil
}

func sessionConversationMessages(messages []LLMBotMessage) []LLMBotMessage {
	filtered := make([]LLMBotMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		filtered = append(filtered, LLMBotMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return filtered
}

func (s *OpenRouterService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}
