package search

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"
)

type SearchLog struct {
	SearchId         string      `json:"search_id"`
	Created          time.Time   `json:"created"`
	LogType          string      `json:"log_type"`
	Query            Query       `json:"query"`
	QueryResult      interface{} `json:"query_result,omitempty"`
	Error            interface{} `json:"error,omitempty"`
	SortBy           string      `json:"sort_by,omitempty"`
	From             uint64      `json:"from,omitempty"`
	Size             uint64      `json:"size,omitempty"`
	Suggestion       string      `json:"suggestion,omitempty"`
	ExecutionTimeLog []TimeLog   `json:"execution_time_log,omitempty"`
	IsDebug          bool        `json:"is_debug"`
}

type TimeLog struct {
	Operation string `json:"operation"`
	Time      int64  `json:"time"`
}

type SearchClick struct {
	SearchId   string    `json:"search_id"`
	Created    time.Time `json:"created"`
	LogType    string    `json:"log_type"`
	MdbUid     string    `json:"mdb_uid",omitempty`
	Index      string    `json:"index",omitempty`
	ResultType string    `json:"result_type",omitempty`
	Rank       uint32    `json:"rank",omitempty`
	IsDebug    bool      `json:"is_debug"`
}

type CreatedSearchLogs []SearchLog

func (csl CreatedSearchLogs) Len() int {
	return len(csl)
}

func (csl CreatedSearchLogs) Less(i, j int) bool {
	return csl[i].Created.Before(csl[j].Created)
}

func (csl CreatedSearchLogs) Swap(i, j int) {
	csl[i], csl[j] = csl[j], csl[i]
}

type CreatedSearchClicks []SearchClick

func (csc CreatedSearchClicks) Len() int {
	return len(csc)
}

func (csc CreatedSearchClicks) Less(i, j int) bool {
	return csc[i].Created.Before(csc[j].Created)
}

func (csc CreatedSearchClicks) Swap(i, j int) {
	csc[i], csc[j] = csc[j], csc[i]
}

type SearchLogger struct {
	esc *elastic.Client
}

func MakeSearchLogger(esc *elastic.Client) *SearchLogger {
	return &SearchLogger{esc: esc}
}

func (searchLogger *SearchLogger) LogClick(mdbUid string, index string, resultType string, rank int, searchId string, isDebug bool) error {
	sc := SearchClick{
		SearchId:   searchId,
		Created:    time.Now(),
		LogType:    "click",
		MdbUid:     mdbUid,
		Index:      index,
		ResultType: resultType,
		Rank:       uint32(rank),
		IsDebug:    isDebug,
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

	resp, err := searchLogger.esc.Index().
		Index("search_logs").
		Type("search_logs").
		BodyJson(sc).
		Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Log Click")
	}
	log.Infof("Create resp: %+v", resp)
	// if !resp.Created {
	// 	return errors.Errorf("Click log not created.")
	// }
	return nil
}

func (searchLogger *SearchLogger) LogSearch(query Query, sortBy string, from int, size int, searchId string, suggestion string, res *QueryResult, executionTimeLog map[string]time.Duration, isDebug bool) error {
	return searchLogger.logSearch(query, sortBy, from, size, searchId, suggestion, res, nil, executionTimeLog, isDebug)
}

func (searchLogger *SearchLogger) LogSearchError(query Query, sortBy string, from int, size int, searchId string, suggestion string, searchErr interface{}, executionTimeLog map[string]time.Duration, isDebug bool) error {
	return searchLogger.logSearch(query, sortBy, from, size, searchId, suggestion, nil, searchErr, executionTimeLog, isDebug)
}

func (searchLogger *SearchLogger) fixHighlight(h *elastic.SearchHitHighlight) *elastic.SearchHitHighlight {
	if h == nil {
		return nil
	}
	hRet := make(elastic.SearchHitHighlight, 0)
	for fieldName, highlights := range *h {
		fixedFieldName := strings.Replace(fieldName, ".", "_", -1)
		hRet[fixedFieldName] = highlights
	}
	return &hRet
}

// Should not change the input, should copy hit and fix highlight fields.
func (searchLogger *SearchLogger) fixResults(res *QueryResult) *QueryResult {
	if res == nil {
		return nil
	}
	if res.SearchResult.Hits != nil && res.SearchResult.Hits.Hits != nil && len(res.SearchResult.Hits.Hits) > 0 {
		hitsCopy := make([]*elastic.SearchHit, len(res.SearchResult.Hits.Hits))
		for i, h := range res.SearchResult.Hits.Hits {
			hCopy := *h
			hCopy.Highlight = *searchLogger.fixHighlight(&hCopy.Highlight)
			hitsCopy[i] = &hCopy
		}
		searchResultCopy := *res.SearchResult
		searchResultCopy.Hits.Hits = hitsCopy
		resCopy := *res
		resCopy.SearchResult = &searchResultCopy
		return &resCopy
	}
	return res
}

func (searchLogger *SearchLogger) logSearch(query Query, sortBy string, from int, size int, searchId string, suggestion string, res *QueryResult, searchErr interface{}, executionTimeLog map[string]time.Duration, isDebug bool) error {

	timeLogArr := []TimeLog{}
	for k := range executionTimeLog {
		ms := int64(executionTimeLog[k] / time.Millisecond)
		timeLogArr = append(timeLogArr, TimeLog{Operation: k, Time: ms})
	}

	sl := SearchLog{
		Created:          time.Now(),
		SearchId:         searchId,
		LogType:          "query",
		Query:            query,
		QueryResult:      searchLogger.fixResults(res),
		Error:            searchErr,
		SortBy:           sortBy,
		From:             uint64(from),
		Size:             uint64(size),
		Suggestion:       suggestion,
		ExecutionTimeLog: timeLogArr,
		IsDebug:          isDebug,
	}
	resp, err := searchLogger.esc.Index().
		Index("search_logs").
		Type("search_logs").
		BodyJson(sl).
		Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Log Search")
	}
	if resp.Result != "created" {
		return errors.Errorf("Search log not created.")
	}
	return nil
}

func (searchLogger *SearchLogger) GetAllQueries(s *elastic.SliceQuery) ([]SearchLog, error) {
	var ret []SearchLog
	var searchResult *elastic.SearchResult
	for true {
		log.Infof("Scrolling...")
		if searchResult != nil && searchResult.Hits != nil {
			log.Infof("Git %d hits...", len(searchResult.Hits.Hits))
			for _, h := range searchResult.Hits.Hits {
				sl := SearchLog{}
				json.Unmarshal(*h.Source, &sl)
				ret = append(ret, sl)
			}
		}
		var err error
		scrollClient := searchLogger.esc.Scroll().
			Index("search_logs").
			Query(elastic.NewTermsQuery("log_type", "query")).
			Scroll("5m").
			Slice(s).
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

func (searchLogger *SearchLogger) GetAllClicks() ([]SearchClick, error) {
	var ret []SearchClick
	var searchResult *elastic.SearchResult
	for true {
		if searchResult != nil && searchResult.Hits != nil {
			for _, h := range searchResult.Hits.Hits {
				sl := SearchClick{}
				json.Unmarshal(*h.Source, &sl)
				ret = append(ret, sl)
			}
		}
		var err error
		scrollClient := searchLogger.esc.Scroll().
			Index("search_logs").
			Type("search_logs").
			Query(elastic.NewTermsQuery("log_type", "click")).
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
