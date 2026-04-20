package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
)

const defaultArceeAPIBaseURL = "https://api.arcee.ai/api/v1"

type ArceeService struct {
	*BaseLLMService
	sessions *ChatReasoningSessionStore
}

type ArceeChatRequest struct {
	Model           string               `json:"model"`
	Messages        []LLMBotMessage      `json:"messages"`
	MaxTokens       *int                 `json:"max_tokens,omitempty"`
	Temperature     *float64             `json:"temperature,omitempty"`
	TopP            *float64             `json:"top_p,omitempty"`
	Stop            []string             `json:"stop,omitempty"`
	ResponseFormat  *ArceeResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort *string              `json:"reasoning_effort,omitempty"`
	Tools           []ToolCall           `json:"tools,omitempty"`
	ToolChoice      *string              `json:"tool_choice,omitempty"`
	Stream          bool                 `json:"stream"`
}

type ArceeResponseFormat struct {
	Type string `json:"type"`
}

type ArceeToolArguments struct {
	Raw json.RawMessage
}

func (a *ArceeToolArguments) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		a.Raw = json.RawMessage("{}")
		return nil
	}

	// Arcee may return function arguments either as a JSON string or a JSON value.
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

type ArceeToolCallFunction struct {
	Name      string             `json:"name"`
	Arguments ArceeToolArguments `json:"arguments"`
}

type ArceeToolCall struct {
	ID       string                `json:"id,omitempty"`
	Type     string                `json:"type,omitempty"`
	Function ArceeToolCallFunction `json:"function"`
}

type ArceeMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	Reasoning        string          `json:"reasoning,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []ArceeToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
}

type ArceeChatResponse struct {
	Choices []struct {
		Index   int          `json:"index"`
		Message ArceeMessage `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

var _ Service = (*ArceeService)(nil)

func NewArceeService(token string) *ArceeService {
	return NewArceeServiceWithOptions(token, nil, nil, "")
}

func NewArceeServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *ArceeService {
	base := newBaseLLMService(token, pricing, normalizeArceeAPIBaseURL(apiBaseURL))

	return &ArceeService{
		BaseLLMService: base,
		sessions:       sessions,
	}
}

func (s *ArceeService) GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, &jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Arcee GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
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

