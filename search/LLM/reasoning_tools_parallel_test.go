package llm

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
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

func TestExecuteReasoningToolExecutionsLimitsCallsPerIteration(t *testing.T) {
	oldLimit := viper.Get("llm.reasoning-search-max-tools-per-iteration")
	defer viper.Set("llm.reasoning-search-max-tools-per-iteration", oldLimit)
	viper.Set("llm.reasoning-search-max-tools-per-iteration", 2)

	var mu sync.Mutex
	executed := 0
	handlers := map[string]ToolHandler{
		"elasticsearch_search": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			mu.Lock()
			executed++
			mu.Unlock()
			return string(arguments), nil
		},
	}
	calls := []ReasoningToolExecution{
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"first"`)},
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"second"`)},
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"third"`)},
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"fourth"`)},
	}

	ctx := ContextWithReasoningToolState(context.Background(), nil, "")
	results, err := ExecuteReasoningToolExecutions(ctx, calls, handlers, nil)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if executed != 2 {
		t.Fatalf("expected two executed calls, got %d", executed)
	}
	if len(results) != len(calls) {
		t.Fatalf("expected an output for every tool call, got %d", len(results))
	}
	if results[0].Output != `"first"` || results[1].Output != `"second"` {
		t.Fatalf("unexpected executed outputs: %#v", results[:2])
	}
	if results[0].Skipped || results[1].Skipped {
		t.Fatalf("executed calls must not be marked skipped: %#v", results[:2])
	}
	for _, result := range results[2:] {
		if !result.Skipped {
			t.Fatalf("limited call must be marked skipped: %#v", result)
		}
		if !strings.Contains(result.Output, `"error":"tool_call_limit_reached"`) {
			t.Fatalf("expected limit output, got %s", result.Output)
		}
	}
	debug := ToolDebugInfoFromContext(ctx)
	if debug == nil || len(debug.SkippedToolCalls) != 2 {
		t.Fatalf("expected two skipped calls in debug output, got %#v", debug)
	}
	if debug.SkippedToolCalls[0].Name != "elasticsearch_search" || debug.SkippedToolCalls[0].Params != `"third"` || debug.SkippedToolCalls[1].Params != `"fourth"` {
		t.Fatalf("unexpected skipped tool debug output: %#v", debug.SkippedToolCalls)
	}
	debugJSON, err := json.Marshal(debug)
	if err != nil {
		t.Fatalf("failed to marshal debug output: %v", err)
	}
	if !strings.Contains(string(debugJSON), `"skipped_tool_calls":[{"name":"elasticsearch_search","params":"\"third\""}`) {
		t.Fatalf("missing skipped_tool_calls in final debug JSON: %s", debugJSON)
	}
}

func TestExecuteReasoningToolExecutionsLimitOneUsesSingleCallPath(t *testing.T) {
	oldLimit := viper.Get("llm.reasoning-search-max-tools-per-iteration")
	defer viper.Set("llm.reasoning-search-max-tools-per-iteration", oldLimit)
	viper.Set("llm.reasoning-search-max-tools-per-iteration", 1)

	results, err := ExecuteReasoningToolExecutions(context.Background(), []ReasoningToolExecution{
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"first"`)},
		{Name: "elasticsearch_search", Arguments: json.RawMessage(`"second"`)},
	}, map[string]ToolHandler{
		"elasticsearch_search": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			return string(arguments), nil
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if len(results) != 2 || results[0].Output != `"first"` || results[0].Skipped || !results[1].Skipped {
		t.Fatalf("unexpected results: %#v", results)
	}
}
