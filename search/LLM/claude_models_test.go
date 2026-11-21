package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestClaudeCurrentModelsReplayThinkingAndEffort(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-2", "claude-mythos-5-2"} {
		t.Run(model, func(t *testing.T) {
			sessions := NewChatReasoningSessionStore(time.Minute)
			defer sessions.Close()
			service := NewClaudeServiceWithOptions("test-token", nil, sessions, "https://claude.test")
			emptyThinking := ""
			toolContent := []ClaudeContentBlock{
				{Type: "thinking", Thinking: &emptyThinking, Signature: "opaque-signature"},
				{Type: "redacted_thinking", Data: "opaque-redacted-data"},
				{Type: "tool_use", ID: "call-lookup", Name: "lookup", Input: json.RawMessage(`{}`)},
			}
			finalContent := []ClaudeContentBlock{{Type: "thinking", Thinking: &emptyThinking, Signature: "final-signature"}, {Type: "text", Text: `{"answer":"done"}`}}
			calls := 0
			service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				var request ClaudeMessageRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
				require.Equal(t, model, request.Model)
				require.Equal(t, &ClaudeCacheControl{Type: "ephemeral"}, request.CacheControl, "automatic caching must cover tool iterations and session follow-ups")
				require.Equal(t, &ClaudeOutputConfig{Effort: "high"}, request.OutputConfig)
				require.NotNil(t, request.MaxTokens)
				require.Positive(t, *request.MaxTokens)
				if calls%2 == 0 {
					assistant := request.Messages[len(request.Messages)-2]
					require.Equal(t, "assistant", assistant.Role)
					require.Equal(t, toolContent, assistant.Content)
					require.Equal(t, "tool_result", request.Messages[len(request.Messages)-1].Content[0].Type)
				}
				if calls == 3 {
					found := false
					for _, message := range request.Messages {
						if message.Role == "assistant" && len(message.Content) > 0 && message.Content[0].Signature == "final-signature" {
							require.Equal(t, finalContent, message.Content)
							found = true
						}
					}
					require.True(t, found, "follow-ups must retain native assistant content")
				}
				content := toolContent
				if calls%2 == 0 {
					content = finalContent
				}
				body, err := json.Marshal(ClaudeMessageResponse{Content: content, Usage: &ClaudeUsage{InputTokens: 100, OutputTokens: 20}})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
			})}
			tools := []ToolCall{{Type: "function", Function: map[string]interface{}{"name": "lookup"}}}
			handlers := map[string]ToolHandler{"lookup": func(context.Context, json.RawMessage) (string, error) { return `{}`, nil }}
			effort := "high"
			messages := []LLMBotMessage{{Role: "system", Content: "Search the archive"}, {Role: "user", Content: "query"}}
			var output struct {
				Answer string `json:"answer"`
			}
			id, err := service.GetReasoningStructuredOutputWithToolsForSession(context.Background(), nil, nil,
				`{"type":"object","required":["answer"]}`, model, nil, messages, tools, handlers, nil, nil, nil, &effort, false, 3, &output)
			require.NoError(t, err)
			require.Equal(t, "done", output.Answer)
			// Continuations must use the stored model and effort rather than these overrides.
			effort = "low"
			_, err = service.GetReasoningStructuredOutputWithToolsForSession(context.Background(), &id, nil,
				`{"type":"object","required":["answer"]}`, "ignored-model", nil, messages, tools, handlers, nil, nil, nil, &effort, false, 3, &output)
			require.NoError(t, err)
			require.Equal(t, 4, calls)
		})
	}
}

