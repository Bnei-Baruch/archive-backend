package llm

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
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
	msg, err := service.GetChatResponse("gpt-5.4", &maxTokens, []LLMBotMessage{
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
