package llm

import "fmt"

type MaxReasoningIterationsError struct {
	MaxIterations int
}

func (e *MaxReasoningIterationsError) Error() string {
	return fmt.Sprintf("reasoning search did not complete within the configured tool-iteration limit (%d)", e.MaxIterations)
}
