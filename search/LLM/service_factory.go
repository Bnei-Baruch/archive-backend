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
	ProviderXAI        = "xai"
	ProviderZAI        = "zai"
	ProviderArcee      = "arcee"
	ProviderStub       = "stub"
)

const (
	defaultReasoningSearchEffort            = "high" // "low", "medium", "high", "xhigh" (not for oss models)
	defaultReasoningSearchMaxTokens         = 8000
	defaultReasoningSearchMaxIterations     = 20 // max iterations for a reasoning search before the verification stage
	defaultReasoningSearchRerunMaxIters     = 8  // max reasoning iterations when the search is rerun after a failed verification stage
	defaultReasoningSearchMaxFollowups      = 2
	defaultReasoningSearchPlanningEffort    = "medium"
	defaultReasoningSearchPlanningMaxTokens = 1500
	defaultAIToolsEffort                    = "low"
	defaultAIToolsMaxTokens                 = 1500
)

type ReasoningSearchConfig struct {
	Provider           string
	Model              string
	Effort             string
	MaxTokens          int
	MaxIterations      int
	RerunMaxIterations int
	MaxFollowups       int
	Planning           *ReasoningSearchPlanningConfig
	Verification       *ReasoningSearchVerificationConfig
}

type ReasoningSearchPlanningConfig struct {
	Provider  string
	Model     string
	Effort    string
	MaxTokens int
}

type ReasoningSearchVerificationConfig struct {
	Provider  string
	Model     string
	Effort    string
	MaxTokens int
}

type AIToolsConfig struct {
	Provider  string
	Model     string
	Effort    string
	MaxTokens int
}

func NewServiceFromConfig() (Service, error) {
	return NewServiceFromConfigWithProgress(nil)
}

func NewServiceFromConfigWithProgress(progress *ReasoningProgressStore) (Service, error) {
	return NewServiceForProviderWithProgress(ProviderFromConfig(), progress)
}

