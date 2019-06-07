package search

import (
	"context"

	"gopkg.in/olivere/elastic.v6"
)

type Intent struct {
	Type     string      `json:"type"`
	Language string      `json:"language"`
	Value    interface{} `json:"value,omitempty"`
}

type QueryResult struct {
	SearchResult *elastic.SearchResult `json:"search_result,omitempty"`
	// TODO: Intents field below is deprecated and not being used.
	Intents []Intent `json:"intents,omitempty"`
}

type Engine interface {
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
	DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error)
}

type SearchRequestOptions struct {
	resultTypes  []string
	docIds       []string
	index        string
	query        Query
	sortBy       string
	from         int
	size         int
	preference   string
	useHighlight bool
	// Following field comes to solve elastic bug with highlight.
	// Just removed the analyzed fields and uses only standard fields
	// for highlighting. Only happens with intents.
	partialHighlight bool
}
