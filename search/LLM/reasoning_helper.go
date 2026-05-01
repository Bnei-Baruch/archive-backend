package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

const ResponsesAPIEmptyAssistantOutputError = "responses API returned empty assistant output"
const finalReasoningIterationInstruction = "This is the final allowed reasoning iteration. Do not call tools. Return the best structured response using only the evidence already gathered. If evidence is partial, return the best results you have and briefly note uncertainty in the summary."

type ResponsesRequest struct {
	Model              string                   `json:"model"`
	Input              []interface{}            `json:"input,omitempty"`
	Instructions       *string                  `json:"instructions,omitempty"`
	PreviousResponseID *string                  `json:"previous_response_id,omitempty"`
	MaxOutputTokens    *int                     `json:"max_output_tokens,omitempty"`
	PromptCacheKey     *string                  `json:"prompt_cache_key,omitempty"`
	Reasoning          *ResponsesReasoning      `json:"reasoning,omitempty"`
	Text               *ResponsesText           `json:"text,omitempty"`
	Tools              []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice         string                   `json:"tool_choice,omitempty"`
	Provider           *ResponsesProvider       `json:"provider,omitempty"`
}

type ResponsesProvider struct {
	Sort                string   `json:"sort,omitempty"`
	RequireParameters   *bool    `json:"require_parameters,omitempty"`
	AllowFallbacks      *bool    `json:"allow_fallbacks,omitempty"`
	Only                []string `json:"only,omitempty"`
	Ignore              []string `json:"ignore,omitempty"`
	PreferredMaxLatency *int     `json:"preferred_max_latency,omitempty"`
}

type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ResponsesText struct {
	Format *ResponsesTextFormat `json:"format,omitempty"`
}

type ResponsesTextFormat struct {
	Type   string      `json:"type"`
	Name   string      `json:"name,omitempty"`
	Schema interface{} `json:"schema,omitempty"`
	Strict bool        `json:"strict,omitempty"`
}

type ResponsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func (c *ResponsesOutputContent) UnmarshalJSON(data []byte) error {
	type alias ResponsesOutputContent
	var value alias
	if err := json.Unmarshal(data, &value); err == nil {
		*c = ResponsesOutputContent(value)
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		c.Type = ""
		c.Text = text
		return nil
	}

	return fmt.Errorf("failed to unmarshal response output content")
}

