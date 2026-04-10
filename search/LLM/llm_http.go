package llm

import (
	"bytes"
	"encoding/json"
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

// callLLMAPI is shared by all providers that send plain JSON requests.
func callLLMAPI(client *http.Client, token string, data interface{}, endpoint string, result interface{}, logRawBody bool) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonBytes))
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
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("json.Unmarshal error: %v\nResponse: %s", err, string(bodyBytes))
	}

	return nil
}
