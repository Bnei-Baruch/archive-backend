package llm

import "strings"

const defaultInceptionAPIBaseURL = "https://api.inceptionlabs.ai/v1"

type InceptionService struct {
	*ReplayChatCompletionsService
}

var _ Service = (*InceptionService)(nil)

func NewInceptionService(token string) *InceptionService {
	return NewInceptionServiceWithOptions(token, nil, nil, "")
}

func NewInceptionServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *InceptionService {
	base := newBaseLLMService(token, pricing, normalizeInceptionAPIBaseURL(apiBaseURL))
	return &InceptionService{
		ReplayChatCompletionsService: &ReplayChatCompletionsService{
			BaseLLMService:       base,
			sessions:             sessions,
			serviceName:          "Inception",
			reasoningEffortValue: inceptionReasoningEffortValue,
			retryTransientErrors: true,
		},
	}
}

func normalizeInceptionAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultInceptionAPIBaseURL
	}

	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/chat/completions") {
		apiBaseURL = strings.TrimSuffix(apiBaseURL, "/chat/completions")
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/v1") {
		return apiBaseURL
	}
	return apiBaseURL + "/v1"
}
