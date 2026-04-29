package llm

import "strings"

type ModelPricing struct {
	Model                     string  `mapstructure:"model"`
	Effort                    string  `mapstructure:"effort"`
	InputPer1MTokensUSD       float64 `mapstructure:"input_per_1m_tokens_usd"`
	CachedInputPer1MTokensUSD float64 `mapstructure:"cached_input_per_1m_tokens_usd"`
	OutputPer1MTokensUSD      float64 `mapstructure:"output_per_1m_tokens_usd"`
}

type LLMUsageTotals struct {
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	ReasoningTokens   int
	TotalTokens       int
}

type LLMCostBreakdown struct {
	PricingConfigured           bool
	InputPer1MTokensUSD         float64
	CachedInputPer1MTokensUSD   float64
	OutputPer1MTokensUSD        float64
	EstimatedInputCostUSD       float64
	EstimatedCachedInputCostUSD float64
	EstimatedOutputCostUSD      float64
	EstimatedCostUSD            float64
}

func (u *LLMUsageTotals) Add(usage *OpenAIUsage) {
	if usage == nil {
		return
	}

	u.InputTokens += usage.InputTokens
	u.OutputTokens += usage.OutputTokens
	u.TotalTokens += usage.TotalTokens

	if usage.InputTokensDetails != nil {
		u.CachedInputTokens += usage.InputTokensDetails.CachedTokens
	}
	if usage.OutputTokensDetails != nil {
		u.ReasoningTokens += usage.OutputTokensDetails.ReasoningTokens
	}
}

func (u *LLMUsageTotals) AddTotals(other LLMUsageTotals) {
	u.InputTokens += other.InputTokens
	u.CachedInputTokens += other.CachedInputTokens
	u.OutputTokens += other.OutputTokens
	u.ReasoningTokens += other.ReasoningTokens
	u.TotalTokens += other.TotalTokens
}

func (u LLMUsageTotals) UncachedInputTokens() int {
	uncached := u.InputTokens - u.CachedInputTokens
	if uncached < 0 {
		return 0
	}
	return uncached
}

func (s *BaseLLMService) estimateCost(model string, effort string, usage LLMUsageTotals) LLMCostBreakdown {
	pricing, ok := s.findPricing(model, effort)
	if !ok {
		return LLMCostBreakdown{}
	}

	cachedInputRate := pricing.CachedInputPer1MTokensUSD
	if cachedInputRate == 0 {
		cachedInputRate = pricing.InputPer1MTokensUSD
	}

	inputCost := float64(usage.UncachedInputTokens()) * pricing.InputPer1MTokensUSD / 1000000
	cachedInputCost := float64(usage.CachedInputTokens) * cachedInputRate / 1000000
	outputCost := float64(usage.OutputTokens) * pricing.OutputPer1MTokensUSD / 1000000

	return LLMCostBreakdown{
		PricingConfigured:           true,
		InputPer1MTokensUSD:         pricing.InputPer1MTokensUSD,
		CachedInputPer1MTokensUSD:   cachedInputRate,
		OutputPer1MTokensUSD:        pricing.OutputPer1MTokensUSD,
		EstimatedInputCostUSD:       inputCost,
		EstimatedCachedInputCostUSD: cachedInputCost,
		EstimatedOutputCostUSD:      outputCost,
		EstimatedCostUSD:            inputCost + cachedInputCost + outputCost,
	}
}

func (s *BaseLLMService) findPricing(model string, effort string) (*ModelPricing, bool) {
	normalizedModel := normalizePricingKey(model)
	normalizedEffort := normalizePricingKey(effort)

	for i := range s.pricing {
		if normalizePricingKey(s.pricing[i].Model) == normalizedModel &&
			normalizePricingKey(s.pricing[i].Effort) == normalizedEffort {
			return &s.pricing[i], true
		}
	}

	for i := range s.pricing {
		if normalizePricingKey(s.pricing[i].Model) == normalizedModel &&
			normalizePricingKey(s.pricing[i].Effort) == "" {
			return &s.pricing[i], true
		}
	}

	return nil, false
}

func normalizePricingKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