type ResponsesOutputItem struct {
	ID        string                   `json:"id,omitempty"`
	Type      string                   `json:"type"`
	Status    string                   `json:"status,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Role      string                   `json:"role,omitempty"`
	Content   []ResponsesOutputContent `json:"content,omitempty"`
	Summary   []ResponsesOutputContent `json:"summary,omitempty"`
}

type ResponsesAPIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ResponsesIncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

type ResponsesResponse struct {
	ID                string                      `json:"id"`
	Status            string                      `json:"status"`
	Output            []ResponsesOutputItem       `json:"output"`
	Error             *ResponsesAPIError          `json:"error,omitempty"`
	IncompleteDetails *ResponsesIncompleteDetails `json:"incomplete_details,omitempty"`
	Usage             *OpenAIUsage                `json:"usage,omitempty"`
}

type OpenAIUsage struct {
	InputTokens         int                        `json:"input_tokens,omitempty"`
	InputTokensDetails  *OpenAIInputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        int                        `json:"output_tokens,omitempty"`
	OutputTokensDetails *OpenAIOutputTokensDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int                        `json:"total_tokens,omitempty"`
}

func (u *OpenAIUsage) UnmarshalJSON(data []byte) error {
	type usageAlias struct {
		InputTokens             int                        `json:"input_tokens,omitempty"`
		InputTokensDetails      *OpenAIInputTokensDetails  `json:"input_tokens_details,omitempty"`
		OutputTokens            int                        `json:"output_tokens,omitempty"`
		OutputTokensDetails     *OpenAIOutputTokensDetails `json:"output_tokens_details,omitempty"`
		TotalTokens             int                        `json:"total_tokens,omitempty"`
		PromptTokens            int                        `json:"prompt_tokens,omitempty"`
		PromptCacheHitTokens    int                        `json:"prompt_cache_hit_tokens,omitempty"`
		PromptTokensDetails     *OpenAIInputTokensDetails  `json:"prompt_tokens_details,omitempty"`
		CompletionTokens        int                        `json:"completion_tokens,omitempty"`
		CompletionTokensDetails *OpenAIOutputTokensDetails `json:"completion_tokens_details,omitempty"`
	}

	var raw usageAlias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	u.InputTokens = raw.InputTokens
	if u.InputTokens == 0 {
		u.InputTokens = raw.PromptTokens
	}
	u.InputTokensDetails = raw.InputTokensDetails
	if u.InputTokensDetails == nil {
		u.InputTokensDetails = raw.PromptTokensDetails
	}
	if u.InputTokensDetails == nil && raw.PromptCacheHitTokens > 0 {
		u.InputTokensDetails = &OpenAIInputTokensDetails{CachedTokens: raw.PromptCacheHitTokens}
	}

	u.OutputTokens = raw.OutputTokens
	if u.OutputTokens == 0 {
		u.OutputTokens = raw.CompletionTokens
	}
	u.OutputTokensDetails = raw.OutputTokensDetails
	if u.OutputTokensDetails == nil {
		u.OutputTokensDetails = raw.CompletionTokensDetails
	}

	u.TotalTokens = raw.TotalTokens
	if u.TotalTokens == 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}

	return nil
}

type OpenAIInputTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type OpenAIOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type reasoningStepsSetter interface {
	SetReasoningSteps([]ReasoningSearchReasoningStep)
}

type reasoningProcessStatsSetter interface {
	SetReasoningProcessStats(int, int)
}

type reasoningDebugInfoSetter interface {
	SetReasoningDebugInfo(*ReasoningSearchDebugInfo)
}

type reasoningUsedToolsSetter interface {
	SetUsedTools([]string)
}

func appendReasoningStepIfDebug(steps []ReasoningSearchReasoningStep, deb bool, number int, thoughts string, toolCalls []ReasoningSearchReasoningToolCall, usage LLMUsageTotals, started time.Time) []ReasoningSearchReasoningStep {
	if !deb {
		return steps
	}
	if toolCalls == nil {
		toolCalls = []ReasoningSearchReasoningToolCall{}
	}
	return append(steps, ReasoningSearchReasoningStep{
		Number:    number,
		Thoughts:  strings.TrimSpace(thoughts),
		ToolCalls: toolCalls,
		Usage: ReasoningSearchReasoningStepUsage{
			TotalTokens:         usage.TotalTokens,
			InputTokens:         usage.InputTokens,
			CachedInputTokens:   usage.CachedInputTokens,
			UncachedInputTokens: usage.UncachedInputTokens(),
			OutputTokens:        usage.OutputTokens,
			ReasoningTokens:     usage.ReasoningTokens,
		},
		LatencyMS: time.Since(started).Milliseconds(),
	})
}

func reasoningUsageTotals(usage *OpenAIUsage) LLMUsageTotals {
	totals := LLMUsageTotals{}
	totals.Add(usage)
	return totals
}

func reasoningToolCallDebug(name string, args json.RawMessage) ReasoningSearchReasoningToolCall {
	return ReasoningSearchReasoningToolCall{
		Name:   name,
		Params: compactToolCallArguments(args),
	}
}

func appendFinalReasoningIterationInstruction(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return finalReasoningIterationInstruction
	}
	return content + "\n\n" + finalReasoningIterationInstruction
}

func buildResponsesText(jsonSchema *string) (*ResponsesText, error) {
	if jsonSchema == nil {
		return nil, nil
	}

	var schema interface{}
	if err := json.Unmarshal([]byte(*jsonSchema), &schema); err != nil {
		return nil, fmt.Errorf("invalid json_schema: %v", err)
	}

	return &ResponsesText{
		Format: &ResponsesTextFormat{
			Type:   "json_schema",
			Name:   "structured_output",
			Schema: schema,
			Strict: true,
		},
	}, nil
}

func appendStructuredOutputInstruction(instructions string, jsonSchema *string) string {
	if jsonSchema == nil {
		return instructions
	}

	grounding := fmt.Sprintf(
		"Return only valid JSON that matches this JSON Schema exactly. Do not add prose, markdown, or code fences.\nJSON Schema:\n%s",
		*jsonSchema,
	)
	if strings.TrimSpace(instructions) == "" {
		return grounding
	}
	return instructions + "\n\n" + grounding
}

func validateJSONRequiredTopLevelFields(content string, jsonSchema string) error {
	var schema struct {
		Type     string   `json:"type"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		return fmt.Errorf("invalid json_schema: %v", err)
	}
	if schema.Type != "object" || len(schema.Required) == 0 {
		return nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return err
	}
	for _, field := range schema.Required {
		value, ok := payload[field]
		if !ok {
			return fmt.Errorf("structured output is missing required field %q", field)
		}
		if strings.TrimSpace(string(value)) == "null" {
			return fmt.Errorf("structured output field %q is null", field)
		}
	}
	return nil
}

