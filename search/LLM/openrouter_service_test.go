package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOpenRouterReasoningWithToolsUsesResponsesAPI(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewOpenRouterServiceWithOptions("test-token", nil, nil, "https://openrouter.test")
	service.providerPreferences = &ResponsesProvider{
		Sort:              "latency",
		RequireParameters: boolPtr(true),
	}
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/v1/responses" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			if payload["previous_response_id"] != nil {
				t.Fatalf("did not expect previous_response_id in OpenRouter request")
			}
			reasoning, ok := payload["reasoning"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected reasoning object, got %T", payload["reasoning"])
			}
			if _, exists := reasoning["summary"]; exists {
				t.Fatalf("did not expect reasoning.summary in OpenRouter request")
			}
			if payload["prompt_cache_key"] != "reasoning-search:m=openai/gpt-oss-120b:e=high" {
				t.Fatalf("unexpected prompt_cache_key: %#v", payload["prompt_cache_key"])
			}
			if payload["text"] == nil {
				t.Fatalf("expected structured output text configuration")
			}
			if payload["tools"] == nil {
				t.Fatalf("expected tools in request")
			}
			expectedToolChoice := "required"
			if len(requests) > 1 {
				expectedToolChoice = "auto"
			}
			if payload["tool_choice"] != expectedToolChoice {
				t.Fatalf("unexpected tool_choice: %#v", payload["tool_choice"])
			}
			provider, ok := payload["provider"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected provider object, got %T", payload["provider"])
			}
			if provider["sort"] != "latency" {
				t.Fatalf("unexpected provider sort: %#v", provider["sort"])
			}
			if provider["require_parameters"] != true {
				t.Fatalf("unexpected provider require_parameters: %#v", provider["require_parameters"])
			}

			var body []byte
			switch len(requests) {
			case 1:
				body = mustJSON(t, map[string]interface{}{
					"id":     "resp_1",
					"status": "completed",
					"output": []map[string]interface{}{
						{
							"type":      "function_call",
							"id":        "fc_1",
							"call_id":   "call_1",
							"name":      "lookup",
							"arguments": `{"term":"זוהר"}`,
						},
					},
					"usage": map[string]interface{}{
						"input_tokens":  10,
						"output_tokens": 5,
						"total_tokens":  15,
					},
				})
			case 2:
				input, ok := payload["input"].([]interface{})
				if !ok {
					t.Fatalf("expected input array, got %T", payload["input"])
				}
				if len(input) != 3 {
					t.Fatalf("expected user message, function call, and tool output, got %d items", len(input))
				}
				functionCall, ok := input[1].(map[string]interface{})
				if !ok || functionCall["type"] != "function_call" {
					t.Fatalf("expected function_call in second request input, got %#v", input[1])
				}
				toolOutput, ok := input[2].(map[string]interface{})
				if !ok || toolOutput["type"] != "function_call_output" {
					t.Fatalf("expected function_call_output in second request input, got %#v", input[2])
				}
				body = mustJSON(t, map[string]interface{}{
					"id":     "resp_2",
					"status": "completed",
					"output": []map[string]interface{}{
						{
							"type": "message",
							"role": "assistant",
							"content": []map[string]interface{}{
								{
									"type": "output_text",
									"text": `{"answer":"done"}`,
								},
							},
						},
					},
					"usage": map[string]interface{}{
						"input_tokens":  12,
						"output_tokens": 7,
						"total_tokens":  19,
					},
				})
			default:
				t.Fatalf("unexpected extra request")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}

	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	promptCacheKey := "reasoning-search:m=openai/gpt-oss-120b:e=high"
	effort := "high"
	maxTokens := 256
	var output struct {
		Answer string `json:"answer"`
	}

	err := service.GetReasoningStructuredOutputWithTools(
		schema,
		"openai/gpt-oss-120b",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "מצא לי זוהר"},
		},
		[]ToolCall{{
			Type: "function",
			Function: map[string]interface{}{
				"name": "lookup",
				"parameters": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"term": map[string]interface{}{"type": "string"},
					},
					"required": []string{"term"},
				},
			},
		}},
		map[string]ToolHandler{
			"lookup": func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return `{"hits":[1]}`, nil
			},
		},
		&promptCacheKey,
		&effort,
		true,
		4,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Answer != "done" {
		t.Fatalf("unexpected answer: %s", output.Answer)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
}

