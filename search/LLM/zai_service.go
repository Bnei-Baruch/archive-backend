package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const defaultZAIAPIBaseURL = "https://api.z.ai/api/paas/v4"

type ZAIService struct {
	*BaseLLMService
	sessions    *ChatReasoningSessionStore
	doSample    *bool
	temperature *float64
	topP        *float64
	stop        []string
}

type ZAIChatRequest struct {
	Model          string             `json:"model"`
	Messages       []LLMBotMessage    `json:"messages"`
	MaxTokens      *int               `json:"max_tokens,omitempty"`
	DoSample       *bool              `json:"do_sample,omitempty"`
	Temperature    *float64           `json:"temperature,omitempty"`
	TopP           *float64           `json:"top_p,omitempty"`
	Stop           []string           `json:"stop,omitempty"`
	ResponseFormat *ZAIResponseFormat `json:"response_format,omitempty"`
	Thinking       *ZAIThinking       `json:"thinking,omitempty"`
	Tools          []ToolCall         `json:"tools,omitempty"`
	ToolChoice     *string            `json:"tool_choice,omitempty"`
	Stream         bool               `json:"stream"`
}

type ZAIResponseFormat struct {
	Type string `json:"type"`
}

type ZAIThinking struct {
	Type          string `json:"type"`
	ClearThinking bool   `json:"clear_thinking,omitempty"`
}

type ZAIToolArguments struct {
	Raw json.RawMessage
}

func (a *ZAIToolArguments) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		a.Raw = json.RawMessage("{}")
		return nil
	}

	// Z.AI may return function arguments either as a JSON string or a JSON value.
	var encoded string
	if err := json.Unmarshal(data, &encoded); err == nil {
		encoded = strings.TrimSpace(encoded)
		if encoded == "" {
			encoded = "{}"
		}
		a.Raw = json.RawMessage(encoded)
		return nil
	}

	a.Raw = append(a.Raw[:0], data...)
	return nil
}

type ZAIToolCallFunction struct {
	Name      string           `json:"name"`
	Arguments ZAIToolArguments `json:"arguments"`
}

type ZAIToolCall struct {
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function ZAIToolCallFunction `json:"function"`
}

type ZAIMessage struct {
	Role             string        `json:"role"`
	Content          string        `json:"content,omitempty"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	ToolCalls        []ZAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
}

type ZAIChatResponse struct {
	Choices []struct {
		Index   int        `json:"index"`
		Message ZAIMessage `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

var _ Service = (*ZAIService)(nil)

func NewZAIService(token string) *ZAIService {
	return NewZAIServiceWithOptions(token, nil, nil, "", nil, nil, nil, nil)
}

func NewZAIServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string, doSample *bool, temperature *float64, topP *float64, stop []string) *ZAIService {
	base := newBaseLLMService(token, pricing, normalizeZAIAPIBaseURL(apiBaseURL))

	return &ZAIService{
		BaseLLMService: base,
		sessions:       sessions,
		doSample:       doSample,
		temperature:    temperature,
		topP:           topP,
		stop:           append([]string(nil), stop...),
	}
}

func (s *ZAIService) GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("ZAI GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	return nil
}

