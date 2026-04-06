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

func TestOllamaReasoningWithToolsUsesChatAPI(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewOllamaServiceWithOptions("", nil, nil, "https://ollama.kab.sh/api/generate", 32768, "", nil, false)
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/chat" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			if _, exists := payload["tool_choice"]; exists {
				t.Fatalf("did not expect tool_choice in Ollama request")
			}
			if payload["stream"] != false {
				t.Fatalf("expected stream=false, got %#v", payload["stream"])
			}
			if payload["think"] != "high" {
				t.Fatalf("unexpected think value: %#v", payload["think"])
			}
			if payload["format"] == nil {
				t.Fatalf("expected structured output format")
			}
			options, ok := payload["options"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected options object, got %T", payload["options"])
			}
			if options["num_ctx"] != float64(32768) {
				t.Fatalf("unexpected num_ctx: %#v", options["num_ctx"])
			}
			if options["num_predict"] != float64(256) {
				t.Fatalf("unexpected num_predict: %#v", options["num_predict"])
			}
			if _, exists := options["temperature"]; exists {
				t.Fatalf("did not expect temperature option by default: %#v", options["temperature"])
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
			if strings.Contains(systemContent, "Return only valid JSON that matches this JSON Schema exactly.") {
				t.Fatalf("did not expect structured-output instruction by default: %q", systemContent)
			}

			var body []byte
			switch len(requests) {
			case 1:
				body = mustJSON(t, map[string]interface{}{
					"model": "gemma4:31b",
					"message": map[string]interface{}{
						"role":     "assistant",
						"thinking": "searching",
						"tool_calls": []map[string]interface{}{
							{
								"type": "function",
								"function": map[string]interface{}{
									"name":      "lookup",
									"arguments": map[string]interface{}{"term": "זוהר"},
								},
							},
						},
					},
					"done":              true,
					"done_reason":       "stop",
					"prompt_eval_count": 10,
					"eval_count":        5,
				})
			case 2:
				messages, ok := payload["messages"].([]interface{})
				if !ok {
					t.Fatalf("expected messages array, got %T", payload["messages"])
				}
				if len(messages) != 4 {
					t.Fatalf("expected system, user, assistant tool call, and tool message; got %d", len(messages))
				}
				toolMessage, ok := messages[3].(map[string]interface{})
				if !ok || toolMessage["role"] != "tool" || toolMessage["tool_name"] != "lookup" {
					t.Fatalf("unexpected tool message: %#v", messages[3])
				}
				body = mustJSON(t, map[string]interface{}{
					"model": "gemma4:31b",
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": `{"answer":"done"}`,
					},
					"done":              true,
					"done_reason":       "stop",
					"prompt_eval_count": 12,
					"eval_count":        7,
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
		"gemma4:31b",
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

func TestOllamaReasoningWithToolsSupportsStructuredOutputPromptSchemaAndTemperature(t *testing.T) {
	requests := []map[string]interface{}{}
	temperature := 0.0
	service := NewOllamaServiceWithOptions("", nil, nil, "https://ollama.kab.sh", 32768, "", &temperature, true)
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			options, ok := payload["options"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected options object, got %T", payload["options"])
			}
			if options["temperature"] != float64(0) {
				t.Fatalf("unexpected temperature: %#v", options["temperature"])
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

			body := mustJSON(t, map[string]interface{}{
				"model": "gemma4:31b",
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": `{"answer":"done"}`,
				},
				"done":              true,
				"done_reason":       "stop",
				"prompt_eval_count": 10,
				"eval_count":        5,
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
	var output struct {
		Answer string `json:"answer"`
	}
	err := service.GetReasoningStructuredOutputWithTools(
		schema,
		"gemma4:31b",
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
		1,
		&output,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Answer != "done" {
		t.Fatalf("unexpected answer: %s", output.Answer)
	}
}

func TestOllamaReasoningSessionReplaysHistory(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewOllamaServiceWithOptions("", nil, NewChatReasoningSessionStore(time.Minute), "https://ollama.kab.sh", 32768, "", nil, false)
	defer service.sessions.Close()
	service.OpenAIService.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			body := mustJSON(t, map[string]interface{}{
				"model": "gemma4:31b",
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": `{"answer":"ok"}`,
				},
				"done":              true,
				"done_reason":       "stop",
				"prompt_eval_count": 10,
				"eval_count":        5,
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
		schema,
		"gemma4:31b",
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

	var secondOutput struct {
		Answer string `json:"answer"`
	}
	_, err = service.GetReasoningStructuredOutputWithToolsForSession(
		&sessionID,
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
	secondMessages, ok := requests[1]["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array on second request, got %T", requests[1]["messages"])
	}
	if len(secondMessages) != 4 {
		t.Fatalf("expected system, prior user, prior assistant, and new user in replayed history, got %d items", len(secondMessages))
	}
	if secondMessages[1].(map[string]interface{})["role"] != "user" {
		t.Fatalf("expected prior user message, got %#v", secondMessages[1])
	}
	if secondMessages[2].(map[string]interface{})["role"] != "assistant" {
		t.Fatalf("expected prior assistant message, got %#v", secondMessages[2])
	}
	if secondMessages[3].(map[string]interface{})["role"] != "user" {
		t.Fatalf("expected new user message, got %#v", secondMessages[3])
	}
}
