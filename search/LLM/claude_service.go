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
	"net/url"
	"strings"
	"time"
)

const defaultClaudeAPIBaseURL = "https://api.anthropic.com/v1"
const claudeAPIVersion = "2023-06-01"

type ClaudeService struct {
	*BaseLLMService
	sessions *ChatReasoningSessionStore
}

type ClaudeMessageRequest struct {
	Model     string          `json:"model"`
	System    string          `json:"system,omitempty"`
	Messages  []ClaudeMessage `json:"messages"`
	MaxTokens *int            `json:"max_tokens,omitempty"`
	Tools     []ClaudeTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
}

type ClaudeMessage struct {
	Role    string               `json:"role"`
	Content []ClaudeContentBlock `json:"content"`
}

type ClaudeContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type ClaudeTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

type ClaudeMessageResponse struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Role       string               `json:"role"`
	Content    []ClaudeContentBlock `json:"content"`
	StopReason string               `json:"stop_reason"`
	Usage      *ClaudeUsage         `json:"usage,omitempty"`
}

type ClaudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

var _ Service = (*ClaudeService)(nil)

func NewClaudeService(token string) *ClaudeService {
	return NewClaudeServiceWithOptions(token, nil, nil, "")
}

func NewClaudeServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *ClaudeService {
	return &ClaudeService{BaseLLMService: newBaseLLMService(token, pricing, normalizeClaudeAPIBaseURL(apiBaseURL)), sessions: sessions}
}

func (s *ClaudeService) GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, output interface{}) error {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Claude GetStructuredOutput total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return err
	}
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	return nil
}

func (s *ClaudeService) GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, _ *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, &jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Claude GetStructuredOutputWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	if err := validateJSONRequiredTopLevelFields(msg.Content, jsonSchema); err != nil {
		return nil, err
	}
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return nil, err
	}
	return s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ClaudeService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Claude GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	return msg, err
}

func (s *ClaudeService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, _ *string, _ *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, jsonSchema, reasoningEffort, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("Claude GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}

func (s *ClaudeService) GetReasoningResponseWithTools(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, _ *string, reasoningEffort *string, deb bool, maxIterations int) (*LLMBotMessage, error) {
	msg, _, _, _, _, _, err := s.getReasoningResponseWithTools(ctx, "GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, nil, nil, reasoningEffort, deb, maxIterations, "")
	return msg, err
}

func (s *ClaudeService) GetReasoningStructuredOutputWithToolsForSession(ctx context.Context, sessionID *string, progressSessionID *string, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, firstIterationTools []ToolCall, firstIterationToolHandlers map[string]ToolHandler, _ *string, reasoningEffort *string, deb bool, maxIterations int, output interface{}) (string, error) {
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
		return "", err
	}
	if err := unmarshalLLMJSONContent(msg.Content, output); err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
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
	} else if err := s.sessions.Update(effectiveSessionID, newHistory); err != nil {
		return "", err
	}
	return effectiveSessionID, nil
}

func (s *ClaudeService) ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error) {
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

func (s *ClaudeService) RefreshReasoningSession(sessionID string) error {
	if s.sessions == nil {
		return errors.New("reasoning sessions are not enabled")
	}
	return s.sessions.Refresh(strings.TrimSpace(sessionID))
}

func (s *ClaudeService) GetEmbeddings(ctx context.Context, content string) ([]float64, error) {
	return nil, errors.New("claude embeddings are not implemented")
}

func (s *ClaudeService) Close() error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.Close()
}

func (s *ClaudeService) getChatResponseWithUsage(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, jsonSchema *string, reasoningEffort *string, logRawBody bool) (*LLMBotMessage, LLMUsageTotals, error) {
	usageTotals := LLMUsageTotals{}
	system, anthropicMessages, err := normalizeClaudeMessages(messages)
	if err != nil {
		return nil, usageTotals, err
	}
	if jsonSchema != nil {
		system = appendStructuredOutputInstruction(system, jsonSchema)
	}
	req := ClaudeMessageRequest{Model: model, System: system, Messages: anthropicMessages, MaxTokens: maxTokens, Stream: false}
	var resp ClaudeMessageResponse
	if err := callClaudeAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/messages", &resp, logRawBody); err != nil {
		return nil, usageTotals, err
	}
	usageTotals.AddTotals(claudeUsageTotals(resp.Usage))
	msg := claudeResponseToLLMMessage(&resp)
	if strings.TrimSpace(msg.Content) == "" {
		return nil, usageTotals, errors.New("claude returned empty assistant output")
	}
	return msg, usageTotals, nil
}

