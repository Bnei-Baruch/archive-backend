package handlers

import (
	"github.com/Bnei-Baruch/archive-backend/search"
	"context"
)

type TagsSearchHandler struct {
	Query *search.Query
}

func (h *TagsSearchHandler) ID() string {
	return "tags"
}

func (h *TagsSearchHandler) DoSearch(ctx context.Context, query *search.Query) (*search.SearchResults, error) {
	return nil, nil
}

func (h *TagsSearchHandler) GetSuggestions(ctx context.Context, query *search.Query) (*search.SearchSuggestions, error) {
	return nil, nil
}