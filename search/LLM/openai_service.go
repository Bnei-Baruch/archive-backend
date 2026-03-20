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
)

const (
	chatEndpoint       = "https://api.openai.com/v1/chat/completions"
	responsesEndpoint  = "https://api.openai.com/v1/responses"
	embeddingsEndpoint = "https://api.openai.com/v1/embeddings"
	embeddingModel     = "text-embedding-3-large"
)

type OpenAIService struct {
	token string
}

var _ Service = (*OpenAIService)(nil)

func NewOpenAIService(token string) *OpenAIService {
	service := &OpenAIService{
		token: token,
	}

	return service
}

// Structs for OpenAI API interaction

type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []LLMBotMessage `json:"messages"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	User             *string         `json:"user,omitempty"`
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

type ResponsesRequest struct {
	Model              string                   `json:"model"`
	Input              []interface{}            `json:"input,omitempty"`
	Instructions       *string                  `json:"instructions,omitempty"`
	PreviousResponseID *string                  `json:"previous_response_id,omitempty"`
	MaxOutputTokens    *int                     `json:"max_output_tokens,omitempty"`
	User               *string                  `json:"user,omitempty"`
	Reasoning          *ResponsesReasoning      `json:"reasoning,omitempty"`
	Text               *ResponsesText           `json:"text,omitempty"`
	Tools              []map[string]interface{} `json:"tools,omitempty"`
}

type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ResponsesText struct {
	Format *ResponsesTextFormat `json:"format,omitempty"`
}

type ResponsesTextFormat struct {
	Type   string      `json:"type"`
	Name   string      `json:"name,omitempty"`
	Schema interface{} `json:"schema,omitempty"`
	Strict bool        `json:"strict,omitempty"`
}

type ResponsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ResponsesOutputItem struct {
	Type      string                   `json:"type"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Role      string                   `json:"role,omitempty"`
	Content   []ResponsesOutputContent `json:"content,omitempty"`
	Summary   []ResponsesOutputContent `json:"summary,omitempty"`
}

type ResponsesAPIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ResponsesIncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

type ResponsesResponse struct {
	ID                string                      `json:"id"`
	Status            string                      `json:"status"`
	Output            []ResponsesOutputItem       `json:"output"`
	Error             *ResponsesAPIError          `json:"error,omitempty"`
	IncompleteDetails *ResponsesIncompleteDetails `json:"incomplete_details,omitempty"`
	Usage             *OpenAIUsage                `json:"usage,omitempty"`
}

type reasoningSummarySetter interface {
	SetReasoningSummary(string)
}

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

type OpenAIUsage struct {
	TotalTokens int `json:"total_tokens,omitempty"`
}

type LLMBotMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	Refusal    *string           `json:"refusal,omitempty"`
	ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

