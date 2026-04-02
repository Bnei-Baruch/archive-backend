package tests

import (
	"context"
	"testing"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

func TestIncrementSourceLookupChunkRequestCount(t *testing.T) {
	ctx := llm.ContextWithReasoningToolState(context.Background())

	if count := llm.IncrementSourceLookupChunkRequestCount(ctx, "source|he"); count != 1 {
		t.Fatalf("unexpected first count: %d", count)
	}
	if count := llm.IncrementSourceLookupChunkRequestCount(ctx, "source|he"); count != 2 {
		t.Fatalf("unexpected second count: %d", count)
	}
	if count := llm.IncrementSourceLookupChunkRequestCount(ctx, "other|he"); count != 1 {
		t.Fatalf("unexpected count for second source: %d", count)
	}
}
