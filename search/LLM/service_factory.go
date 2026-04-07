package llm

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderOllama     = "ollama"
)

const (
	defaultReasoningSearchEffort        = "high" // "low", "medium", "high", "xhigh" (not for oss models)
	defaultReasoningSearchMaxTokens     = 8000
	defaultReasoningSearchMaxIterations = 20
)

type ReasoningSearchConfig struct {
	Model         string
	Effort        string
	MaxTokens     int
	MaxIterations int
}

func NewServiceFromConfig() (Service, error) {
	return NewServiceFromConfigWithProgress(nil)
}

func NewServiceFromConfigWithProgress(progress *ReasoningProgressStore) (Service, error) {
	provider := ProviderFromConfig()

	switch provider {
	case ProviderOpenAI:
		token := viper.GetString("openai.token")
		apiEndpoint := viper.GetString("openai.api-endpoint")
		if token == "" && strings.TrimSpace(apiEndpoint) == "" {
			return nil, fmt.Errorf("openai.token is empty")
		}
		pricing := []OpenAIModelPricing{}
		if err := viper.UnmarshalKey("openai.pricing", &pricing); err != nil {
			return nil, fmt.Errorf("failed to read openai.pricing: %w", err)
		}
		sessionTTL := viper.GetDuration("openai.reasoning-session-ttl")
		if sessionTTL <= 0 {
			sessionTTL = defaultOpenAIReasoningSessionTTL
		}
		service := NewOpenAIServiceWithOptions(token, pricing, NewOpenAIReasoningSessionStore(sessionTTL), apiEndpoint)
		service.progress = progress
		service.client.Timeout = requestTimeoutFromConfig("openai.request-timeout")
		return service, nil
	case ProviderOpenRouter:
		token := viper.GetString("openrouter.token")
		if strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("openrouter.token is empty")
		}
		apiEndpoint := viper.GetString("openrouter.api-endpoint")
		pricing := []OpenAIModelPricing{}
		if err := viper.UnmarshalKey("openrouter.pricing", &pricing); err != nil {
			return nil, fmt.Errorf("failed to read openrouter.pricing: %w", err)
		}
		sessionTTL := viper.GetDuration("openrouter.reasoning-session-ttl")
		if sessionTTL <= 0 {
			sessionTTL = defaultOpenAIReasoningSessionTTL
		}
		service := NewOpenRouterServiceWithOptions(token, pricing, NewChatReasoningSessionStore(sessionTTL), apiEndpoint)
		service.progress = progress
		service.client.Timeout = requestTimeoutFromConfig("openrouter.request-timeout")
		providerPreferences, err := openRouterProviderPreferencesFromConfig()
		if err != nil {
			return nil, err
		}
		service.providerPreferences = providerPreferences
		service.requiredToolIterations = openRouterRequiredToolIterationsFromConfig()
		return service, nil
	case ProviderOllama:
		token := viper.GetString("ollama.token")
		apiEndpoint := viper.GetString("ollama.api-endpoint")
		pricing := []OpenAIModelPricing{}
		if err := viper.UnmarshalKey("ollama.pricing", &pricing); err != nil {
			return nil, fmt.Errorf("failed to read ollama.pricing: %w", err)
		}
		sessionTTL := viper.GetDuration("ollama.reasoning-session-ttl")
		if sessionTTL <= 0 {
			sessionTTL = defaultOpenAIReasoningSessionTTL
		}
		numCtx := viper.GetInt("ollama.num-ctx")
		if numCtx <= 0 {
			numCtx = defaultOllamaNumCtx
		}
		keepAlive := viper.GetString("ollama.keep-alive")
		var temperature *float64
		if viper.IsSet("ollama.temperature") {
			value := viper.GetFloat64("ollama.temperature")
			if value < 0 {
				return nil, fmt.Errorf("ollama.temperature must be >= 0")
			}
			temperature = &value
		}
		structuredOutputPromptSchema := viper.GetBool("ollama.structured-output-prompt-schema")
		service := NewOllamaServiceWithOptions(token, pricing, NewChatReasoningSessionStore(sessionTTL), apiEndpoint, numCtx, keepAlive, temperature, structuredOutputPromptSchema)
		service.progress = progress
		service.client.Timeout = requestTimeoutFromConfig("ollama.request-timeout")
		return service, nil
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", provider)
	}
}

func ReasoningSessionTTLFromConfig() time.Duration {
	switch ProviderFromConfig() {
	case ProviderOpenAI:
		ttl := viper.GetDuration("openai.reasoning-session-ttl")
		if ttl <= 0 {
			return defaultOpenAIReasoningSessionTTL
		}
		return ttl
	case ProviderOpenRouter:
		ttl := viper.GetDuration("openrouter.reasoning-session-ttl")
		if ttl <= 0 {
			return defaultOpenAIReasoningSessionTTL
		}
		return ttl
	case ProviderOllama:
		ttl := viper.GetDuration("ollama.reasoning-session-ttl")
		if ttl <= 0 {
			return defaultOpenAIReasoningSessionTTL
		}
		return ttl
	default:
		return defaultOpenAIReasoningSessionTTL
	}
}

