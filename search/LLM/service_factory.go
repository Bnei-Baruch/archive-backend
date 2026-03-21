package llm

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const ProviderOpenAI = "openai"

const (
	defaultReasoningSearchEffort        = "xhigh"
	defaultReasoningSearchMaxTokens     = 8000
	defaultReasoningSearchMaxIterations = 16
)

type ReasoningSearchConfig struct {
	Model         string
	Effort        string
	MaxTokens     int
	MaxIterations int
}

func NewServiceFromConfig() (Service, error) {
	provider := providerFromConfig()

	switch provider {
	case ProviderOpenAI:
		token := viper.GetString("openai.token")
		if token == "" {
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
		return NewOpenAIServiceWithOptions(token, pricing, NewOpenAIReasoningSessionStore(sessionTTL)), nil
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", provider)
	}
}

func ReasoningSearchConfigFromConfig() (*ReasoningSearchConfig, error) {
	provider := providerFromConfig()

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
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", provider)
	}
}

func providerFromConfig() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("llm.provider")))
	if provider == "" {
		provider = ProviderOpenAI
	}
	return provider
}
