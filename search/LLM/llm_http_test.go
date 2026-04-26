package llm

import (
	"context"
	"errors"
	"testing"
)

func TestCallLLMAPIHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var response map[string]interface{}
	err := callLLMAPI(ctx, nil, "", map[string]string{"q": "test"}, "http://127.0.0.1:1", &response, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
