package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ReasoningSearchResponse struct {
	SessionID                string                               `json:"session_id"`
	CacheHit                 bool                                 `json:"cache_hit"`
	Query                    string                               `json:"query"`
	QueryMismatchRetry       bool                                 `json:"query_mismatch_retry,omitempty"`
	Summary                  string                               `json:"summary"`
	ReasoningSummary         []ReasoningSearchReasoningStep       `json:"reasoning_summary"`
	PlanningOutput           *ReasoningSearchPlanningResponse     `json:"planning_output,omitempty"`
	PlanningReasoningSummary string                               `json:"planning_reasoning_summary,omitempty"`
	VerificationOutput       *ReasoningSearchVerificationResponse `json:"verification_output,omitempty"`
	MaxFollowups             int                                  `json:"max_followups"`
	FollowupsUsed            int                                  `json:"followups_used"`
	FollowupsRemaining       int                                  `json:"followups_remaining"`
	UsedTokens               int                                  `json:"used_tokens"`
	ReasoningIterations      int                                  `json:"reasoning_iterations"`
	UsedTools                []string                             `json:"used_tools"`
	Results                  []ReasoningSearchResult              `json:"results"`
	Debug                    *ReasoningSearchDebugInfo            `json:"debug,omitempty"`
}

type ReasoningSearchReasoningStep struct {
	Number    int                                `json:"number"`
	Thoughts  string                             `json:"thoughts"`
	ToolCalls []ReasoningSearchReasoningToolCall `json:"tool_calls"`
	Usage     ReasoningSearchReasoningStepUsage  `json:"usage"`
	LatencyMS int64                              `json:"latency_ms"`
}

type ReasoningSearchReasoningStepUsage struct {
	TotalTokens         int `json:"total_tokens"`
	InputTokens         int `json:"input_tokens"`
	CachedInputTokens   int `json:"cached_input_tokens"`
	UncachedInputTokens int `json:"uncached_input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	ReasoningTokens     int `json:"reasoning_tokens"`
}

type ReasoningSearchReasoningToolCall struct {
	Name   string `json:"name"`
	Params string `json:"params"`
}

type ReasoningSearchResult struct {
	MDBUID           string   `json:"mdb_uid"`
	Origin           string   `json:"origin,omitempty"`
	ResultType       string   `json:"result_type"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	ContentType      string   `json:"content_type"`
	ProgramName      string   `json:"program_name,omitempty"`
	Date             string   `json:"date"`
	Reason           string   `json:"reason"`
	Highlights       []string `json:"highlights"`
	IsGroupingResult bool     `json:"is_grouping_result"`
	// DraftEvidence is internal draft input only and is cleared before responses are stored.
	DraftEvidence []ReasoningSearchResultEvidence `json:"draft_evidence,omitempty"`
}

type ReasoningSearchResultEvidence struct {
	ToolName          string `json:"tool_name"`
	DocumentType      string `json:"document_type"`
	DocumentID        string `json:"document_id"`
	Query             string `json:"query"`
	ChunkNumber       int    `json:"chunk_number,omitempty"`
	EndChunkNumber    int    `json:"end_chunk_number,omitempty"`
	Content           string `json:"content"`
	Reason            string `json:"reason,omitempty"`
	SupportingSnippet string `json:"supporting_snippet,omitempty"`
}

const (
	ReasoningSearchResultOriginOriginal = "original"
	ReasoningSearchResultOriginRerun    = "rerun"
)

