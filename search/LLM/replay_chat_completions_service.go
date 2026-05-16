package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type ReplayChatCompletionsService struct {
	*BaseLLMService
	sessions             *ChatReasoningSessionStore
	serviceName          string
	reasoningEffortValue func(model string, reasoningEffort *string) (*string, error)
	// Opt-in only for providers with observed transient 5xx instability.
	retryTransientErrors bool
}

var _ Service = (*ReplayChatCompletionsService)(nil)

var errChatCompletionsToolCallsInPlainResponse = errors.New("chat completions returned tool calls in a plain structured-output response")
var errChatCompletionsLengthWithoutOutput = errors.New("chat completions reached max_tokens before producing output")

type ChatCompletionsRequest struct {
	Model           string                         `json:"model"`
	Messages        []LLMBotMessage                `json:"messages"`
	MaxTokens       *int                           `json:"max_tokens,omitempty"`
	Temperature     *float64                       `json:"temperature,omitempty"`
	TopP            *float64                       `json:"top_p,omitempty"`
	Stop            []string                       `json:"stop,omitempty"`
	ResponseFormat  *ChatCompletionsResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort *string                        `json:"reasoning_effort,omitempty"`
	Tools           []ToolCall                     `json:"tools,omitempty"`
	ToolChoice      *string                        `json:"tool_choice,omitempty"`
	Stream          bool                           `json:"stream"`
}

type ChatCompletionsResponseFormat struct {
	Type string `json:"type"`
}

type ChatCompletionsToolArguments struct {
	Raw json.RawMessage
}

func (a *ChatCompletionsToolArguments) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		a.Raw = json.RawMessage("{}")
		return nil
	}
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

type ChatCompletionsToolCallFunction struct {
	Name      string                       `json:"name"`
	Arguments ChatCompletionsToolArguments `json:"arguments"`
}

type ChatCompletionsToolCall struct {
	ID       string                          `json:"id,omitempty"`
	Type     string                          `json:"type,omitempty"`
	Function ChatCompletionsToolCallFunction `json:"function"`
}

type ChatCompletionsMessage struct {
	Role             string                    `json:"role"`
	Content          string                    `json:"content,omitempty"`
	Reasoning        string                    `json:"reasoning,omitempty"`
	ReasoningContent string                    `json:"reasoning_content,omitempty"`
	ToolCalls        []ChatCompletionsToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string                    `json:"tool_call_id,omitempty"`
}

