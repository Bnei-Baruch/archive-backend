package llm

const defaultXAIAPIBaseURL = "https://api.x.ai/v1"

type XAIService struct {
	*OpenAICompatibleAPIService
}

var _ Service = (*XAIService)(nil)

func NewXAIService(token string) *XAIService {
	return NewXAIServiceWithOptions(token, nil, nil, "")
}

func NewXAIServiceWithOptions(token string, pricing []ModelPricing, sessions *OpenAIReasoningSessionStore, apiBaseURL string) *XAIService {
	if apiBaseURL == "" {
		apiBaseURL = defaultXAIAPIBaseURL
	}
	service := newOpenAICompatibleAPIServiceWithOptions(token, pricing, sessions, apiBaseURL)
	service.omitInstructionsWithPreviousResponseID = true
	return &XAIService{OpenAICompatibleAPIService: service}
}
