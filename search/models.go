package search

import "context"

type Query struct {
	Term          string              `json:"term,omitempty"`
	ExactTerms    []string            `json:"exact_terms,omitempty"`
	Filters       map[string][]string `json:"filters,omitempty"`
	LanguageOrder []string            `json:"language_order",omitempty`
	Deb           bool                `json:"deb",omitempty`
}

type Engine interface {
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
	DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error)
}
