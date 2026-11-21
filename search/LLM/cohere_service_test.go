package llm

import "testing"

func TestNormalizeCohereAPIBaseURL(t *testing.T) {
	tests := map[string]string{
		"":                                       "https://api.cohere.ai/compatibility/v1",
		"https://api.cohere.ai":                  "https://api.cohere.ai/compatibility/v1",
		"https://api.cohere.ai/compatibility/v1": "https://api.cohere.ai/compatibility/v1",
		"https://api.cohere.ai/compatibility/v1/chat/completions": "https://api.cohere.ai/compatibility/v1",
		"https://custom.example.com/v1":                           "https://custom.example.com/v1",
		"https://custom.example.com/v1/chat/completions":          "https://custom.example.com/v1",
	}

	for input, expected := range tests {
		if got := normalizeCohereAPIBaseURL(input); got != expected {
			t.Fatalf("normalizeCohereAPIBaseURL(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestCohereReasoningEffortValue(t *testing.T) {
	high := "high"
	value, err := cohereReasoningEffortValue("command-a-03-2025", &high)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value == nil || *value != "high" {
		t.Fatalf("unexpected effort value: %v", value)
	}

	low := "low"
	if _, err := cohereReasoningEffortValue("command-a-03-2025", &low); err == nil {
		t.Fatalf("expected error for unsupported effort")
	}
}
