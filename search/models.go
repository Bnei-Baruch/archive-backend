package search

import (
	"context"

	"gopkg.in/olivere/elastic.v6"
	null "gopkg.in/volatiletech/null.v6"
)

type Intent struct {
	Type     string      `json:"type"`
	Language string      `json:"language"`
	Value    interface{} `json:"value,omitempty"`
}

type QueryResult struct {
	SearchResult *elastic.SearchResult `json:"search_result,omitempty"`
	TypoSuggest  null.String           `json:"typo_suggest"`
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
}