type ReasoningSearchDebugInfo struct {
	Enabled                      bool                             `json:"enabled"`
	Model                        string                           `json:"model"`
	ReasoningEffort              string                           `json:"reasoning_effort"`
	ReasoningSummary             string                           `json:"reasoning_summary,omitempty"`
	PlanningModelUsage           *ReasoningSearchUsageBreakdown   `json:"planning_model_usage,omitempty"`
	MainModelUsage               *ReasoningSearchUsageBreakdown   `json:"main_model_usage,omitempty"`
	AIToolsUsage                 *ReasoningSearchUsageBreakdown   `json:"ai_tools_usage,omitempty"`
	DraftModelUsage              *ReasoningSearchUsageBreakdown   `json:"draft_model_usage,omitempty"`
	DraftModelRuns               []ReasoningSearchUsageBreakdown  `json:"draft_model_runs,omitempty"`
	AIToolsCalls                 []ReasoningSearchAIToolCallDebug `json:"ai_tools_calls,omitempty"`
	VerificationModel            string                           `json:"verification_model,omitempty"`
	VerificationReasoningEffort  string                           `json:"verification_reasoning_effort,omitempty"`
	VerificationTotalTokens      int                              `json:"verification_total_tokens,omitempty"`
	VerificationEstimatedCostUSD float64                          `json:"verification_estimated_cost_usd,omitempty"`
	TotalTokens                  int                              `json:"total_tokens"`
	InputTokens                  int                              `json:"input_tokens"`
	CachedInputTokens            int                              `json:"cached_input_tokens"`
	UncachedInputTokens          int                              `json:"uncached_input_tokens"`
	OutputTokens                 int                              `json:"output_tokens"`
	ReasoningTokens              int                              `json:"reasoning_tokens"`
	PricingConfigured            bool                             `json:"pricing_configured"`
	InputPer1MTokensUSD          float64                          `json:"input_per_1m_tokens_usd"`
	CachedInputPer1MTokensUSD    float64                          `json:"cached_input_per_1m_tokens_usd"`
	OutputPer1MTokensUSD         float64                          `json:"output_per_1m_tokens_usd"`
	EstimatedInputCostUSD        float64                          `json:"estimated_input_cost_usd"`
	EstimatedCachedInputCostUSD  float64                          `json:"estimated_cached_input_cost_usd"`
	EstimatedOutputCostUSD       float64                          `json:"estimated_output_cost_usd"`
	EstimatedCostUSD             float64                          `json:"estimated_cost_usd"`
}

type ReasoningSearchAIToolCallDebug struct {
	ToolName           string  `json:"tool_name"`
	DocumentType       string  `json:"document_type"`
	DocumentID         string  `json:"document_id"`
	Query              string  `json:"query"`
	DocumentBatchCount int     `json:"document_batch_count"`
	SelectedBatchCount int     `json:"selected_batch_count"`
	LoadLatencyMS      int64   `json:"load_latency_ms"`
	BatchLatenciesMS   []int64 `json:"batch_latencies_ms"`
	MaxBatchLatencyMS  int64   `json:"max_batch_latency_ms"`
	ReaderLatencyMS    int64   `json:"reader_latency_ms"`
	ReturnedCount      int     `json:"returned_count"`
	Error              string  `json:"error,omitempty"`
}

type ReasoningSearchUsageBreakdown struct {
	Model                       string  `json:"model"`
	ReasoningEffort             string  `json:"reasoning_effort"`
	TotalTokens                 int     `json:"total_tokens"`
	InputTokens                 int     `json:"input_tokens"`
	CachedInputTokens           int     `json:"cached_input_tokens"`
	UncachedInputTokens         int     `json:"uncached_input_tokens"`
	OutputTokens                int     `json:"output_tokens"`
	ReasoningTokens             int     `json:"reasoning_tokens"`
	PricingConfigured           bool    `json:"pricing_configured"`
	InputPer1MTokensUSD         float64 `json:"input_per_1m_tokens_usd"`
	CachedInputPer1MTokensUSD   float64 `json:"cached_input_per_1m_tokens_usd"`
	OutputPer1MTokensUSD        float64 `json:"output_per_1m_tokens_usd"`
	EstimatedInputCostUSD       float64 `json:"estimated_input_cost_usd"`
	EstimatedCachedInputCostUSD float64 `json:"estimated_cached_input_cost_usd"`
	EstimatedOutputCostUSD      float64 `json:"estimated_output_cost_usd"`
	EstimatedCostUSD            float64 `json:"estimated_cost_usd"`
}

type ReasoningSearchVerificationResponse struct {
	NeedsAnotherIteration bool   `json:"needs_another_iteration"`
	Recommendation        string `json:"recommendation"`
}

type ReasoningSearchPlanningResponse struct {
	InstructionText     string                            `json:"instruction_text"`
	FirstIterationTools []ReasoningSearchPlanningToolSpec `json:"first_iteration_tools"`
}

type ReasoningSearchPlanningToolSpec struct {
	ToolName           string   `json:"tool_name"`
	ParamsJSON         string   `json:"params_json"`
	AlternativeQueries []string `json:"alternative_queries"`
}