func (s *ClaudeService) getReasoningResponseWithTools(ctx context.Context, methodName string, jsonSchema *string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, firstIterationTools []ToolCall, firstIterationToolHandlers map[string]ToolHandler, reasoningEffort *string, deb bool, maxIterations int, progressSessionID string) (*LLMBotMessage, []ReasoningSearchReasoningStep, LLMUsageTotals, int, []string, *ReasoningSearchDebugInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	usageTotals := LLMUsageTotals{}
	iterations := 0
	usedTools := []string{}
	usedToolsSet := map[string]bool{}
	defer func() {
		if usageTotals.TotalTokens > 0 {
			log.Printf("Claude %s total tokens: %d", methodName, usageTotals.TotalTokens)
		}
	}()
	reasoningSteps := []ReasoningSearchReasoningStep{}
	if len(tools) == 0 {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("tools must contain at least one tool definition")
	}
	if len(toolHandlers) == 0 {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("toolHandlers must contain at least one handler")
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}

	system, anthropicMessages, err := normalizeClaudeMessages(messages)
	if err != nil {
		return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
	}
	firstSystem, firstMessages := system, anthropicMessages
	if len(firstIterationTools) > 0 {
		firstIterationMessages := WithFirstIterationReasoningSearchSystemMessage(messages, firstIterationTools)
		firstSystem, firstMessages, err = normalizeClaudeMessages(firstIterationMessages)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
	}
	if jsonSchema != nil {
		system = appendStructuredOutputInstruction(system, jsonSchema)
		if len(firstIterationTools) > 0 {
			firstSystem = appendStructuredOutputInstruction(firstSystem, jsonSchema)
		}
	}
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
		currentSystem, currentMessages := system, anthropicMessages
		currentTools, currentHandlers := tools, toolHandlers
		if i == 0 && len(firstIterationTools) > 0 {
			currentSystem, currentMessages = firstSystem, firstMessages
			currentTools, currentHandlers = firstIterationTools, firstIterationToolHandlers
		}
		if isFinalIteration {
			currentTools, currentHandlers = nil, nil
			currentMessages = append([]ClaudeMessage{}, currentMessages...)
			currentMessages = append(currentMessages, ClaudeMessage{Role: "user", Content: []ClaudeContentBlock{{Type: "text", Text: finalReasoningIterationInstruction}}})
		}

		req := ClaudeMessageRequest{Model: model, System: currentSystem, Messages: currentMessages, MaxTokens: maxTokens, Tools: claudeTools(currentTools), Stream: false}
		var resp ClaudeMessageResponse
		if err := callClaudeAPI(ctx, s.client, s.token, req, s.apiBaseURL+"/messages", &resp, deb); err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		iterations = i + 1
		stepUsage := claudeUsageTotals(resp.Usage)
		usageTotals.AddTotals(stepUsage)
		msg := claudeResponseToLLMMessage(&resp)
		stepThoughts := ""

		if len(msg.ToolCalls) == 0 {
			if strings.TrimSpace(msg.Content) == "" {
				return nil, nil, LLMUsageTotals{}, 0, nil, nil, errors.New("claude returned empty assistant output")
			}
			if len(usedTools) == 0 {
				anthropicMessages = append(anthropicMessages, claudeAssistantMessage(msg), ClaudeMessage{Role: "user", Content: []ClaudeContentBlock{{Type: "text", Text: "Use the available archive search tools before giving final results. Do not invent or use example result IDs."}}})
				reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
				continue
			}
			if jsonSchema != nil {
				if err := validateJSONRequiredTopLevelFields(msg.Content, *jsonSchema); err != nil {
					anthropicMessages = append(anthropicMessages, claudeAssistantMessage(msg), ClaudeMessage{Role: "user", Content: []ClaudeContentBlock{{Type: "text", Text: fmt.Sprintf("The previous response is invalid: %v. Return only a complete JSON object that matches the required schema, using the archive results already found. Do not return an empty object.", err)}}})
					reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
					if isFinalIteration {
						return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
					}
					continue
				}
			}
			reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, nil, stepUsage, stepStarted)
			return msg, reasoningSteps, usageTotals, iterations, usedTools, ToolDebugInfoFromContext(reasoningCtx), nil
		}

		anthropicMessages = append(anthropicMessages, claudeAssistantMessage(msg))
		stepToolCalls := []ReasoningSearchReasoningToolCall{}
		toolExecutions := make([]ReasoningToolExecution, 0, len(msg.ToolCalls))
		for _, toolCall := range msg.ToolCalls {
			canonicalToolName := CanonicalReasoningToolName(toolCall.Function.Name)
			if !usedToolsSet[canonicalToolName] {
				usedTools = append(usedTools, canonicalToolName)
				usedToolsSet[canonicalToolName] = true
			}
			rawArgs := json.RawMessage(toolCall.Function.Arguments)
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
		toolResults, err := ExecuteReasoningToolExecutions(reasoningCtx, toolExecutions, currentHandlers, firstIterationToolHandlers)
		if err != nil {
			return nil, nil, LLMUsageTotals{}, 0, nil, nil, err
		}
		anthropicMessages = append(anthropicMessages, claudeToolResultMessage(msg.ToolCalls, toolResults))
		reasoningSteps = appendReasoningStepIfDebug(reasoningSteps, deb, i+1, stepThoughts, stepToolCalls, stepUsage, stepStarted)
	}
	return nil, reasoningSteps, LLMUsageTotals{}, 0, nil, ToolDebugInfoFromContext(reasoningCtx), &MaxReasoningIterationsError{MaxIterations: maxIterations}
}