func (s *ArceeService) GetStructuredOutputWithDebugInfo(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, &jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Arcee GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
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

func (s *ArceeService) GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Arcee GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *ArceeService) GetChatResponseWithDebugInfo(model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(model, maxTokens, messages, jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Arcee GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ArceeService) GetReasoningResponseWithTools(
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
	msg, _, _, _, _, _, err := s.getReasoningResponseWithTools("GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
	return msg, err
}

func (s *ArceeService) GetReasoningStructuredOutputWithTools(
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
	msg, reasoningSummary, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools("GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
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

func (s *ArceeService) GetReasoningStructuredOutputWithToolsForSession(
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

func (s *ArceeService) ReserveReasoningSession(model string, reasoningEffort *string) (string, error) {
	if s.sessions == nil {
		return "", errors.New("reasoning sessions are not enabled")
	}

	storedReasoningEffort := ""
	if reasoningEffort != nil {
		storedReasoningEffort = *reasoningEffort
	}
	return s.sessions.CreateReserved(model, storedReasoningEffort)
}

func (s *ArceeService) GetEmbeddings(content string) ([]float64, error) {
	return nil, errors.New("arcee embeddings are not implemented")
}

func (s *ArceeService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *ArceeService) getChatResponseWithUsage(model string, maxTokens *int, messages []LLMBotMessage, jsonSchema *string, reasoningEffort *string, logRawBody bool) (*LLMBotMessage, LLMUsageTotals, error) {
	usageTotals := LLMUsageTotals{}
	normalizedMessages, err := normalizeArceeMessages(messages)
	if err != nil {
		return nil, usageTotals, err
	}
	responseFormat, err := arceeResponseFormat(jsonSchema)
	if err != nil {
		return nil, usageTotals, err
	}
	if jsonSchema != nil {
		normalizedMessages[0].Content = appendStructuredOutputInstruction(normalizedMessages[0].Content, jsonSchema)
	}
	reasoningEffort, err = arceeReasoningEffortValue(model, reasoningEffort)
	if err != nil {
		return nil, usageTotals, err
	}

	req := ArceeChatRequest{
		Model:           model,
		Messages:        normalizedMessages,
		MaxTokens:       maxTokens,
		ResponseFormat:  responseFormat,
		ReasoningEffort: reasoningEffort,
		Stream:          false,
	}

	var chatResp ArceeChatResponse
	if err := callLLMAPI(s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, logRawBody); err != nil {
		return nil, usageTotals, err
	}
	usageTotals.Add(chatResp.Usage)

	msg, err := firstArceeMessage(&chatResp)
	if err != nil {
		return nil, usageTotals, err
	}
	if strings.TrimSpace(msg.Content) == "" {
		return nil, usageTotals, errors.New("arcee chat returned empty assistant output")
	}
	return msg, usageTotals, nil
}

func (s *ArceeService) getReasoningResponseWithTools(
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
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("Arcee %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf(
				"Arcee %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t",
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

	normalizedMessages, err := normalizeArceeMessages(messages)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	firstIterationNormalizedMessages := normalizedMessages
	if len(firstIterationTools) > 0 {
		firstIterationMessages := WithFirstIterationReasoningSearchSystemMessage(messages, firstIterationTools)
		firstIterationNormalizedMessages, err = normalizeArceeMessages(firstIterationMessages)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
	}
	if _, err := arceeResponseFormat(jsonSchema); err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	reasoningEffort, err = arceeReasoningEffortValue(model, reasoningEffort)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	toolChoice := "auto"
	reasoningCtx := ContextWithReasoningToolState(ContextWithDeb(context.Background(), deb))

	for i := 0; i < maxIterations; i++ {
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

		req := ArceeChatRequest{
			Model:           model,
			Messages:        currentMessages,
			MaxTokens:       maxTokens,
			ResponseFormat:  nil,
			ReasoningEffort: reasoningEffort,
			Tools:           currentTools,
			ToolChoice:      &toolChoice,
			Stream:          false,
		}

		var chatResp ArceeChatResponse
		if err := callLLMAPI(s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, deb); err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		iterations = i + 1
		usageTotals.Add(chatResp.Usage)

		message, err := firstArceeChoice(&chatResp)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		if strings.TrimSpace(message.reasoningText()) != "" {
			summary := strings.TrimSpace(message.reasoningText())
			reasoningSummaries = append(reasoningSummaries, summary)
			if deb {
				log.Printf("LLM reasoning summary iteration %d:\n%s", i+1, summary)
			}
		}

		if len(message.ToolCalls) == 0 {
			content := message.contentText()
			if strings.TrimSpace(content) == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("arcee chat returned empty assistant output")
			}
			if len(usedTools) == 0 {
				normalizedMessages = append(normalizedMessages, LLMBotMessage{
					Role:      "assistant",
					Content:   content,
					Reasoning: message.reasoningText(),
				}, LLMBotMessage{
					Role:    "user",
					Content: "Use the available archive search tools before giving final results. Do not invent or use example result IDs.",
				})
				continue
			}
			if jsonSchema != nil {
				if err := validateJSONRequiredTopLevelFields(content, *jsonSchema); err != nil {
					normalizedMessages = append(normalizedMessages, LLMBotMessage{
						Role:      "assistant",
						Content:   content,
						Reasoning: message.reasoningText(),
					})
					msg, finalizeUsage, err := s.getChatResponseWithUsage(model, maxTokens, normalizedMessages, jsonSchema, reasoningEffort, deb)
					usageTotals.InputTokens += finalizeUsage.InputTokens
					usageTotals.CachedInputTokens += finalizeUsage.CachedInputTokens
					usageTotals.OutputTokens += finalizeUsage.OutputTokens
					usageTotals.ReasoningTokens += finalizeUsage.ReasoningTokens
					usageTotals.TotalTokens += finalizeUsage.TotalTokens
					if err != nil {
						return nil, "", LLMUsageTotals{}, 0, nil, nil, err
					}
					return msg, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations + 1, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
				}
			}
			return &LLMBotMessage{
				Role:      "assistant",
				Content:   content,
				Reasoning: message.reasoningText(),
			}, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		assistantMessage, err := arceeAssistantMessage(message)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		normalizedMessages = append(normalizedMessages, *assistantMessage)

		toolCallLogs := []string{}
		for _, toolCall := range message.ToolCalls {
			if toolCall.Function.Name == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("arcee tool call is missing function name")
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
			result = sanitizeArceeToolResult(model, canonicalToolName, result)

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

func normalizeArceeMessages(messages []LLMBotMessage) ([]LLMBotMessage, error) {
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
			return nil, fmt.Errorf("unsupported message role for Arcee: %s", normalizedMessage.Role)
		}
		normalized = append(normalized, normalizedMessage)
	}

	if sysMsgCount != 1 {
		return nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}
	return normalized, nil
}

func arceeResponseFormat(jsonSchema *string) (*ArceeResponseFormat, error) {
	if jsonSchema == nil {
		return nil, nil
	}

	if !json.Valid([]byte(*jsonSchema)) {
		return nil, fmt.Errorf("invalid json_schema")
	}
	return &ArceeResponseFormat{Type: "json_object"}, nil
}

func arceeReasoningEffortValue(model string, reasoningEffort *string) (*string, error) {
	if reasoningEffort == nil {
		return nil, nil
	}

	effort := strings.TrimSpace(*reasoningEffort)
	switch effort {
	case "":
		return nil, nil
	case "minimal", "low", "medium", "high":
		if arceeSkipsReasoningEffort(model) {
			return nil, nil
		}
		return &effort, nil
	default:
		return nil, fmt.Errorf("reasoning effort %q is not supported for Arcee models; supported values are minimal, low, medium, high", *reasoningEffort)
	}
}

func arceeSkipsReasoningEffort(model string) bool {
	normalized := normalizeArceeModelName(model)
	return normalized == "trinity-large-thinking"
}

func arceeIsTrinityModel(model string) bool {
	return strings.HasPrefix(normalizeArceeModelName(model), "trinity-")
}

func normalizeArceeModelName(model string) string {
	normalized := strings.TrimSpace(strings.ToLower(model))
	normalized = strings.TrimPrefix(normalized, "arcee-ai/")
	return normalized
}

func sanitizeArceeToolResult(model string, canonicalToolName string, result string) string {
	if !arceeIsTrinityModel(model) || canonicalToolName != "elasticsearch_search" {
		return result
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return result
	}
	removeJSONKey(payload, "_id")
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return result
	}
	return string(sanitized)
}

func removeJSONKey(value interface{}, key string) {
	switch typed := value.(type) {
	case map[string]interface{}:
		delete(typed, key)
		for _, child := range typed {
			removeJSONKey(child, key)
		}
	case []interface{}:
		for _, child := range typed {
			removeJSONKey(child, key)
		}
	}
}

func (m *ArceeMessage) reasoningText() string {
	if strings.TrimSpace(m.Reasoning) != "" {
		return m.Reasoning
	}
	return m.ReasoningContent
}

func (m *ArceeMessage) contentText() string {
	if strings.TrimSpace(m.Content) != "" {
		return m.Content
	}
	return m.ReasoningContent
}

func firstArceeChoice(resp *ArceeChatResponse) (*ArceeMessage, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, errors.New("arcee chat returned no choices")
	}
	for _, choice := range resp.Choices {
		if choice.Index == 0 {
			return &choice.Message, nil
		}
	}
	return &resp.Choices[0].Message, nil
}

func firstArceeMessage(resp *ArceeChatResponse) (*LLMBotMessage, error) {
	message, err := firstArceeChoice(resp)
	if err != nil {
		return nil, err
	}
	return &LLMBotMessage{
		Role:      message.Role,
		Content:   message.contentText(),
		Reasoning: message.reasoningText(),
	}, nil
}

func arceeAssistantMessage(message *ArceeMessage) (*LLMBotMessage, error) {
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
		Content:   message.contentText(),
		Reasoning: message.reasoningText(),
		ToolCalls: toolCalls,
	}, nil
}

func normalizeArceeAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultArceeAPIBaseURL
	}

	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/chat/completions") {
		apiBaseURL = strings.TrimSuffix(apiBaseURL, "/chat/completions")
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")

	parsed, err := url.Parse(apiBaseURL)
	if err == nil && strings.Trim(parsed.Path, "/") == "" {
		if parsed.Host == "conductor.arcee.ai" || parsed.Host == "models.arcee.ai" {
			return apiBaseURL + "/v1"
		}
		return apiBaseURL + "/api/v1"
	}
	return apiBaseURL
}
