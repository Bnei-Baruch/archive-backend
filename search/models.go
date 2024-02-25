package search

import (
	"context"

	"github.com/volatiletech/null/v8"
	"gopkg.in/olivere/elastic.v6"
)

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
	SearchResult     *elastic.SearchResult `json:"search_result,omitempty"`
	TypoSuggest      null.String           `json:"typo_suggest"`
	Language         string                `json:"language"`
	ExecutionTimeLog []TimeLog             `json:"execution_time_log,omitempty"`
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