func GenerateReasoningSearchResponseJSONSchemaForLanguage(languageName string) (string, error) {
	languageName = strings.TrimSpace(languageName)
	summaryDescription := "A short explanation of the best results found for the user. Include clarification requests, or follow-up guidance here when needed. Refer to the user's requests and not to internal verification feedback messages if there are any, because they are hidden from the user."
	reasonDescription := "Short description of the result and explanation of why this result was selected for the user."
	if languageName != "" {
		summaryDescription += " Write this field in " + languageName + "."
		reasonDescription += " Write this field in " + languageName + "."
	}

	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The original user query.",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": summaryDescription,
			},
			"reasoning_summary": map[string]interface{}{
				"type":        "array",
				"description": "Return an empty array. The backend fills this field with per-step debug data when debug mode is enabled.",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"number": map[string]interface{}{
							"type": "integer",
						},
						"thoughts": map[string]interface{}{
							"type": "string",
						},
						"tool_calls": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type":                 "object",
								"additionalProperties": false,
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"type": "string",
									},
									"params": map[string]interface{}{
										"type": "string",
									},
								},
								"required": []string{"name", "params"},
							},
						},
						"usage": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]interface{}{
								"total_tokens": map[string]interface{}{
									"type": "integer",
								},
								"input_tokens": map[string]interface{}{
									"type": "integer",
								},
								"cached_input_tokens": map[string]interface{}{
									"type": "integer",
								},
								"uncached_input_tokens": map[string]interface{}{
									"type": "integer",
								},
								"output_tokens": map[string]interface{}{
									"type": "integer",
								},
								"reasoning_tokens": map[string]interface{}{
									"type": "integer",
								},
							},
							"required": []string{"total_tokens", "input_tokens", "cached_input_tokens", "uncached_input_tokens", "output_tokens", "reasoning_tokens"},
						},
						"latency_ms": map[string]interface{}{
							"type": "integer",
						},
					},
					"required": []string{"number", "thoughts", "tool_calls", "usage", "latency_ms"},
				},
			},
			"results": map[string]interface{}{
				"type":        "array",
				"description": "Best matching results from the archive. Prefer direct content results when possible.",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"mdb_uid": map[string]interface{}{
							"type":        "string",
							"description": "The result MDB UID.",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": reasonDescription,
						},
						"highlights": map[string]interface{}{
							"type":        "array",
							"description": "Relevant highlight fragments from search results. Return an empty array if unavailable.",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"is_grouping_result": map[string]interface{}{
							"type":        "boolean",
							"description": "True for grouping or narrowing results such as collections or tags.",
						},
					},
					"required": []string{"mdb_uid", "reason", "highlights", "is_grouping_result"},
				},
			},
		},
		"required": []string{"query", "summary", "reasoning_summary", "results"},
	}

	payload, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal reasoning search response schema: %w", err)
	}
	return string(payload), nil
}

func (r *ReasoningSearchResponse) SetReasoningSteps(steps []ReasoningSearchReasoningStep) {
	if steps == nil {
		r.ReasoningSummary = []ReasoningSearchReasoningStep{}
		return
	}
	r.ReasoningSummary = steps
}

func (r *ReasoningSearchResponse) SetReasoningProcessStats(usedTokens int, reasoningIterations int) {
	r.UsedTokens = usedTokens
	r.ReasoningIterations = reasoningIterations
}

func (r *ReasoningSearchResponse) SetReasoningDebugInfo(debug *ReasoningSearchDebugInfo) {
	r.Debug = debug
}

func (r *ReasoningSearchResponse) SetUsedTools(usedTools []string) {
	if usedTools == nil {
		r.UsedTools = []string{}
		return
	}
	r.UsedTools = usedTools
}

func (r *ReasoningSearchResponse) SetSessionID(sessionID string) {
	r.SessionID = sessionID
}

func (r *ReasoningSearchResponse) SetFollowupBudget(maxFollowups int, used int, remaining int) {
	r.MaxFollowups = maxFollowups
	r.FollowupsUsed = used
	r.FollowupsRemaining = remaining
}

func (d *ReasoningSearchDebugInfo) Add(other *ReasoningSearchDebugInfo) {
	if d == nil || other == nil {
		return
	}

	hadUsageOrCost := d.hasUsageOrCost()
	hadPricingConfigured := d.PricingConfigured
	otherHasUsageOrCost := other.hasUsageOrCost()

	d.TotalTokens += other.TotalTokens
	d.InputTokens += other.InputTokens
	d.CachedInputTokens += other.CachedInputTokens
	d.UncachedInputTokens += other.UncachedInputTokens
	d.OutputTokens += other.OutputTokens
	d.ReasoningTokens += other.ReasoningTokens
	d.EstimatedInputCostUSD += other.EstimatedInputCostUSD
	d.EstimatedCachedInputCostUSD += other.EstimatedCachedInputCostUSD
	d.EstimatedOutputCostUSD += other.EstimatedOutputCostUSD
	d.EstimatedCostUSD += other.EstimatedCostUSD
	if len(other.AIToolsCalls) != 0 {
		d.AIToolsCalls = append(d.AIToolsCalls, other.AIToolsCalls...)
	}
	if d.DraftModelUsage == nil && other.DraftModelUsage != nil {
		copy := *other.DraftModelUsage
		d.DraftModelUsage = &copy
	}
	if len(other.DraftModelRuns) != 0 {
		d.DraftModelRuns = append(d.DraftModelRuns, other.DraftModelRuns...)
		d.DraftModelUsage = aggregateReasoningSearchUsageBreakdowns(d.DraftModelRuns)
	}
	if otherHasUsageOrCost {
		if !hadUsageOrCost {
			d.PricingConfigured = other.PricingConfigured
		} else {
			d.PricingConfigured = hadPricingConfigured && other.PricingConfigured
		}
	}
}

