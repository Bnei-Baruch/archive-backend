package llm

import (
	"fmt"
	"strings"
)

const defaultCohereAPIBaseURL = "https://api.cohere.ai/compatibility/v1"

type CohereService struct {
	*ReplayChatCompletionsService
}

var _ Service = (*CohereService)(nil)

func NewCohereService(token string) *CohereService {
	return NewCohereServiceWithOptions(token, nil, nil, "")
}

func NewCohereServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *CohereService {
	base := newBaseLLMService(token, pricing, normalizeCohereAPIBaseURL(apiBaseURL))
	return &CohereService{
		ReplayChatCompletionsService: &ReplayChatCompletionsService{
			BaseLLMService:       base,
			sessions:             sessions,
			serviceName:          "Cohere",
			reasoningEffortValue: cohereReasoningEffortValue,
		},
	}
}

func cohereReasoningEffortValue(_ string, reasoningEffort *string) (*string, error) {
	if reasoningEffort == nil {
		return nil, nil
	}
	effort := strings.TrimSpace(*reasoningEffort)
	if err := validateCohereReasoningEffort(effort); err != nil {
		return nil, err
	}
	if effort == "" {
		return nil, nil
	}
	return &effort, nil
}

func validateCohereReasoningEffort(effort string) error {
	switch strings.TrimSpace(effort) {
	case "", "none", "high":
		return nil
	default:
		return fmt.Errorf("reasoning effort %q is not supported for Cohere models; supported values are none, high", effort)
	}
}

func normalizeCohereAPIBaseURL(apiBaseURL string) string {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		return defaultCohereAPIBaseURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/chat/completions") {
		apiBaseURL = strings.TrimSuffix(apiBaseURL, "/chat/completions")
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if strings.HasSuffix(apiBaseURL, "/compatibility/v1") {
		return apiBaseURL
	}
	if strings.HasSuffix(apiBaseURL, "/v1") {
		return apiBaseURL
	}
	return apiBaseURL + "/compatibility/v1"
}
