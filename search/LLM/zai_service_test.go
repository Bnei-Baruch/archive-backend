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

func TestZAIReasoningWithToolsUsesChatCompletionsAPI(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewZAIServiceWithOptions("test-token", nil, nil, "https://api.z.ai", nil, nil, nil, nil)
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/paas/v4/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			if payload["stream"] != false {
				t.Fatalf("expected stream=false, got %#v", payload["stream"])
			}
			if payload["tool_choice"] != "auto" {
				t.Fatalf("expected tool_choice=auto, got %#v", payload["tool_choice"])
			}
			if payload["tools"] == nil {
				t.Fatalf("expected tools in request")
			}
			responseFormat, ok := payload["response_format"].(map[string]interface{})
			if !ok || responseFormat["type"] != "json_object" {
				t.Fatalf("expected response_format.type=json_object, got %#v", payload["response_format"])
			}
			messages, ok := payload["messages"].([]interface{})
			if !ok || len(messages) == 0 {
				t.Fatalf("expected messages array, got %#v", payload["messages"])
			}
			systemMessage, ok := messages[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected system message object, got %#v", messages[0])
			}
			systemContent, _ := systemMessage["content"].(string)
			if !strings.Contains(systemContent, "Return only valid JSON that matches this JSON Schema exactly.") {
				t.Fatalf("expected structured-output instruction in system content: %q", systemContent)
			}
			if !strings.Contains(systemContent, `"required":["answer"]`) {
				t.Fatalf("expected schema text in system content: %q", systemContent)
			}
			thinking, ok := payload["thinking"].(map[string]interface{})
			if !ok || thinking["type"] != "enabled" || thinking["clear_thinking"] != true {
				t.Fatalf("unexpected thinking payload: %#v", payload["thinking"])
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
								"reasoning_content": "searching",
								"tool_calls": []map[string]interface{}{
									{
										"id":   "call_1",
										"type": "function",
										"function": map[string]interface{}{
											"name":      "lookup",
											"arguments": map[string]interface{}{"term": "זוהר"},
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
				messages, ok := payload["messages"].([]interface{})
				if !ok {
					t.Fatalf("expected messages array, got %T", payload["messages"])
				}
				if len(messages) != 4 {
					t.Fatalf("expected system, user, assistant, tool messages; got %d", len(messages))
				}
				assistantMessage, ok := messages[2].(map[string]interface{})
				if !ok || assistantMessage["role"] != "assistant" {
					t.Fatalf("unexpected assistant message: %#v", messages[2])
				}
				toolMessage, ok := messages[3].(map[string]interface{})
				if !ok || toolMessage["role"] != "tool" || toolMessage["tool_call_id"] != "call_1" {
					t.Fatalf("unexpected tool message: %#v", messages[3])
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
						"prompt_tokens":     12,
						"completion_tokens": 7,
						"total_tokens":      19,
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
		"glm-5.1",
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
}

func TestZAIGetStructuredOutputWithDebugUsesJSONMode(t *testing.T) {
	service := NewZAIServiceWithOptions("test-token", nil, nil, "https://api.z.ai/api/paas/v4/chat/completions", nil, nil, nil, nil)
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/paas/v4/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["tools"] != nil {
				t.Fatalf("did not expect tools in verification request")
			}
			if payload["tool_choice"] != nil {
				t.Fatalf("did not expect tool_choice in verification request")
			}
			responseFormat, ok := payload["response_format"].(map[string]interface{})
			if !ok || responseFormat["type"] != "json_object" {
				t.Fatalf("expected response_format.type=json_object, got %#v", payload["response_format"])
			}
			messages, ok := payload["messages"].([]interface{})
			if !ok || len(messages) == 0 {
				t.Fatalf("expected messages array, got %#v", payload["messages"])
			}
			systemMessage, ok := messages[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected system message object, got %#v", messages[0])
			}
			systemContent, _ := systemMessage["content"].(string)
			if !strings.Contains(systemContent, "Return only valid JSON that matches this JSON Schema exactly.") {
				t.Fatalf("expected structured-output instruction in system content: %q", systemContent)
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

	effort := "low"
	maxTokens := 128
	output := ReasoningSearchVerificationResponse{}
	debug, err := service.GetStructuredOutputWithDebug(
		GenerateReasoningSearchVerificationResponseJSONSchema(),
		"glm-5.1",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "Verify the results."},
			{Role: "user", Content: `{"query":"ד בחינות דאור ישר"}`},
		},
		nil,
		&effort,
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

func TestZAIReasoningStructuredOutputRejectsNullRequiredTopLevelFields(t *testing.T) {
	service := NewZAIServiceWithOptions("test-token", nil, nil, "https://api.z.ai", nil, nil, nil, nil)
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := mustJSON(t, map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": `{"query":null,"summary":null,"reasoning_summary":"x","results":null}`,
						},
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
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
	var output struct {
		Query            string        `json:"query"`
		Summary          string        `json:"summary"`
		ReasoningSummary string        `json:"reasoning_summary"`
		Results          []interface{} `json:"results"`
	}

	err := service.GetStructuredOutput(
		GenerateReasoningSearchResponseJSONSchema(),
		"glm-5.1",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "You are a search assistant."},
			{Role: "user", Content: "ד בחינות דאור ישר"},
		},
		nil,
		&effort,
		&output,
	)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `structured output field "query" is null`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestZAIChatRequestIncludesSamplingControls(t *testing.T) {
	doSample := false
	temperature := 0.1
	topP := 0.8
	stop := []string{"</json>"}
	service := NewZAIServiceWithOptions("test-token", nil, nil, "https://api.z.ai", &doSample, &temperature, &topP, stop)
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["do_sample"] != false {
				t.Fatalf("unexpected do_sample: %#v", payload["do_sample"])
			}
			if payload["temperature"] != 0.1 {
				t.Fatalf("unexpected temperature: %#v", payload["temperature"])
			}
			if payload["top_p"] != 0.8 {
				t.Fatalf("unexpected top_p: %#v", payload["top_p"])
			}
			stops, ok := payload["stop"].([]interface{})
			if !ok || len(stops) != 1 || stops[0] != "</json>" {
				t.Fatalf("unexpected stop payload: %#v", payload["stop"])
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

	maxTokens := 128
	effort := "low"
	output := ReasoningSearchVerificationResponse{}
	if _, err := service.GetStructuredOutputWithDebug(
		GenerateReasoningSearchVerificationResponseJSONSchema(),
		"glm-5.1",
		&maxTokens,
		[]LLMBotMessage{
			{Role: "system", Content: "Verify the results."},
			{Role: "user", Content: `{"query":"ד בחינות דאור ישר"}`},
		},
		nil,
		&effort,
		&output,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
