package search

import (
	"context"

	"gopkg.in/olivere/elastic.v5"
)

const (
	I_TAG    = 0
	I_SOURCE = 1
)

type Intent struct {
	Type     int         `json:"type"`
	Language string      `json:"language"`
	Value    interface{} `json:"value,omitempty"`
}

type Query struct {
	Term          string              `json:"term,omitempty"`
	ExactTerms    []string            `json:"exact_terms,omitempty"`
	Filters       map[string][]string `json:"filters,omitempty"`
	LanguageOrder []string            `json:"language_order,omitempty"`
	Deb           bool                `json:"deb,omitempty"`
	Intents       []Intent            `json:"intents,omitempty"`
}

type QueryResult struct {
	SearchResult *elastic.SearchResult `json:"search_result,omitempty"`
	Intents      []Intent              `json:"intents,omitempty"`
}

type Engine interface {
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
	DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error)
}
