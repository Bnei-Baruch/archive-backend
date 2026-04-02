package llm

import (
	"encoding/json"
	"fmt"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

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

var reasoningSearchContentTypeEnum = []string{
	"",
	consts.CT_ARTICLES,
	consts.CT_BOOKS,
	// consts.CT_CHILDREN_LESSONS,
	consts.CT_CLIPS,
	consts.CT_CONGRESS,
	consts.CT_DAILY_LESSON,
	consts.CT_FRIENDS_GATHERINGS,
	consts.CT_HOLIDAY,
	consts.CT_LECTURE_SERIES,
	consts.CT_LESSONS_SERIES,
	consts.CT_MEALS,
	consts.CT_PICNIC,
	consts.CT_SONGS,
	consts.CT_SPECIAL_LESSON,
	consts.CT_UNITY_DAY,
	consts.CT_VIDEO_PROGRAM,
	consts.CT_VIRTUAL_LESSONS,
	consts.CT_WOMEN_LESSONS,
	consts.CT_ARTICLE,
	consts.CT_BLOG_POST,
	consts.CT_BOOK,
	// consts.CT_CHILDREN_LESSON,
	consts.CT_CLIP,
	consts.CT_EVENT_PART,
	consts.CT_FRIENDS_GATHERING,
	consts.CT_FULL_LESSON,
	consts.CT_KITEI_MAKOR,
	consts.CT_LECTURE,
	// consts.CT_LELO_MIKUD,
	consts.CT_LESSON_PART,
	consts.CT_MEAL,
	consts.CT_PUBLICATION,
	consts.CT_RESEARCH_MATERIAL,
	// consts.CT_KTAIM_NIVCHARIM,
	consts.CT_SONG,
	consts.CT_TRAINING,
	consts.CT_UNKNOWN,
	consts.CT_VIDEO_PROGRAM_CHAPTER,
	consts.CT_VIRTUAL_LESSON,
	consts.CT_WOMEN_LESSON,
	consts.CT_SOURCE,
	consts.CT_LIKUTIM,
}

func GenerateReasoningSearchResponseJSONSchema() string {
	contentTypeEnumJSON, err := json.Marshal(reasoningSearchContentTypeEnum)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal reasoning search content_type enum: %v", err))
	}

	return fmt.Sprintf(`{
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
          "title": {
            "type": "string",
            "description": "Title as returned by elasticsearch or mdb"
          },
          "full_title": {
            "type": "string",
            "description": "Full title as returned by elasticsearch or mdb if available, otherwise an empty string."
          },
          "content_type": {
            "type": "string",
            "description": "filter_values.content_type value for the result, or empty string if unavailable (ignore if result_type is 'sources').",
            "enum": %s
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
          "title",
          "full_title",
          "content_type",
          "language",
          "date",
          "reason",
          "highlights",
          "is_grouping_result"
        ]
      }
    }
  },
  "required": ["query", "summary", "reasoning_summary", "results"]
}`, string(contentTypeEnumJSON))
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