func NewServiceForProviderWithProgress(provider string, progress *ReasoningProgressStore) (Service, error) {
	switch provider {
	case ProviderOpenAI:
		token := viper.GetString("openai.token")
		apiEndpoint := viper.GetString("openai.api-endpoint")
		if token == "" && strings.TrimSpace(apiEndpoint) == "" {
			return nil, fmt.Errorf("openai.token is empty")
		}
		pricing := []ModelPricing{}
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
		pricing := []ModelPricing{}
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
		service.reasoningSearchProviderPreferences, err = openRouterProviderPreferencesForScopeFromConfig("openrouter.reasoning-search", providerPreferences)
		if err != nil {
			return nil, err
		}
		service.reasoningSearchPlanningProviderPrefs, err = openRouterProviderPreferencesForScopeFromConfig("openrouter.reasoning-search-planning", providerPreferences)
		if err != nil {
			return nil, err
		}
		service.reasoningSearchVerificationProviderPrefs, err = openRouterProviderPreferencesForScopeFromConfig("openrouter.reasoning-search-verification", providerPreferences)
		if err != nil {
			return nil, err
		}
		service.aiToolsProviderPreferences, err = openRouterProviderPreferencesForScopeFromConfig("openrouter.ai-tools", providerPreferences)
		if err != nil {
			return nil, err
		}
		service.requiredToolIterations = viper.GetInt("openrouter.enforced-tool-use-iterations")
		return service, nil
	case ProviderOllama:
		token := viper.GetString("ollama.token")
		apiEndpoint := viper.GetString("ollama.api-endpoint")
		pricing := []ModelPricing{}
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
	case ProviderXAI:
		token := viper.GetString("xai.token")
		apiEndpoint := viper.GetString("xai.api-endpoint")
		if token == "" && strings.TrimSpace(apiEndpoint) == "" {
			return nil, fmt.Errorf("xai.token is empty")
		}
		pricing := []ModelPricing{}
		if err := viper.UnmarshalKey("xai.pricing", &pricing); err != nil {
			return nil, fmt.Errorf("failed to read xai.pricing: %w", err)
		}
		sessionTTL := viper.GetDuration("xai.reasoning-session-ttl")
		if sessionTTL <= 0 {
			sessionTTL = defaultOpenAIReasoningSessionTTL
		}
		service := NewXAIServiceWithOptions(token, pricing, NewOpenAIReasoningSessionStore(sessionTTL), apiEndpoint)
		service.progress = progress
		service.client.Timeout = requestTimeoutFromConfig("xai.request-timeout")
		return service, nil
	case ProviderZAI:
		token := viper.GetString("zai.token")
		if strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("zai.token is empty")
		}
		apiEndpoint := viper.GetString("zai.api-endpoint")
		pricing := []ModelPricing{}
		if err := viper.UnmarshalKey("zai.pricing", &pricing); err != nil {
			return nil, fmt.Errorf("failed to read zai.pricing: %w", err)
		}
		sessionTTL := viper.GetDuration("zai.reasoning-session-ttl")
		if sessionTTL <= 0 {
			sessionTTL = defaultOpenAIReasoningSessionTTL
		}
		var doSample *bool
		if viper.IsSet("zai.do-sample") {
			value := viper.GetBool("zai.do-sample")
			doSample = &value
		}
		var temperature *float64
		if viper.IsSet("zai.temperature") {
			value := viper.GetFloat64("zai.temperature")
			if value < 0 || value > 1 {
				return nil, fmt.Errorf("zai.temperature must be between 0 and 1")
			}
			temperature = &value
		}
		var topP *float64
		if viper.IsSet("zai.top-p") {
			value := viper.GetFloat64("zai.top-p")
			if value <= 0 || value > 1 {
				return nil, fmt.Errorf("zai.top-p must be > 0 and <= 1")
			}
			topP = &value
		}
		stop := []string{}
		if viper.IsSet("zai.stop") {
			for _, value := range viper.GetStringSlice("zai.stop") {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				stop = append(stop, value)
			}
		}
		service := NewZAIServiceWithOptions(token, pricing, NewChatReasoningSessionStore(sessionTTL), apiEndpoint, doSample, temperature, topP, stop)
		service.progress = progress
		service.client.Timeout = requestTimeoutFromConfig("zai.request-timeout")
		return service, nil
	case ProviderArcee:
		token := viper.GetString("arcee.token")
		if strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("arcee.token is empty")
		}
		apiEndpoint := viper.GetString("arcee.api-endpoint")
		pricing := []ModelPricing{}
		if err := viper.UnmarshalKey("arcee.pricing", &pricing); err != nil {
			return nil, fmt.Errorf("failed to read arcee.pricing: %w", err)
		}
		sessionTTL := viper.GetDuration("arcee.reasoning-session-ttl")
		if sessionTTL <= 0 {
			sessionTTL = defaultOpenAIReasoningSessionTTL
		}
		service := NewArceeServiceWithOptions(token, pricing, NewChatReasoningSessionStore(sessionTTL), apiEndpoint)
		service.progress = progress
		service.client.Timeout = requestTimeoutFromConfig("arcee.request-timeout")
		return service, nil
	case ProviderStub:
		return NewStubLLMServiceFromConfig(progress)
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
	case ProviderXAI:
		ttl := viper.GetDuration("xai.reasoning-session-ttl")
		if ttl <= 0 {
			return defaultOpenAIReasoningSessionTTL
		}
		return ttl
	case ProviderZAI:
		ttl := viper.GetDuration("zai.reasoning-session-ttl")
		if ttl <= 0 {
			return defaultOpenAIReasoningSessionTTL
		}
		return ttl
	case ProviderArcee:
		ttl := viper.GetDuration("arcee.reasoning-session-ttl")
		if ttl <= 0 {
			return defaultOpenAIReasoningSessionTTL
		}
		return ttl
	case ProviderStub:
		return defaultOpenAIReasoningSessionTTL
	default:
		return defaultOpenAIReasoningSessionTTL
	}
}

func ReasoningSearchCacheEnabledFromConfig() bool {
	return viper.GetBool("llm.reasoning-search-cache-enabled")
}

func ReasoningSearchCacheTTLFromConfig() time.Duration {
	ttl := viper.GetDuration("llm.reasoning-search-cache-ttl")
	if ttl <= 0 {
		return defaultReasoningSearchCacheTTL
	}
	return ttl
}