func normalizeClaudeMessages(messages []LLMBotMessage) (string, []ClaudeMessage, error) {
	systemParts := []string{}
	ret := []ClaudeMessage{}
	for _, message := range messages {
		role := message.Role
		if role == "developer" {
			role = "system"
		}
		switch role {
		case "system":
			systemParts = append(systemParts, message.Content)
		case "user":
			ret = append(ret, ClaudeMessage{Role: "user", Content: []ClaudeContentBlock{{Type: "text", Text: message.Content}}})
		case "assistant":
			ret = append(ret, claudeAssistantMessage(&message))
		case "tool":
			if message.ToolCallID == "" {
				return "", nil, errors.New("tool message is missing tool_call_id")
			}
			ret = append(ret, ClaudeMessage{Role: "user", Content: []ClaudeContentBlock{{Type: "tool_result", ToolUseID: message.ToolCallID, Content: message.Content}}})
		default:
			return "", nil, fmt.Errorf("unsupported message role for Claude: %s", message.Role)
		}
	}
	if len(systemParts) == 0 {
		return "", nil, errors.New("must include a system message")
	}
	return strings.Join(systemParts, "\n\n"), ret, nil
}

func claudeAssistantMessage(message *LLMBotMessage) ClaudeMessage {
	blocks := []ClaudeContentBlock{}
	if strings.TrimSpace(message.Content) != "" {
		blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: message.Content})
	}
	for _, toolCall := range message.ToolCalls {
		args := strings.TrimSpace(toolCall.Function.Arguments)
		if args == "" {
			args = "{}"
		}
		blocks = append(blocks, ClaudeContentBlock{Type: "tool_use", ID: toolCall.ID, Name: toolCall.Function.Name, Input: json.RawMessage(args)})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: ""})
	}
	return ClaudeMessage{Role: "assistant", Content: blocks}
}

func claudeToolResultMessage(toolCalls []MessageToolCall, results []ReasoningToolExecutionResult) ClaudeMessage {
	blocks := make([]ClaudeContentBlock, 0, len(toolCalls))
	for i, toolCall := range toolCalls {
		content := ""
		if i < len(results) {
			content = results[i].Output
		}
		blocks = append(blocks, ClaudeContentBlock{Type: "tool_result", ToolUseID: toolCall.ID, Content: content})
	}
	return ClaudeMessage{Role: "user", Content: blocks}
}

func claudeResponseToLLMMessage(resp *ClaudeMessageResponse) *LLMBotMessage {
	texts := []string{}
	toolCalls := []MessageToolCall{}
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				texts = append(texts, block.Text)
			}
		case "tool_use":
			args := compactToolCallArguments(block.Input)
			toolCalls = append(toolCalls, MessageToolCall{ID: block.ID, Type: "function", Function: ToolCallFunction{Name: block.Name, Arguments: args}})
		}
	}
	return &LLMBotMessage{Role: "assistant", Content: strings.Join(texts, "\n"), ToolCalls: toolCalls}
}

func claudeTools(tools []ToolCall) []ClaudeTool {
	ret := make([]ClaudeTool, 0, len(tools))
	for _, tool := range tools {
		function, ok := tool.Function.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		description, _ := function["description"].(string)
		inputSchema := function["parameters"]
		if inputSchema == nil {
			inputSchema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		ret = append(ret, ClaudeTool{Name: name, Description: description, InputSchema: inputSchema})
	}
	return ret
}

func claudeUsageTotals(usage *ClaudeUsage) LLMUsageTotals {
	if usage == nil {
		return LLMUsageTotals{}
	}
	inputTokens := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	return LLMUsageTotals{InputTokens: inputTokens, CachedInputTokens: usage.CacheReadInputTokens, OutputTokens: usage.OutputTokens, TotalTokens: inputTokens + usage.OutputTokens}
}

func callClaudeAPI(ctx context.Context, client *http.Client, token string, data interface{}, endpoint string, result interface{}, logRawBody bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", claudeAPIVersion)
	if strings.TrimSpace(token) != "" {
		req.Header.Set("x-api-key", token)
	}
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
	if logRawBody {
		log.Printf("%sLLM API raw response endpoint=%s body=%s%s", rawResponseLogColor, endpoint, string(bodyBytes), rawResponseLogReset)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("json.Unmarshal error: %v\nResponse: %s", err, string(bodyBytes))
	}
	return nil
}

func normalizeClaudeAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultClaudeAPIBaseURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/messages") {
		apiBaseURL = strings.TrimSuffix(apiBaseURL, "/messages")
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	parsed, err := url.Parse(apiBaseURL)
	if err == nil && strings.Trim(parsed.Path, "/") == "" {
		return apiBaseURL + "/v1"
	}
	return apiBaseURL
}
