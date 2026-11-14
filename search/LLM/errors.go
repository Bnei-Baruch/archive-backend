package llm

import "fmt"

type RecoverableToolError struct {
	Tool     string `json:"tool,omitempty"`
	Message  string `json:"message"`
	Guidance string `json:"guidance,omitempty"`
}

func (e *RecoverableToolError) Error() string {
	return e.Message
}

func NewRecoverableToolError(tool string, message string, guidance string) error {
	return &RecoverableToolError{Tool: tool, Message: message, Guidance: guidance}
}

type MaxReasoningIterationsError struct {
	MaxIterations int
}

func (e *MaxReasoningIterationsError) Error() string {
	return fmt.Sprintf("reasoning search did not complete within the configured tool-iteration limit (%d)", e.MaxIterations)
}

type MaxReasoningFollowupsError struct {
	MaxFollowups int
}

func (e *MaxReasoningFollowupsError) Error() string {
	return fmt.Sprintf("reasoning follow-up limit reached (%d); start a new session", e.MaxFollowups)
}