func normalizeResponseTools(tools []ToolCall) ([]map[string]interface{}, error) {
	normalized := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		tool := map[string]interface{}{
			"type": t.Type,
		}
		if t.Function != nil {
			functionBytes, err := json.Marshal(t.Function)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool definition: %w", err)
			}
			functionPayload := map[string]interface{}{}
			if err := json.Unmarshal(functionBytes, &functionPayload); err != nil {
				return nil, fmt.Errorf("failed to parse tool definition: %w", err)
			}
			for k, v := range functionPayload {
				tool[k] = v
			}
		}
		normalized = append(normalized, tool)
	}
	return normalized, nil
}

func extractAssistantOutputText(items []ResponsesOutputItem) string {
	content := ""
	for _, item := range items {
		if item.Type != "message" || item.Role != "assistant" {
			continue
		}
		for _, c := range item.Content {
			if c.Type == "output_text" || c.Type == "" {
				content += c.Text
			}
		}
	}
	return content
}

func describeResponsesOutputItems(items []ResponsesOutputItem) string {
	if len(items) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(items))
	for i, item := range items {
		contentTypes := make([]string, 0, len(item.Content))
		hasContentText := false
		for _, content := range item.Content {
			contentTypes = append(contentTypes, content.Type)
			if strings.TrimSpace(content.Text) != "" {
				hasContentText = true
			}
		}

		summaryTypes := make([]string, 0, len(item.Summary))
		hasSummaryText := false
		for _, summary := range item.Summary {
			summaryTypes = append(summaryTypes, summary.Type)
			if strings.TrimSpace(summary.Text) != "" {
				hasSummaryText = true
			}
		}

		parts = append(parts, fmt.Sprintf(
			"#%d{type=%q role=%q status=%q content_items=%d content_types=%v has_content_text=%t summary_items=%d summary_types=%v has_summary_text=%t name=%q call_id=%q args_len=%d}",
			i,
			item.Type,
			item.Role,
			item.Status,
			len(item.Content),
			contentTypes,
			hasContentText,
			len(item.Summary),
			summaryTypes,
			hasSummaryText,
			item.Name,
			item.CallID,
			len(item.Arguments),
		))
	}
	return strings.Join(parts, "; ")
}

func extractReasoningSummaryText(items []ResponsesOutputItem) []string {
	parts := []string{}
	for _, item := range items {
		if item.Type != "reasoning" {
			continue
		}
		for _, summary := range item.Summary {
			if (summary.Type == "summary_text" || summary.Type == "") && strings.TrimSpace(summary.Text) != "" {
				parts = append(parts, summary.Text)
			}
		}
		for _, content := range item.Content {
			if (content.Type == "reasoning_text" || content.Type == "") && strings.TrimSpace(content.Text) != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	return parts
}

func printReasoningOutputIfDeb(deb bool, iteration int, items []ResponsesOutputItem) {
	if !deb {
		return
	}

	parts := extractReasoningSummaryText(items)
	if len(parts) == 0 {
		return
	}

	log.Printf("LLM reasoning summary iteration %d:\n%s", iteration, strings.Join(parts, "\n\n"))
}

func responsesCompletionError(resp *ResponsesResponse) error {
	if resp == nil {
		return nil
	}
	if resp.Error != nil {
		if resp.Error.Code != "" {
			return fmt.Errorf("responses API error (%s): %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("responses API error: %s", resp.Error.Message)
	}
	if resp.IncompleteDetails != nil && strings.TrimSpace(resp.IncompleteDetails.Reason) != "" {
		return fmt.Errorf("responses API returned incomplete output: %s", resp.IncompleteDetails.Reason)
	}
	if resp.Status != "" && resp.Status != "completed" {
		return fmt.Errorf("responses API returned non-completed status: %s", resp.Status)
	}
	return nil
}

func compactToolCallArguments(arguments json.RawMessage) string {
	if len(arguments) == 0 {
		return "{}"
	}

	buffer := bytes.NewBuffer(nil)
	if err := json.Compact(buffer, arguments); err == nil {
		return buffer.String()
	}

	trimmed := strings.TrimSpace(string(arguments))
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}
