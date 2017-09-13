package search

import "context"

type Query struct {
	Term          string
	LanguageOrder []string
}

type Engine interface {
	DoSearch(ctx context.Context, query Query) (interface{}, error)
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
}
