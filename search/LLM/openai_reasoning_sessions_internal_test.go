package llm

import (
	"testing"
	"time"
)

func TestOpenAIReasoningSessionCleanupInterval(t *testing.T) {
	testCases := []struct {
		ttl      time.Duration
		expected time.Duration
	}{
		{30 * time.Second, 30 * time.Second},
		{5 * time.Minute, time.Minute},
		{30 * time.Minute, 5 * time.Minute},
		{6 * time.Hour, time.Hour},
		{5 * 24 * time.Hour, 24 * time.Hour},
	}

	for _, tc := range testCases {
		actual := openAIReasoningSessionCleanupInterval(tc.ttl)
		if actual != tc.expected {
			t.Fatalf("cleanup interval for ttl %s: expected %s, got %s", tc.ttl, tc.expected, actual)
		}
	}
}
