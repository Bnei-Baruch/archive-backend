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

const defaultDeepSeekAPIBaseURL = "https://api.deepseek.com"

type DeepSeekService struct {
	*BaseLLMService
	sessions *ChatReasoningSessionStore
}

type DeepSeekChatRequest struct {
	Model           string                   `json:"model"`
	Messages        []DeepSeekRequestMessage `json:"messages"`
	MaxTokens       *int                     `json:"max_tokens,omitempty"`
	ResponseFormat  *DeepSeekResponseFormat  `json:"response_format,omitempty"`
	Thinking        *DeepSeekThinking        `json:"thinking,omitempty"`
	ReasoningEffort *string                  `json:"reasoning_effort,omitempty"`
	Tools           []ToolCall               `json:"tools,omitempty"`
	ToolChoice      *string                  `json:"tool_choice,omitempty"`
	Stream          bool                     `json:"stream"`
}

type DeepSeekResponseFormat struct {
	Type string `json:"type"`
}

type DeepSeekThinking struct {
	Type string `json:"type"`
}

type DeepSeekRequestMessage struct {
	Role             string            `json:"role"`
	Content          string            `json:"content"`
	ReasoningContent string            `json:"reasoning_content,omitempty"`
	ToolCalls        []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string            `json:"tool_call_id,omitempty"`
}

type DeepSeekToolArguments struct {
	Raw json.RawMessage
}

