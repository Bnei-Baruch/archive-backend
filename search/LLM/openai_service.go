package llm

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
