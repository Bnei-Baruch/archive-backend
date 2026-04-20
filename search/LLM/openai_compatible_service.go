package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const embeddingModel = "text-embedding-3-large"
const defaultOpenAIAPIBaseURL = "https://api.openai.com/v1"

type OpenAICompatibleAPIService struct {
	*BaseLLMService
	sessions                               *OpenAIReasoningSessionStore
	omitInstructionsWithPreviousResponseID bool
}

func newOpenAICompatibleAPIServiceWithOptions(token string, pricing []ModelPricing, sessions *OpenAIReasoningSessionStore, apiBaseURL string) *OpenAICompatibleAPIService {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = defaultOpenAIAPIBaseURL
	} else {
		apiBaseURL = strings.TrimRight(apiBaseURL, "/")
		if !strings.HasSuffix(apiBaseURL, "/v1") {
			apiBaseURL += "/v1"
		}
	}

	return &OpenAICompatibleAPIService{
		BaseLLMService: newBaseLLMService(token, pricing, apiBaseURL),
		sessions:       sessions,
	}
}

// Structs for OpenAI API interaction

type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []LLMBotMessage `json:"messages"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	PromptCacheKey      *string         `json:"prompt_cache_key,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort     *string         `json:"reasoning_effort,omitempty"`
	Tools               []ToolCall      `json:"tools,omitempty"`
}

type ToolCall struct {
	Type     string      `json:"type"`
	Function interface{} `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type MessageToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolHandler func(ctx context.Context, arguments json.RawMessage) (string, error)

type ResponseFormat struct {
	Type       string      `json:"type"`
	JsonSchema interface{} `json:"json_schema"`
}

type ChatResponse struct {
	Choices []struct {
		Index   int           `json:"index"`
		Message LLMBotMessage `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

type EmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

type LLMBotMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	Reasoning  string            `json:"reasoning,omitempty"`
	Refusal    *string           `json:"refusal,omitempty"`
	ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

func (s *OpenAICompatibleAPIService) GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, output interface{}) error {
	msg, _, usageTotals, err := s.getStructuredOutputWithUsage(model, maxTokens, messages, promptCacheKey, jsonSchema, reasoningEffort, nil, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return err
	}
	if msg.Refusal != nil {
		log.Printf("Received a 'Refusal': %s", *msg.Refusal)
		return nil
	}

	err = json.Unmarshal([]byte(msg.Content), output)
	if err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	return nil
}

func (s *OpenAICompatibleAPIService) GetStructuredOutputWithDebugInfo(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, reasoningSummary, usageTotals, err := s.getStructuredOutputWithUsage(model, maxTokens, messages, promptCacheKey, jsonSchema, reasoningEffort, nil, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	if msg.Refusal != nil {
		log.Printf("Received a 'Refusal': %s", *msg.Refusal)
		debugInfo := s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals)
		debugInfo.ReasoningSummary = reasoningSummary
		return debugInfo, nil
	}

	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	debugInfo := s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals)
	debugInfo.ReasoningSummary = reasoningSummary
	return debugInfo, nil
}

