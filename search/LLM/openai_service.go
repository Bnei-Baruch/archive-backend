package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const embeddingModel = "text-embedding-3-large"
const defaultOpenAIAPIBaseURL = "https://api.openai.com/v1"
const defaultLLMHTTPRequestTimeout = 2 * time.Minute

type OpenAIService struct {
	token      string
	pricing    []OpenAIModelPricing
	sessions   *OpenAIReasoningSessionStore
	progress   *ReasoningProgressStore
	apiBaseURL string
	client     *http.Client
}

var _ Service = (*OpenAIService)(nil)

func NewOpenAIService(token string) *OpenAIService {
	return NewOpenAIServiceWithOptions(token, nil, nil, "")
}

func NewOpenAIServiceWithPricing(token string, pricing []OpenAIModelPricing) *OpenAIService {
	return NewOpenAIServiceWithOptions(token, pricing, nil, "")
}

func NewOpenAIServiceWithOptions(token string, pricing []OpenAIModelPricing, sessions *OpenAIReasoningSessionStore, apiBaseURL string) *OpenAIService {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = defaultOpenAIAPIBaseURL
	} else {
		apiBaseURL = strings.TrimRight(apiBaseURL, "/")
		if !strings.HasSuffix(apiBaseURL, "/v1") {
			apiBaseURL += "/v1"
		}
	}

	service := &OpenAIService{
		token:      token,
		pricing:    pricing,
		sessions:   sessions,
		apiBaseURL: apiBaseURL,
		client:     &http.Client{},
	}

	return service
}

// Structs for OpenAI API interaction

type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []LLMBotMessage `json:"messages"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	PromptCacheKey   *string         `json:"prompt_cache_key,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort  *string         `json:"reasoning_effort,omitempty"`
	Tools            []ToolCall      `json:"tools,omitempty"`
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
	Refusal    *string           `json:"refusal,omitempty"`
	ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

func (s *OpenAIService) GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getStructuredOutputWithUsage(model, maxTokens, messages, promptCacheKey, jsonSchema, reasoningEffort, nil)
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

