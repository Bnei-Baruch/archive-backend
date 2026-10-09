package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDeepSeekReasoningWithToolsUsesOfficialChatCompletions(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewDeepSeekServiceWithOptions("test-token", nil, nil, "https://api.deepseek.test")
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			thinking, ok := payload["thinking"].(map[string]interface{})
			if !ok || thinking["type"] != "enabled" {
				t.Fatalf("expected thinking.enabled, got %#v", payload["thinking"])
			}
			if payload["reasoning_effort"] != "high" {
				t.Fatalf("expected medium effort to map to high, got %#v", payload["reasoning_effort"])
			}
			format, ok := payload["response_format"].(map[string]interface{})
			if !ok || format["type"] != "json_object" {
				t.Fatalf("expected json_object response format, got %#v", payload["response_format"])
			}
			if payload["tools"] == nil {
				t.Fatalf("expected tools in request")
			}
			if payload["tool_choice"] != "auto" {
				t.Fatalf("unexpected tool_choice: %#v", payload["tool_choice"])
			}

			var body []byte
			switch len(requests) {
			case 1:
				body = mustJSON(t, map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":              "assistant",
								"content":           "",
								"reasoning_content": "searching",
								"tool_calls": []map[string]interface{}{
									{
										"id":   "call_1",
										"type": "function",
										"function": map[string]interface{}{
											"name":      "lookup",
											"arguments": `{"term":"זוהר"}`,
										},
									},
								},
							},
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":           10,
						"prompt_cache_hit_tokens": 4,
						"completion_tokens":       5,
						"total_tokens":            15,
					},
				})
			case 2:
				messages, ok := payload["messages"].([]interface{})
				if !ok {
					t.Fatalf("expected messages array, got %T", payload["messages"])
				}
				var assistant map[string]interface{}
				var tool map[string]interface{}
				for _, item := range messages {
					msg, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					if msg["role"] == "assistant" && msg["tool_calls"] != nil {
						assistant = msg
					}
					if msg["role"] == "tool" {
						tool = msg
					}
				}
				if assistant == nil || assistant["reasoning_content"] != "searching" {
					t.Fatalf("expected assistant tool-turn reasoning_content, got %#v", assistant)
				}
				if tool == nil || tool["tool_call_id"] != "call_1" || tool["content"] != `{"hits":[1]}` {
					t.Fatalf("unexpected tool response message: %#v", tool)
				}
				body = mustJSON(t, map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":              "assistant",
								"content":           `{"answer":"done"}`,
								"reasoning_content": "finalizing",
							},
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     12,
						"completion_tokens": 7,
						"total_tokens":      19,
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
	effort := "medium"
	maxTokens := 256
	var output struct {
		Answer string `json:"answer"`
	}

	err := service.GetReasoningStructuredOutputWithTools(
		context.Background(),
		schema,
		"deepseek-v4-flash",
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
	messages := requests[0]["messages"].([]interface{})
	system := messages[0].(map[string]interface{})["content"].(string)
	if !strings.Contains(system, "Return only valid JSON") {
		t.Fatalf("expected JSON schema instruction in system message: %s", system)
	}
}

func TestDeepSeekThinkingEffortMapping(t *testing.T) {
	effort := "minimal"
	thinking, mapped, err := deepseekThinkingAndEffort(&effort)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thinking == nil || thinking.Type != "disabled" || mapped != nil {
		t.Fatalf("unexpected minimal mapping: thinking=%#v effort=%#v", thinking, mapped)
	}

	effort = "xhigh"
	thinking, mapped, err = deepseekThinkingAndEffort(&effort)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thinking == nil || thinking.Type != "enabled" || mapped == nil || *mapped != "max" {
		t.Fatalf("unexpected xhigh mapping: thinking=%#v effort=%#v", thinking, mapped)
	}
}

func TestNormalizeDeepSeekAPIBaseURL(t *testing.T) {
	if got := normalizeDeepSeekAPIBaseURL(""); got != "https://api.deepseek.com" {
		t.Fatalf("unexpected default endpoint: %s", got)
	}
	if got := normalizeDeepSeekAPIBaseURL("https://api.deepseek.com/chat/completions"); got != "https://api.deepseek.com" {
		t.Fatalf("unexpected normalized endpoint: %s", got)
	}
}

func TestOpenAIUsageReadsDeepSeekCacheHitTokens(t *testing.T) {
	var usage OpenAIUsage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12,"prompt_cache_hit_tokens":4}`), &usage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.InputTokens != 10 || usage.OutputTokens != 2 || usage.TotalTokens != 12 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if usage.InputTokensDetails == nil || usage.InputTokensDetails.CachedTokens != 4 {
		t.Fatalf("expected cached tokens from prompt_cache_hit_tokens, got %#v", usage.InputTokensDetails)
	}
}