func TestClaudeEffortValidation(t *testing.T) {
	for _, tc := range []struct {
		model, effort string
		valid         bool
	}{
		{"claude-opus-5-5", "", true}, {"claude-opus-5-5", "low", true},
		{"claude-sonnet-5-5", "medium", true}, {"claude-haiku-5-5", "high", true},
		{"claude-fable-5-2", "xhigh", true}, {"claude-mythos-5-2", "max", true},
		{"claude-opus-4-8", "xhigh", true}, {"claude-sonnet-4-6", "high", true},
		{"claude-haiku-4-5", "", true}, {"claude-haiku-4-5", "high", false},
		{"claude-sonnet-4-6", "xhigh", false}, {"claude-opus-5-5", "minimal", false},
	} {
		t.Run(tc.model+"/"+tc.effort, func(t *testing.T) {
			config, err := claudeOutputConfig(tc.model, &tc.effort)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.effort == "" {
				require.Nil(t, config)
			} else {
				require.Equal(t, tc.effort, config.Effort)
			}
		})
	}
}

func TestClaudeCurrentModelsStageConfiguration(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-2", "claude-mythos-5-2"} {
		t.Run(model, func(t *testing.T) {
			settings := map[string]interface{}{
				"llm.provider":                               ProviderClaude,
				"llm.ai-tools-provider":                      ProviderClaude,
				"llm.reasoning-search-planning-provider":     ProviderClaude,
				"llm.reasoning-search-verification-provider": ProviderClaude,
				"llm.reasoning-search-draft-provider":        ProviderClaude,
				"llm.reasoning-search-planning-enabled":      true,
				"llm.reasoning-search-verification-enabled":  true,
				"llm.reasoning-search-draft-enabled":         true,
			}
			for _, stage := range []string{"reasoning-search", "reasoning-search-planning", "reasoning-search-verification", "ai-tools", "reasoning-search-draft", "reasoning-search-rapid-gather", "reasoning-search-rapid-classifier", "reasoning-search-rapid-finalizer"} {
				settings["claude."+stage+"-model"] = model
				settings["claude."+stage+"-effort"] = "xhigh"
			}
			for key, value := range settings {
				restoreViperKeys(t, key)
				viper.Set(key, value)
			}
			_, err := ReasoningSearchConfigFromConfig()
			require.NoError(t, err)
			_, err = AIToolsConfigFromConfig()
			require.NoError(t, err)
			_, err = ReasoningSearchDraftConfigFromConfig()
			require.NoError(t, err)
			for _, stage := range []string{"gather", "classifier", "finalizer"} {
				config, err := reasoningSearchRapidStageConfigFromProvider(ProviderClaude, stage, 8000, 0)
				require.NoError(t, err)
				require.Equal(t, model, config.Model)
				require.Equal(t, "xhigh", config.Effort)
			}
		})
	}
}

