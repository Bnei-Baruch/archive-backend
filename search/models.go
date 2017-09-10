package search

import "context"

type Query struct {
	Language string
	Term string
}

type SearchResults struct {

}

type SearchSuggestions struct {

}

type SearchHandler interface {
	ID() string
	DoSearch(ctx context.Context, query *Query) (*SearchResults, error)
	GetSuggestions(ctx context.Context, query *Query) (*SearchSuggestions, error)
}