func (a *DeepSeekToolArguments) UnmarshalJSON(data []byte) error {
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

type DeepSeekToolCallFunction struct {
	Name      string                `json:"name"`
	Arguments DeepSeekToolArguments `json:"arguments"`
}

type DeepSeekToolCall struct {
	ID       string                   `json:"id,omitempty"`
	Type     string                   `json:"type,omitempty"`
	Function DeepSeekToolCallFunction `json:"function"`
}

type DeepSeekMessage struct {
	Role             string             `json:"role"`
	Content          string             `json:"content,omitempty"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	ToolCalls        []DeepSeekToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
}

type DeepSeekChatResponse struct {
	Choices []struct {
		Index   int             `json:"index"`
		Message DeepSeekMessage `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

var _ Service = (*DeepSeekService)(nil)

func NewDeepSeekService(token string) *DeepSeekService {
	return NewDeepSeekServiceWithOptions(token, nil, nil, "")
}

func NewDeepSeekServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *DeepSeekService {
	return &DeepSeekService{
		BaseLLMService: newBaseLLMService(token, pricing, normalizeDeepSeekAPIBaseURL(apiBaseURL)),
		sessions:       sessions,
	}
}

func (s *DeepSeekService) GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("DeepSeek GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
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

func (s *DeepSeekService) GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("DeepSeek GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
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

func (s *DeepSeekService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("DeepSeek GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *DeepSeekService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("DeepSeek GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *DeepSeekService) GetReasoningResponseWithTools(
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

func (s *DeepSeekService) GetReasoningStructuredOutputWithTools(
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
	msg, reasoningSteps, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools(ctx, "GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
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

func (s *DeepSeekService) GetReasoningStructuredOutputWithToolsForSession(
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

	msg, reasoningSteps, usageTotals, reasoningIterations, usedTools, toolDebug, err := s.getReasoningResponseWithTools(
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

func (s *DeepSeekService) ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error) {
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

func (s *DeepSeekService) RefreshReasoningSession(sessionID string) error {
	if s.sessions == nil {
		return errors.New("reasoning sessions are not enabled")
	}
	return s.sessions.Refresh(strings.TrimSpace(sessionID))
}

func (s *DeepSeekService) GetEmbeddings(ctx context.Context, content string) ([]float64, error) {
	return nil, errors.New("deepseek embeddings are not implemented")
}

func (s *DeepSeekService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *DeepSeekService) getChatResponseWithUsage(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, jsonSchema *string, reasoningEffort *string, logRawBody bool) (*LLMBotMessage, LLMUsageTotals, error) {
	usageTotals := LLMUsageTotals{}
	normalizedMessages, err := normalizeDeepSeekMessages(messages)
	if err != nil {
		return nil, usageTotals, err
	}
	responseFormat, err := deepseekResponseFormat(jsonSchema)
	if err != nil {
		return nil, usageTotals, err
	}
	if jsonSchema != nil {
		normalizedMessages[0].Content = appendStructuredOutputInstruction(normalizedMessages[0].Content, jsonSchema)
	}
	thinking, effort, err := deepseekThinkingAndEffort(reasoningEffort)
	if err != nil {
		return nil, usageTotals, err
	}

	req := DeepSeekChatRequest{
		Model:           model,
		Messages:        normalizedMessages,
		MaxTokens:       maxTokens,
		ResponseFormat:  responseFormat,
		Thinking:        thinking,
		ReasoningEffort: effort,
		Stream:          false,
	}

	var chatResp DeepSeekChatResponse
	if err := callLLMAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, logRawBody); err != nil {
		return nil, usageTotals, err
	}
	usageTotals.Add(chatResp.Usage)

	msg, err := firstDeepSeekMessage(&chatResp)
	if err != nil {
		return nil, usageTotals, err
	}
	if strings.TrimSpace(msg.Content) == "" {
		return nil, usageTotals, errors.New("deepseek chat returned empty assistant output")
	}
	return msg, usageTotals, nil
}

func (s *DeepSeekService) getReasoningResponseWithTools(
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
) (*LLMBotMessage, []ReasoningSearchReasoningStep, LLMUsageTotals, int, []string, *ReasoningSearchDebugInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("DeepSeek %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf(
				"DeepSeek %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t",
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

	normalizedMessages, err := normalizeDeepSeekMessages(messages)
	if err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	firstIterationNormalizedMessages := normalizedMessages
	if len(firstIterationTools) > 0 {
		firstIterationMessages := WithFirstIterationReasoningSearchSystemMessage(messages, firstIterationTools)
		firstIterationNormalizedMessages, err = normalizeDeepSeekMessages(firstIterationMessages)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
	}
	responseFormat, err := deepseekResponseFormat(jsonSchema)
	if err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	if jsonSchema != nil {
		normalizedMessages[0].Content = appendStructuredOutputInstruction(normalizedMessages[0].Content, jsonSchema)
		if len(firstIterationTools) > 0 {
			firstIterationNormalizedMessages[0].Content = appendStructuredOutputInstruction(firstIterationNormalizedMessages[0].Content, jsonSchema)
		}
	}
	thinking, effort, err := deepseekThinkingAndEffort(reasoningEffort)
	if err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	toolChoice := "auto"
	reasoningCtx := ContextWithReasoningToolState(ContextWithDeb(ctx, deb), s.progress, progressSessionID)

	for i := 0; i < maxIterations; i++ {
		stepStarted := time.Now()
		isFinalIteration := i == maxIterations-1
		if err := ctx.Err(); err != nil {
			return nil, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
		}
		if s.progress != nil && progressSessionID != "" {
			s.progress.Thinking(progressSessionID, i+1, isFinalIteration)
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
		currentThinking := thinking
		currentEffort := effort
		currentReasoningEffort := reasoningEffort
		if isFinalIteration {
			currentTools = nil
			currentToolHandlers = nil
			currentToolChoice = nil
			currentThinking = &DeepSeekThinking{Type: "disabled"}
			currentEffort = nil
			minimalEffort := "minimal"
			currentReasoningEffort = &minimalEffort
			currentMessages = append([]DeepSeekRequestMessage{}, currentMessages...)
			currentMessages = append(currentMessages, DeepSeekRequestMessage{Role: "user", Content: finalReasoningIterationInstruction})
		}

		req := DeepSeekChatRequest{
			Model:           model,
			Messages:        currentMessages,
			MaxTokens:       maxTokens,
			ResponseFormat:  responseFormat,
			Thinking:        currentThinking,
			ReasoningEffort: currentEffort,
			Tools:           currentTools,
			ToolChoice:      currentToolChoice,
			Stream:          false,
		}

		var chatResp DeepSeekChatResponse
		if err := callLLMAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/chat/completions", &chatResp, deb); err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		iterations = i + 1
		stepUsage := reasoningUsageTotals(chatResp.Usage)
		usageTotals.Add(chatResp.Usage)

		message, err := firstDeepSeekChoice(&chatResp)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		stepThoughts := strings.TrimSpace(message.ReasoningContent)
		if strings.TrimSpace(message.ReasoningContent) != "" {
			if deb {
				log.Printf("LLM reasoning summary iteration %d:\n%s", i+1, stepThoughts)
			}
		}

		if len(message.ToolCalls) == 0 {
			if strings.TrimSpace(message.Content) == "" {
				return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("deepseek chat returned empty assistant output")
			}
			if len(usedTools) == 0 {
				normalizedMessages = append(normalizedMessages, DeepSeekRequestMessage{Role: "assistant", Content: message.Content, ReasoningContent: message.ReasoningContent}, DeepSeekRequestMessage{Role: "user", Content: "Use the available archive search tools before giving final results. Do not invent or use example result IDs."})
				reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
				continue
			}
			if jsonSchema != nil {
				if err := validateJSONRequiredTopLevelFields(message.Content, *jsonSchema); err != nil {
					normalizedMessages = append(normalizedMessages, DeepSeekRequestMessage{Role: "assistant", Content: message.Content, ReasoningContent: message.ReasoningContent})
					msg, finalizeUsage, err := s.getChatResponseWithUsage(ctx, model, maxTokens, deepseekMessagesToLLM(normalizedMessages), jsonSchema, currentReasoningEffort, deb)
					usageTotals.AddTotals(finalizeUsage)
					stepUsage.AddTotals(finalizeUsage)
					if err != nil {
						return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
					}
					if err := validateJSONRequiredTopLevelFields(msg.Content, *jsonSchema); err != nil {
						return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
					}
					reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
					return msg, reasoningSteps, usageTotals, iterations + 1, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
				}
			}
			reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
			return &LLMBotMessage{Role: "assistant", Content: message.Content, Reasoning: message.ReasoningContent}, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		assistantMessage := deepseekAssistantRequestMessage(message)
		normalizedMessages = append(normalizedMessages, assistantMessage)

		stepToolCalls := []ReasoningSearchReasoningToolCall{}
		toolExecutions := make([]ReasoningToolExecution, 0, len(message.ToolCalls))
		for _, toolCall := range message.ToolCalls {
			if err := ctx.Err(); err != nil {
				return nil, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
			}
			if toolCall.Function.Name == "" {
				return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("deepseek tool call is missing function name")
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
			normalizedMessages = append(normalizedMessages, DeepSeekRequestMessage{Role: "tool", ToolCallID: toolCall.ID, Content: toolResults[idx].Output})
		}

		reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, stepToolCalls, stepUsage, stepStarted)
	}

	return nil, reasoningSteps, LLMUsageTotals{}, 0, nil, ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func normalizeDeepSeekMessages(messages []LLMBotMessage) ([]DeepSeekRequestMessage, error) {
	sysMsgCount := 0
	normalized := make([]DeepSeekRequestMessage, 0, len(messages))

	for _, message := range messages {
		role := message.Role
		if role == "developer" {
			role = "system"
		}
		switch role {
		case "system":
			sysMsgCount++
		case "user", "assistant":
		case "tool":
			if message.ToolCallID == "" {
				return nil, errors.New("tool message is missing tool_call_id")
			}
		default:
			return nil, fmt.Errorf("unsupported message role for DeepSeek: %s", message.Role)
		}
		normalized = append(normalized, DeepSeekRequestMessage{Role: role, Content: message.Content, ReasoningContent: message.Reasoning, ToolCalls: message.ToolCalls, ToolCallID: message.ToolCallID})
	}

	if sysMsgCount != 1 {
		return nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}
	return normalized, nil
}

func deepseekResponseFormat(jsonSchema *string) (*DeepSeekResponseFormat, error) {
	if jsonSchema == nil {
		return nil, nil
	}
	if !json.Valid([]byte(*jsonSchema)) {
		return nil, fmt.Errorf("invalid json_schema")
	}
	return &DeepSeekResponseFormat{Type: "json_object"}, nil
}

func deepseekThinkingAndEffort(reasoningEffort *string) (*DeepSeekThinking, *string, error) {
	if reasoningEffort == nil {
		return nil, nil, nil
	}
	switch strings.TrimSpace(*reasoningEffort) {
	case "":
		return nil, nil, nil
	case "minimal":
		return &DeepSeekThinking{Type: "disabled"}, nil, nil
	case "low", "medium", "high":
		effort := "high"
		return &DeepSeekThinking{Type: "enabled"}, &effort, nil
	case "xhigh", "max":
		effort := "max"
		return &DeepSeekThinking{Type: "enabled"}, &effort, nil
	default:
		return nil, nil, fmt.Errorf("reasoning effort %q is not supported for DeepSeek models; supported values are minimal, low, medium, high, xhigh, max", *reasoningEffort)
	}
}

func firstDeepSeekChoice(resp *DeepSeekChatResponse) (*DeepSeekMessage, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, errors.New("deepseek chat returned no choices")
	}
	for _, choice := range resp.Choices {
		if choice.Index == 0 {
			return &choice.Message, nil
		}
	}
	return &resp.Choices[0].Message, nil
}

func firstDeepSeekMessage(resp *DeepSeekChatResponse) (*LLMBotMessage, error) {
	message, err := firstDeepSeekChoice(resp)
	if err != nil {
		return nil, err
	}
	return &LLMBotMessage{Role: message.Role, Content: message.Content, Reasoning: message.ReasoningContent}, nil
}

func deepseekAssistantRequestMessage(message *DeepSeekMessage) DeepSeekRequestMessage {
	toolCalls := make([]MessageToolCall, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		rawArgs := toolCall.Function.Arguments.Raw
		if len(rawArgs) == 0 {
			rawArgs = json.RawMessage("{}")
		}
		toolCalls = append(toolCalls, MessageToolCall{ID: toolCall.ID, Type: "function", Function: ToolCallFunction{Name: toolCall.Function.Name, Arguments: compactToolCallArguments(rawArgs)}})
	}
	return DeepSeekRequestMessage{Role: "assistant", Content: message.Content, ReasoningContent: message.ReasoningContent, ToolCalls: toolCalls}
}

func deepseekMessagesToLLM(messages []DeepSeekRequestMessage) []LLMBotMessage {
	ret := make([]LLMBotMessage, 0, len(messages))
	for _, message := range messages {
		ret = append(ret, LLMBotMessage{Role: message.Role, Content: message.Content, Reasoning: message.ReasoningContent, ToolCalls: message.ToolCalls, ToolCallID: message.ToolCallID})
	}
	return ret
}

func normalizeDeepSeekAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultDeepSeekAPIBaseURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/chat/completions") {
		apiBaseURL = strings.TrimSuffix(apiBaseURL, "/chat/completions")
	}
	return strings.TrimRight(apiBaseURL, "/")
}
