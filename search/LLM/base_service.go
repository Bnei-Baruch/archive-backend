package llm

import "net/http"

type BaseLLMService struct {
	token      string
	pricing    []ModelPricing
	progress   *ReasoningProgressStore
	apiBaseURL string
	client     *http.Client
}

func newBaseLLMService(token string, pricing []ModelPricing, apiBaseURL string) *BaseLLMService {
	return &BaseLLMService{
		token:      token,
		pricing:    pricing,
		apiBaseURL: apiBaseURL,
		client:     &http.Client{},
	}
}

func (s *BaseLLMService) buildReasoningDebugInfo(model string, reasoningEffort *string, usageTotals LLMUsageTotals) *ReasoningSearchDebugInfo {
	effort := ""
	if reasoningEffort != nil {
		effort = *reasoningEffort
	}
	cost := s.estimateCost(model, effort, usageTotals)

	return &ReasoningSearchDebugInfo{
		Enabled:                     true,
		Model:                       model,
		ReasoningEffort:             effort,
		TotalTokens:                 usageTotals.TotalTokens,
		InputTokens:                 usageTotals.InputTokens,
		CachedInputTokens:           usageTotals.CachedInputTokens,
		CacheWriteTokens:            usageTotals.CacheWriteTokens,
		UncachedInputTokens:         usageTotals.UncachedInputTokens(),
		OutputTokens:                usageTotals.OutputTokens,
		ReasoningTokens:             usageTotals.ReasoningTokens,
		PricingConfigured:           cost.PricingConfigured,
		InputPer1MTokensUSD:         cost.InputPer1MTokensUSD,
		CachedInputPer1MTokensUSD:   cost.CachedInputPer1MTokensUSD,
		CacheWritePer1MTokensUSD:    cost.CacheWritePer1MTokensUSD,
		OutputPer1MTokensUSD:        cost.OutputPer1MTokensUSD,
		EstimatedInputCostUSD:       cost.EstimatedInputCostUSD,
		EstimatedCachedInputCostUSD: cost.EstimatedCachedInputCostUSD,
		EstimatedCacheWriteCostUSD:  cost.EstimatedCacheWriteCostUSD,
		EstimatedOutputCostUSD:      cost.EstimatedOutputCostUSD,
		EstimatedCostUSD:            cost.EstimatedCostUSD,
	}
}
