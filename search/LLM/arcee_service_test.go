package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestArceeReasoningWithToolsUsesChatCompletionsAPI(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewArceeServiceWithOptions("test-token", nil, nil, "https://api.arcee.ai/api/v1/chat/completions")
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if req.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("unexpected authorization header: %s", req.Header.Get("Authorization"))
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			if payload["stream"] != false {
				t.Fatalf("expected stream=false, got %#v", payload["stream"])
			}
			if payload["reasoning_effort"] != "high" {
				t.Fatalf("unexpected reasoning_effort: %#v", payload["reasoning_effort"])
			}
			messages, ok := payload["messages"].([]interface{})
			if !ok || len(messages) == 0 {
				t.Fatalf("expected messages array, got %#v", payload["messages"])
			}

			var body []byte
			switch len(requests) {
			case 1:
				if payload["tool_choice"] != "auto" {
					t.Fatalf("expected tool_choice=auto, got %#v", payload["tool_choice"])
				}
				if payload["tools"] == nil {
					t.Fatalf("expected tools in request")
				}
				if payload["response_format"] != nil {
					t.Fatalf("did not expect response_format during Arcee tool calls, got %#v", payload["response_format"])
				}
				systemMessage, ok := messages[0].(map[string]interface{})
				if !ok || strings.Contains(fmt.Sprint(systemMessage["content"]), "Return only valid JSON that matches this JSON Schema exactly.") {
					t.Fatalf("did not expect structured output instruction during Arcee tool calls: %#v", messages[0])
				}
				body = mustJSON(t, map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":      "assistant",
								"reasoning": "searching",
								"tool_calls": []map[string]interface{}{
									{
										"id":   "call_1",
										"type": "function",
										"function": map[string]interface{}{
											"name":      "elasticsearch_search",
											"arguments": `{"term":"זוהר"}`,
										},
									},
								},
							},
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     10,
						"completion_tokens": 5,
						"total_tokens":      15,
					},
				})
			case 2:
				if payload["tool_choice"] != "auto" {
					t.Fatalf("expected tool_choice=auto, got %#v", payload["tool_choice"])
				}
				if payload["tools"] == nil {
					t.Fatalf("expected tools in request")
				}
				if payload["response_format"] != nil {
					t.Fatalf("did not expect response_format during Arcee tool calls, got %#v", payload["response_format"])
				}
				if len(messages) != 4 {
					t.Fatalf("expected system, user, assistant, tool messages; got %d", len(messages))
				}
				assistantMessage, ok := messages[2].(map[string]interface{})
				if !ok || assistantMessage["role"] != "assistant" {
					t.Fatalf("unexpected assistant message: %#v", messages[2])
				}
				if assistantMessage["reasoning"] != "searching" {
					t.Fatalf("expected assistant reasoning to be preserved, got %#v", assistantMessage["reasoning"])
				}
				toolMessage, ok := messages[3].(map[string]interface{})
				if !ok || toolMessage["role"] != "tool" || toolMessage["tool_call_id"] != "call_1" {
					t.Fatalf("unexpected tool message: %#v", messages[3])
				}
				toolContent := fmt.Sprint(toolMessage["content"])
				if strings.Contains(toolContent, "_id") || strings.Contains(toolContent, "bad-hit-id") {
					t.Fatalf("expected ES hit _id to be hidden from Trinity models, got %s", toolContent)
				}
				if !strings.Contains(toolContent, "good-mdb-uid") {
					t.Fatalf("expected canonical mdb_uid to remain in tool content, got %s", toolContent)
				}
				body = mustJSON(t, map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":              "assistant",
								"content":           nil,
								"reasoning_content": "I found the answer.",
							},
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     12,
						"completion_tokens": 7,
						"total_tokens":      19,
					},
				})
			case 3:
				if payload["tools"] != nil {
					t.Fatalf("did not expect tools in Arcee final structured call")
				}
				if payload["tool_choice"] != nil {
					t.Fatalf("did not expect tool_choice in Arcee final structured call")
				}
				responseFormat, ok := payload["response_format"].(map[string]interface{})
				if !ok || responseFormat["type"] != "json_object" {
					t.Fatalf("expected response_format.type=json_object, got %#v", payload["response_format"])
				}
				systemMessage, ok := messages[0].(map[string]interface{})
				if !ok || !strings.Contains(fmt.Sprint(systemMessage["content"]), "Return only valid JSON that matches this JSON Schema exactly.") {
					t.Fatalf("expected structured output instruction in Arcee final structured call: %#v", messages[0])
				}
				body = mustJSON(t, map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":              "assistant",
								"content":           nil,
								"reasoning_content": `{}`,
							},
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     14,
						"completion_tokens": 4,
						"total_tokens":      18,
					},
				})
			case 4:
				if payload["tools"] != nil {
					t.Fatalf("did not expect tools in Arcee retry structured call")
				}
				if payload["tool_choice"] != nil {
					t.Fatalf("did not expect tool_choice in Arcee retry structured call")
				}
				responseFormat, ok := payload["response_format"].(map[string]interface{})
				if !ok || responseFormat["type"] != "json_object" {
					t.Fatalf("expected response_format.type=json_object in retry, got %#v", payload["response_format"])
				}
				lastMessage, ok := messages[len(messages)-1].(map[string]interface{})
				if !ok || !strings.Contains(fmt.Sprint(lastMessage["content"]), "Do not return an empty object") {
					t.Fatalf("expected correction message before retry: %#v", messages[len(messages)-1])
				}
				body = mustJSON(t, map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":    "assistant",
								"content": `{"answer":"done"}`,
							},
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     16,
						"completion_tokens": 4,
						"total_tokens":      20,
					},
				})
			default:
				t.Fatalf("unexpected extra request")
				return nil, nil
			}

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
	var output struct {
		Answer string `json:"answer"`
	}

	err := service.GetReasoningStructuredOutputWithTools(
		schema,
		"trinity-mini",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "מצא לי זוהר"},
		},
		[]ToolCall{{
			Type: "function",
			Function: map[string]interface{}{
				"name": "elasticsearch_search",
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
			"elasticsearch_search": func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return `{"result":{"search_result":{"hits":{"hits":[{"_id":"bad-hit-id","_source":{"mdb_uid":"good-mdb-uid","title":"Good"}}]}}}}`, nil
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
	if len(requests) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(requests))
	}
}

