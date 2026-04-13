package llm

import "fmt"

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