func (s *OpenAIService) GetStructuredOutputWithDebug(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getStructuredOutputWithUsage(model, maxTokens, messages, promptCacheKey, jsonSchema, reasoningEffort, nil)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetStructuredOutputWithDebug total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	if msg.Refusal != nil {
		log.Printf("Received a 'Refusal': %s", *msg.Refusal)
		return s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
	}

	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	return s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *OpenAIService) getStructuredOutputWithUsage(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, jsonSchema string, reasoningEffort *string, provider *ResponsesProvider) (*LLMBotMessage, OpenAIUsageTotals, error) {
	if reasoningEffort != nil {
		if strings.HasPrefix(model, "gpt-oss") {
			switch *reasoningEffort {
			case "low", "medium", "high":
			default:
				return nil, OpenAIUsageTotals{}, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", *reasoningEffort, model)
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
				return nil, OpenAIUsageTotals{}, errors.New("tool message is missing tool_call_id")
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
		return nil, OpenAIUsageTotals{}, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	text, err := buildResponsesText(&jsonSchema)
	if err != nil {
		return nil, OpenAIUsageTotals{}, err
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
	if reasoningEffort != nil {
		req.Reasoning = &ResponsesReasoning{Effort: *reasoningEffort}
	}

	var responsesResp ResponsesResponse
	if err := s.callAPI(req, s.apiBaseURL+"/responses", &responsesResp); err != nil {
		return nil, OpenAIUsageTotals{}, err
	}
	usageTotals := OpenAIUsageTotals{}
	usageTotals.Add(responsesResp.Usage)

	if err := responsesCompletionError(&responsesResp); err != nil {
		return nil, usageTotals, err
	}
	if len(responsesResp.Output) == 0 {
		return nil, usageTotals, errors.New("responses API returned no output")
	}

	content := extractAssistantOutputText(responsesResp.Output)
	if content == "" {
		return nil, usageTotals, errors.New("responses API returned empty assistant output")
	}

	return &LLMBotMessage{
		Role:    "assistant",
		Content: content,
	}, usageTotals, nil
}

func (s *OpenAIService) GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, promptCacheKey, frequencyPenalty, jsonSchema, reasoningEffort)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *OpenAIService) getChatResponseWithUsage(model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, OpenAIUsageTotals, error) {
	if reasoningEffort != nil {
		if strings.HasPrefix(model, "gpt-oss") {
			switch *reasoningEffort {
			case "low", "medium", "high":
			default:
				return nil, OpenAIUsageTotals{}, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", *reasoningEffort, model)
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
		return nil, OpenAIUsageTotals{}, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	var respFmt *ResponseFormat
	if jsonSchema != nil {
		var JsonSchemaData interface{}
		err := json.Unmarshal([]byte(*jsonSchema), &JsonSchemaData)
		if err != nil {
			return nil, OpenAIUsageTotals{}, fmt.Errorf("invalid json_schema: %v", err)
		}
		respFmt = &ResponseFormat{
			Type:       "json_schema",
			JsonSchema: JsonSchemaData,
		}
	}

	req := ChatRequest{
		Model:            model,
		Messages:         messages,
		MaxTokens:        maxTokens,
		PromptCacheKey:   promptCacheKey,
		FrequencyPenalty: frequencyPenalty,
		ResponseFormat:   respFmt,
		ReasoningEffort:  reasoningEffort,
	}

	var chatResp ChatResponse
	if err := s.callAPI(req, s.apiBaseURL+"/chat/completions", &chatResp); err != nil {
		return nil, OpenAIUsageTotals{}, err
	}
	usageTotals := OpenAIUsageTotals{}
	usageTotals.Add(chatResp.Usage)

	for _, choice := range chatResp.Choices {
		if choice.Index == 0 {
			return &choice.Message, usageTotals, nil
		}
	}
	return nil, usageTotals, errors.New("no valid chat choices returned")
}

func (s *OpenAIService) GetReasoningResponseWithTools(
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
	msg, _, _, _, _, _, err := s.getReasoningResponseWithTools("GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, promptCacheKey, reasoningEffort, deb, maxIterations, nil, "")
	return msg, err
}

func (s *OpenAIService) GetReasoningStructuredOutputWithTools(
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
	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, _, err := s.getReasoningResponseWithTools("GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, promptCacheKey, reasoningEffort, deb, maxIterations, nil, "")
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
	if setter, ok := output.(reasoningProcessStatsSetter); ok {
		setter.SetReasoningProcessStats(usageTotals.TotalTokens, reasoningIterations)
	}
	if setter, ok := output.(reasoningUsedToolsSetter); ok {
		setter.SetUsedTools(usedTools)
	}
	if deb {
		if setter, ok := output.(reasoningDebugInfoSetter); ok {
			setter.SetReasoningDebugInfo(s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals))
		}
	}
	return nil
}

func (s *OpenAIService) GetReasoningStructuredOutputWithToolsForSession(
	sessionID *string,
	progressSessionID *string,
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

	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, finalResponseID, err := s.getReasoningResponseWithTools(
		"GetReasoningStructuredOutputWithToolsForSession",
		&jsonSchema,
		effectiveModel,
		maxTokens,
		messages,
		tools,
		toolHandlers,
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
	if setter, ok := output.(reasoningProcessStatsSetter); ok {
		setter.SetReasoningProcessStats(usageTotals.TotalTokens, reasoningIterations)
	}
	if setter, ok := output.(reasoningUsedToolsSetter); ok {
		setter.SetUsedTools(usedTools)
	}
	if deb {
		if setter, ok := output.(reasoningDebugInfoSetter); ok {
			setter.SetReasoningDebugInfo(s.buildReasoningDebugInfo(effectiveModel, effectiveReasoningEffort, usageTotals))
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

func (s *OpenAIService) ReserveReasoningSession(model string, reasoningEffort *string) (string, error) {
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

func (s *OpenAIService) getReasoningResponseWithTools(
	methodName string,
	jsonSchema *string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	promptCacheKey *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	initialPreviousResponseID *string,
	progressSessionID string,
) (*LLMBotMessage, string, OpenAIUsageTotals, int, []string, string, error) {
	if reasoningEffort != nil {
		if strings.HasPrefix(model, "gpt-oss") {
			switch *reasoningEffort {
			case "low", "medium", "high":
			default:
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", *reasoningEffort, model)
			}
		}
	}
	usageTotals := OpenAIUsageTotals{}
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
		return nil, "", OpenAIUsageTotals{}, 0, nil, "", errors.New("tools must contain at least one tool definition")
	}
	if len(toolHandlers) == 0 {
		return nil, "", OpenAIUsageTotals{}, 0, nil, "", errors.New("toolHandlers must contain at least one handler")
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
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", errors.New("tool message is missing tool_call_id")
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
		return nil, "", OpenAIUsageTotals{}, 0, nil, "", fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	normalizedTools, err := normalizeResponseTools(tools)
	if err != nil {
		return nil, "", OpenAIUsageTotals{}, 0, nil, "", err
	}

	text, err := buildResponsesText(jsonSchema)
	if err != nil {
		return nil, "", OpenAIUsageTotals{}, 0, nil, "", err
	}

	previousResponseID := initialPreviousResponseID
	nextInput := initialInput

	for i := 0; i < maxIterations; i++ {
		if s.progress != nil && progressSessionID != "" {
			s.progress.Thinking(progressSessionID, i+1)
		}
		req := ResponsesRequest{
			Model:              model,
			Input:              nextInput,
			Instructions:       &instructions,
			PreviousResponseID: previousResponseID,
			MaxOutputTokens:    maxTokens,
			PromptCacheKey:     promptCacheKey,
			Text:               text,
			Tools:              normalizedTools,
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
		if err := s.callAPI(req, s.apiBaseURL+"/responses", &responsesResp); err != nil {
			return nil, "", OpenAIUsageTotals{}, 0, nil, "", err
		}
		iterations = i + 1
		iterationSummary := strings.Join(extractReasoningSummaryText(responsesResp.Output), "\n\n")
		if strings.TrimSpace(iterationSummary) != "" {
			reasoningSummaries = append(reasoningSummaries, iterationSummary)
		}
		printReasoningOutputIfDeb(deb, i+1, responsesResp.Output)
		usageTotals.Add(responsesResp.Usage)

		if err := responsesCompletionError(&responsesResp); err != nil {
			return nil, "", OpenAIUsageTotals{}, 0, nil, "", err
		}

		if len(responsesResp.Output) == 0 {
			return nil, "", OpenAIUsageTotals{}, 0, nil, "", errors.New("responses API returned no output")
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
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", errors.New("responses API returned empty assistant output")
			}
			return &LLMBotMessage{
				Role:    "assistant",
				Content: content,
			}, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations, usedTools, responsesResp.ID, nil
		}

		nextInput = []interface{}{}
		toolCallLogs := []string{}
		for _, toolCall := range functionCalls {
			if toolCall.CallID == "" {
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", fmt.Errorf("tool call for '%s' is missing call_id", toolCall.Name)
			}
			if !usedToolsSet[toolCall.Name] {
				usedTools = append(usedTools, toolCall.Name)
				usedToolsSet[toolCall.Name] = true
			}

			handler, ok := toolHandlers[toolCall.Name]
			if !ok {
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", fmt.Errorf("missing handler for tool '%s'", toolCall.Name)
			}

			rawArgs := json.RawMessage(toolCall.Arguments)
			if len(rawArgs) == 0 {
				rawArgs = json.RawMessage("{}")
			}
			if !json.Valid(rawArgs) {
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", fmt.Errorf("invalid arguments for tool '%s': %s", toolCall.Name, toolCall.Arguments)
			}
			if deb {
				toolCallLogs = append(toolCallLogs, fmt.Sprintf("- %s args: %s", toolCall.Name, compactToolCallArguments(rawArgs)))
			}

			if s.progress != nil && progressSessionID != "" {
				s.progress.RunningTool(progressSessionID, i+1, toolCall.Name)
			}
			result, err := handler(reasoningCtx, rawArgs)
			if err != nil {
				return nil, "", OpenAIUsageTotals{}, 0, nil, "", fmt.Errorf("tool '%s' execution failed: %w", toolCall.Name, err)
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

	return nil, "", OpenAIUsageTotals{}, 0, nil, "", &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func (s *OpenAIService) buildReasoningDebugInfo(model string, reasoningEffort *string, usageTotals OpenAIUsageTotals) *ReasoningSearchDebugInfo {
	effort := ""
	if reasoningEffort != nil {
		effort = *reasoningEffort
	}
	cost := s.estimateCost(model, effort, usageTotals)

	return &ReasoningSearchDebugInfo{
		Enabled:                     true,
		Model:                       model,
		ReasoningEffort:             effort,
		TotalTokens:                 usageTotals.TotalTokens,
		InputTokens:                 usageTotals.InputTokens,
		CachedInputTokens:           usageTotals.CachedInputTokens,
		UncachedInputTokens:         usageTotals.UncachedInputTokens(),
		OutputTokens:                usageTotals.OutputTokens,
		ReasoningTokens:             usageTotals.ReasoningTokens,
		PricingConfigured:           cost.PricingConfigured,
		InputPer1MTokensUSD:         cost.InputPer1MTokensUSD,
		CachedInputPer1MTokensUSD:   cost.CachedInputPer1MTokensUSD,
		OutputPer1MTokensUSD:        cost.OutputPer1MTokensUSD,
		EstimatedInputCostUSD:       cost.EstimatedInputCostUSD,
		EstimatedCachedInputCostUSD: cost.EstimatedCachedInputCostUSD,
		EstimatedOutputCostUSD:      cost.EstimatedOutputCostUSD,
		EstimatedCostUSD:            cost.EstimatedCostUSD,
	}
}

func (s *OpenAIService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *OpenAIService) GetEmbeddings(content string) ([]float64, error) {
	payload := map[string]interface{}{
		"input": content,
		"model": embeddingModel,
	}

	var resp EmbeddingResponse
	if err := s.callAPI(payload, s.apiBaseURL+"/embeddings", &resp); err != nil {
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

func (s *OpenAIService) callAPI(data interface{}, endpoint string, result interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	if strings.TrimSpace(s.token) != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	req.Header.Set("Content-Type", "application/json")

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: defaultLLMHTTPRequestTimeout}
	}
	startedAt := time.Now()
	log.Printf("LLM API request start endpoint=%s", endpoint)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("LLM API request error endpoint=%s elapsed=%s err=%v", endpoint, time.Since(startedAt), err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("LLM API response endpoint=%s status=%d elapsed=%s", endpoint, resp.StatusCode, time.Since(startedAt))

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("json.Unmarshal error: %v\nResponse: %s", err, string(bodyBytes))
	}
	return nil
}
