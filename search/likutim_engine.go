package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"
)

func (e *ESEngine) LikutimSeries(query Query, preference string) (map[string]*elastic.SearchResult, error) {
	byLang := make(map[string]*elastic.SearchResult)
	mss := e.esc.MultiSearch()
	_, queryTermHasDigit := utils.HasNumeric(query.Term)
	filter := map[string][]string{consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]: {consts.CT_LIKUTIM}}
	for _, language := range query.LanguageOrder {
		index := es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, language)
		req, err := NewResultsSearchRequest(
			SearchRequestOptions{
				resultTypes:      []string{consts.ES_RESULT_TYPE_UNITS},
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

	e.timeTrack(before, consts.LAT_DOSEARCH_MULTISEARCHTWEETSDO)
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
	return combineLikutimByTag(byLang), nil
}

func combineLikutimByTag(byLang map[string]*elastic.SearchResult) map[string]*elastic.SearchResult {
	for l, r := range byLang {
		hitByTag := make(map[string]*elastic.SearchHit)
		hitsWithoutSourceOrTag := []*elastic.SearchHit{}
		var maxScore *float64
		for _, h := range r.Hits.Hits {
			tuid := getLikutimHitTag(h)
			if tuid != "" {
				if val, hasKey := hitByTag[tuid]; !hasKey || (h.Score != nil && *h.Score > *val.Score) {
					hitByTag[tuid] = h
				}
			} else {
				hitsWithoutSourceOrTag = append(hitsWithoutSourceOrTag, h)
				if maxScore == nil || *h.Score > *maxScore {
					maxScore = h.Score
				}
			}
		}

		byLang[l].Hits = new(elastic.SearchHits)
		byLang[l].Hits.MaxScore = maxScore
		byLang[l].Hits.Hits = hitsWithoutSourceOrTag

		for k, h := range hitByTag {
			newH := &elastic.SearchHit{
				Source:      h.Source,
				Type:        consts.SEARCH_RESULT_LIKUTIM_SERIES_BY_TAG,
				Score:       h.Score,
				Uid:         k,
				Explanation: h.Explanation,
			}
			if byLang[l].Hits.MaxScore == nil || *h.Score > *byLang[l].Hits.MaxScore {
				byLang[l].Hits.MaxScore = h.Score
			}
			byLang[l].Hits.TotalHits++
			byLang[l].Hits.Hits = append(byLang[l].Hits.Hits, newH)
		}
		if byLang[l].Hits == nil {
			delete(byLang, l)
		}
	}
	return byLang
}

func getLikutimHitTag(hit *elastic.SearchHit) string {
	var res es.Result
	if err := json.Unmarshal(*hit.Source, &res); err != nil {
		return ""
	}
	tagKeys := make(map[string]bool)

	if res.ResultType != consts.ES_RESULT_TYPE_UNITS {
		return ""
	}

	for _, tId := range res.TypedUids {
		l := strings.Split(tId, ":")
		if l[0] == consts.ES_UID_TYPE_TAG && l[1] != "" {
			if _, ok := tagKeys[l[1]]; !ok { // avoid duplicates
				tagKeys[l[1]] = true
			}
		}
	}
	var tagList []string
	for k := range tagKeys {
		tagList = append(tagList, k)
	}
	sort.Strings(tagList)
	tag := strings.Join(tagList, "_")

	return tag
}
