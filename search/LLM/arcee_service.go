package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const defaultArceeAPIBaseURL = "https://api.arcee.ai/api/v1"

type ArceeService struct {
	*ReplayChatCompletionsService
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
	return &ArceeService{ReplayChatCompletionsService: &ReplayChatCompletionsService{BaseLLMService: base, sessions: sessions, serviceName: "Arcee", reasoningEffortValue: arceeReasoningEffortValue}}
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
	return normalizeArceeModelName(model) == "trinity-large-thinking"
}

func normalizeArceeModelName(model string) string {
	normalized := strings.TrimSpace(strings.ToLower(model))
	return strings.TrimPrefix(normalized, "arcee-ai/")
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
	return &LLMBotMessage{Role: message.Role, Content: message.contentText(), Reasoning: message.reasoningText()}, nil
}

func arceeAssistantMessage(message *ArceeMessage) (*LLMBotMessage, error) {
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

func inceptionReasoningEffortValue(_ string, reasoningEffort *string) (*string, error) {
	if reasoningEffort == nil {
		return nil, nil
	}
	effort := strings.TrimSpace(*reasoningEffort)
	switch effort {
	case "":
		return nil, nil
	case "instant", "low", "medium", "high":
		return &effort, nil
	default:
		return nil, fmt.Errorf("reasoning effort %q is not supported for Inception models; supported values are instant, low, medium, high", *reasoningEffort)
	}
}
