package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type StubLLMResponseConfig struct {
	Model    string `mapstructure:"model"`
	Query    string `mapstructure:"query"`
	Response string `mapstructure:"response"`
}

type StubLLMService struct {
	*BaseLLMService
	responses map[string]map[string]string
}

var _ Service = (*StubLLMService)(nil)

func NewStubLLMServiceFromConfig(progress *ReasoningProgressStore) (*StubLLMService, error) {
	entries := []StubLLMResponseConfig{}
	if err := viper.UnmarshalKey("stub.responses", &entries); err != nil {
		return nil, fmt.Errorf("failed to read stub.responses: %w", err)
	}
	service := NewStubLLMService(entries)
	service.progress = progress
	return service, nil
}

func NewStubLLMService(entries []StubLLMResponseConfig) *StubLLMService {
	responses := map[string]map[string]string{}
	for _, entry := range entries {
		model := strings.TrimSpace(entry.Model)
		query := strings.TrimSpace(entry.Query)
		response := strings.TrimSpace(entry.Response)
		if response == "" {
			continue
		}
		if responses[model] == nil {
			responses[model] = map[string]string{}
		}
		responses[model][query] = response
	}
	return &StubLLMService{
		BaseLLMService: newBaseLLMService("", nil, ""),
		responses:      responses,
	}
}

func (s *StubLLMService) GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, output interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	content, err := s.responseFor(model, messages)
	if err != nil {
		return err
	}
	return unmarshalStubResponse(jsonSchema, content, output)
}

func (s *StubLLMService) GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := s.responseFor(model, messages)
	if err != nil {
		return nil, err
	}
	if err := unmarshalStubResponse(jsonSchema, content, output); err != nil {
		return nil, err
	}
	return s.stubDebugInfo(model, reasoningEffort), nil
}

func (s *StubLLMService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := s.responseFor(model, messages)
	if err != nil {
		return nil, err
	}
	return &LLMBotMessage{Role: "assistant", Content: content}, nil
}

func (s *StubLLMService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, err := s.GetChatResponse(ctx, model, maxTokens, messages, promptCacheKey, frequencyPenalty, jsonSchema, reasoningEffort)
	if err != nil {
		return nil, nil, err
	}
	return msg, s.stubDebugInfo(model, reasoningEffort), nil
}

func (s *StubLLMService) GetReasoningResponseWithTools(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, promptCacheKey *string, reasoningEffort *string, deb bool, maxIterations int) (*LLMBotMessage, error) {
	return s.GetChatResponse(ctx, model, maxTokens, messages, promptCacheKey, nil, nil, reasoningEffort)
}

func (s *StubLLMService) GetReasoningStructuredOutputWithToolsForSession(ctx context.Context, sessionID *string, progressSessionID *string, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, firstIterationTools []ToolCall, firstIterationToolHandlers map[string]ToolHandler, promptCacheKey *string, reasoningEffort *string, deb bool, maxIterations int, output interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	content, err := s.responseFor(model, messages)
	if err != nil {
		return "", err
	}
	if err := unmarshalStubResponse(jsonSchema, content, output); err != nil {
		return "", err
	}
	if setter, ok := output.(reasoningProcessStatsSetter); ok {
		setter.SetReasoningProcessStats(0, 1)
	}
	if setter, ok := output.(reasoningUsedToolsSetter); ok {
		setter.SetUsedTools(nil)
	}
	if deb {
		if setter, ok := output.(reasoningDebugInfoSetter); ok {
			debugInfo := s.stubDebugInfo(model, reasoningEffort)
			debugInfo.MainModelUsage = debugInfo.UsageBreakdown()
			setter.SetReasoningDebugInfo(debugInfo)
		}
	}
	if sessionID != nil && strings.TrimSpace(*sessionID) != "" {
		return strings.TrimSpace(*sessionID), nil
	}
	return newReasoningSessionID()
}

func (s *StubLLMService) ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return newReasoningSessionID()
}

func (s *StubLLMService) RefreshReasoningSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrReasoningSessionNotFoundOrExpired
	}
	return nil
}

func (s *StubLLMService) GetEmbeddings(ctx context.Context, content string) ([]float64, error) {
	return nil, fmt.Errorf("stub provider does not support embeddings")
}

func (s *StubLLMService) responseFor(model string, messages []LLMBotMessage) (string, error) {
	query := stubQueryFromMessages(messages)
	if response, ok := s.lookupResponse(model, query); ok {
		return response, nil
	}
	return "", fmt.Errorf("stub response not configured for model %q query %q", model, query)
}

func (s *StubLLMService) lookupResponse(model string, query string) (string, bool) {
	model = strings.TrimSpace(model)
	query = strings.TrimSpace(query)
	for _, key := range []struct {
		model string
		query string
	}{
		{model: model, query: query},
		{model: model, query: "*"},
		{model: "", query: query},
		{model: "", query: "*"},
	} {
		if byQuery, ok := s.responses[key.model]; ok {
			if response, ok := byQuery[key.query]; ok {
				return response, true
			}
		}
	}
	return "", false
}

func stubQueryFromMessages(messages []LLMBotMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		content := strings.TrimSpace(messages[i].Content)
		if content == "" {
			continue
		}
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(content), &payload); err == nil && strings.TrimSpace(payload.Query) != "" {
			return strings.TrimSpace(payload.Query)
		}
		const userQueryPrefix = "User query:\n"
		if strings.HasPrefix(content, userQueryPrefix) {
			query := strings.TrimPrefix(content, userQueryPrefix)
			if idx := strings.Index(query, "\n\n"); idx >= 0 {
				query = query[:idx]
			}
			if strings.TrimSpace(query) != "" {
				return strings.TrimSpace(query)
			}
		}
		return content
	}
	return ""
}

func unmarshalStubResponse(jsonSchema string, content string, output interface{}) error {
	if err := json.Unmarshal([]byte(content), output); err != nil {
		return fmt.Errorf("failed to unmarshal stub response for schema %q: %w", jsonSchema, err)
	}
	return nil
}

func (s *StubLLMService) stubDebugInfo(model string, reasoningEffort *string) *ReasoningSearchDebugInfo {
	debugInfo := s.buildReasoningDebugInfo(model, reasoningEffort, LLMUsageTotals{})
	debugInfo.PricingConfigured = true
	return debugInfo
}
