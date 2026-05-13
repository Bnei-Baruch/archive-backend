package llm

import (
	"fmt"
	"net/url"
	"strings"
)

const defaultArceeAPIBaseURL = "https://api.arcee.ai/api/v1"

type ArceeService struct {
	*ReplayChatCompletionsService
}

var _ Service = (*ArceeService)(nil)

func NewArceeService(token string) *ArceeService {
	return NewArceeServiceWithOptions(token, nil, nil, "")
}

func NewArceeServiceWithOptions(token string, pricing []ModelPricing, sessions *ChatReasoningSessionStore, apiBaseURL string) *ArceeService {
	base := newBaseLLMService(token, pricing, normalizeArceeAPIBaseURL(apiBaseURL))
	return &ArceeService{ReplayChatCompletionsService: &ReplayChatCompletionsService{BaseLLMService: base, sessions: sessions, serviceName: "Arcee", reasoningEffortValue: arceeReasoningEffortValue}}
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