func TestClaudeTruncatedToolCallIsNotExecuted(t *testing.T) {
	service := NewClaudeServiceWithOptions("test-token", nil, nil, "https://claude.test")
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"stop_reason":"max_tokens","content":[{"type":"tool_use","id":"call-lookup","name":"lookup","input":{}}]}`))}, nil
	})}
	tools := []ToolCall{{Type: "function", Function: map[string]interface{}{"name": "lookup"}}}
	handlers := map[string]ToolHandler{"lookup": func(context.Context, json.RawMessage) (string, error) {
		t.Fatal("must not execute a truncated tool call")
		return "", nil
	}}
	_, err := service.GetReasoningResponseWithTools(context.Background(), "claude-opus-5-5", nil,
		[]LLMBotMessage{{Role: "system", Content: "Search"}, {Role: "user", Content: "query"}}, tools, handlers, nil, nil, false, 3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "including thinking")
}

func TestClaudePlainStructuredCallUsesEffortAndReportsTruncation(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		service := NewClaudeServiceWithOptions("test-token", nil, nil, "https://claude.test")
		service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var request ClaudeMessageRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, &ClaudeOutputConfig{Effort: "low"}, request.OutputConfig)
			response := ClaudeMessageResponse{Content: []ClaudeContentBlock{{Type: "text", Text: `{"answer":"done"}`}}}
			if truncated {
				response.StopReason = "max_tokens"
			}
			body, err := json.Marshal(response)
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		})}
		effort := "low"
		var output map[string]string
		_, err := service.GetStructuredOutputWithDebugInfo(context.Background(), `{"type":"object","required":["answer"]}`, "claude-sonnet-5-5", nil,
			[]LLMBotMessage{{Role: "system", Content: "Return JSON"}, {Role: "user", Content: "query"}}, nil, &effort, false, &output)
		if truncated {
			require.Error(t, err)
			require.Contains(t, err.Error(), "including thinking")
		} else {
			require.NoError(t, err)
			require.Equal(t, "done", output["answer"])
		}
	}
}

func TestClaudeSamplePricingAndPerRequestContextTier(t *testing.T) {
	config := viper.New()
	config.SetConfigFile("../../config.sample.toml")
	require.NoError(t, config.ReadInConfig())
	var pricing []ModelPricing
	require.NoError(t, config.UnmarshalKey("claude.pricing", &pricing))
	service := NewClaudeServiceWithOptions("test-token", pricing, nil, "")
	for _, tc := range []struct {
		model                 string
		input, cached, output float64
	}{
		{"claude-opus-5-5", 4, .2, 20}, {"claude-sonnet-5-5", 2, .1, 10},
		{"claude-haiku-5-5", .1, .01, .5}, {"claude-fable-5-2", 8, .2, 40}, {"claude-mythos-5-2", 8, .2, 40},
	} {
		cost := service.estimateCost(tc.model, "high", claudeUsageTotals(&ClaudeUsage{InputTokens: 1000, CacheReadInputTokens: 1000, CacheCreationInputTokens: 1000, OutputTokens: 1000}))
		require.True(t, cost.PricingConfigured, tc.model)
		require.Equal(t, tc.input, cost.InputPer1MTokensUSD)
		require.Equal(t, tc.cached, cost.CachedInputPer1MTokensUSD)
		require.Equal(t, tc.input*1.25, cost.CacheWritePer1MTokensUSD)
		require.Equal(t, tc.output, cost.OutputPer1MTokensUSD)
	}
	for _, tc := range []struct {
		name                    string
		usages                  []ClaudeUsage
		wantCost, wantInputRate float64
	}{
		{"threshold", []ClaudeUsage{{InputTokens: 90000, CacheReadInputTokens: 5000, CacheCreationInputTokens: 5000, OutputTokens: 1000}}, .010175, .1},
		{"above-threshold", []ClaudeUsage{{InputTokens: 90001, CacheReadInputTokens: 5000, CacheCreationInputTokens: 5000, OutputTokens: 1000}}, .0508755, .5},
		{"cached-prompt-above-threshold", []ClaudeUsage{{InputTokens: 1, CacheReadInputTokens: 100000, OutputTokens: 1000}}, .0075005, .5},
		{"two-short-requests", []ClaudeUsage{{InputTokens: 60000, OutputTokens: 1000}, {InputTokens: 60000, OutputTokens: 1000}}, .013, .1},
		{"later-cache-write", []ClaudeUsage{{InputTokens: 60000, OutputTokens: 1000}, {CacheCreationInputTokens: 60000, OutputTokens: 1000}}, .0145, .1},
		{"mixed-tiers", []ClaudeUsage{{InputTokens: 60000, OutputTokens: 1000}, {InputTokens: 100001, OutputTokens: 1000}}, .0590005, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var usage LLMUsageTotals
			for i := range tc.usages {
				usage.AddTotals(claudeUsageTotals(&tc.usages[i]))
			}
			cost := service.estimateCost("claude-haiku-5-5", "", usage)
			require.InDelta(t, tc.wantCost, cost.EstimatedCostUSD, 1e-10)
			require.Equal(t, tc.wantInputRate, cost.InputPer1MTokensUSD)
			if tc.name == "later-cache-write" {
				require.Equal(t, .125, cost.CacheWritePer1MTokensUSD)
			}
		})
	}
}
