package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestXAIReasoningWithToolsOmitsInstructionsOnFollowup(t *testing.T) {
	requests := []map[string]interface{}{}
	service := NewXAIServiceWithOptions("test-token", nil, NewOpenAIReasoningSessionStore(time.Minute), "https://xai.test")
	defer service.sessions.Close()

	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/v1/responses" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requests = append(requests, payload)

			switch len(requests) {
			case 1:
				if payload["instructions"] != "You are a search assistant." {
					t.Fatalf("expected instructions on first request, got %#v", payload["instructions"])
				}
				if payload["previous_response_id"] != nil {
					t.Fatalf("did not expect previous_response_id on first request, got %#v", payload["previous_response_id"])
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewReader(mustJSON(t, map[string]interface{}{
						"id":     "resp_1",
						"status": "completed",
						"output": []map[string]interface{}{
							{
								"type": "message",
								"role": "assistant",
								"content": []map[string]interface{}{
									{
										"type": "output_text",
										"text": `{"answer":"first"}`,
									},
								},
							},
						},
						"usage": map[string]interface{}{
							"input_tokens":  10,
							"output_tokens": 5,
							"total_tokens":  15,
						},
					}))),
				}, nil
			case 2:
				if payload["instructions"] != nil {
					t.Fatalf("did not expect instructions on follow-up request, got %#v", payload["instructions"])
				}
				if payload["previous_response_id"] != "resp_1" {
					t.Fatalf("expected previous_response_id on follow-up request, got %#v", payload["previous_response_id"])
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewReader(mustJSON(t, map[string]interface{}{
						"id":     "resp_2",
						"status": "completed",
						"output": []map[string]interface{}{
							{
								"type": "message",
								"role": "assistant",
								"content": []map[string]interface{}{
									{
										"type": "output_text",
										"text": `{"answer":"second"}`,
									},
								},
							},
						},
						"usage": map[string]interface{}{
							"input_tokens":  12,
							"output_tokens": 6,
							"total_tokens":  18,
						},
					}))),
				}, nil
			default:
				t.Fatalf("unexpected extra request")
				return nil, nil
			}
		}),
	}

	schema := `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`
	maxTokens := 256
	tools := []ToolCall{{
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
	}}
	toolHandlers := map[string]ToolHandler{
		"lookup": func(ctx context.Context, arguments json.RawMessage) (string, error) {
			return `{"hits":[1]}`, nil
		},
	}
	messages := []LLMBotMessage{
		{Role: "system", Content: "You are a search assistant."},
		{Role: "user", Content: "מצא לי משה"},
	}

	var firstOutput struct {
		Answer string `json:"answer"`
	}
	sessionID, err := service.GetReasoningStructuredOutputWithToolsForSession(nil, nil, schema, "grok-4-1-fast-reasoning", &maxTokens, messages, tools, toolHandlers, nil, nil, nil, nil, false, 4, &firstOutput)
	if err != nil {
		t.Fatalf("first request returned error: %v", err)
	}
	if firstOutput.Answer != "first" {
		t.Fatalf("unexpected first answer: %s", firstOutput.Answer)
	}
	if sessionID == "" {
		t.Fatalf("expected session id from first request")
	}

	var secondOutput struct {
		Answer string `json:"answer"`
	}
	if _, err := service.GetReasoningStructuredOutputWithToolsForSession(&sessionID, nil, schema, "grok-4-1-fast-reasoning", &maxTokens, messages, tools, toolHandlers, nil, nil, nil, nil, false, 4, &secondOutput); err != nil {
		t.Fatalf("follow-up request returned error: %v", err)
	}
	if secondOutput.Answer != "second" {
		t.Fatalf("unexpected follow-up answer: %s", secondOutput.Answer)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
}
