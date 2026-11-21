package llm

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIGetChatResponseUsesMaxCompletionTokens(t *testing.T) {
	var raw map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	service := NewOpenAIServiceWithOptions("test-token", nil, nil, server.URL)
	service.client = server.Client()
	maxTokens := 123
	msg, err := service.GetChatResponse(context.Background(), "gpt-5.4", &maxTokens, []LLMBotMessage{
		{Role: "system", Content: "You are a planner."},
		{Role: "user", Content: "אהבה"},
	}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil || msg.Content != "ok" {
		t.Fatalf("unexpected response: %+v", msg)
	}
	if _, ok := raw["max_completion_tokens"]; !ok {
		t.Fatalf("expected max_completion_tokens in request body: %+v", raw)
	}
	if _, ok := raw["max_tokens"]; ok {
		t.Fatalf("did not expect max_tokens in request body: %+v", raw)
	}
}

func TestOpenAIGPT56ChatUsesExplicitPromptCaching(t *testing.T) {
	var raw map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	service := NewOpenAIServiceWithOptions("test-token", nil, nil, server.URL)
	service.client = server.Client()
	cacheKey := "reasoning-search-planning"
	_, err := service.GetChatResponse(context.Background(), "gpt-5.6-terra", nil, []LLMBotMessage{
		{Role: "system", Content: "Stable instructions."},
		{Role: "user", Content: "Variable query."},
	}, &cacheKey, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	options := raw["prompt_cache_options"].(map[string]interface{})
	if options["mode"] != "explicit" {
		t.Fatalf("expected explicit prompt cache mode, got %#v", options)
	}
	messages := raw["messages"].([]interface{})
	system := messages[0].(map[string]interface{})
	content := system["content"].([]interface{})
	block := content[0].(map[string]interface{})
	if block["type"] != "text" || block["text"] != "Stable instructions." {
		t.Fatalf("unexpected system content block: %#v", block)
	}
	breakpoint := block["prompt_cache_breakpoint"].(map[string]interface{})
	if breakpoint["mode"] != "explicit" {
		t.Fatalf("expected explicit cache breakpoint, got %#v", breakpoint)
	}
	if messages[1].(map[string]interface{})["content"] != "Variable query." {
		t.Fatalf("expected variable user content after the breakpoint: %#v", messages)
	}
}

func TestOpenAIGPT56ResponsesUsesExplicitPromptCaching(t *testing.T) {
	var raw map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"resp_1",
			"status":"completed",
			"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"answer\":\"ok\"}"}]}]
		}`))
	}))
	defer server.Close()

	service := NewOpenAIServiceWithOptions("test-token", nil, nil, server.URL)
	service.client = server.Client()
	cacheKey := "reasoning-search-classifier"
	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	output := struct {
		Answer string `json:"answer"`
	}{}
	if err := service.GetStructuredOutput(context.Background(), schema, "gpt-5.6-luna", nil, []LLMBotMessage{
		{Role: "system", Content: "Stable classifier instructions."},
		{Role: "user", Content: "Variable candidates."},
	}, &cacheKey, nil, &output); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Answer != "ok" {
		t.Fatalf("unexpected output: %+v", output)
	}
	if _, ok := raw["instructions"]; ok {
		t.Fatalf("did not expect top-level instructions with an explicit breakpoint: %#v", raw)
	}
	options := raw["prompt_cache_options"].(map[string]interface{})
	if options["mode"] != "explicit" {
		t.Fatalf("expected explicit prompt cache mode, got %#v", options)
	}
	input := raw["input"].([]interface{})
	system := input[0].(map[string]interface{})
	content := system["content"].([]interface{})
	block := content[0].(map[string]interface{})
	if block["type"] != "input_text" || block["text"] != "Stable classifier instructions." {
		t.Fatalf("unexpected Responses instruction block: %#v", block)
	}
	if input[1].(map[string]interface{})["content"] != "Variable candidates." {
		t.Fatalf("expected variable input after the breakpoint: %#v", input)
	}
}

func TestOpenAIGPT56ReasoningSessionCachesInstructionsBeforeUserInput(t *testing.T) {
	requests := []map[string]interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		requests = append(requests, raw)
		responseID := "resp_1"
		answer := "first"
		if len(requests) == 2 {
			responseID = "resp_2"
			answer = "second"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mustJSON(t, map[string]interface{}{
			"id":     responseID,
			"status": "completed",
			"output": []map[string]interface{}{{
				"type": "message",
				"role": "assistant",
				"content": []map[string]interface{}{{
					"type": "output_text",
					"text": `{"answer":"` + answer + `"}`,
				}},
			}},
		}))
	}))
	defer server.Close()

	sessions := NewOpenAIReasoningSessionStore(time.Minute)
	defer sessions.Close()
	service := NewOpenAIServiceWithOptions("test-token", nil, sessions, server.URL)
	service.client = server.Client()
	cacheKey := "reasoning-search:m=gpt-5.6-terra"
	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	tools := []ToolCall{{
		Type: "function",
		Function: map[string]interface{}{
			"name": "lookup",
			"parameters": map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
	}}
	output := struct {
		Answer string `json:"answer"`
	}{}
	sessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		context.Background(), nil, nil, schema, "gpt-5.6-terra", nil,
		[]LLMBotMessage{
			{Role: "system", Content: "Stable search instructions."},
			{Role: "user", Content: "Variable search query."},
		},
		tools,
		map[string]ToolHandler{"lookup": func(context.Context, json.RawMessage) (string, error) { return `{}`, nil }},
		nil, nil, &cacheKey, nil, false, 2, &output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Answer != "first" {
		t.Fatalf("unexpected output: %+v", output)
	}
	raw := requests[0]
	if _, ok := raw["instructions"]; ok {
		t.Fatalf("did not expect top-level instructions: %#v", raw)
	}
	input := raw["input"].([]interface{})
	system := input[0].(map[string]interface{})
	block := system["content"].([]interface{})[0].(map[string]interface{})
	if block["text"] != "Stable search instructions." || block["prompt_cache_breakpoint"] == nil {
		t.Fatalf("expected cached system prefix first, got %#v", input)
	}
	if input[1].(map[string]interface{})["content"] != "Variable search query." {
		t.Fatalf("expected user input after the cache breakpoint: %#v", input)
	}

	output.Answer = ""
	_, err = service.GetReasoningStructuredOutputWithToolsForSession(
		context.Background(), &sessionID, nil, schema, "ignored-on-resume", nil,
		[]LLMBotMessage{
			{Role: "system", Content: "Updated follow-up instructions."},
			{Role: "user", Content: "Follow-up query."},
		},
		tools,
		map[string]ToolHandler{"lookup": func(context.Context, json.RawMessage) (string, error) { return `{}`, nil }},
		nil, nil, &cacheKey, nil, false, 2, &output,
	)
	if err != nil {
		t.Fatalf("unexpected follow-up error: %v", err)
	}
	if output.Answer != "second" {
		t.Fatalf("unexpected follow-up output: %+v", output)
	}
	followup := requests[1]
	if followup["previous_response_id"] != "resp_1" {
		t.Fatalf("expected provider continuation, got %#v", followup["previous_response_id"])
	}
	followupInput := followup["input"].([]interface{})
	if followupInput[0].(map[string]interface{})["content"] != "Updated follow-up instructions." {
		t.Fatalf("expected updated instructions without a new breakpoint: %#v", followupInput)
	}
}

func TestOpenAIOlderModelKeepsAutomaticPromptCachingPayload(t *testing.T) {
	var raw map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	service := NewOpenAIServiceWithOptions("test-token", nil, nil, server.URL)
	service.client = server.Client()
	cacheKey := "reasoning-search-planning"
	_, err := service.GetChatResponse(context.Background(), "gpt-5.4", nil, []LLMBotMessage{
		{Role: "system", Content: "Stable instructions."},
		{Role: "user", Content: "Variable query."},
	}, &cacheKey, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := raw["prompt_cache_options"]; ok {
		t.Fatalf("older models must not receive prompt_cache_options: %#v", raw)
	}
	messages := raw["messages"].([]interface{})
	if messages[0].(map[string]interface{})["content"] != "Stable instructions." {
		t.Fatalf("expected the existing string message format: %#v", messages[0])
	}
}

func TestOpenAIGPT56CanUseAutomaticPromptCaching(t *testing.T) {
	var raw map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	service := NewOpenAIServiceWithOptions("test-token", nil, nil, server.URL)
	service.explicitPromptCachingEnabled = false
	cacheKey := "reasoning-search"
	_, err := service.GetChatResponse(context.Background(), "gpt-5.6-terra", nil, []LLMBotMessage{
		{Role: "system", Content: "Stable instructions."},
		{Role: "user", Content: "Variable query."},
	}, &cacheKey, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := raw["prompt_cache_options"]; ok {
		t.Fatalf("automatic caching must not send prompt_cache_options: %#v", raw)
	}
	if raw["prompt_cache_key"] != cacheKey {
		t.Fatalf("expected prompt_cache_key to remain available, got %#v", raw["prompt_cache_key"])
	}
	messages := raw["messages"].([]interface{})
	if messages[0].(map[string]interface{})["content"] != "Stable instructions." {
		t.Fatalf("expected the original string message format: %#v", messages[0])
	}
}

func TestOpenAIExplicitPromptCachingRapidStageOverride(t *testing.T) {
	service := NewOpenAIService("test-token")
	service.explicitPromptCachingEnabled = false
	service.explicitPromptCachingOverrides = map[string]bool{
		"reasoning-search-rapid-classifier:": true,
	}

	gatherKey := "reasoning-search-rapid-gather:m=gpt-5.6-terra:e=medium"
	if options := service.promptCacheOptions("gpt-5.6-terra", &gatherKey); options != nil {
		t.Fatalf("expected gather to inherit automatic caching, got %#v", options)
	}
	classifierKey := "reasoning-search-rapid-classifier:m=gpt-5.6-luna:e=medium"
	if options := service.promptCacheOptions("gpt-5.6-luna", &classifierKey); options == nil || options.Mode != "explicit" {
		t.Fatalf("expected classifier override to enable explicit caching, got %#v", options)
	}
}

func TestSupportsExplicitOpenAIPromptCaching(t *testing.T) {
	tests := map[string]bool{
		"gpt-5.4":       false,
		"gpt-5.6":       true,
		"gpt-5.6-terra": true,
		"gpt-6":         true,
		"gpt-6.1":       true,
		"gpt-oss-120b":  false,
	}
	for model, expected := range tests {
		if actual := supportsExplicitOpenAIPromptCaching(model); actual != expected {
			t.Errorf("model %q: expected %t, got %t", model, expected, actual)
		}
	}
}

func TestOpenAICacheWriteTokensHaveSeparateCost(t *testing.T) {
	service := NewOpenAIServiceWithPricing("test-token", []ModelPricing{{
		Model:                     "gpt-5.6",
		InputPer1MTokensUSD:       4,
		CachedInputPer1MTokensUSD: 1,
		OutputPer1MTokensUSD:      10,
	}})
	var apiUsage OpenAIUsage
	if err := json.Unmarshal([]byte(`{
		"input_tokens":1000,
		"input_tokens_details":{"cached_tokens":200,"cache_write_tokens":300},
		"output_tokens":100,
		"total_tokens":1100
	}`), &apiUsage); err != nil {
		t.Fatalf("failed to parse usage: %v", err)
	}
	usage := reasoningUsageTotals(&apiUsage)
	cost := service.estimateCost("gpt-5.6", "", usage)

	if usage.UncachedInputTokens() != 500 {
		t.Fatalf("unexpected ordinary input token count: %d", usage.UncachedInputTokens())
	}
	if math.Abs(cost.EstimatedInputCostUSD-0.002) > 1e-12 {
		t.Fatalf("unexpected ordinary input cost: %.6f", cost.EstimatedInputCostUSD)
	}
	if math.Abs(cost.EstimatedCachedInputCostUSD-0.0002) > 1e-12 {
		t.Fatalf("unexpected cached input cost: %.6f", cost.EstimatedCachedInputCostUSD)
	}
	if cost.CacheWritePer1MTokensUSD != 5 || math.Abs(cost.EstimatedCacheWriteCostUSD-0.0015) > 1e-12 {
		t.Fatalf("unexpected cache write cost: rate=%.2f cost=%.6f", cost.CacheWritePer1MTokensUSD, cost.EstimatedCacheWriteCostUSD)
	}
	if math.Abs(cost.EstimatedCostUSD-0.0047) > 1e-12 {
		t.Fatalf("unexpected total cost: %.6f", cost.EstimatedCostUSD)
	}
}
