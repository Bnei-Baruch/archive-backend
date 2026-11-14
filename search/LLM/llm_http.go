package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const defaultLLMHTTPRequestTimeout = 2 * time.Minute
const rawResponseLogColor = "\x1b[36m"
const rawResponseLogReset = "\x1b[0m"

type llmHTTPError struct {
	StatusCode int
	Body       string
}

func (e *llmHTTPError) Error() string {
	return fmt.Sprintf("HTTP error %d: %s", e.StatusCode, e.Body)
}

// callLLMAPI is shared by providers that send plain JSON requests.
func callLLMAPI(ctx context.Context, client *http.Client, token string, data interface{}, endpoint string, result interface{}, logRawBody bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")

	if client == nil {
		client = &http.Client{Timeout: defaultLLMHTTPRequestTimeout}
	}
	startedAt := time.Now()
	log.Printf("LLM API request start endpoint=%s", endpoint)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("LLM API request error endpoint=%s elapsed=%s err=%v", endpoint, time.Since(startedAt), err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("LLM API response endpoint=%s status=%d elapsed=%s", endpoint, resp.StatusCode, time.Since(startedAt))

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	if logRawBody {
		log.Printf("%sLLM API raw response endpoint=%s body=%s%s", rawResponseLogColor, endpoint, string(bodyBytes), rawResponseLogReset)
	}
	if resp.StatusCode >= 300 {
		return &llmHTTPError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}
	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("json.Unmarshal error: %v\nResponse: %s", err, string(bodyBytes))
	}

	return nil
}

func callLLMAPIWithTransientRetry(ctx context.Context, client *http.Client, token string, data interface{}, endpoint string, result interface{}, logRawBody bool) error {
	delays := []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond}
	return callLLMAPIWithTransientRetryDelays(ctx, client, token, data, endpoint, result, logRawBody, delays)
}

func callLLMAPIWithTransientRetryDelays(ctx context.Context, client *http.Client, token string, data interface{}, endpoint string, result interface{}, logRawBody bool, delays []time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	for attempt := 0; attempt <= len(delays); attempt++ {
		err = callLLMAPI(ctx, client, token, data, endpoint, result, logRawBody)
		if err == nil || !isTransientLLMHTTPError(err) || attempt == len(delays) {
			return err
		}
		delay := delays[attempt]
		log.Printf("LLM API transient error endpoint=%s attempt=%d/%d retry_in=%s err=%v", endpoint, attempt+1, len(delays)+1, delay, err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

func isTransientLLMHTTPError(err error) bool {
	var httpErr *llmHTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	switch httpErr.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