func (s *ZAIService) GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("ZAI GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	return s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ZAIService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("ZAI GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *ZAIService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("ZAI GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ZAIService) GetReasoningResponseWithTools(
	ctx context.Context,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	_ *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
) (*LLMBotMessage, error) {
	msg, _, _, _, _, _, err := s.getReasoningResponseWithTools(ctx, "GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
	return msg, err
}

func (s *ZAIService) GetReasoningStructuredOutputWithTools(
	ctx context.Context,
	jsonSchema string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	_ *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	output interface{},
) error {
	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools(ctx, "GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
	if err != nil {
		return err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
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

func (s *ZAIService) GetReasoningStructuredOutputWithToolsForSession(
	ctx context.Context,
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
	_ *string,
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

	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools(
		ctx,
		"GetReasoningStructuredOutputWithToolsForSession",
		&jsonSchema,
		effectiveModel,
		maxTokens,
		effectiveMessages,
		tools,
		toolHandlers,
		firstIterationTools,
		firstIterationToolHandlers,
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
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
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

func (s *ZAIService) ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.sessions == nil {
		return "", errors.New("reasoning sessions are not enabled")
	}

	storedReasoningEffort := ""
	if reasoningEffort != nil {
		storedReasoningEffort = *reasoningEffort
	}
	return s.sessions.CreateReserved(model, storedReasoningEffort)
}

func (s *ZAIService) GetEmbeddings(ctx context.Context, content string) ([]float64, error) {
	return nil, errors.New("zai embeddings are not implemented")
}

func (s *ZAIService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *ZAIService) getChatResponseWithUsage(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, jsonSchema *string, reasoningEffort *string, logRawBody bool) (*LLMBotMessage, LLMUsageTotals, error) {
	usageTotals := LLMUsageTotals{}
	normalizedMessages, err := normalizeZAIMessages(messages)
	if err != nil {
		return nil, usageTotals, err
	}
	if jsonSchema != nil {
		// Z.AI only offers JSON mode on chat completions, so we ground the
		// exact schema in the system prompt and still validate client-side.
		normalizedMessages[0].Content = appendStructuredOutputInstruction(normalizedMessages[0].Content, jsonSchema)
	}
	thinking, err := zaiThinkingValue(reasoningEffort)
	if err != nil {
		return nil, usageTotals, err
	}

	req := ZAIChatRequest{
		Model:          model,
		Messages:       normalizedMessages,
		MaxTokens:      maxTokens,
		DoSample:       s.doSample,
		Temperature:    s.temperature,
		TopP:           s.topP,
		Stop:           s.stop,
		ResponseFormat: zaiResponseFormat(jsonSchema),
		Thinking:       thinking,
		Stream:         false,
	}

	var chatResp ZAIChatResponse
	if err := callLLMAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, logRawBody); err != nil {
		return nil, usageTotals, err
	}
	usageTotals.Add(chatResp.Usage)

	msg, err := firstZAIMessage(&chatResp)
	if err != nil {
		return nil, usageTotals, err
	}
	if strings.TrimSpace(msg.Content) == "" {
		return nil, usageTotals, errors.New("zai chat returned empty assistant output")
	}
	return msg, usageTotals, nil
}

func (s *ZAIService) getReasoningResponseWithTools(
	ctx context.Context,
	methodName string,
	jsonSchema *string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	firstIterationTools []ToolCall,
	firstIterationToolHandlers map[string]ToolHandler,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	progressSessionID string,
) (*LLMBotMessage, string, LLMUsageTotals, int, []string, *ReasoningSearchDebugInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("ZAI %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf(
				"ZAI %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t",
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
		return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("tools must contain at least one tool definition")
	}
	if len(toolHandlers) == 0 {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("toolHandlers must contain at least one handler")
	}
	if len(firstIterationTools) > 0 && len(firstIterationToolHandlers) == 0 {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("firstIterationToolHandlers must contain at least one handler when firstIterationTools are provided")
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}

	normalizedMessages, err := normalizeZAIMessages(messages)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	firstIterationNormalizedMessages := normalizedMessages
	if len(firstIterationTools) > 0 {
		firstIterationMessages := WithFirstIterationReasoningSearchSystemMessage(messages, firstIterationTools)
		firstIterationNormalizedMessages, err = normalizeZAIMessages(firstIterationMessages)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
	}
	if jsonSchema != nil {
		// Same grounding as the one-shot path. Without this, glm-5.1 often
		// returns arbitrary JSON objects instead of the required schema.
		normalizedMessages[0].Content = appendStructuredOutputInstruction(normalizedMessages[0].Content, jsonSchema)
		if len(firstIterationTools) > 0 {
			firstIterationNormalizedMessages[0].Content = appendStructuredOutputInstruction(firstIterationNormalizedMessages[0].Content, jsonSchema)
		}
	}
	thinking, err := zaiThinkingValue(reasoningEffort)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	responseFormat := zaiResponseFormat(jsonSchema)
	toolChoice := "auto"
	reasoningCtx := ContextWithReasoningToolState(ContextWithDeb(ctx, deb))

	for i := 0; i < maxIterations; i++ {
		if err := ctx.Err(); err != nil {
			return nil, "", usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
		}
		if s.progress != nil && progressSessionID != "" {
			s.progress.Thinking(progressSessionID, i+1)
		}
		currentTools := tools
		currentToolHandlers := toolHandlers
		if i == 0 && len(firstIterationTools) > 0 {
			currentTools = firstIterationTools
			currentToolHandlers = firstIterationToolHandlers
		}
		currentMessages := normalizedMessages
		if i == 0 && len(firstIterationTools) > 0 {
			currentMessages = firstIterationNormalizedMessages
		}

		req := ZAIChatRequest{
			Model:          model,
			Messages:       currentMessages,
			MaxTokens:      maxTokens,
			DoSample:       s.doSample,
			Temperature:    s.temperature,
			TopP:           s.topP,
			Stop:           s.stop,
			ResponseFormat: responseFormat,
			Thinking:       thinking,
			Tools:          currentTools,
			ToolChoice:     &toolChoice,
			Stream:         false,
		}

		var chatResp ZAIChatResponse
		if err := callLLMAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, deb); err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		iterations = i + 1
		usageTotals.Add(chatResp.Usage)

		message, err := firstZAIChoice(&chatResp)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		if strings.TrimSpace(message.ReasoningContent) != "" {
			summary := strings.TrimSpace(message.ReasoningContent)
			reasoningSummaries = append(reasoningSummaries, summary)
			if deb {
				log.Printf("LLM reasoning summary iteration %d:\n%s", i+1, summary)
			}
		}

		if len(message.ToolCalls) == 0 {
			if strings.TrimSpace(message.Content) == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("zai chat returned empty assistant output")
			}
			return &LLMBotMessage{
				Role:    "assistant",
				Content: message.Content,
			}, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		assistantMessage, err := zaiAssistantMessage(message)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		normalizedMessages = append(normalizedMessages, *assistantMessage)

		toolCallLogs := []string{}
		for _, toolCall := range message.ToolCalls {
			if err := ctx.Err(); err != nil {
				return nil, "", usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
			}
			if toolCall.Function.Name == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("zai tool call is missing function name")
			}
			canonicalToolName := CanonicalReasoningToolName(toolCall.Function.Name)
			if !usedToolsSet[canonicalToolName] {
				usedTools = append(usedTools, canonicalToolName)
				usedToolsSet[canonicalToolName] = true
			}

			handler, ok := ResolveReasoningToolHandler(toolCall.Function.Name, currentToolHandlers, firstIterationToolHandlers)
			if !ok {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, fmt.Errorf("missing handler for tool '%s'", toolCall.Function.Name)
			}

			rawArgs := toolCall.Function.Arguments.Raw
			if len(rawArgs) == 0 {
				rawArgs = json.RawMessage("{}")
			}
			if deb {
				toolCallLogs = append(toolCallLogs, fmt.Sprintf("- %s args: %s", toolCall.Function.Name, compactToolCallArguments(rawArgs)))
			}

			if s.progress != nil && progressSessionID != "" {
				s.progress.RunningTool(progressSessionID, i+1, canonicalToolName)
			}
			result, err := handler(reasoningCtx, rawArgs)
			if err != nil {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, fmt.Errorf("tool '%s' execution failed: %w", toolCall.Function.Name, err)
			}

			normalizedMessages = append(normalizedMessages, LLMBotMessage{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}

		if deb && len(toolCallLogs) > 0 {
			reasoningSummaries = append(reasoningSummaries, "Tool calls:\n"+strings.Join(toolCallLogs, "\n"))
		}
	}

	return nil, "", LLMUsageTotals{}, 0, nil, ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func normalizeZAIMessages(messages []LLMBotMessage) ([]LLMBotMessage, error) {
	sysMsgCount := 0
	normalized := make([]LLMBotMessage, 0, len(messages))

	for _, message := range messages {
		normalizedMessage := message
		if normalizedMessage.Role == "developer" {
			normalizedMessage.Role = "system"
		}
		switch normalizedMessage.Role {
		case "system":
			sysMsgCount++
		case "user", "assistant":
		case "tool":
			if normalizedMessage.ToolCallID == "" {
				return nil, errors.New("tool message is missing tool_call_id")
			}
		default:
			return nil, fmt.Errorf("unsupported message role for Z.AI: %s", normalizedMessage.Role)
		}
		normalized = append(normalized, normalizedMessage)
	}

	if sysMsgCount != 1 {
		return nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}
	return normalized, nil
}

func zaiResponseFormat(jsonSchema *string) *ZAIResponseFormat {
	if jsonSchema == nil {
		return nil
	}
	return &ZAIResponseFormat{Type: "json_object"}
}

func zaiThinkingValue(reasoningEffort *string) (*ZAIThinking, error) {
	if reasoningEffort == nil {
		return nil, nil
	}

	// Z.AI exposes thinking as enabled/disabled rather than graded effort.
	switch *reasoningEffort {
	case "minimal":
		return &ZAIThinking{Type: "disabled", ClearThinking: true}, nil
	case "low", "medium", "high", "xhigh":
		return &ZAIThinking{Type: "enabled", ClearThinking: true}, nil
	default:
		return nil, fmt.Errorf("reasoning effort %q is not supported for Z.AI models; supported values are minimal, low, medium, high, xhigh", *reasoningEffort)
	}
}

func firstZAIChoice(resp *ZAIChatResponse) (*ZAIMessage, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, errors.New("zai chat returned no choices")
	}
	for _, choice := range resp.Choices {
		if choice.Index == 0 {
			return &choice.Message, nil
		}
	}
	return &resp.Choices[0].Message, nil
}

func firstZAIMessage(resp *ZAIChatResponse) (*LLMBotMessage, error) {
	message, err := firstZAIChoice(resp)
	if err != nil {
		return nil, err
	}
	return &LLMBotMessage{
		Role:    message.Role,
		Content: message.Content,
	}, nil
}

func zaiAssistantMessage(message *ZAIMessage) (*LLMBotMessage, error) {
	toolCalls := make([]MessageToolCall, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		rawArgs := toolCall.Function.Arguments.Raw
		if len(rawArgs) == 0 {
			rawArgs = json.RawMessage("{}")
		}
		toolCalls = append(toolCalls, MessageToolCall{
			ID:   toolCall.ID,
			Type: "function",
			Function: ToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: compactToolCallArguments(rawArgs),
			},
		})
	}
	return &LLMBotMessage{
		Role:      "assistant",
		Content:   message.Content,
		ToolCalls: toolCalls,
	}, nil
}

func normalizeZAIAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultZAIAPIBaseURL
	}

	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	for _, suffix := range []string{"/api/paas/v4/chat/completions", "/chat/completions", "/api/paas/v4"} {
		if strings.HasSuffix(apiBaseURL, suffix) {
			apiBaseURL = strings.TrimSuffix(apiBaseURL, suffix)
			break
		}
	}
	return strings.TrimRight(apiBaseURL, "/") + "/api/paas/v4"
}
