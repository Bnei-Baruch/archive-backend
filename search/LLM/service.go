package llm

// Service defines a provider-agnostic LLM contract used by the backend.
// Implementations may target OpenAI or any other LLM vendor.
type Service interface {
	GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, output interface{}) error
	GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error)
	GetReasoningResponseWithTools(model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, user *string, reasoningEffort *string, maxIterations int) (*LLMBotMessage, error)
	GetEmbeddings(content string) ([]float64, error)
}