func (s *OpenAIService) GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, output interface{}) error {
	msg, totalTokens, err := s.getChatResponseWithUsage(model, maxTokens, messages, user, nil, &jsonSchema, reasoningEffort)
	if totalTokens > 0 {
		log.Printf("OpenAI GetStructuredOutput total tokens: %d", totalTokens)
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

func (s *OpenAIService) GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, totalTokens, err := s.getChatResponseWithUsage(model, maxTokens, messages, user, frequencyPenalty, jsonSchema, reasoningEffort)
	if totalTokens > 0 {
		log.Printf("OpenAI GetChatResponse total tokens: %d", totalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *OpenAIService) getChatResponseWithUsage(model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, int, error) {
	sysMsgCount := 0
	for _, m := range messages {
		if m.Role == "system" || m.Role == "developer" {
			sysMsgCount++
		}
	}
	if sysMsgCount != 1 {
		return nil, 0, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	var respFmt *ResponseFormat
	if jsonSchema != nil {
		var JsonSchemaData interface{}
		err := json.Unmarshal([]byte(*jsonSchema), &JsonSchemaData)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid json_schema: %v", err)
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
		User:             user,
		FrequencyPenalty: frequencyPenalty,
		ResponseFormat:   respFmt,
		ReasoningEffort:  reasoningEffort,
	}

	var chatResp ChatResponse
	if err := s.callAPI(req, chatEndpoint, &chatResp); err != nil {
		return nil, 0, err
	}
	totalTokens := 0
	if chatResp.Usage != nil {
		totalTokens = chatResp.Usage.TotalTokens
	}

	for _, choice := range chatResp.Choices {
		if choice.Index == 0 {
			return &choice.Message, totalTokens, nil
		}
	}
	return nil, totalTokens, errors.New("no valid chat choices returned")
}

func (s *OpenAIService) GetReasoningResponseWithTools(
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	user *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
) (*LLMBotMessage, error) {
	msg, _, err := s.getReasoningResponseWithTools("GetReasoningResponseWithTools", nil, model, maxTokens, messages, tools, toolHandlers, user, reasoningEffort, deb, maxIterations)
	return msg, err
}

func (s *OpenAIService) GetReasoningStructuredOutputWithTools(
	jsonSchema string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	user *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
	output interface{},
) error {
	msg, reasoningSummary, err := s.getReasoningResponseWithTools("GetReasoningStructuredOutputWithTools", &jsonSchema, model, maxTokens, messages, tools, toolHandlers, user, reasoningEffort, deb, maxIterations)
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
	return nil
}

func (s *OpenAIService) getReasoningResponseWithTools(
	methodName string,
	jsonSchema *string,
	model string,
	maxTokens *int,
	messages []LLMBotMessage,
	tools []ToolCall,
	toolHandlers map[string]ToolHandler,
	user *string,
	reasoningEffort *string,
	deb bool,
	maxIterations int,
) (*LLMBotMessage, string, error) {
	totalTokens := 0
	defer func() {
		if totalTokens > 0 {
			log.Printf("OpenAI %s total tokens: %d", methodName, totalTokens)
		}
	}()
	reasoningSummaries := []string{}

	if len(tools) == 0 {
		return nil, "", errors.New("tools must contain at least one tool definition")
	}
	if len(toolHandlers) == 0 {
		return nil, "", errors.New("toolHandlers must contain at least one handler")
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}
	reasoningCtx := ContextWithDeb(context.Background(), deb)

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
				return nil, "", errors.New("tool message is missing tool_call_id")
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
		return nil, "", fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	normalizedTools, err := normalizeResponseTools(tools)
	if err != nil {
		return nil, "", err
	}

	text, err := buildResponsesText(jsonSchema)
	if err != nil {
		return nil, "", err
	}

	var previousResponseID *string
	nextInput := initialInput

	for i := 0; i < maxIterations; i++ {
		req := ResponsesRequest{
			Model:              model,
			Input:              nextInput,
			Instructions:       &instructions,
			PreviousResponseID: previousResponseID,
			MaxOutputTokens:    maxTokens,
			User:               user,
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
		if err := s.callAPI(req, responsesEndpoint, &responsesResp); err != nil {
			return nil, "", err
		}
		iterationSummary := strings.Join(extractReasoningSummaryText(responsesResp.Output), "\n\n")
		if strings.TrimSpace(iterationSummary) != "" {
			reasoningSummaries = append(reasoningSummaries, iterationSummary)
		}
		printReasoningOutputIfDeb(deb, i+1, responsesResp.Output)
		if responsesResp.Usage != nil {
			totalTokens += responsesResp.Usage.TotalTokens
		}

		if responsesResp.Error != nil {
			if responsesResp.Error.Code != "" {
				return nil, "", fmt.Errorf("responses API error (%s): %s", responsesResp.Error.Code, responsesResp.Error.Message)
			}
			return nil, "", fmt.Errorf("responses API error: %s", responsesResp.Error.Message)
		}

		functionCalls := []ResponsesOutputItem{}
		for _, item := range responsesResp.Output {
			if item.Type == "function_call" {
				functionCalls = append(functionCalls, item)
			}
		}

		if len(functionCalls) == 0 {
			content := extractAssistantOutputText(responsesResp.Output)
			if content == "" && responsesResp.IncompleteDetails != nil && responsesResp.IncompleteDetails.Reason != "" {
				return nil, "", fmt.Errorf("responses API returned incomplete output: %s", responsesResp.IncompleteDetails.Reason)
			}
			return &LLMBotMessage{
				Role:    "assistant",
				Content: content,
			}, strings.Join(reasoningSummaries, "\n\n"), nil
		}

		nextInput = []interface{}{}
		for _, toolCall := range functionCalls {
			if toolCall.CallID == "" {
				return nil, "", fmt.Errorf("tool call for '%s' is missing call_id", toolCall.Name)
			}

			handler, ok := toolHandlers[toolCall.Name]
			if !ok {
				return nil, "", fmt.Errorf("missing handler for tool '%s'", toolCall.Name)
			}

			rawArgs := json.RawMessage(toolCall.Arguments)
			if len(rawArgs) == 0 {
				rawArgs = json.RawMessage("{}")
			}
			if !json.Valid(rawArgs) {
				return nil, "", fmt.Errorf("invalid arguments for tool '%s': %s", toolCall.Name, toolCall.Arguments)
			}

			result, err := handler(reasoningCtx, rawArgs)
			if err != nil {
				return nil, "", fmt.Errorf("tool '%s' execution failed: %w", toolCall.Name, err)
			}

			nextInput = append(nextInput, map[string]string{
				"type":    "function_call_output",
				"call_id": toolCall.CallID,
				"output":  result,
			})
		}
		previousResponseID = &responsesResp.ID
	}

	return nil, "", fmt.Errorf("max reasoning iterations reached (%d)", maxIterations)
}

func buildResponsesText(jsonSchema *string) (*ResponsesText, error) {
	if jsonSchema == nil {
		return nil, nil
	}

	var schema interface{}
	if err := json.Unmarshal([]byte(*jsonSchema), &schema); err != nil {
		return nil, fmt.Errorf("invalid json_schema: %v", err)
	}

	return &ResponsesText{
		Format: &ResponsesTextFormat{
			Type:   "json_schema",
			Name:   "structured_output",
			Schema: schema,
			Strict: true,
		},
	}, nil
}

func normalizeResponseTools(tools []ToolCall) ([]map[string]interface{}, error) {
	normalized := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		tool := map[string]interface{}{
			"type": t.Type,
		}
		if t.Function != nil {
			functionBytes, err := json.Marshal(t.Function)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool definition: %w", err)
			}
			functionPayload := map[string]interface{}{}
			if err := json.Unmarshal(functionBytes, &functionPayload); err != nil {
				return nil, fmt.Errorf("failed to parse tool definition: %w", err)
			}
			for k, v := range functionPayload {
				tool[k] = v
			}
		}
		normalized = append(normalized, tool)
	}
	return normalized, nil
}

func extractAssistantOutputText(items []ResponsesOutputItem) string {
	content := ""
	for _, item := range items {
		if item.Type != "message" || item.Role != "assistant" {
			continue
		}
		for _, c := range item.Content {
			if c.Type == "output_text" {
				content += c.Text
			}
		}
	}
	return content
}

func extractReasoningSummaryText(items []ResponsesOutputItem) []string {
	parts := []string{}
	for _, item := range items {
		if item.Type != "reasoning" {
			continue
		}
		for _, summary := range item.Summary {
			if summary.Type == "summary_text" && strings.TrimSpace(summary.Text) != "" {
				parts = append(parts, summary.Text)
			}
		}
		for _, content := range item.Content {
			if content.Type == "reasoning_text" && strings.TrimSpace(content.Text) != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	return parts
}

func printReasoningOutputIfDeb(deb bool, iteration int, items []ResponsesOutputItem) {
	if !deb {
		return
	}

	parts := extractReasoningSummaryText(items)
	if len(parts) == 0 {
		return
	}

	log.Printf("OpenAI reasoning summary iteration %d:\n%s", iteration, strings.Join(parts, "\n\n"))
}

func (s *OpenAIService) GetEmbeddings(content string) ([]float64, error) {
	payload := map[string]interface{}{
		"input": content,
		"model": embeddingModel,
	}

	var resp EmbeddingResponse
	if err := s.callAPI(payload, embeddingsEndpoint, &resp); err != nil {
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
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
