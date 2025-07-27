package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

const (
	chatEndpoint       = "https://api.openai.com/v1/chat/completions"
	embeddingsEndpoint = "https://api.openai.com/v1/embeddings"
	embeddingModel     = "text-embedding-3-large"
)

type OpenAIService struct {
	token string
}

func NewOpenAIService(token string) *OpenAIService {
	service := &OpenAIService{
		token: token,
	}

	return service
}

// Structs for OpenAI API interaction

type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []LLMBotMessage `json:"messages"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	User             *string         `json:"user,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort  *string         `json:"reasoning_effort,omitempty"`
}

type ToolCall struct {
	Type     string      `json:"type"`
	Function interface{} `json:"function"`
}

type ResponseFormat struct {
	Type       string      `json:"type"`
	JsonSchema interface{} `json:"json_schema"`
}

type ChatResponse struct {
	Choices []struct {
		Index   int           `json:"index"`
		Message LLMBotMessage `json:"message"`
	} `json:"choices"`
}

type EmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type LLMBotMessage struct {
	Role    string  `json:"role"`
	Content string  `json:"content"`
	Refusal *string `json:"refusal,omitempty"`
}

func (s *OpenAIService) GetStructuredOutput(jsonSchema string, model string, maxTokens *int, messages []LLMBotMessage, user *string, reasoningEffort *string, output interface{}) error {
	msg, err := s.GetChatResponse(model, maxTokens, messages, user, nil, &jsonSchema, reasoningEffort)
	if err != nil {
		return err
	}
	if msg.Refusal != nil {
		log.Printf("Received a 'Refusal': %s", *msg.Refusal)
		return nil
	}

	err = json.Unmarshal([]byte(msg.Content), output)
	if err != nil {
		log.Printf("Deserialization failed for schema '%s': %v\nContent: %s", jsonSchema, err, msg.Content)
		return err
	}
	return nil
}

func (s *OpenAIService) GetChatResponse(model string, maxTokens *int, messages []LLMBotMessage, user *string, frequencyPenalty *float64, jsonSchema *string, reasoningEffort *string) (*LLMBotMessage, error) {
	sysMsgCount := 0
	for _, m := range messages {
		if m.Role == "system" || m.Role == "developer" {
			sysMsgCount++
		}
	}
	if sysMsgCount != 1 {
		return nil, fmt.Errorf("must include exactly one system message, found %d", sysMsgCount)
	}

	var respFmt *ResponseFormat
	if jsonSchema != nil {
		respFmt = &ResponseFormat{
			Type:       "json_schema",
			JsonSchema: jsonSchema,
		}
	}

	req := ChatRequest{
		Model:            model,
		Messages:         messages,
		MaxTokens:        maxTokens,
		User:             user,
		FrequencyPenalty: frequencyPenalty,
		ResponseFormat:   respFmt,
		ReasoningEffort:  reasoningEffort,
	}

	var chatResp ChatResponse
	if err := s.callAPI(req, chatEndpoint, &chatResp); err != nil {
		return nil, err
	}

	for _, choice := range chatResp.Choices {
		if choice.Index == 0 {
			return &choice.Message, nil
		}
	}
	return nil, errors.New("no valid chat choices returned")
}

func (s *OpenAIService) GetEmbeddings(content string) ([]float64, error) {
	payload := map[string]interface{}{
		"input": content,
		"model": embeddingModel,
	}

	var resp EmbeddingResponse
	if err := s.callAPI(payload, embeddingsEndpoint, &resp); err != nil {
		return nil, err
	}

	for _, d := range resp.Data {
		if d.Index == 0 {
			if len(d.Embedding) == 0 {
				return nil, errors.New("embedding list is empty")
			}
			return d.Embedding, nil
		}
	}
	return nil, errors.New("embedding response missing")
}

func (s *OpenAIService) callAPI(data interface{}, endpoint string, result interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("json.Unmarshal error: %v\nResponse: %s", err, string(bodyBytes))
	}
	return nil
}
