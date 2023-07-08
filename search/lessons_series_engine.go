package search

import (
	"context"
	"fmt"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
	"gopkg.in/olivere/elastic.v6"
)

func (e *ESEngine) LessonsSeries(query Query, preference string) (map[string]*elastic.SearchResult, error) {
	byLang := make(map[string]*elastic.SearchResult)
	mss := e.esc.MultiSearch()
	_, queryTermHasDigit := utils.HasNumeric(query.Term)
	filter := map[string][]string{consts.FILTER_CONTENT_TYPE: {consts.CT_LESSONS_SERIES}}
	for _, language := range query.LanguageOrder {
		index := es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, language)
		req, err := NewResultsSearchRequest(
			SearchRequestOptions{
				resultTypes:      []string{consts.ES_RESULT_TYPE_COLLECTIONS},
				index:            index,
				query:            Query{Term: query.Term, ExactTerms: query.ExactTerms, Filters: filter, LanguageOrder: query.LanguageOrder, Deb: query.Deb},
				sortBy:           consts.SORT_BY_RELEVANCE,
				from:             0,
				size:             100,
				preference:       preference,
				useHighlight:     false,
				partialHighlight: false})
		if err != nil {
			return nil, err
		}
		mss.Add(req)
	}
	before := time.Now()
	mr, err := mss.Do(context.TODO())

	e.timeTrack(before, consts.LAT_DOSEARCH_MULTISEARCHLESSONSERIESDO)
	if err != nil {
		return nil, err
	}

	for i, res := range mr.Responses {
		if res.Error != nil {
			err := errors.New(fmt.Sprintf("Failed series get: %+v", res.Error))
			return nil, err
		}
		if haveHits(res) {
			lang := query.LanguageOrder[i]
			byLang[lang] = res
		}
	}

	if queryTermHasDigit {
		// When the query has a number, we assume that the user is looking for a specific collection and we avoid grouping.
		return byLang, nil
	}
	return CombineBySourceOrTag(byLang, consts.ES_RESULT_TYPE_COLLECTIONS, null.StringFrom(consts.SEARCH_RESULT_LESSONS_SERIES_BY_SOURCE), null.StringFrom(consts.SEARCH_RESULT_LESSONS_SERIES_BY_TAG)), nil
}
