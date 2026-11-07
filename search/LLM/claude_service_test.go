package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClaudeReasoningCachesSystemAndToolsAcrossIterations(t *testing.T) {
	sessions := NewChatReasoningSessionStore(time.Minute)
	defer sessions.Close()
	service := NewClaudeServiceWithOptions("test-token", nil, sessions, "https://claude.test")
	requests := []ClaudeMessageRequest{}
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request ClaudeMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, request)
		if len(request.System) != 1 || request.System[0].Type != "text" || request.System[0].CacheControl == nil || request.System[0].CacheControl.Type != "ephemeral" {
			t.Fatalf("expected cacheable system text block, got %#v", request.System)
		}
		if !strings.Contains(request.System[0].Text, "Stable instructions") {
			t.Fatal("system instructions were lost")
		}
		for _, message := range request.Messages {
			for _, block := range message.Content {
				if block.CacheControl != nil {
					t.Fatalf("conversation must not contain cache breakpoints: %#v", block)
				}
			}
		}
		usage := &ClaudeUsage{InputTokens: 10, OutputTokens: 5}
		content := []ClaudeContentBlock{{Type: "text", Text: `{"answer":"done"}`}}
		if len(requests) <= 2 {
			if len(request.Tools) != 2 || request.Tools[0].CacheControl != nil || request.Tools[1].CacheControl == nil || request.Tools[1].CacheControl.Type != "ephemeral" {
				t.Fatalf("expected breakpoint only on last tool: %#v", request.Tools)
			}
			content = []ClaudeContentBlock{{Type: "tool_use", ID: "call-lookup", Name: "lookup", Input: json.RawMessage(`{}`)}}
			if len(requests) == 1 {
				usage.CacheCreationInputTokens = 100
			} else {
				usage.CacheReadInputTokens = 100
				if !reflect.DeepEqual(request.System, requests[0].System) || !reflect.DeepEqual(request.Tools, requests[0].Tools) {
					t.Fatal("tool/system cache prefix changed between iterations")
				}
				if len(request.Messages) != 3 {
					t.Fatalf("expected user, assistant tool call, and tool result; got %d messages", len(request.Messages))
				}
			}
		} else if len(request.Tools) != 0 {
			t.Fatal("final iteration must still disable tools")
		}
		body := mustJSON(t, ClaudeMessageResponse{Content: content, Usage: usage})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	schema := `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}`
	output := struct {
		ReasoningSearchResponse `json:"-"`
		Answer                  string `json:"answer"`
	}{}
	_, err := service.GetReasoningStructuredOutputWithToolsForSession(context.Background(), nil, nil, schema, "claude-test-model", nil,
		[]LLMBotMessage{{Role: "system", Content: "Stable instructions"}, {Role: "user", Content: "Variable query"}},
		[]ToolCall{
			{Type: "function", Function: map[string]interface{}{"name": "lookup"}},
			{Type: "function", Function: map[string]interface{}{"name": "other_lookup"}},
		},
		map[string]ToolHandler{"lookup": func(context.Context, json.RawMessage) (string, error) { return `{"hits":[]}`, nil }},
		nil, nil, nil, nil, true, 3, &output)
	if err != nil {
		t.Fatal(err)
	}
	if output.Answer != "done" || len(requests) != 3 {
		t.Fatalf("unexpected final answer or iteration count: answer=%q requests=%d", output.Answer, len(requests))
	}
	if output.Debug == nil || output.Debug.CacheWriteTokens != 100 || output.Debug.CachedInputTokens != 100 || output.Debug.UncachedInputTokens != 30 || output.Debug.TotalTokens != 245 {
		t.Fatalf("cache usage was not preserved in final debug output: %#v", output.Debug)
	}
}

func TestClaudeStructuredOutputCachesSystemPrefix(t *testing.T) {
	service := NewClaudeServiceWithOptions("test-token", nil, nil, "https://claude.test")
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request ClaudeMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.System) != 1 || request.System[0].CacheControl == nil || request.System[0].CacheControl.Type != "ephemeral" {
			t.Fatalf("expected system cache breakpoint: %#v", request.System)
		}
		if len(request.Tools) != 0 || len(request.Messages) != 1 || request.Messages[0].Content[0].Text != "Variable query" || request.Messages[0].Content[0].CacheControl != nil {
			t.Fatalf("expected uncached user query following system prefix: %#v", request)
		}
		body := mustJSON(t, ClaudeMessageResponse{Content: []ClaudeContentBlock{{Type: "text", Text: `{"answer":"done"}`}}})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	output := struct {
		Answer string `json:"answer"`
	}{}
	err := service.GetStructuredOutput(context.Background(), `{"required":["answer"]}`, "claude-test-model", nil,
		[]LLMBotMessage{{Role: "system", Content: "Stable instructions"}, {Role: "user", Content: "Variable query"}}, nil, nil, &output)
	if err != nil || output.Answer != "done" {
		t.Fatalf("unexpected structured output: answer=%q err=%v", output.Answer, err)
	}
}
