package llm

import (
	"strings"
	"testing"
)

func TestDescribeResponsesOutputItemsIncludesDiagnosticFields(t *testing.T) {
	description := describeResponsesOutputItems([]ResponsesOutputItem{
		{
			Type:   "reasoning",
			Status: "completed",
			Summary: []ResponsesOutputContent{
				{Type: "summary_text", Text: "thinking"},
			},
		},
		{
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []ResponsesOutputContent{
				{Type: "output_text", Text: "done"},
			},
		},
	})

	requiredSnippets := []string{
		`#0{type="reasoning"`,
		`summary_items=1`,
		`has_summary_text=true`,
		`#1{type="message" role="assistant" status="completed"`,
		`content_items=1`,
		`has_content_text=true`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(description, snippet) {
			t.Fatalf("expected %q in %q", snippet, description)
		}
	}
}

func TestUnmarshalLLMJSONContentAcceptsDuplicateTopLevelJSON(t *testing.T) {
	var payload struct {
		Query string `json:"query"`
	}

	content := `{"query":"first"}{"query":"second"}`
	if err := unmarshalLLMJSONContent(content, &payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Query != "first" {
		t.Fatalf("expected first object to be used, got %q", payload.Query)
	}
}

func TestUnmarshalLLMJSONContentRejectsInvalidTrailingText(t *testing.T) {
	var payload struct {
		Query string `json:"query"`
	}

	if err := unmarshalLLMJSONContent(`{"query":"first"} trailing`, &payload); err == nil {
		t.Fatalf("expected invalid trailing text to fail")
	}
}
