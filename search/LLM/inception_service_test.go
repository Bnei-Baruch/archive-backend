package llm

import "testing"

func TestNormalizeInceptionAPIBaseURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "https://api.inceptionlabs.ai/v1"},
		{"https://api.inceptionlabs.ai", "https://api.inceptionlabs.ai/v1"},
		{"https://api.inceptionlabs.ai/", "https://api.inceptionlabs.ai/v1"},
		{"https://api.inceptionlabs.ai/v1", "https://api.inceptionlabs.ai/v1"},
		{"https://api.inceptionlabs.ai/v1/chat/completions", "https://api.inceptionlabs.ai/v1"},
		{"https://proxy.example.com/inception", "https://proxy.example.com/inception/v1"},
		{"https://proxy.example.com/inception/v1/chat/completions", "https://proxy.example.com/inception/v1"},
	}

	for _, tc := range tests {
		actual := normalizeInceptionAPIBaseURL(tc.input)
		if actual != tc.expected {
			t.Fatalf("normalizeInceptionAPIBaseURL(%q)=%q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestInceptionReasoningEffortValueSupportsInstant(t *testing.T) {
	effort := "instant"
	actual, err := inceptionReasoningEffortValue("mercury-2", &effort)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual == nil || *actual != "instant" {
		t.Fatalf("unexpected effort: %#v", actual)
	}
}