func TestOpenRouterUsesConfiguredRequiredToolIterations(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewOpenRouterServiceWithOptions("test-token", nil, nil, "https://openrouter.test")
	service.requiredToolIterations = 5
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			expectedToolChoice := "required"
			if len(requests) > 5 {
				expectedToolChoice = "auto"
			}
			if payload["tool_choice"] != expectedToolChoice {
				t.Fatalf("request %d expected tool_choice %q, got %#v", len(requests), expectedToolChoice, payload["tool_choice"])
			}

			body := map[string]interface{}{
				"id":     "resp_tool",
				"status": "completed",
				"output": []map[string]interface{}{
					{
						"type":      "function_call",
						"id":        "fc_1",
						"call_id":   "call_1",
						"name":      "lookup",
						"arguments": `{"term":"אור ישר"}`,
					},
				},
				"usage": map[string]interface{}{
					"input_tokens":  10,
					"output_tokens": 5,
					"total_tokens":  15,
				},
			}
			if len(requests) == 6 {
				body = map[string]interface{}{
					"id":     "resp_final",
					"status": "completed",
					"output": []map[string]interface{}{
						{
							"type": "message",
							"role": "assistant",
							"content": []map[string]interface{}{
								{
									"type": "output_text",
									"text": `{"answer":"done"}`,
								},
							},
						},
					},
					"usage": map[string]interface{}{
						"input_tokens":  12,
						"output_tokens": 7,
						"total_tokens":  19,
					},
				}
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(mustJSON(t, body))),
			}, nil
		}),
	}

	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	effort := "high"
	maxTokens := 256
	var output struct {
		Answer string `json:"answer"`
	}

	err := service.GetReasoningStructuredOutputWithTools(
		schema,
		"openai/gpt-oss-120b",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "מצא לי זוהר"},
		},
		[]ToolCall{{
			Type: "function",
			Function: map[string]interface{}{
				"name": "lookup",
				"parameters": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"term": map[string]interface{}{"type": "string"},
					},
					"required": []string{"term"},
				},
			},
		}},
		map[string]ToolHandler{
			"lookup": func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return `{"hits":[1]}`, nil
			},
		},
		nil,
		&effort,
		false,
		6,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Answer != "done" {
		t.Fatalf("unexpected answer: %s", output.Answer)
	}
	if len(requests) != 6 {
		t.Fatalf("expected 6 requests, got %d", len(requests))
	}
}

func TestOpenRouterGetStructuredOutputWithDebugReturnsUsage(t *testing.T) {
	service := NewOpenRouterServiceWithOptions("test-token", nil, nil, "https://openrouter.test")
	service.providerPreferences = &ResponsesProvider{
		Sort:              "latency",
		RequireParameters: boolPtr(true),
	}
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/v1/responses" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["text"] == nil {
				t.Fatalf("expected text.format in request")
			}
			if payload["max_output_tokens"] != float64(64) {
				t.Fatalf("unexpected max_output_tokens: %#v", payload["max_output_tokens"])
			}
			provider, ok := payload["provider"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected provider object, got %T", payload["provider"])
			}
			if provider["sort"] != "latency" {
				t.Fatalf("unexpected provider sort: %#v", provider["sort"])
			}
			if provider["require_parameters"] != true {
				t.Fatalf("unexpected provider require_parameters: %#v", provider["require_parameters"])
			}

			body := mustJSON(t, map[string]interface{}{
				"id":     "resp_structured",
				"status": "completed",
				"output": []map[string]interface{}{
					{
						"type": "message",
						"role": "assistant",
						"content": []map[string]interface{}{
							{
								"type": "output_text",
								"text": `{"answer":"done"}`,
							},
						},
					},
				},
				"usage": map[string]interface{}{
					"input_tokens":  10,
					"output_tokens": 5,
					"total_tokens":  15,
				},
			})

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}

	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	effort := "high"
	maxTokens := 64
	var output struct {
		Answer string `json:"answer"`
	}

	debug, err := service.GetStructuredOutputWithDebug(
		schema,
		"moonshotai/kimi-k2.5",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "מצא לי זוהר"},
		},
		nil,
		&effort,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Answer != "done" {
		t.Fatalf("unexpected answer: %s", output.Answer)
	}
	if debug == nil {
		t.Fatalf("expected debug info")
	}
	if debug.TotalTokens != 15 {
		t.Fatalf("unexpected total tokens: %d", debug.TotalTokens)
	}
	if debug.Model != "moonshotai/kimi-k2.5" {
		t.Fatalf("unexpected model: %s", debug.Model)
	}
}

