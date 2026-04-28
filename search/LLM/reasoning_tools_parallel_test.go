package llm

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestExecuteReasoningToolExecutionsParallelizesElasticsearchAndPreservesOrder(t *testing.T) {
	var mu sync.Mutex
	active := 0
	maxActive := 0
	handlers := map[string]ToolHandler{
		"elasticsearch_search": func(ctx context.Context, arguments json.RawMessage) (string, error) {
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
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"first"`)},
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"second"`)},
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"third"`)},
	}, handlers, nil)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if maxActive <= 1 {
		t.Fatalf("expected elasticsearch calls to overlap, max active calls: %d", maxActive)
	}
	if len(results) != 3 || results[0].Output != `"first"` || results[1].Output != `"second"` || results[2].Output != `"third"` {
		t.Fatalf("unexpected ordered results: %#v", results)
	}
}
