package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const (
	defaultOllamaAPIBaseURL = "https://ollama.kab.sh"
	defaultOllamaNumCtx     = 32768
)

type OllamaService struct {
	*BaseLLMService
	sessions                     *ChatReasoningSessionStore
	numCtx                       int
	keepAlive                    string
	temperature                  *float64
	structuredOutputPromptSchema bool
}

type OllamaChatRequest struct {
	Model     string          `json:"model"`
	Messages  []OllamaMessage `json:"messages"`
	Tools     []ToolCall      `json:"tools,omitempty"`
	Format    interface{}     `json:"format,omitempty"`
	Options   *OllamaOptions  `json:"options,omitempty"`
	Stream    bool            `json:"stream"`
	Think     interface{}     `json:"think,omitempty"`
	KeepAlive string          `json:"keep_alive,omitempty"`
}

type OllamaOptions struct {
	NumCtx      *int     `json:"num_ctx,omitempty"`
	NumPredict  *int     `json:"num_predict,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
}

type OllamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
}

type OllamaToolCall struct {
	Type     string                 `json:"type,omitempty"`
	Function OllamaToolCallFunction `json:"function"`
}

type OllamaToolCallFunction struct {
	Index       int                    `json:"index,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`
}

type OllamaChatResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at,omitempty"`
	Message            OllamaMessage `json:"message"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason,omitempty"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

var _ Service = (*OllamaService)(nil)

func NewOllamaService(token string) *OllamaService {
	return NewOllamaServiceWithOptions(token, nil, nil, "", defaultOllamaNumCtx, "", nil, false)
}

func NewOllamaServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string, numCtx int, keepAlive string, temperature *float64, structuredOutputPromptSchema bool) *OllamaService {
	if numCtx <= 0 {
		numCtx = defaultOllamaNumCtx
	}

	base := newBaseLLMService(token, pricing, normalizeOllamaAPIBaseURL(apiBaseURL))

	return &OllamaService{
		BaseLLMService:               base,
		sessions:                     sessions,
		numCtx:                       numCtx,
		keepAlive:                    strings.TrimSpace(keepAlive),
		temperature:                  temperature,
		structuredOutputPromptSchema: structuredOutputPromptSchema,
	}
}

func (s *OllamaService) GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, false, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Ollama GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
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

func (s *OllamaService) GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, false, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Ollama GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(msg.Content), output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	return s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *OllamaService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, false, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Ollama GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *OllamaService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, false, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Ollama GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *OllamaService) GetReasoningResponseWithTools(
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

func (s *OllamaService) GetReasoningStructuredOutputWithTools(
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

func (s *OllamaService) GetReasoningStructuredOutputWithToolsForSession(
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

func (s *OllamaService) ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error) {
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
	sessionID, err := s.sessions.CreateReserved(model, storedReasoningEffort)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func (s *OllamaService) GetEmbeddings(ctx context.Context, content string) ([]float64, error) {
	return nil, errors.New("ollama embeddings are not implemented")
}

func (s *OllamaService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *OllamaService) getChatResponseWithUsage(
	ctx context.Context,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	jsonSchema *string,
	reasoningEffort *string,
	deb bool,
	logRawBody bool,
) (*LLMBotMessage, LLMUsageTotals, error) {
	usageTotals := LLMUsageTotals{}
	ollamaMessages, err := buildInitialOllamaMessages(messages, jsonSchema, s.structuredOutputPromptSchema)
	if err != nil {
		return nil, usageTotals, err
	}
	format, err := buildOllamaFormat(jsonSchema)
	if err != nil {
		return nil, usageTotals, err
	}
	think, err := ollamaThinkValue(reasoningEffort, deb)
	if err != nil {
		return nil, usageTotals, err
	}

	req := s.newChatRequest(model, maxTokens, ollamaMessages, nil, format, think)
	var resp OllamaChatResponse
	if err := callLLMAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/api/chat", &resp, logRawBody); err != nil {
		return nil, usageTotals, err
	}
	usageTotals = resp.usageTotals()
	s.logThinkingIfDeb(deb, 1, resp.Message.Thinking)
	if strings.TrimSpace(resp.Message.Content) == "" {
		return nil, usageTotals, errors.New("ollama chat returned empty assistant output")
	}

	return &LLMBotMessage{
		Role:    "assistant",
		Content: resp.Message.Content,
	}, usageTotals, nil
}

func (s *OllamaService) getReasoningResponseWithTools(
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
			log.Printf("Ollama %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
		if deb && usageTotals.TotalTokens > 0 {
			effort := ""
			if reasoningEffort != nil {
				effort = *reasoningEffort
			}
			cost := s.estimateCost(model, effort, usageTotals)
			log.Printf(
				"Ollama %s usage: input=%d cached_input=%d output=%d reasoning=%d total=%d estimated_cost_usd=%.8f pricing_configured=%t",
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

	ollamaMessages, err := buildInitialOllamaMessages(messages, jsonSchema, s.structuredOutputPromptSchema)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	firstIterationOllamaMessages := ollamaMessages
	if len(firstIterationTools) > 0 {
		firstIterationMessages := WithFirstIterationReasoningSearchSystemMessage(messages, firstIterationTools)
		firstIterationOllamaMessages, err = buildInitialOllamaMessages(firstIterationMessages, jsonSchema, s.structuredOutputPromptSchema)
		if err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
	}
	format, err := buildOllamaFormat(jsonSchema)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
	think, err := ollamaThinkValue(reasoningEffort, deb)
	if err != nil {
		return nil, "", LLMUsageTotals{}, 0, nil, nil, err
	}
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
		currentMessages := ollamaMessages
		if i == 0 && len(firstIterationTools) > 0 {
			currentMessages = firstIterationOllamaMessages
		}
		req := s.newChatRequest(model, maxTokens, currentMessages, currentTools, format, think)

		var resp OllamaChatResponse
		if err := callLLMAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/api/chat", &resp, deb); err != nil {
			return nil, "", LLMUsageTotals{}, 0, nil, nil, err
		}
		iterations = i + 1
		usageTotals.Add(resp.usage())

		if strings.TrimSpace(resp.Message.Thinking) != "" {
			reasoningSummaries = append(reasoningSummaries, strings.TrimSpace(resp.Message.Thinking))
		}
		s.logThinkingIfDeb(deb, i+1, resp.Message.Thinking)

		if len(resp.Message.ToolCalls) == 0 {
			if strings.TrimSpace(resp.Message.Content) == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("ollama chat returned empty assistant output")
			}
			return &LLMBotMessage{
				Role:    "assistant",
				Content: resp.Message.Content,
			}, strings.Join(reasoningSummaries, "\n\n"), usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		ollamaMessages = append(ollamaMessages, resp.Message)
		toolCallLogs := []string{}
		for _, toolCall := range resp.Message.ToolCalls {
			if err := ctx.Err(); err != nil {
				return nil, "", usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), err
			}
			if toolCall.Function.Name == "" {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, errors.New("ollama tool call is missing function name")
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

			rawArgs, err := json.Marshal(toolCall.Function.Arguments)
			if err != nil {
				return nil, "", LLMUsageTotals{}, 0, nil, nil, fmt.Errorf("failed to encode arguments for tool '%s': %w", toolCall.Function.Name, err)
			}
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

			ollamaMessages = append(ollamaMessages, OllamaMessage{
				Role:     "tool",
				ToolName: toolCall.Function.Name,
				Content:  result,
			})
		}

		if deb && len(toolCallLogs) > 0 {
			reasoningSummaries = append(reasoningSummaries, "Tool calls:\n"+strings.Join(toolCallLogs, "\n"))
		}
	}

	return nil, "", LLMUsageTotals{}, 0, nil, ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func (s *OllamaService) newChatRequest(model string, maxTokens *int, messages []OllamaMessage, tools []ToolCall, format interface{}, think interface{}) OllamaChatRequest {
	req := OllamaChatRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
		Format:   format,
		Options:  s.chatOptions(maxTokens, format != nil),
		Stream:   false,
		Think:    think,
	}
	if s.keepAlive != "" {
		req.KeepAlive = s.keepAlive
	}
	return req
}

func (s *OllamaService) chatOptions(maxTokens *int, _ bool) *OllamaOptions {
	options := &OllamaOptions{}
	if s.numCtx > 0 {
		numCtx := s.numCtx
		options.NumCtx = &numCtx
	}
	if maxTokens != nil && *maxTokens > 0 {
		numPredict := *maxTokens
		options.NumPredict = &numPredict
	}
	if s.temperature != nil {
		temperature := *s.temperature
		options.Temperature = &temperature
	}
	if options.NumCtx == nil && options.NumPredict == nil && options.Temperature == nil {
		return nil
	}
	return options
}

func (s *OllamaService) logThinkingIfDeb(deb bool, iteration int, thinking string) {
	if !deb || strings.TrimSpace(thinking) == "" {
		return
	}
	log.Printf("LLM reasoning summary iteration %d:\n%s", iteration, strings.TrimSpace(thinking))
}

func (r *OllamaChatResponse) usage() *OpenAIUsage {
	totalTokens := r.PromptEvalCount + r.EvalCount
	if totalTokens == 0 {
		return nil
	}
	return &OpenAIUsage{
		InputTokens:  r.PromptEvalCount,
		OutputTokens: r.EvalCount,
		TotalTokens:  totalTokens,
	}
}

func (r *OllamaChatResponse) usageTotals() LLMUsageTotals {
	totals := LLMUsageTotals{}
	totals.Add(r.usage())
	return totals
}

func buildInitialOllamaMessages(messages []LLMBotMessage, jsonSchema *string, structuredOutputPromptSchema bool) ([]OllamaMessage, error) {
	instructions, conversation, err := splitInstructionsAndConversation(messages)
	if err != nil {
		return nil, err
	}
	if structuredOutputPromptSchema {
		instructions = appendOllamaStructuredOutputInstruction(instructions, jsonSchema)
	}

	ollamaMessages := []OllamaMessage{{Role: "system", Content: instructions}}
	for _, message := range conversation {
		switch message.Role {
		case "user", "assistant":
			ollamaMessages = append(ollamaMessages, OllamaMessage{
				Role:    message.Role,
				Content: message.Content,
			})
		default:
			return nil, fmt.Errorf("unsupported message role for Ollama: %s", message.Role)
		}
	}

	return ollamaMessages, nil
}

func appendOllamaStructuredOutputInstruction(instructions string, jsonSchema *string) string {
	if jsonSchema == nil {
		return instructions
	}

	grounding := fmt.Sprintf(
		"Return only valid JSON that matches this JSON Schema exactly. Do not add prose, markdown, or code fences.\nJSON Schema:\n%s",
		*jsonSchema,
	)
	if strings.TrimSpace(instructions) == "" {
		return grounding
	}
	return instructions + "\n\n" + grounding
}

func buildOllamaFormat(jsonSchema *string) (interface{}, error) {
	if jsonSchema == nil {
		return nil, nil
	}

	var schema interface{}
	if err := json.Unmarshal([]byte(*jsonSchema), &schema); err != nil {
		return nil, fmt.Errorf("invalid json_schema: %v", err)
	}
	return schema, nil
}

func ollamaThinkValue(reasoningEffort *string, deb bool) (interface{}, error) {
	if reasoningEffort != nil {
		switch *reasoningEffort {
		case "minimal":
			if deb {
				return true, nil
			}
			return false, nil
		case "low", "medium", "high":
			return *reasoningEffort, nil
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Ollama models; supported values are minimal, low, medium, high", *reasoningEffort)
		}
	}
	if deb {
		return true, nil
	}
	return nil, nil
}

func normalizeOllamaAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultOllamaAPIBaseURL
	}

	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	for _, suffix := range []string{"/api/chat", "/api/generate", "/api"} {
		if strings.HasSuffix(apiBaseURL, suffix) {
			apiBaseURL = strings.TrimSuffix(apiBaseURL, suffix)
			break
		}
	}
	return strings.TrimRight(apiBaseURL, "/")
}
