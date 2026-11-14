package llm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestCallLLMAPIHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var response map[string]interface{}
	err := callLLMAPI(ctx, nil, "", map[string]string{"q": "test"}, "http://127.0.0.1:1", &response, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCallLLMAPIWithTransientRetryRetries503(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: llmHTTPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewBufferString(`{"error":"temporary"}`))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"ok":true}`))}, nil
	})}

	var response map[string]interface{}
	err := callLLMAPIWithTransientRetryDelays(context.Background(), client, "", map[string]string{"q": "test"}, "http://example.test", &response, false, []time.Duration{time.Millisecond})
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if response["ok"] != true {
		t.Fatalf("unexpected response: %#v", response)
	}
}

type llmHTTPRoundTripFunc func(*http.Request) (*http.Response, error)

func (f llmHTTPRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
