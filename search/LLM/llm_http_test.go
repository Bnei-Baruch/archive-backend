package llm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
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

func TestCallLLMAPIDoesNotRetryHTTPError(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: llmHTTPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewBufferString(`{"error":"temporary"}`))}, nil
	})}

	var response map[string]interface{}
	err := callLLMAPI(context.Background(), client, "", map[string]string{"q": "test"}, "http://example.test", &response, false)
	if err == nil || err.Error() != `HTTP error 503: {"error":"temporary"}` {
		t.Fatalf("expected HTTP error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

type llmHTTPRoundTripFunc func(*http.Request) (*http.Response, error)

func (f llmHTTPRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