type ChatCompletionsResponse struct {
	Choices []struct {
		Index        int                    `json:"index"`
		Message      ChatCompletionsMessage `json:"message"`
		FinishReason string                 `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

func (s *ReplayChatCompletionsService) GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("%s GetStructuredOutput total tokens: %d", s.label(), usageTotals.TotalTokens)
	}
	if err != nil {
		return err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return err
	}
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema %s: %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	return nil
}

func (s *ReplayChatCompletionsService) GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("%s GetStructuredOutputWithDebugInfo total tokens: %d", s.label(), usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return nil, err
	}
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema %s: %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	return s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ReplayChatCompletionsService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("%s GetChatResponse total tokens: %d", s.label(), usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *ReplayChatCompletionsService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("%s GetChatResponseWithDebugInfo total tokens: %d", s.label(), usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ReplayChatCompletionsService) GetReasoningResponseWithTools(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, _ *string, reasoningEffort *string, deb bool, maxIterations int) (*LLMBotMessage, error) {
	msg, _, _, _, _, _, err := s.getReasoningResponseWithTools(ctx, "GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
	return msg, err
}

func (s *ReplayChatCompletionsService) GetReasoningStructuredOutputWithTools(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, _ *string, reasoningEffort *string, deb bool, maxIterations int, output interface{}) error {
	msg, reasoningSteps, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools(ctx, "GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
	if err != nil {
		return err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return err
	}
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema %s: %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	if deb && len(reasoningSteps) > 0 {
		if setter, ok := output.(reasoningStepsSetter); ok {
			setter.SetReasoningSteps(reasoningSteps)
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

func (s *ReplayChatCompletionsService) GetReasoningStructuredOutputWithToolsForSession(ctx context.Context, sessionID *string, progressSessionID *string, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, firstIterationTools []ToolCall, firstIterationToolHandlers map[string]ToolHandler, _ *string, reasoningEffort *string, deb bool, maxIterations int, output interface{}) (string, error) {
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

	msg, reasoningSteps, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools(ctx, "GetReasoningStructuredOutputWithToolsForSession", &jsonSchema, effectiveModel, maxTokens, effectiveMessages, tools, toolHandlers, firstIterationTools, firstIterationToolHandlers, effectiveReasoningEffort, deb, maxIterations, effectiveProgressSessionID)
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
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema %s: %v\nContent: %s", jsonSchema, err, msg.Content)
		if s.progress != nil && effectiveProgressSessionID != "" {
			s.progress.Fail(effectiveProgressSessionID, reasoningIterations)
		}
		return "", err
	}
	if deb && len(reasoningSteps) > 0 {
		if setter, ok := output.(reasoningStepsSetter); ok {
			setter.SetReasoningSteps(reasoningSteps)
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

func (s *ReplayChatCompletionsService) ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error) {
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

func (s *ReplayChatCompletionsService) RefreshReasoningSession(sessionID string) error {
	if s.sessions == nil {
		return errors.New("reasoning sessions are not enabled")
	}
	return s.sessions.Refresh(strings.TrimSpace(sessionID))
}

func (s *ReplayChatCompletionsService) GetEmbeddings(ctx context.Context, content string) ([]float64, error) {
	return nil, errors.New("embeddings are not implemented")
}

func (s *ReplayChatCompletionsService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func normalizeChatCompletionsMessages(messages []LLMBotMessage) ([]LLMBotMessage, error) {
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
			return nil, fmt.Errorf("unsupported message role for chat completions: %s", normalizedMessage.Role)
		}
		normalized = append(normalized, normalizedMessage)
	}
	if sysMsgCount != 1 {
		return nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}
	return normalized, nil
}

func chatCompletionsResponseFormat(jsonSchema *string) (*ChatCompletionsResponseFormat, error) {
	if jsonSchema == nil {
		return nil, nil
	}
	if !json.Valid([]byte(*jsonSchema)) {
		return nil, fmt.Errorf("invalid json_schema")
	}
	return &ChatCompletionsResponseFormat{Type: "json_object"}, nil
}

func (m *ChatCompletionsMessage) reasoningText() string {
	if strings.TrimSpace(m.Reasoning) != "" {
		return m.Reasoning
	}
	return m.ReasoningContent
}

func (m *ChatCompletionsMessage) contentText() string {
	if strings.TrimSpace(m.Content) != "" {
		return m.Content
	}
	return m.ReasoningContent
}

func firstChatCompletionsChoice(resp *ChatCompletionsResponse) (*ChatCompletionsMessage, string, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, "", errors.New("chat completions returned no choices")
	}
	for _, choice := range resp.Choices {
		if choice.Index == 0 {
			return &choice.Message, choice.FinishReason, nil
		}
	}
	return &resp.Choices[0].Message, resp.Choices[0].FinishReason, nil
}

func chatCompletionsAssistantMessage(message *ChatCompletionsMessage) (*LLMBotMessage, error) {
	toolCalls := make([]MessageToolCall, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		rawArgs := toolCall.Function.Arguments.Raw
		if len(rawArgs) == 0 {
			rawArgs = json.RawMessage("{}")
		}
		toolCalls = append(toolCalls, MessageToolCall{ID: toolCall.ID, Type: "function", Function: ToolCallFunction{Name: toolCall.Function.Name, Arguments: compactToolCallArguments(rawArgs)}})
	}
	return &LLMBotMessage{Role: "assistant", Content: message.contentText(), Reasoning: message.reasoningText(), ToolCalls: toolCalls}, nil
}

func (s *ReplayChatCompletionsService) getChatResponseWithUsage(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, jsonSchema *string, reasoningEffort *string, logRawBody bool) (*LLMBotMessage, LLMUsageTotals, error) {
	usageTotals := LLMUsageTotals{}
	normalizedMessages, err := normalizeChatCompletionsMessages(messages)
	if err != nil {
		return nil, usageTotals, err
	}
	responseFormat, err := chatCompletionsResponseFormat(jsonSchema)
	if err != nil {
		return nil, usageTotals, err
	}
	if jsonSchema != nil {
		normalizedMessages[0].Content = appendStructuredOutputInstruction(normalizedMessages[0].Content, jsonSchema)
	}
	reasoningEffort, err = s.normalizeReasoningEffort(model, reasoningEffort)
	if err != nil {
		return nil, usageTotals, err
	}

	req := ChatCompletionsRequest{Model: model, Messages: normalizedMessages, MaxTokens: maxTokens, ResponseFormat: responseFormat, ReasoningEffort: reasoningEffort, Stream: false}
	var chatResp ChatCompletionsResponse
	if err := s.callAPI(ctx, req, s.apiBaseURL+"/chat/completions", &chatResp, logRawBody); err != nil {
		return nil, usageTotals, err
	}
	usageTotals.Add(chatResp.Usage)
	message, finishReason, err := firstChatCompletionsChoice(&chatResp)
	if err != nil {
		return nil, usageTotals, err
	}
	if len(message.ToolCalls) > 0 {
		return nil, usageTotals, errChatCompletionsToolCallsInPlainResponse
	}
	msg := &LLMBotMessage{Role: message.Role, Content: message.contentText(), Reasoning: message.reasoningText()}
	if strings.TrimSpace(msg.Content) == "" {
		if finishReason == "length" {
			return nil, usageTotals, errChatCompletionsLengthWithoutOutput
		}
		return nil, usageTotals, errors.New("chat completions returned empty assistant output")
	}
	return msg, usageTotals, nil
}

func (s *ReplayChatCompletionsService) getReasoningResponseWithTools(ctx context.Context, methodName string, jsonSchema *string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, firstIterationTools []ToolCall, firstIterationToolHandlers map[string]ToolHandler, reasoningEffort *string, deb bool, maxIterations int, progressSessionID string) (*LLMBotMessage, []ReasoningSearchReasoningStep, LLMUsageTotals, int, []string, *ReasoningSearchDebugInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("%s %s total tokens: %d", s.label(), methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf("%s %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t", s.label(), methodName, usageTotals.InputTokens, usageTotals.CachedInputTokens, usageTotals.OutputTokens, usageTotals.ReasoningTokens, usageTotals.TotalTokens, cost.EstimatedCostUSD, cost.PricingConfigured)
		}
	}()
	reasoningSteps := []ReasoningSearchReasoningStep{}
	if len(tools) == 0 {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("tools must contain at least one tool definition")
	}
	if len(toolHandlers) == 0 {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("toolHandlers must contain at least one handler")
	}
	if len(firstIterationTools) > 0 && len(firstIterationToolHandlers) == 0 {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("firstIterationToolHandlers must contain at least one handler when firstIterationTools are provided")
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}

	normalizedMessages, err := normalizeChatCompletionsMessages(messages)
	if err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	firstIterationNormalizedMessages := normalizedMessages
	if len(firstIterationTools) > 0 {
		firstIterationMessages := WithFirstIterationReasoningSearchSystemMessage(messages, firstIterationTools)
		firstIterationNormalizedMessages, err = normalizeChatCompletionsMessages(firstIterationMessages)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
	}
	if _, err := chatCompletionsResponseFormat(jsonSchema); err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	reasoningEffort, err = s.normalizeReasoningEffort(model, reasoningEffort)
	if err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	toolChoice := "auto"
	reasoningCtx := ContextWithReasoningToolState(ContextWithDeb(ctx, deb), s.progress, progressSessionID)

	for i := 0; i < maxIterations; i++ {
		stepStarted := time.Now()
		isFinalIteration := i == maxIterations-1
		currentIteration := i + 1
		isNearFinish := false
		if err := ctx.Err(); err != nil {
			return nil, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
		}
		if s.progress != nil && progressSessionID != "" {
			isNearFinish = s.progress.IsNearFinish(progressSessionID, currentIteration, maxIterations)
			s.progress.Thinking(progressSessionID, currentIteration, isNearFinish)
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
		currentToolChoice := &toolChoice
		if isFinalIteration {
			currentTools = nil
			currentToolHandlers = nil
			currentToolChoice = nil
			currentMessages = append([]LLMBotMessage{}, currentMessages...)
			currentMessages = append(currentMessages, LLMBotMessage{Role: "user", Content: finalReasoningIterationInstruction})
		}

		req := ChatCompletionsRequest{Model: model, Messages: currentMessages, MaxTokens: maxTokens, ResponseFormat: nil, ReasoningEffort: reasoningEffort, Tools: currentTools, ToolChoice: currentToolChoice, Stream: false}
		var chatResp ChatCompletionsResponse
		if err := s.callAPI(ctx, req, s.apiBaseURL+"/chat/completions", &chatResp, deb); err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		iterations = i + 1
		stepUsage := reasoningUsageTotals(chatResp.Usage)
		usageTotals.Add(chatResp.Usage)
		message, finishReason, err := firstChatCompletionsChoice(&chatResp)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		stepThoughts := strings.TrimSpace(message.reasoningText())
		if strings.TrimSpace(message.reasoningText()) != "" && deb {
			log.Printf("LLM reasoning summary iteration %d:\n%s", i+1, stepThoughts)
		}

		if len(message.ToolCalls) == 0 {
			content := message.contentText()
			if strings.TrimSpace(content) == "" {
				if finishReason == "length" {
					return nil, nil, LLMUsageTotals{}, 0, nil, nil, errChatCompletionsLengthWithoutOutput
				}
				return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("chat completions returned empty assistant output")
			}
			if len(usedTools) == 0 {
				normalizedMessages = append(normalizedMessages, LLMBotMessage{Role: "assistant", Content: content, Reasoning: message.reasoningText()}, LLMBotMessage{Role: "user", Content: "Use the available archive search tools before giving final results. Do not invent or use example result IDs."})
				reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
				continue
			}
			if jsonSchema != nil {
				if err := validateJSONRequiredTopLevelFields(content, *jsonSchema); err != nil {
					normalizedMessages = append(normalizedMessages, LLMBotMessage{Role: "assistant", Content: content, Reasoning: message.reasoningText()})
					msg, finalizeUsage, err := s.getChatResponseWithUsage(ctx, model, maxTokens, normalizedMessages, jsonSchema, reasoningEffort, deb)
					if errors.Is(err, errChatCompletionsToolCallsInPlainResponse) {
						// Some chat-completions providers still emit tool calls after tools were removed.
						// Keep the recovery local to reasoning finalization so plain structured calls stay strict.
						normalizedMessages = append(normalizedMessages, LLMBotMessage{Role: "user", Content: "Tools are not available in this final response step. Return only a complete JSON object that matches the required schema, using the archive results already found. Do not call tools."})
						var correctionUsage LLMUsageTotals
						msg, correctionUsage, err = s.getChatResponseWithUsage(ctx, model, maxTokens, normalizedMessages, jsonSchema, reasoningEffort, deb)
						finalizeUsage.AddTotals(correctionUsage)
					}
					usageTotals.AddTotals(finalizeUsage)
					stepUsage.AddTotals(finalizeUsage)
					if err != nil {
						return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
					}
					if err := validateJSONRequiredTopLevelFields(msg.Content, *jsonSchema); err != nil {
						normalizedMessages = append(normalizedMessages, LLMBotMessage{Role: "assistant", Content: msg.Content, Reasoning: msg.Reasoning}, LLMBotMessage{Role: "user", Content: fmt.Sprintf("The previous response is invalid: %v. Return only a complete JSON object that matches the required schema, using the archive results already found. Do not return an empty object.", err)})
						msg, retryUsage, err := s.getChatResponseWithUsage(ctx, model, maxTokens, normalizedMessages, jsonSchema, reasoningEffort, deb)
						if errors.Is(err, errChatCompletionsToolCallsInPlainResponse) {
							// Same provider quirk can happen after the schema correction prompt.
							normalizedMessages = append(normalizedMessages, LLMBotMessage{Role: "user", Content: "Tools are not available in this final response step. Return only a complete JSON object that matches the required schema, using the archive results already found. Do not call tools."})
							var correctionUsage LLMUsageTotals
							msg, correctionUsage, err = s.getChatResponseWithUsage(ctx, model, maxTokens, normalizedMessages, jsonSchema, reasoningEffort, deb)
							retryUsage.AddTotals(correctionUsage)
						}
						usageTotals.AddTotals(retryUsage)
						stepUsage.AddTotals(retryUsage)
						if err != nil {
							return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
						}
						if err := validateJSONRequiredTopLevelFields(msg.Content, *jsonSchema); err != nil {
							return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
						}
						reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
						return msg, reasoningSteps, usageTotals, iterations + 2, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
					}
					reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
					return msg, reasoningSteps, usageTotals, iterations + 1, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
				}
			}
			reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
			return &LLMBotMessage{Role: "assistant", Content: content, Reasoning: message.reasoningText()}, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		assistantMessage, err := chatCompletionsAssistantMessage(message)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		normalizedMessages = append(normalizedMessages, *assistantMessage)
		stepToolCalls := []ReasoningSearchReasoningToolCall{}
		toolExecutions := make([]ReasoningToolExecution, 0, len(message.ToolCalls))
		for _, toolCall := range message.ToolCalls {
			if err := ctx.Err(); err != nil {
				return nil, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
			}
			if toolCall.Function.Name == "" {
				return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("chat completions tool call is missing function name")
			}
			canonicalToolName := CanonicalReasoningToolName(toolCall.Function.Name)
			if !usedToolsSet[canonicalToolName] {
				usedTools = append(usedTools, canonicalToolName)
				usedToolsSet[canonicalToolName] = true
			}
			rawArgs := toolCall.Function.Arguments.Raw
			if len(rawArgs) == 0 {
				rawArgs = json.RawMessage("{}")
			}
			if deb {
				stepToolCalls = append(stepToolCalls, reasoningToolCallDebug(toolCall.Function.Name, rawArgs))
			}
			if s.progress != nil && progressSessionID != "" {
				s.progress.RunningTool(progressSessionID, i+1, canonicalToolName)
			}
			toolExecutions = append(toolExecutions, ReasoningToolExecution{Name: toolCall.Function.Name, Arguments: rawArgs})
		}
		toolResults, err := ExecuteReasoningToolExecutions(reasoningCtx, toolExecutions, currentToolHandlers, firstIterationToolHandlers)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		for idx, toolCall := range message.ToolCalls {
			normalizedMessages = append(normalizedMessages, LLMBotMessage{Role: "tool", ToolCallID: toolCall.ID, Content: toolResults[idx].Output})
		}
		reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, stepToolCalls, stepUsage, stepStarted)
	}
	return nil, reasoningSteps, LLMUsageTotals{}, 0, nil, ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func (s *ReplayChatCompletionsService) label() string {
	if strings.TrimSpace(s.serviceName) == "" {
		return "ChatCompletions"
	}
	return s.serviceName
}

func (s *ReplayChatCompletionsService) callAPI(ctx context.Context, data interface{}, endpoint string, result interface{}, logRawBody bool) error {
	if s.retryTransientErrors {
		return callLLMAPIWithTransientRetry(ctx, s.client, s.token, data, endpoint, result, logRawBody)
	}
	return callLLMAPI(ctx, s.client, s.token, data, endpoint, result, logRawBody)
}

func (s *ReplayChatCompletionsService) normalizeReasoningEffort(model string, reasoningEffort *string) (*string, error) {
	if s != nil && s.reasoningEffortValue != nil {
		return s.reasoningEffortValue(model, reasoningEffort)
	}
	return reasoningEffort, nil
}
