package llm

type ReasoningSearchResponse struct {
	SessionID           string                    `json:"session_id"`
	Query               string                    `json:"query"`
	Summary             string                    `json:"summary"`
	ReasoningSummary    string                    `json:"reasoning_summary"`
	UsedTokens          int                       `json:"used_tokens"`
	ReasoningIterations int                       `json:"reasoning_iterations"`
	UsedTools           []string                  `json:"used_tools"`
	Results             []ReasoningSearchResult   `json:"results"`
	Debug               *ReasoningSearchDebugInfo `json:"debug,omitempty"`
}

type ReasoningSearchResult struct {
	MDBUID           string   `json:"mdb_uid"`
	ResultType       string   `json:"result_type"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	ContentType      string   `json:"content_type"`
	Language         string   `json:"language"`
	Date             string   `json:"date"`
	Reason           string   `json:"reason"`
	Highlights       []string `json:"highlights"`
	IsGroupingResult bool     `json:"is_grouping_result"`
}

type ReasoningSearchDebugInfo struct {
	Enabled                     bool    `json:"enabled"`
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

func GenerateReasoningSearchResponseJSONSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "query": {
      "type": "string",
      "description": "The original user query."
    },
    "summary": {
      "type": "string",
      "description": "A short explanation of the best results found for the user. Include caveats, clarification requests, or follow-up guidance here when needed."
    },
    "reasoning_summary": {
      "type": "string",
      "description": "A short summary of the reasoning process when debug mode is enabled, otherwise an empty string."
    },
    "results": {
      "type": "array",
      "description": "Best matching results from the archive. Prefer direct content results when possible.",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "mdb_uid": {
            "type": "string",
            "description": "The result MDB UID."
          },
          "result_type": {
            "type": "string",
            "enum": ["units", "sources", "collections", "posts", "tweets", "tags"],
            "description": "Elasticsearch result type."
          },
          "language": {
            "type": "string",
            "description": "Best available language code for the result, or empty string if unavailable."
          },
          "reason": {
            "type": "string",
            "description": "Short description of the result and explanation of why this result was selected for the user."
          },
          "highlights": {
            "type": "array",
            "description": "Relevant highlight fragments from search results. Return an empty array if unavailable.",
            "items": {
              "type": "string"
            }
          },
          "is_grouping_result": {
            "type": "boolean",
            "description": "True for grouping or narrowing results such as collections or tags."
          }
        },
        "required": [
          "mdb_uid",
          "result_type",
          "language",
          "reason",
          "highlights",
          "is_grouping_result"
        ]
      }
    }
  },
  "required": ["query", "summary", "reasoning_summary", "results"]
}`
}

func (r *ReasoningSearchResponse) SetReasoningSummary(summary string) {
	r.ReasoningSummary = summary
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
