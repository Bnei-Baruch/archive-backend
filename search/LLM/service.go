package llm

import "context"

// Service defines a provider-agnostic LLM contract used by the backend.
// Implementations may target OpenAI or any other LLM vendor.
type Service interface {
	GetStructuredOutput(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, output interface{}) error
	// GetStructuredOutputWithDebugInfo returns structured output plus token/cost debug info.
	// Use debug=false for stats only; use debug=true to additionally enable raw response logging.
	GetStructuredOutputWithDebugInfo(ctx context.Context, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, debug bool, output interface{}) (*ReasoningSearchDebugInfo, error)
	GetChatResponse(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error)
	// GetChatResponseWithDebugInfo keeps raw-response logging available for chat-completions based calls.
	// Use debug=false for stats only; use debug=true to additionally enable raw response logging.
	GetChatResponseWithDebugInfo(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string, debug bool) (*LLMBotMessage, *ReasoningSearchDebugInfo, error)
	GetReasoningResponseWithTools(ctx context.Context, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, user *string, reasoningEffort *string, deb bool, maxIterations int) (*LLMBotMessage, error)
	GetReasoningStructuredOutputWithToolsForSession(ctx context.Context, sessionID *string, progressSessionID *string, jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, tools []ToolCall, toolHandlers map[string]ToolHandler, firstIterationTools []ToolCall, firstIterationToolHandlers map[string]ToolHandler, user *string, reasoningEffort *string, deb bool, maxIterations int, output interface{}) (string, error)
	// ReserveReasoningSession is used by /search/reasoning/start to pre-allocate
	// provider-session state before the first actual reasoning call. It returns the
	// provider session id to persist in workflow state, but does not call the provider API yet.
	ReserveReasoningSession(ctx context.Context, model string, reasoningEffort *string) (string, error)
	GetEmbeddings(ctx context.Context, content string) ([]float64, error)
}

type ReasoningSessionRefresher interface {
	// RefreshReasoningSession extends the provider-side continuation state TTL.
	// The public API workflow session is refreshed separately by the workflow store.
	RefreshReasoningSession(sessionID string) error
}

type Runtime struct {
	Tools          *ReasoningToolManager
	Progress       *ReasoningProgressStore
	Workflow       *ReasoningWorkflowSessionStore
	Cancellations  *ReasoningCancellationStore
	ReasoningCache *ReasoningSearchCacheStore
	Services       map[string]Service
	AIToolsConfig  *AIToolsConfig
	DraftConfig    *ReasoningSearchDraftConfig
}