func (d *ReasoningSearchDebugInfo) hasUsageOrCost() bool {
	if d == nil {
		return false
	}
	return d.TotalTokens != 0 ||
		d.InputTokens != 0 ||
		d.CachedInputTokens != 0 ||
		d.UncachedInputTokens != 0 ||
		d.OutputTokens != 0 ||
		d.ReasoningTokens != 0 ||
		d.EstimatedInputCostUSD != 0 ||
		d.EstimatedCachedInputCostUSD != 0 ||
		d.EstimatedOutputCostUSD != 0 ||
		d.EstimatedCostUSD != 0
}

func (d *ReasoningSearchDebugInfo) UsageBreakdown() *ReasoningSearchUsageBreakdown {
	if d == nil {
		return nil
	}
	return &ReasoningSearchUsageBreakdown{
		Model:                       d.Model,
		ReasoningEffort:             d.ReasoningEffort,
		TotalTokens:                 d.TotalTokens,
		InputTokens:                 d.InputTokens,
		CachedInputTokens:           d.CachedInputTokens,
		UncachedInputTokens:         d.UncachedInputTokens,
		OutputTokens:                d.OutputTokens,
		ReasoningTokens:             d.ReasoningTokens,
		PricingConfigured:           d.PricingConfigured,
		InputPer1MTokensUSD:         d.InputPer1MTokensUSD,
		CachedInputPer1MTokensUSD:   d.CachedInputPer1MTokensUSD,
		OutputPer1MTokensUSD:        d.OutputPer1MTokensUSD,
		EstimatedInputCostUSD:       d.EstimatedInputCostUSD,
		EstimatedCachedInputCostUSD: d.EstimatedCachedInputCostUSD,
		EstimatedOutputCostUSD:      d.EstimatedOutputCostUSD,
		EstimatedCostUSD:            d.EstimatedCostUSD,
	}
}

func GenerateReasoningSearchVerificationResponseJSONSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "needs_another_iteration": {
      "type": "boolean",
      "description": "True when the reasoning step should run one more time using the recommendation below."
    },
    "recommendation": {
      "type": "string",
      "description": "A short recommendation on how to improve the current results. Return an empty string when no extra iteration is needed."
    }
  },
  "required": ["needs_another_iteration", "recommendation"]
}`
}

func GenerateReasoningSearchPlanningResponseJSONSchema(tools []ReasoningTool) string {
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		name := tool.Definition().Name
		if name == "" {
			continue
		}
		toolNames = append(toolNames, name)
	}

	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"instruction_text": map[string]interface{}{
				"type":        "string",
				"description": "Short request-specific instruction text to inject into the reasoning system prompt.",
			},
			"first_iteration_tools": map[string]interface{}{
				"type":        "array",
				"description": "Planned first-iteration tool options. These will be the only tool options available on the first reasoning iteration.",
				"maxItems":    4,
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"tool_name": map[string]interface{}{
							"type":        "string",
							"enum":        toolNames,
							"description": "One of the available tool names.",
						},
						"params_json": map[string]interface{}{
							"type":        "string",
							"description": "A JSON object string with predefined arguments for the selected tool. Example: {\"query\":\"חיים חדשים\",\"language\":\"he\"}.",
						},
						"alternative_queries": map[string]interface{}{
							"type":        "array",
							"description": "Only for elasticsearch_search. Alternative query strings that should be converted into additional first-iteration tool options using the same non-query params. Use an empty array for other tools.",
							"items": map[string]interface{}{
								"type": "string",
							},
							"maxItems": 4,
						},
					},
					"required": []string{"tool_name", "params_json", "alternative_queries"},
				},
			},
		},
		"required": []string{"instruction_text", "first_iteration_tools"},
	}

	payload, err := json.Marshal(schema)
	if err != nil {
		return `{"type":"object","additionalProperties":false,"properties":{"instruction_text":{"type":"string"},"first_iteration_tools":{"type":"array","items":{"type":"object","additionalProperties":false,"properties":{"tool_name":{"type":"string"},"params_json":{"type":"string"},"alternative_queries":{"type":"array","items":{"type":"string"}}},"required":["tool_name","params_json","alternative_queries"]}}},"required":["instruction_text","first_iteration_tools"]}`
	}
	return string(payload)
}
