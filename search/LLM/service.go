package llm

// Service defines a provider-agnostic LLM contract used by the backend.
// Implementations may target OpenAI or any other LLM vendor.
type Service interface {
	GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, output interface{}) error
	// GetStructuredOutputWithDebugInfo returns structured output plus token/cost debug info.
	// Use debug=false for stats only; use debug=true to additionally enable raw response logging.
	GetStructuredOutputWithDebugInfo(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error)
	GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error)
	GetReasoningResponseWithTools(model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, user *string, reasoningEffort *string, deb bool, maxIterations int) (*LLMBotMessage, error)
	GetReasoningStructuredOutputWithToolsForSession(sessionID *string, progressSessionID *string, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, user *string, reasoningEffort *string, deb bool, maxIterations int, output interface{}) (string, error)
	// ReserveReasoningSession is used by /search/reasoning/start to pre-allocate
	// provider-session state before the first actual reasoning call. It returns the
	// provider session id to persist in workflow state, but does not call the provider API yet.
	ReserveReasoningSession(model string, reasoningEffort *string) (string, error)
	GetEmbeddings(content string) ([]float64, error)
}

type Runtime struct {
	Tools    *ReasoningToolManager
	Progress *ReasoningProgressStore
	Workflow *ReasoningWorkflowSessionStore
	Services map[string]Service
}
