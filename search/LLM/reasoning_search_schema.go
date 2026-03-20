package llm

type ReasoningSearchResponse struct {
	Query   string                  `json:"query"`
	Summary string                  `json:"summary"`
	Results []ReasoningSearchResult `json:"results"`
	Notes   []string                `json:"notes"`
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
  "required": ["query", "summary", "results", "notes"]
}`
}