func (s *OpenAICompatibleAPIService) getStructuredOutputWithUsage(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, jsonSchema string, reasoningEffort *string, provider *ResponsesProvider, logRawBody bool) (*LLMBotMessage, string, LLMUsageTotals, error) {
	reasoningEffort = normalizeOpenAICompatibleReasoningEffort(reasoningEffort)
	if reasoningEffort != nil {
		if strings.HasPrefix(model, "gpt-oss") {
			switch *reasoningEffort {
			case "low", "medium", "high":
			default:
				return nil, "", LLMUsageTotals{}, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", *reasoningEffort, model)
			}
		}
	}

	instructions := ""
	input := []interface{}{}
	sysMsgCount := 0
	for _, m := range messages {
		if m.Role == "system" || m.Role == "developer" {
			sysMsgCount++
			instructions = m.Content
			continue
		}
		if m.Role == "tool" {
			if m.ToolCallID == "" {
				return nil, "", LLMUsageTotals{}, errors.New("tool message is missing tool_call_id")
			}
			input = append(input, map[string]string{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  m.Content,
			})
			continue
		}
		input = append(input, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	if sysMsgCount != 1 {
		return nil, "", LLMUsageTotals{}, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	text, err := buildResponsesText(&jsonSchema)
	if err != nil {
		return nil, "", LLMUsageTotals{}, err
	}

	req := ResponsesRequest{
		Model:           model,
		Input:           input,
		Instructions:    &instructions,
		MaxOutputTokens: maxTokens,
		PromptCacheKey:  promptCacheKey,
		Text:            text,
		Provider:        provider,
	}
	if reasoningEffort != nil || logRawBody {
		req.Reasoning = &ResponsesReasoning{}
		if reasoningEffort != nil {
			req.Reasoning.Effort = *reasoningEffort
		}
		if logRawBody {
			req.Reasoning.Summary = "auto"
		}
	}

	var responsesResp ResponsesResponse
	if err := callLLMAPI(s.client, s.token, req, s.apiBaseURL+"/responses", &responsesResp, logRawBody); err != nil {
		return nil, "", LLMUsageTotals{}, err
	}
	usageTotals := LLMUsageTotals{}
	usageTotals.Add(responsesResp.Usage)
	reasoningSummary := strings.Join(extractReasoningSummaryText(responsesResp.Output), "\n\n")

	if err := responsesCompletionError(&responsesResp); err != nil {
		return nil, reasoningSummary, usageTotals, err
	}
	if len(responsesResp.Output) == 0 {
		return nil, reasoningSummary, usageTotals, errors.New("responses API returned no output")
	}

	content := extractAssistantOutputText(responsesResp.Output)
	if content == "" {
		return nil, reasoningSummary, usageTotals, errors.New(ResponsesAPIEmptyAssistantOutputError)
	}

	return &LLMBotMessage{
		Role:    "assistant",
		Content: content,
	}, reasoningSummary, usageTotals, nil
}

func (s *OpenAICompatibleAPIService) GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, promptCacheKey, frequencyPenalty, jsonSchema, reasoningEffort, false, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *OpenAICompatibleAPIService) GetChatResponseWithDebugInfo(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, promptCacheKey, frequencyPenalty, jsonSchema, reasoningEffort, false, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *OpenAICompatibleAPIService) getChatResponseWithUsage(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string, useMaxCompletionTokens bool, logRawBody bool) (*LLMBotMessage, LLMUsageTotals, error) {
	reasoningEffort = normalizeOpenAICompatibleReasoningEffort(reasoningEffort)
	if reasoningEffort != nil {
		if strings.HasPrefix(model, "gpt-oss") {
			switch *reasoningEffort {
			case "low", "medium", "high":
			default:
				return nil, LLMUsageTotals{}, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", *reasoningEffort, model)
			}
		}
	}
	sysMsgCount := 0
	for _, m := range messages {
		if m.Role == "system" || m.Role == "developer" {
			sysMsgCount++
		}
	}
	if sysMsgCount != 1 {
		return nil, LLMUsageTotals{}, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	var respFmt *ResponseFormat
	if jsonSchema != nil {
		var JsonSchemaData interface{}
		err := json.Unmarshal([]byte(*jsonSchema), &JsonSchemaData)
		if err != nil {
			return nil, LLMUsageTotals{}, fmt.Errorf("invalid json_schema: %v", err)
		}
		respFmt = &ResponseFormat{
			Type:       "json_schema",
			JsonSchema: JsonSchemaData,
		}
	}

	req := ChatRequest{
		Model:            model,
		Messages:         messages,
		PromptCacheKey:   promptCacheKey,
		FrequencyPenalty: frequencyPenalty,
		ResponseFormat:   respFmt,
		ReasoningEffort:  reasoningEffort,
	}
	if useMaxCompletionTokens {
		req.MaxCompletionTokens = maxTokens
	} else {
		req.MaxTokens = maxTokens
	}

	var chatResp ChatResponse
	if err := callLLMAPI(s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, logRawBody); err != nil {
		return nil, LLMUsageTotals{}, err
	}
	usageTotals := LLMUsageTotals{}
	usageTotals.Add(chatResp.Usage)

	for _, choice := range chatResp.Choices {
		if choice.Index == 0 {
			return &choice.Message, usageTotals, nil
		}
	}
	return nil, usageTotals, errors.New("no valid chat choices returned")
}

func (s *OpenAICompatibleAPIService) GetReasoningResponseWithTools(
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
	msg, _, _, _, _, _, _, err := s.getReasoningResponseWithTools("GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, nil, nil, promptCacheKey, reasoningEffort, deb, maxIterations, nil, "")
	return msg, err
}

func (s *OpenAICompatibleAPIService) GetReasoningStructuredOutputWithTools(
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
	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, _, toolDebug, err := s.getReasoningResponseWithTools("GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, nil, nil, promptCacheKey, reasoningEffort, deb, maxIterations, nil, "")
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

func (s *OpenAICompatibleAPIService) GetReasoningStructuredOutputWithToolsForSession(
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
	var previousResponseID *string

	if sessionID != nil && strings.TrimSpace(*sessionID) != "" {
		session, err := s.sessions.Get(strings.TrimSpace(*sessionID))
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
		if session.LastResponseID != "" {
			previousResponseID = &session.LastResponseID
		}
	}
	if progressSessionID != nil && strings.TrimSpace(*progressSessionID) != "" {
		effectiveProgressSessionID = strings.TrimSpace(*progressSessionID)
	}

	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, finalResponseID, toolDebug, err := s.getReasoningResponseWithTools(
		"GetReasoningStructuredOutputWithToolsForSession",
		&jsonSchema,
		effectiveModel,
		maxTokens,
		messages,
		tools,
		toolHandlers,
		firstIterationTools,
		firstIterationToolHandlers,
		promptCacheKey,
		effectiveReasoningEffort,
		deb,
		maxIterations,
		previousResponseID,
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

	if effectiveSessionID == "" {
		storedReasoningEffort := ""
		if effectiveReasoningEffort != nil {
			storedReasoningEffort = *effectiveReasoningEffort
		}
		effectiveSessionID, err = s.sessions.Create(finalResponseID, effectiveModel, storedReasoningEffort)
		if err != nil {
			return "", err
		}
	} else {
		if err := s.sessions.Update(effectiveSessionID, finalResponseID); err != nil {
			if s.progress != nil && effectiveProgressSessionID != "" {
				s.progress.Fail(effectiveProgressSessionID, reasoningIterations)
			}
			return "", err
		}
	}
	return effectiveSessionID, nil
}

func (s *OpenAICompatibleAPIService) ReserveReasoningSession(model string, reasoningEffort *string) (string, error) {
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

func (s *OpenAICompatibleAPIService) getReasoningResponseWithTools(
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
	initialPreviousResponseID *string,
	progressSessionID string,
) (*LLMBotMessage, string, LLMUsageTotals, int, []string, string, *ReasoningSearchDebugInfo, error) {
	reasoningEffort = normalizeOpenAICompatibleReasoningEffort(reasoningEffort)
	if reasoningEffort != nil {
		if strings.HasPrefix(model, "gpt-oss") {
			switch *reasoningEffort {
			case "low", "medium", "high":
			default:
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", *reasoningEffort, model)
			}
		}
	}
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("OpenAI %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf(
				"OpenAI %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t",
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

	sysMsgCount := 0
	var instructions string
	initialInput := []interface{}{}
	for _, m := range messages {
		if m.Role == "system" || m.Role == "developer" {
			sysMsgCount++
			instructions = m.Content
			continue
		}
		if m.Role == "tool" {
			if m.ToolCallID == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New("tool message is missing tool_call_id")
			}
			initialInput = append(initialInput, map[string]string{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  m.Content,
			})
			continue
		}
		initialInput = append(initialInput, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	if sysMsgCount != 1 {
		return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}
	firstIterationInstructions := instructions
	if len(firstIterationTools) > 0 {
		firstIterationInstructions = BuildFirstIterationReasoningSearchSystemMessage(instructions, firstIterationTools)
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

	previousResponseID := initialPreviousResponseID
	nextInput := initialInput

	for i := 0; i < maxIterations; i++ {
		if s.progress != nil && progressSessionID != "" {
			s.progress.Thinking(progressSessionID, i+1)
		}
		instructionsForRequest := &instructions
		if i == 0 && len(firstIterationTools) > 0 {
			instructionsForRequest = &firstIterationInstructions
		}
		if previousResponseID != nil && s.omitInstructionsWithPreviousResponseID {
			instructionsForRequest = nil
		}
		currentTools := normalizedTools
		currentToolHandlers := toolHandlers
		if i == 0 && len(firstIterationTools) > 0 {
			currentTools = firstIterationNormalizedTools
			currentToolHandlers = firstIterationToolHandlers
		}
		req := ResponsesRequest{
			Model:              model,
			Input:              nextInput,
			Instructions:       instructionsForRequest,
			PreviousResponseID: previousResponseID,
			MaxOutputTokens:    maxTokens,
			PromptCacheKey:     promptCacheKey,
			Text:               text,
			Tools:              currentTools,
		}
		if reasoningEffort != nil || deb {
			req.Reasoning = &ResponsesReasoning{}
			if reasoningEffort != nil {
				req.Reasoning.Effort = *reasoningEffort
			}
			if deb {
				req.Reasoning.Summary = "auto"
			}
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
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, errors.New(ResponsesAPIEmptyAssistantOutputError)
			}
			return &LLMBotMessage{
				Role:    "assistant",
				Content: content,
			}, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations, usedTools, responsesResp.ID, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		nextInput = []interface{}{}
		toolCallLogs := []string{}
		for _, toolCall := range functionCalls {
			if toolCall.CallID == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("tool call for '%s' is missing call_id", toolCall.Name)
			}
			canonicalToolName := CanonicalReasoningToolName(toolCall.Name)
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

			if s.progress != nil && progressSessionID != "" {
				s.progress.RunningTool(progressSessionID, i+1, canonicalToolName)
			}
			result, err := handler(reasoningCtx, rawArgs)
			if err != nil {
				return nil, "", LLMUsageTotals{}, 0, nil, "", nil, fmt.Errorf("tool '%s' execution failed: %w", toolCall.Name, err)
			}

			nextInput = append(nextInput, map[string]string{
				"type":    "function_call_output",
				"call_id": toolCall.CallID,
				"output":  result,
			})
		}
		if deb && len(toolCallLogs) > 0 {
			reasoningSummaries = append(reasoningSummaries, "Tool calls:\n"+strings.Join(toolCallLogs, "\n"))
		}
		previousResponseID = &responsesResp.ID
	}

	return nil, "", LLMUsageTotals{}, 0, nil, "", ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func normalizeOpenAICompatibleReasoningEffort(reasoningEffort *string) *string {
	if reasoningEffort == nil {
		return nil
	}
	effort := strings.TrimSpace(*reasoningEffort)
	if effort == "" {
		return nil
	}
	return &effort
}

func (s *OpenAICompatibleAPIService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *OpenAICompatibleAPIService) GetEmbeddings(content string) ([]float64, error) {
	payload := map[string]interface{}{
		"input": content,
		"model": embeddingModel,
	}

	var resp EmbeddingResponse
	if err := callLLMAPI(s.client, s.token, payload, s.apiBaseURL+"/embeddings", &resp, false); err != nil {
		return nil, err
	}
	totalTokens := 0
	if resp.Usage != nil {
		totalTokens = resp.Usage.TotalTokens
	}
	if totalTokens > 0 {
		log.Printf("OpenAI GetEmbeddings total tokens: %d", totalTokens)
	}

	for _, d := range resp.Data {
		if d.Index == 0 {
			if len(d.Embedding) == 0 {
				return nil, errors.New("embedding list is empty")
			}
			return d.Embedding, nil
		}
	}
	return nil, errors.New("embedding response missing")
}
