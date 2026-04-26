package llm

import (
	"context"
	"log"
)

type OpenAIService struct {
	*OpenAICompatibleAPIService
}

var _ Service = (*OpenAIService)(nil)

func NewOpenAIService(token string) *OpenAIService {
	return NewOpenAIServiceWithOptions(token, nil, nil, "")
}

func NewOpenAIServiceWithPricing(token string, pricing []ModelPricing) *OpenAIService {
	return NewOpenAIServiceWithOptions(token, pricing, nil, "")
}

func NewOpenAIServiceWithOptions(token string, pricing []ModelPricing, sessions *OpenAIReasoningSessionStore, apiBaseURL string) *OpenAIService {
	return &OpenAIService{
		OpenAICompatibleAPIService: newOpenAICompatibleAPIServiceWithOptions(token, pricing, sessions, apiBaseURL),
	}
}

func (s *OpenAIService) GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, promptCacheKey, frequencyPenalty, jsonSchema, reasoningEffort, true, false)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetChatResponse total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *OpenAIService) GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, promptCacheKey *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error) {
	msg, usageTotals, err := s.getChatResponseWithUsage(ctx, model, maxTokens, messages, promptCacheKey, frequencyPenalty, jsonSchema, reasoningEffort, true, debug)
	if usageTotals.TotalTokens > 0 {
		log.Printf("OpenAI GetChatResponseWithDebugInfo total tokens: %d", usageTotals.TotalTokens)
	}
	if err != nil {
		return nil, nil, err
	}
	return msg, s.buildReasoningDebugInfo(model, reasoningEffort, usageTotals), nil
}
