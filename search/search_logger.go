package search

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"
)

type SearchLog struct {
	Created  time.Time   `json:"created",omitempty`
	Query    Query       `json:"query"`
	Results  interface{} `json:"results,omitempty"`
	Error    interface{} `json:"error,omitempty"`
	SortBy   string      `json:"sort_by,omitempty"`
	From     uint64      `json:"from"`
	Size     uint64      `json:"size,omitempty"`
	SearchId string      `json:"search_id"`
}

type SearchClick struct {
	Created  time.Time `json:"click_created,omitempty"`
	MdbUid   string    `json:"mdb_uid",omitempty`
	Index    string    `json:"index",omitempty`
	Type     string    `json:"type",omitempty`
	Rank     uint32    `json:"rank",omitempty`
	SearchId string    `json:"search_id",omitempty`
}

type SearchLogger struct {
	esc *elastic.Client
}

func MakeSearchLogger(esc *elastic.Client) *SearchLogger {
	return &SearchLogger{esc: esc}
}

func (searchLogger *SearchLogger) LogClick(mdbUid string, index string, indexType string, rank int, searchId string) error {
	sc := SearchClick{
		Created:  time.Now(),
		MdbUid:   mdbUid,
		Index:    index,
		Type:     indexType,
		Rank:     uint32(rank),
		SearchId: searchId,
	}

	sr, err := elastic.NewSearchService(searchLogger.esc).
		Index("search_logs").
		Type("search_logs").
		Query(elastic.NewMatchQuery("search_id", searchId)).
		Do(context.TODO())
	if sr.Hits == nil || sr.Hits.Hits == nil || len(sr.Hits.Hits) == 0 {
		return errors.Errorf("Did not find appropriate search id %s", searchId)
	}
	if len(sr.Hits.Hits) > 1 {
		return errors.Errorf("Found more then one search id %s", searchId)
	}
	parentId := sr.Hits.Hits[0].Id

	resp, err := searchLogger.esc.Index().
		Index("search_logs").
		Type("search_clicks").
		BodyJson(sc).
		Parent(parentId).
		Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Log Click")
	}
	if !resp.Created {
		return errors.Errorf("Click log not created.")
	}
	return nil
}

func (searchLogger *SearchLogger) LogSearch(query Query, sortBy string, from int, size int, searchId string, res interface{}) error {
	return searchLogger.logSearch(query, sortBy, from, size, searchId, res, nil)
}

func (searchLogger *SearchLogger) LogSearchError(query Query, sortBy string, from int, size int, searchId string, searchErr interface{}) error {
	return searchLogger.logSearch(query, sortBy, from, size, searchId, nil, searchErr)
}

func (searchLogger *SearchLogger) logSearch(query Query, sortBy string, from int, size int, searchId string, res interface{}, searchErr interface{}) error {
	sl := SearchLog{
		Created:  time.Now(),
		Query:    query,
		Results:  res,
		Error:    searchErr,
		SortBy:   sortBy,
		From:     uint64(from),
		Size:     uint64(size),
		SearchId: searchId,
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

func (searchLogger *SearchLogger) GetAllQueries() ([]SearchLog, error) {
	var ret []SearchLog
	var searchResult *elastic.SearchResult
	for true {
		if searchResult != nil && searchResult.Hits != nil {
			for _, h := range searchResult.Hits.Hits {
				sl := SearchLog{}
				json.Unmarshal(*h.Source, &sl)
				ret = append(ret, sl)
			}
		}
		var err error
		scrollClient := searchLogger.esc.Scroll().
			Index("search_logs").
			Query(elastic.NewMatchAllQuery()).
			Scroll("1m").
			Size(100)
		if searchResult != nil {
			scrollClient = scrollClient.ScrollId(searchResult.ScrollId)
		}
		searchResult, err = scrollClient.Do(context.TODO())
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return ret, nil
}
