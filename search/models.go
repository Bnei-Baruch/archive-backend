package search

import (
	"context"
	"encoding/json"

	"github.com/volatiletech/null/v8"
)

// ---------------------------------------------------------------------------
// Shared result types — used by both ES6 and ES9 engines
// ---------------------------------------------------------------------------

// SearchHitHighlight maps field name to highlighted fragments.
type SearchHitHighlight map[string][]string

// SearchExplanation holds the relevance score explanation for a hit.
type SearchExplanation struct {
	Value       float64             `json:"value"`
	Description string              `json:"description"`
	Details     []SearchExplanation `json:"details,omitempty"`
}

// SearchHitInnerHits holds nested inner hits (used by tweets aggregation).
type SearchHitInnerHits struct {
	Hits *SearchHits `json:"hits,omitempty"`
}

// SearchHit represents a single document match returned by Elasticsearch.
type SearchHit struct {
	Index       string                         `json:"_index"`
	Type        string                         `json:"_type,omitempty"` // populated by ES6, empty in ES9
	ID          string                         `json:"_id"`
	Uid         string                         `json:"_uid,omitempty"` // ES6 meta field; also used internally as a grouping key
	Score       *float64                       `json:"_score,omitempty"`
	Source      *json.RawMessage               `json:"_source,omitempty"`
	Highlight   SearchHitHighlight             `json:"highlight,omitempty"`
	InnerHits   map[string]*SearchHitInnerHits `json:"inner_hits,omitempty"`
	Explanation *SearchExplanation             `json:"_explanation,omitempty"`
}

// SearchHits holds a page of search hits plus aggregate metadata.
type SearchHits struct {
	TotalHits int64        `json:"total"`
	MaxScore  *float64     `json:"max_score,omitempty"`
	Hits      []*SearchHit `json:"hits"`
}

// SearchResult is the top-level response from an Elasticsearch search.
type SearchResult struct {
	Hits    *SearchHits   `json:"hits"`
	Suggest SearchSuggest `json:"suggest,omitempty"`
}

// SearchSuggest maps suggester name to its list of suggestions.
type SearchSuggest map[string][]SearchSuggestion

// SearchSuggestion is one entry returned by a suggester.
type SearchSuggestion struct {
	Text    string                 `json:"text"`
	Offset  int                    `json:"offset"`
	Length  int                    `json:"length"`
	Options []SearchSuggestionOption `json:"options"`
}

// SearchSuggestionOption is a single candidate within a suggestion.
type SearchSuggestionOption struct {
	Text   string           `json:"text"`
	Score  float64          `json:"score"`
	Freq   int              `json:"freq,omitempty"`
	Source *json.RawMessage `json:"_source,omitempty"`
}

// ---------------------------------------------------------------------------
// Engine-level types
// ---------------------------------------------------------------------------

type Intent struct {
	Type     string      `json:"type"`
	Language string      `json:"language"`
	Value    interface{} `json:"value,omitempty"`
}

type TimeLog struct {
	Operation string `json:"operation"`
	Time      int64  `json:"time"`
}

type QueryResult struct {
	SearchResult     *SearchResult `json:"search_result,omitempty"`
	TypoSuggest      null.String   `json:"typo_suggest"`
	Language         string        `json:"language"`
	ExecutionTimeLog []TimeLog     `json:"execution_time_log,omitempty"`
}

type Engine interface {
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
	DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error)
}

type SearchRequestOptions struct {
	resultTypes          []string
	docIds               []string
	index                string
	query                Query
	sortBy               string
	from                 int
	size                 int
	preference           string
	useHighlight         bool
	highlightFullContent bool
	// Following field comes to solve elastic bug with highlight.
	// Just removed the analyzed fields and uses only standard fields
	// for highlighting. Only happens with intents.
	partialHighlight bool
	// Following field comes to reduce results duplication.
	// If we have classification intent (carousel) by source, filter out this results from the main search.
	filterOutCUSources []string
	// Setting the following field to 'true' will ignore the search of content and in some cases also description.
	// Description is considered as subtitle in sources,
	//  so the 'description' field will be included only when this field is true and resultTypes contains only 'sources'.
	// This field is used for classification intents (carousel) search and grammar filter for 'books'.
	titlesOnly bool
	// If not nil, set how long a search is allowed to take, e.g. "1s" or "500ms". Note: Not always respected by ES.
	Timeout *string
}

type CreateFacetAggregationOptions struct {
	tagUIDs                []string
	mediaLanguageValues    []string
	originalLanguageValues []string
	contentTypeValues      []string
	sourceUIDs             []string
	dateRanges             []string
	personUIDs             []string
}

type FacetSearchResults struct {
	Tags              map[string]int64 `json:"tags,omitempty"`
	MediaLanguages    map[string]int64 `json:"languages,omitempty"`
	OriginalLanguages map[string]int64 `json:"original_languages,omitempty"`
	ContentTypes      map[string]int64 `json:"content_types,omitempty"`
	Sources           map[string]int64 `json:"sources,omitempty"`
	Dates             map[string]int64 `json:"dates,omitempty"`
	Persons           map[string]int64 `json:"persons,omitempty"`
}
