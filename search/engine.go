package search

import (
	"context"

	"github.com/Bnei-Baruch/archive-backend/search/handlers"
)

type SearchEngine interface {
	DoSearch(query Query) (*SearchResults, error)
	GetSuggestions(query Query) (*SearchSuggestions, error)
}

type DummySearchEngine struct {
}

func (e *DummySearchEngine) DoSearch(query Query) (*SearchResults, error) {
	handler := handlers.TagsSearchHandler{Query: &query}
	res, err := handler.DoSearch(context.TODO(), &query)
	if err != nil {
		return nil, err
	}

	return res, nil
}