func ReasoningSearchConfigFromConfig() (*ReasoningSearchConfig, error) {
	provider := ProviderFromConfig()
	planningEnabled := viper.GetBool("llm.reasoning-search-planning-enabled")
	planningProvider := ReasoningSearchPlanningProviderFromConfig()
	verificationEnabled := viper.GetBool("llm.reasoning-search-verification-enabled")
	verificationProvider := ReasoningSearchVerificationProviderFromConfig()
	maxFollowups := defaultReasoningSearchMaxFollowups
	if viper.IsSet("llm.reasoning-search-max-followups") {
		maxFollowups = viper.GetInt("llm.reasoning-search-max-followups")
		if maxFollowups < 0 {
			return nil, fmt.Errorf("llm.reasoning-search-max-followups must be >= 0")
		}
	}

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
		rerunMaxIterations := viper.GetInt("openai.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
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
		rerunMaxIterations := viper.GetInt("openrouter.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
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
		rerunMaxIterations := viper.GetInt("ollama.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
		}, nil
	case ProviderXAI:
		model := strings.TrimSpace(viper.GetString("xai.reasoning-search-model"))
		if model == "" {
			model = "grok-4-1-fast-reasoning"
		}

		effort := strings.TrimSpace(viper.GetString("xai.reasoning-search-effort"))
		if effort != "" {
			return nil, fmt.Errorf("xai.reasoning-search-effort is not supported for model %q", model)
		}

		maxTokens := viper.GetInt("xai.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("xai.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = defaultReasoningSearchMaxIterations
		}
		rerunMaxIterations := viper.GetInt("xai.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
		}, nil
	case ProviderZAI:
		model := viper.GetString("zai.reasoning-search-model")
		if model == "" {
			model = viper.GetString("zai.reasoning-model")
		}
		if model == "" {
			model = "glm-5.1"
		}

		effort := viper.GetString("zai.reasoning-search-effort")
		if effort == "" {
			effort = viper.GetString("zai.reasoning-effort")
		}
		if effort == "" {
			effort = defaultReasoningSearchEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high", "xhigh":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Z.AI models; supported values are minimal, low, medium, high, xhigh", effort)
		}

		maxTokens := viper.GetInt("zai.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("zai.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = defaultReasoningSearchMaxIterations
		}
		rerunMaxIterations := viper.GetInt("zai.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
		}, nil
	case ProviderArcee:
		model := viper.GetString("arcee.reasoning-search-model")
		if model == "" {
			model = viper.GetString("arcee.reasoning-model")
		}
		if model == "" {
			model = "trinity-mini"
		}

		effort := viper.GetString("arcee.reasoning-search-effort")
		if effort == "" {
			effort = viper.GetString("arcee.reasoning-effort")
		}
		if effort != "" {
			switch effort {
			case "minimal", "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for Arcee models; supported values are minimal, low, medium, high", effort)
			}
		}

		maxTokens := viper.GetInt("arcee.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("arcee.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = defaultReasoningSearchMaxIterations
		}
		rerunMaxIterations := viper.GetInt("arcee.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
		}, nil
	case ProviderStub:
		model := strings.TrimSpace(viper.GetString("stub.reasoning-search-model"))
		if model == "" {
			model = "stub-reasoning"
		}

		effort := strings.TrimSpace(viper.GetString("stub.reasoning-search-effort"))

		maxTokens := viper.GetInt("stub.reasoning-search-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}

		maxIterations := viper.GetInt("stub.reasoning-search-max-iterations")
		if maxIterations <= 0 {
			maxIterations = 1
		}
		rerunMaxIterations := viper.GetInt("stub.reasoning-search-rerun-max-iterations")
		if rerunMaxIterations <= 0 {
			rerunMaxIterations = defaultReasoningSearchRerunMaxIters
		}

		var planning *ReasoningSearchPlanningConfig
		if planningEnabled {
			var err error
			planning, err = reasoningSearchPlanningConfigFromProvider(planningProvider, effort)
			if err != nil {
				return nil, err
			}
		}
		var verification *ReasoningSearchVerificationConfig
		if verificationEnabled {
			var err error
			verification, err = reasoningSearchVerificationConfigFromProvider(verificationProvider, effort)
			if err != nil {
				return nil, err
			}
		}

		return &ReasoningSearchConfig{
			Provider:           provider,
			Model:              model,
			Effort:             effort,
			MaxTokens:          maxTokens,
			MaxIterations:      maxIterations,
			RerunMaxIterations: rerunMaxIterations,
			MaxFollowups:       maxFollowups,
			Planning:           planning,
			Verification:       verification,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", provider)
	}
}

func ReasoningSearchPlanningProviderFromConfig() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("llm.reasoning-search-planning-provider")))
	if provider == "" {
		return ProviderFromConfig()
	}
	return provider
}

