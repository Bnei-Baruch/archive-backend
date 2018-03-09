package search

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"
)

type SearchLog struct {
	Created time.Time   `json:"created,omitempty"`
	Query   Query       `json:"query"`
	Results interface{} `json:"results,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	SortBy  string      `json:"sort_by,omitempty"`
	From    uint64      `json:"from"`
	Size    uint64      `json:"size,omitempty"`
}

type SearchLogger struct {
	esc *elastic.Client
}

func MakeSearchLogger(esc *elastic.Client) *SearchLogger {
	return &SearchLogger{esc: esc}
}

func (searchLogger *SearchLogger) LogSearch(query Query, sortBy string, from int, size int, res interface{}) error {
	return searchLogger.logSearch(query, sortBy, from, size, res, nil)
}

func (searchLogger *SearchLogger) LogSearchError(query Query, sortBy string, from int, size int, searchErr interface{}) error {
	return searchLogger.logSearch(query, sortBy, from, size, nil, searchErr)
}

func (searchLogger *SearchLogger) logSearch(query Query, sortBy string, from int, size int, res interface{}, searchErr interface{}) error {
	sl := SearchLog{
		Created: time.Now(),
		Query:   query,
		Results: res,
		Error:   searchErr,
		SortBy:  sortBy,
		From:    uint64(from),
		Size:    uint64(size),
	}
	resp, err := searchLogger.esc.Index().
		Index("search_logs").
		Type("search_logs").
		BodyJson(sl).
		Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Log Search")
	}
	if !resp.Created {
		return errors.Errorf("Search log not created.")
	}
	return nil
}