func ReasoningSearchConfigFromConfig() (*ReasoningSearchConfig, error) {
	provider := ProviderFromConfig()

	switch provider {
	case ProviderOpenAI:
		model := viper.GetString("openai.reasoning-search-model")
		if model == "" {
			model = viper.GetString("openai.reasoning-model")
		}
		if model == "" {
			model = viper.GetString("openai.model")
		}
		if model == "" {
			return nil, fmt.Errorf("openai.reasoning-search-model is empty")
		}

		effort := viper.GetString("openai.reasoning-search-effort")
		if effort == "" {
			effort = viper.GetString("openai.reasoning-effort")
		}
		if effort == "" {
			effort = defaultReasoningSearchEffort
		}
		if strings.HasPrefix(model, "gpt-oss") {
			switch effort {
			case "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", effort, model)
			}
		}

		maxTokens := viper.GetInt("openai.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("openai.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = defaultReasoningSearchMaxIterations
		}

		return &ReasoningSearchConfig{
			Model:         model,
			Effort:        effort,
			MaxTokens:     maxTokens,
			MaxIterations: maxIterations,
		}, nil
	case ProviderOpenRouter:
		model := viper.GetString("openrouter.reasoning-search-model")
		if model == "" {
			model = viper.GetString("openrouter.reasoning-model")
		}
		if model == "" {
			model = viper.GetString("openrouter.model")
		}
		if model == "" {
			return nil, fmt.Errorf("openrouter.reasoning-search-model is empty")
		}

		effort := viper.GetString("openrouter.reasoning-search-effort")
		if effort == "" {
			effort = viper.GetString("openrouter.reasoning-effort")
		}
		if effort == "" {
			effort = defaultReasoningSearchEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for OpenRouter models; supported values are minimal, low, medium, high", effort)
		}

		maxTokens := viper.GetInt("openrouter.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("openrouter.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = 8
		}

		return &ReasoningSearchConfig{
			Model:         model,
			Effort:        effort,
			MaxTokens:     maxTokens,
			MaxIterations: maxIterations,
		}, nil
	case ProviderOllama:
		model := viper.GetString("ollama.reasoning-search-model")
		if model == "" {
			model = viper.GetString("ollama.reasoning-model")
		}
		if model == "" {
			model = viper.GetString("ollama.model")
		}
		if model == "" {
			return nil, fmt.Errorf("ollama.reasoning-search-model is empty")
		}

		effort := viper.GetString("ollama.reasoning-search-effort")
		if effort == "" {
			effort = viper.GetString("ollama.reasoning-effort")
		}
		if effort == "" {
			effort = defaultReasoningSearchEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Ollama models; supported values are minimal, low, medium, high", effort)
		}

		maxTokens := viper.GetInt("ollama.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("ollama.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = defaultReasoningSearchMaxIterations
		}

		return &ReasoningSearchConfig{
			Model:         model,
			Effort:        effort,
			MaxTokens:     maxTokens,
			MaxIterations: maxIterations,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", provider)
	}
}

func ProviderFromConfig() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("llm.provider")))
	if provider == "" {
		provider = ProviderOpenAI
	}
	return provider
}

func requestTimeoutFromConfig(key string) time.Duration {
	timeout := viper.GetDuration(key)
	if timeout <= 0 {
		return defaultLLMHTTPRequestTimeout
	}
	return timeout
}

func openRouterProviderPreferencesFromConfig() (*ResponsesProvider, error) {
	provider := &ResponsesProvider{}

	sort := strings.TrimSpace(viper.GetString("openrouter.provider-sort"))
	if sort == "" {
		sort = "latency"
	}
	switch sort {
	case "latency", "price", "throughput":
		provider.Sort = sort
	default:
		return nil, fmt.Errorf("openrouter.provider-sort %q is invalid; supported values are latency, price, throughput", sort)
	}

	requireParameters := false
	if viper.IsSet("openrouter.provider-require-parameters") {
		requireParameters = viper.GetBool("openrouter.provider-require-parameters")
	}
	provider.RequireParameters = &requireParameters

	if viper.IsSet("openrouter.provider-allow-fallbacks") {
		allowFallbacks := viper.GetBool("openrouter.provider-allow-fallbacks")
		provider.AllowFallbacks = &allowFallbacks
	}

	if viper.IsSet("openrouter.provider-only") {
		only := filterEmptyStrings(viper.GetStringSlice("openrouter.provider-only"))
		if len(only) > 0 {
			provider.Only = only
		}
	}

	if viper.IsSet("openrouter.provider-ignore") {
		ignore := filterEmptyStrings(viper.GetStringSlice("openrouter.provider-ignore"))
		if len(ignore) > 0 {
			provider.Ignore = ignore
		}
	}

	if viper.IsSet("openrouter.provider-preferred-max-latency") {
		latency := viper.GetInt("openrouter.provider-preferred-max-latency")
		if latency > 0 {
			provider.PreferredMaxLatency = &latency
		}
	}

	return provider, nil
}

func openRouterRequiredToolIterationsFromConfig() int {
	requiredToolIterations := viper.GetInt("openrouter.enforced-tool-use-iterations")
	if requiredToolIterations <= 0 {
		return 1
	}
	return requiredToolIterations
}

func filterEmptyStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}