func ProviderFromConfig() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("llm.provider")))
	if provider == "" {
		provider = ProviderOpenAI
	}
	return provider
}

func ReasoningSearchVerificationProviderFromConfig() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("llm.reasoning-search-verification-provider")))
	if provider == "" {
		return ProviderFromConfig()
	}
	return provider
}

func reasoningSearchPlanningConfigFromProvider(provider string, fallbackEffort string) (*ReasoningSearchPlanningConfig, error) {
	switch provider {
	case ProviderOpenAI:
		model := strings.TrimSpace(viper.GetString("openai.reasoning-search-planning-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("openai.reasoning-search-model"))
		}
		if model == "" {
			return nil, fmt.Errorf("openai.reasoning-search-planning-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("openai.reasoning-search-planning-effort"))
		if effort == "" {
			effort = strings.TrimSpace(fallbackEffort)
		}
		if effort == "" {
			effort = defaultReasoningSearchPlanningEffort
		}
		maxTokens := viper.GetInt("openai.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderOpenRouter:
		model := strings.TrimSpace(viper.GetString("openrouter.reasoning-search-planning-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("openrouter.reasoning-search-model"))
		}
		if model == "" {
			return nil, fmt.Errorf("openrouter.reasoning-search-planning-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("openrouter.reasoning-search-planning-effort"))
		if effort == "" {
			effort = strings.TrimSpace(fallbackEffort)
		}
		if effort == "" {
			effort = defaultReasoningSearchPlanningEffort
		}
		maxTokens := viper.GetInt("openrouter.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderOllama:
		model := strings.TrimSpace(viper.GetString("ollama.reasoning-search-planning-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("ollama.reasoning-search-model"))
		}
		if model == "" {
			return nil, fmt.Errorf("ollama.reasoning-search-planning-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("ollama.reasoning-search-planning-effort"))
		if effort == "" {
			effort = strings.TrimSpace(fallbackEffort)
		}
		if effort == "" {
			effort = defaultReasoningSearchPlanningEffort
		}
		maxTokens := viper.GetInt("ollama.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderXAI:
		model := strings.TrimSpace(viper.GetString("xai.reasoning-search-planning-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("xai.reasoning-search-model"))
		}
		if model == "" {
			model = "grok-4-1-fast-reasoning"
		}
		effort := strings.TrimSpace(viper.GetString("xai.reasoning-search-planning-effort"))
		if effort != "" {
			return nil, fmt.Errorf("xai.reasoning-search-planning-effort is not supported for model %q", model)
		}
		maxTokens := viper.GetInt("xai.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: "", MaxTokens: maxTokens}, nil
	case ProviderZAI:
		model := strings.TrimSpace(viper.GetString("zai.reasoning-search-planning-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("zai.reasoning-search-model"))
		}
		if model == "" {
			return nil, fmt.Errorf("zai.reasoning-search-planning-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("zai.reasoning-search-planning-effort"))
		if effort == "" {
			effort = strings.TrimSpace(fallbackEffort)
		}
		if effort == "" {
			effort = defaultReasoningSearchPlanningEffort
		}
		maxTokens := viper.GetInt("zai.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderArcee:
		model := strings.TrimSpace(viper.GetString("arcee.reasoning-search-planning-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("arcee.reasoning-search-model"))
		}
		if model == "" {
			model = "trinity-mini"
		}
		effort := strings.TrimSpace(viper.GetString("arcee.reasoning-search-planning-effort"))
		if effort == "" {
			effort = strings.TrimSpace(fallbackEffort)
		}
		if effort != "" {
			switch effort {
			case "minimal", "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for Arcee models; supported values are minimal, low, medium, high", effort)
			}
		}
		maxTokens := viper.GetInt("arcee.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderStub:
		model := strings.TrimSpace(viper.GetString("stub.reasoning-search-planning-model"))
		if model == "" {
			model = "stub-planning"
		}
		effort := strings.TrimSpace(viper.GetString("stub.reasoning-search-planning-effort"))
		if effort == "" {
			effort = strings.TrimSpace(fallbackEffort)
		}
		maxTokens := viper.GetInt("stub.reasoning-search-planning-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchPlanningMaxTokens
		}
		return &ReasoningSearchPlanningConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	default:
		return nil, fmt.Errorf("unsupported planning provider: %s", provider)
	}
}

func AIToolsProviderFromConfig() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("llm.ai-tools-provider")))
	if provider == "" {
		return ProviderFromConfig()
	}
	return provider
}

func AIToolsConfigFromConfig() (*AIToolsConfig, error) {
	return aiToolsConfigFromProvider(AIToolsProviderFromConfig())
}

func reasoningSearchVerificationConfigFromProvider(provider string, defaultEffort string) (*ReasoningSearchVerificationConfig, error) {
	switch provider {
	case ProviderOpenAI:
		model := strings.TrimSpace(viper.GetString("openai.reasoning-search-verification-model"))
		if model == "" {
			return nil, fmt.Errorf("openai.reasoning-search-verification-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("openai.reasoning-search-verification-effort"))
		if effort == "" {
			effort = defaultEffort
		}
		if strings.HasPrefix(model, "gpt-oss") {
			switch effort {
			case "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", effort, model)
			}
		}
		maxTokens := viper.GetInt("openai.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	case ProviderOpenRouter:
		model := strings.TrimSpace(viper.GetString("openrouter.reasoning-search-verification-model"))
		if model == "" {
			return nil, fmt.Errorf("openrouter.reasoning-search-verification-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("openrouter.reasoning-search-verification-effort"))
		if effort == "" {
			effort = defaultEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for OpenRouter models; supported values are minimal, low, medium, high", effort)
		}
		maxTokens := viper.GetInt("openrouter.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	case ProviderOllama:
		model := strings.TrimSpace(viper.GetString("ollama.reasoning-search-verification-model"))
		if model == "" {
			return nil, fmt.Errorf("ollama.reasoning-search-verification-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("ollama.reasoning-search-verification-effort"))
		if effort == "" {
			effort = defaultEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Ollama models; supported values are minimal, low, medium, high", effort)
		}
		maxTokens := viper.GetInt("ollama.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	case ProviderXAI:
		model := strings.TrimSpace(viper.GetString("xai.reasoning-search-verification-model"))
		if model == "" {
			return nil, fmt.Errorf("xai.reasoning-search-verification-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("xai.reasoning-search-verification-effort"))
		if effort != "" {
			return nil, fmt.Errorf("xai.reasoning-search-verification-effort is not supported for model %q", model)
		}
		maxTokens := viper.GetInt("xai.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	case ProviderZAI:
		model := strings.TrimSpace(viper.GetString("zai.reasoning-search-verification-model"))
		if model == "" {
			return nil, fmt.Errorf("zai.reasoning-search-verification-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("zai.reasoning-search-verification-effort"))
		if effort == "" {
			effort = defaultEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high", "xhigh":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Z.AI models; supported values are minimal, low, medium, high, xhigh", effort)
		}
		maxTokens := viper.GetInt("zai.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	case ProviderArcee:
		model := strings.TrimSpace(viper.GetString("arcee.reasoning-search-verification-model"))
		if model == "" {
			return nil, fmt.Errorf("arcee.reasoning-search-verification-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("arcee.reasoning-search-verification-effort"))
		if effort == "" {
			effort = defaultEffort
		}
		if effort != "" {
			switch effort {
			case "minimal", "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for Arcee models; supported values are minimal, low, medium, high", effort)
			}
		}
		maxTokens := viper.GetInt("arcee.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	case ProviderStub:
		model := strings.TrimSpace(viper.GetString("stub.reasoning-search-verification-model"))
		if model == "" {
			model = "stub-verification"
		}
		effort := strings.TrimSpace(viper.GetString("stub.reasoning-search-verification-effort"))
		if effort == "" {
			effort = defaultEffort
		}
		maxTokens := viper.GetInt("stub.reasoning-search-verification-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultReasoningSearchMaxTokens
		}
		return &ReasoningSearchVerificationConfig{
			Provider:  provider,
			Model:     model,
			Effort:    effort,
			MaxTokens: maxTokens,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported verification provider: %s", provider)
	}
}

func aiToolsConfigFromProvider(provider string) (*AIToolsConfig, error) {
	switch provider {
	case ProviderOpenAI:
		model := strings.TrimSpace(viper.GetString("openai.ai-tools-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("openai.reasoning-search-model"))
		}
		if model == "" {
			model = strings.TrimSpace(viper.GetString("openai.model"))
		}
		if model == "" {
			return nil, fmt.Errorf("openai.ai-tools-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("openai.ai-tools-effort"))
		if effort == "" {
			effort = defaultAIToolsEffort
		}
		if strings.HasPrefix(model, "gpt-oss") {
			switch effort {
			case "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for model %q; gpt-oss supports only low, medium, high", effort, model)
			}
		}
		maxTokens := viper.GetInt("openai.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderOpenRouter:
		model := strings.TrimSpace(viper.GetString("openrouter.ai-tools-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("openrouter.reasoning-search-model"))
		}
		if model == "" {
			model = strings.TrimSpace(viper.GetString("openrouter.model"))
		}
		if model == "" {
			return nil, fmt.Errorf("openrouter.ai-tools-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("openrouter.ai-tools-effort"))
		if effort == "" {
			effort = defaultAIToolsEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for OpenRouter models; supported values are minimal, low, medium, high", effort)
		}
		maxTokens := viper.GetInt("openrouter.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderOllama:
		model := strings.TrimSpace(viper.GetString("ollama.ai-tools-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("ollama.reasoning-search-model"))
		}
		if model == "" {
			model = strings.TrimSpace(viper.GetString("ollama.model"))
		}
		if model == "" {
			return nil, fmt.Errorf("ollama.ai-tools-model is empty")
		}
		effort := strings.TrimSpace(viper.GetString("ollama.ai-tools-effort"))
		if effort == "" {
			effort = defaultAIToolsEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Ollama models; supported values are minimal, low, medium, high", effort)
		}
		maxTokens := viper.GetInt("ollama.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderXAI:
		model := strings.TrimSpace(viper.GetString("xai.ai-tools-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("xai.reasoning-search-model"))
		}
		if model == "" {
			model = "grok-4-1-fast-reasoning"
		}
		effort := strings.TrimSpace(viper.GetString("xai.ai-tools-effort"))
		if effort != "" {
			return nil, fmt.Errorf("xai.ai-tools-effort is not supported for model %q", model)
		}
		maxTokens := viper.GetInt("xai.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderZAI:
		model := strings.TrimSpace(viper.GetString("zai.ai-tools-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("zai.reasoning-search-model"))
		}
		if model == "" {
			model = "glm-5.1"
		}
		effort := strings.TrimSpace(viper.GetString("zai.ai-tools-effort"))
		if effort == "" {
			effort = defaultAIToolsEffort
		}
		switch effort {
		case "minimal", "low", "medium", "high", "xhigh":
		default:
			return nil, fmt.Errorf("reasoning effort %q is not supported for Z.AI models; supported values are minimal, low, medium, high, xhigh", effort)
		}
		maxTokens := viper.GetInt("zai.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderArcee:
		model := strings.TrimSpace(viper.GetString("arcee.ai-tools-model"))
		if model == "" {
			model = strings.TrimSpace(viper.GetString("arcee.reasoning-search-model"))
		}
		if model == "" {
			model = "trinity-mini"
		}
		effort := strings.TrimSpace(viper.GetString("arcee.ai-tools-effort"))
		if effort != "" {
			switch effort {
			case "minimal", "low", "medium", "high":
			default:
				return nil, fmt.Errorf("reasoning effort %q is not supported for Arcee models; supported values are minimal, low, medium, high", effort)
			}
		}
		maxTokens := viper.GetInt("arcee.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	case ProviderStub:
		model := strings.TrimSpace(viper.GetString("stub.ai-tools-model"))
		if model == "" {
			model = "stub-ai-tools"
		}
		effort := strings.TrimSpace(viper.GetString("stub.ai-tools-effort"))
		if effort == "" {
			effort = defaultAIToolsEffort
		}
		maxTokens := viper.GetInt("stub.ai-tools-max-output-tokens")
		if maxTokens <= 0 {
			maxTokens = defaultAIToolsMaxTokens
		}
		return &AIToolsConfig{Provider: provider, Model: model, Effort: effort, MaxTokens: maxTokens}, nil
	default:
		return nil, fmt.Errorf("unsupported ai tools provider: %s", provider)
	}
}

func requestTimeoutFromConfig(key string) time.Duration {
	timeout := viper.GetDuration(key)
	if timeout <= 0 {
		return defaultLLMHTTPRequestTimeout
	}
	return timeout
}

func openRouterProviderPreferencesFromConfig() (*ResponsesProvider, error) {
	return openRouterProviderPreferencesForScopeFromConfig("openrouter", nil)
}

func openRouterProviderPreferencesForScopeFromConfig(scope string, fallback *ResponsesProvider) (*ResponsesProvider, error) {
	provider := cloneResponsesProvider(fallback)
	if provider == nil {
		provider = &ResponsesProvider{}
	}

	sortKey := openRouterProviderPreferenceConfigKey(scope, "sort")
	sort := strings.TrimSpace(viper.GetString(sortKey))
	if sort == "" {
		sort = strings.TrimSpace(provider.Sort)
		if sort == "" {
			sort = "latency"
		}
	}
	switch sort {
	case "latency", "price", "throughput":
		provider.Sort = sort
	default:
		return nil, fmt.Errorf("%s %q is invalid; supported values are latency, price, throughput", sortKey, sort)
	}

	requireParameters := false
	if provider.RequireParameters != nil {
		requireParameters = *provider.RequireParameters
	}
	if viper.IsSet(openRouterProviderPreferenceConfigKey(scope, "require-parameters")) {
		requireParameters = viper.GetBool(openRouterProviderPreferenceConfigKey(scope, "require-parameters"))
	}
	provider.RequireParameters = &requireParameters

	allowFallbacksKey := openRouterProviderPreferenceConfigKey(scope, "allow-fallbacks")
	if viper.IsSet(allowFallbacksKey) {
		allowFallbacks := viper.GetBool(allowFallbacksKey)
		provider.AllowFallbacks = &allowFallbacks
	}

	onlyKey := openRouterProviderPreferenceConfigKey(scope, "only")
	if viper.IsSet(onlyKey) {
		provider.Only = filterEmptyStrings(viper.GetStringSlice(onlyKey))
	}

	ignoreKey := openRouterProviderPreferenceConfigKey(scope, "ignore")
	if viper.IsSet(ignoreKey) {
		provider.Ignore = filterEmptyStrings(viper.GetStringSlice(ignoreKey))
	}

	latencyKey := openRouterProviderPreferenceConfigKey(scope, "preferred-max-latency")
	if viper.IsSet(latencyKey) {
		latency := viper.GetInt(latencyKey)
		if latency > 0 {
			provider.PreferredMaxLatency = &latency
		} else {
			provider.PreferredMaxLatency = nil
		}
	}

	return provider, nil
}

func openRouterProviderPreferenceConfigKey(scope string, name string) string {
	if scope == "openrouter" {
		return "openrouter.provider-" + name
	}
	return scope + "-provider-" + name
}

func cloneResponsesProvider(provider *ResponsesProvider) *ResponsesProvider {
	if provider == nil {
		return nil
	}
	clone := *provider
	if provider.RequireParameters != nil {
		value := *provider.RequireParameters
		clone.RequireParameters = &value
	}
	if provider.AllowFallbacks != nil {
		value := *provider.AllowFallbacks
		clone.AllowFallbacks = &value
	}
	if provider.PreferredMaxLatency != nil {
		value := *provider.PreferredMaxLatency
		clone.PreferredMaxLatency = &value
	}
	if provider.Only != nil {
		clone.Only = append([]string(nil), provider.Only...)
	}
	if provider.Ignore != nil {
		clone.Ignore = append([]string(nil), provider.Ignore...)
	}
	return &clone
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