func TestArceeGetStructuredOutputWithDebugUsesJSONMode(t *testing.T) {
	service := NewArceeServiceWithOptions("test-token", nil, nil, "https://api.arcee.ai")
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["tools"] != nil {
				t.Fatalf("did not expect tools in structured output request")
			}
			if payload["tool_choice"] != nil {
				t.Fatalf("did not expect tool_choice in structured output request")
			}
			if payload["reasoning_effort"] != nil {
				t.Fatalf("did not expect reasoning_effort for trinity-large-thinking, got %#v", payload["reasoning_effort"])
			}
			responseFormat, ok := payload["response_format"].(map[string]interface{})
			if !ok || responseFormat["type"] != "json_object" {
				t.Fatalf("expected response_format.type=json_object, got %#v", payload["response_format"])
			}
			if responseFormat["json_schema"] != nil {
				t.Fatalf("did not expect json_schema wrapper: %#v", responseFormat["json_schema"])
			}
			messages, ok := payload["messages"].([]interface{})
			if !ok || len(messages) == 0 {
				t.Fatalf("expected messages array, got %#v", payload["messages"])
			}
			systemMessage, ok := messages[0].(map[string]interface{})
			if !ok || !strings.Contains(fmt.Sprint(systemMessage["content"]), "Return only valid JSON that matches this JSON Schema exactly.") {
				t.Fatalf("expected structured output instruction in system message: %#v", messages[0])
			}

			body := mustJSON(t, map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": `{"needs_another_iteration":false,"recommendation":""}`,
						},
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     20,
					"completion_tokens": 8,
					"total_tokens":      28,
				},
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}

	effort := "high"
	maxTokens := 128
	output := ReasoningSearchVerificationResponse{}
	debug, err := service.GetStructuredOutputWithDebugInfo(
		GenerateReasoningSearchVerificationResponseJSONSchema(),
		"trinity-large-thinking",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "Verify the results."},
			{Role: "user", Content: `{"query":"ד בחינות דאור ישר"}`},
		},
		nil,
		&effort,
		true,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.NeedsAnotherIteration {
		t.Fatalf("expected no extra iteration")
	}
	if debug == nil || debug.TotalTokens != 28 {
		t.Fatalf("unexpected debug totals: %#v", debug)
	}
}

func TestNormalizeArceeAPIBaseURL(t *testing.T) {
	tests := map[string]string{
		"":                            "https://api.arcee.ai/api/v1",
		"https://api.arcee.ai":        "https://api.arcee.ai/api/v1",
		"https://api.arcee.ai/api/v1": "https://api.arcee.ai/api/v1",
		"https://api.arcee.ai/api/v1/chat/completions":         "https://api.arcee.ai/api/v1",
		"https://conductor.arcee.ai":                           "https://conductor.arcee.ai/v1",
		"https://conductor.arcee.ai/v1/chat/completions":       "https://conductor.arcee.ai/v1",
		"https://custom.example.com/arcee/v1/chat/completions": "https://custom.example.com/arcee/v1",
	}

	for input, expected := range tests {
		actual := normalizeArceeAPIBaseURL(input)
		if actual != expected {
			t.Fatalf("normalizeArceeAPIBaseURL(%q)=%q, want %q", input, actual, expected)
		}
	}
}

func TestArceeRejectsInvalidReasoningEffort(t *testing.T) {
	effort := "xhigh"
	_, err := arceeReasoningEffortValue("trinity-mini", &effort)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "supported values are minimal, low, medium, high") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArceeSkipsReasoningEffortForTrinityLargeThinking(t *testing.T) {
	effort := "high"
	value, err := arceeReasoningEffortValue("trinity-large-thinking", &effort)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != nil {
		t.Fatalf("expected reasoning effort to be skipped, got %#v", *value)
	}

	value, err = arceeReasoningEffortValue("arcee-ai/trinity-large-thinking", &effort)
	if err != nil {
		t.Fatalf("unexpected error for OpenRouter-style slug: %v", err)
	}
	if value != nil {
		t.Fatalf("expected reasoning effort to be skipped for OpenRouter-style slug, got %#v", *value)
	}
}
