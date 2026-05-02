package llm

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestExecuteReasoningToolExecutionsParallelizesAllToolsAndPreservesOrder(t *testing.T) {
	var mu sync.Mutex
	active := 0
	maxActive := 0
	handler := func(ctx context.Context, arguments json.RawMessage) (string, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()

		time.Sleep(25 * time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()
		return string(arguments), nil
	}
	handlers := map[string]ToolHandler{
		"elasticsearch_search":  handler,
		"get_content_unit":      handler,
		"get_sources_by_source": handler,
		"query_source_ai":       handler,
	}

	results, err := ExecuteReasoningToolExecutions(context.Background(), []ReasoningToolExecution{
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"first"`)},
		{Name: "get_content_unit", Arguments: json.RawMessage(`"second"`)},
		{Name: "get_sources_by_source", Arguments: json.RawMessage(`"third"`)},
		{Name: "query_source_ai", Arguments: json.RawMessage(`"fourth"`)},
	}, handlers, nil)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if maxActive <= 1 {
		t.Fatalf("expected tool calls to overlap, max active calls: %d", maxActive)
	}
	if len(results) != 4 || results[0].Output != `"first"` || results[1].Output != `"second"` || results[2].Output != `"third"` || results[3].Output != `"fourth"` {
		t.Fatalf("unexpected ordered results: %#v", results)
	}
}

func TestExecuteReasoningToolExecutionsLimitsParallelism(t *testing.T) {
	var mu sync.Mutex
	active := 0
	maxActive := 0
	handlers := map[string]ToolHandler{
		"get_content_unit": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()

			time.Sleep(25 * time.Millisecond)

			mu.Lock()
			active--
			mu.Unlock()
			return string(arguments), nil
		},
	}

	results, err := ExecuteReasoningToolExecutions(context.Background(), []ReasoningToolExecution{
		{Name: "get_content_unit", Arguments: json.RawMessage(`"first"`)},
		{Name: "get_content_unit", Arguments: json.RawMessage(`"second"`)},
		{Name: "get_content_unit", Arguments: json.RawMessage(`"third"`)},
		{Name: "get_content_unit", Arguments: json.RawMessage(`"fourth"`)},
		{Name: "get_content_unit", Arguments: json.RawMessage(`"fifth"`)},
	}, handlers, nil)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if maxActive <= 1 || maxActive > parallelReasoningToolConcurrency {
		t.Fatalf("expected parallelism within concurrency limit, got max active calls %d with limit %d", maxActive, parallelReasoningToolConcurrency)
	}
	if len(results) != 5 || results[0].Output != `"first"` || results[1].Output != `"second"` || results[2].Output != `"third"` || results[3].Output != `"fourth"` || results[4].Output != `"fifth"` {
		t.Fatalf("unexpected ordered results: %#v", results)
	}
}