func TestOpenRouterReasoningSessionReplaysHistory(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewOpenRouterServiceWithOptions("test-token", nil, NewChatReasoningSessionStore(time.Minute), "https://openrouter.test")
	defer service.sessions.Close()
	service.providerPreferences = &ResponsesProvider{
		Sort:              "latency",
		RequireParameters: boolPtr(true),
	}
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			body := mustJSON(t, map[string]interface{}{
				"id":     "resp_final",
				"status": "completed",
				"output": []map[string]interface{}{
					{
						"type": "message",
						"role": "assistant",
						"content": []map[string]interface{}{
							{
								"type": "output_text",
								"text": `{"answer":"ok"}`,
							},
						},
					},
				},
				"usage": map[string]interface{}{
					"input_tokens":  10,
					"output_tokens": 5,
					"total_tokens":  15,
				},
			})

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}

	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	effort := "high"
	maxTokens := 256
	tools := []ToolCall{{
		Type: "function",
		Function: map[string]interface{}{
			"name": "lookup",
			"parameters": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties":           map[string]interface{}{},
			},
		},
	}}
	handlers := map[string]ToolHandler{
		"lookup": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			return `{}`, nil
		},
	}

	var firstOutput struct {
		Answer string `json:"answer"`
	}
	sessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(
		nil,
		nil,
		schema,
		"google/gemma-4-31b-it",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "first question"},
		},
		tools,
		handlers,
		nil,
		&effort,
		false,
		3,
		&firstOutput,
	)
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}
	if sessionID == "" {
		t.Fatalf("expected session id")
	}

	var secondOutput struct {
		Answer string `json:"answer"`
	}
	_, err = service.GetReasoningStructuredOutputWithToolsForSession(
		&sessionID,
		nil,
		schema,
		"ignored-model",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "second question"},
		},
		tools,
		handlers,
		nil,
		&effort,
		false,
		3,
		&secondOutput,
	)
	if err != nil {
		t.Fatalf("unexpected error continuing session: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	secondInput, ok := requests[1]["input"].([]interface{})
	if !ok {
		t.Fatalf("expected input array on second request, got %T", requests[1]["input"])
	}
	if len(secondInput) != 3 {
		t.Fatalf("expected prior user, prior assistant, and new user in replayed history, got %d items", len(secondInput))
	}

	expectMessageRole(t, secondInput[0], "user")
	expectMessageRole(t, secondInput[1], "assistant")
	expectMessageRole(t, secondInput[2], "user")
}

func expectMessageRole(t *testing.T, item interface{}, expectedRole string) {
	t.Helper()

	message, ok := item.(map[string]interface{})
	if !ok {
		t.Fatalf("expected message object, got %T", item)
	}
	if message["type"] != "message" || message["role"] != expectedRole {
		t.Fatalf("unexpected message item: %#v", message)
	}
}

func mustJSON(t *testing.T, payload interface{}) []byte {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func boolPtr(value bool) *bool {
	return &value
}

func TestOpenRouterReasoningSessionReplayPreservesOnlyConversationMessages(t *testing.T) {
	history := sessionConversationMessages([]LLMBotMessage{
		{Role: "user", Content: "u"},
		{Role: "assistant", Content: "a"},
		{Role: "tool", Content: "ignored", ToolCallID: "1"},
	})
	if len(history) != 2 {
		t.Fatalf("expected only user and assistant messages, got %d", len(history))
	}
	if strings.Join([]string{history[0].Role, history[1].Role}, ",") != "user,assistant" {
		t.Fatalf("unexpected replay roles: %#v", history)
	}
}
