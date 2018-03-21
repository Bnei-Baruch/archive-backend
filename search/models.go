package search

import "context"

type Query struct {
	Term          string
	ExactTerms    []string
	Filters       map[string][]string
	LanguageOrder []string
	Deb           bool
}

type Engine interface {
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
	DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error)
}
