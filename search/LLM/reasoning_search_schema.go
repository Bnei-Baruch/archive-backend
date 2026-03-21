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
	Notes               []string                  `json:"notes"`
	Debug               *ReasoningSearchDebugInfo `json:"debug,omitempty"`
}

type ReasoningSearchResult struct {
	MDBUID           string   `json:"mdb_uid"`
	ResultType       string   `json:"result_type"`
	Title            string   `json:"title"`
	FullTitle        string   `json:"full_title"`
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
      "description": "A short explanation of the best results found for the user."
    },
    "reasoning_summary": {
      "type": "string",
      "description": "A short summary of the reasoning process when debug mode is enabled, otherwise an empty string."
    },
    "used_tokens": {
      "type": "integer",
      "description": "The actual number of tokens used by the reasoning process."
    },
    "reasoning_iterations": {
      "type": "integer",
      "description": "The actual number of reasoning iterations executed."
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
          "title": {
            "type": "string",
            "description": "Best title to display for the result."
          },
          "full_title": {
            "type": "string",
            "description": "Full title if available, otherwise an empty string."
          },
          "description": {
            "type": "string",
            "description": "Short description if available, otherwise an empty string."
          },
          "content_type": {
            "type": "string",
            "description": "Content type if available, otherwise an empty string."
          },
          "language": {
            "type": "string",
            "description": "Best available language code for the result, or empty string if unavailable."
          },
          "date": {
            "type": "string",
            "description": "Best available result date in YYYY-MM-DD format, or empty string if unavailable."
          },
          "reason": {
            "type": "string",
            "description": "Why this result was selected for the user."
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
          "title",
          "full_title",
          "description",
          "content_type",
          "language",
          "date",
          "reason",
          "highlights",
          "is_grouping_result"
        ]
      }
    },
    "notes": {
      "type": "array",
      "description": "Short notes, caveats, or follow-up guidance. Return an empty array if there are no notes.",
      "items": {
        "type": "string"
      }
    }
  },
  "required": ["query", "summary", "reasoning_summary", "used_tokens", "reasoning_iterations", "results", "notes"]
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
