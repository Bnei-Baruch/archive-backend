package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ESEngine struct {
	esc              *elastic.Client
	mdb              *sql.DB
	cache            cache.CacheManager
	ExecutionTimeLog map[string]time.Duration
}

type byRelevance []*elastic.SearchHit
type byNewerToOlder []*elastic.SearchHit
type byOlderToNewer []*elastic.SearchHit
type bySourceFirst []*elastic.SearchHit

func (s byRelevance) Len() int {
	return len(s)
}
func (s byRelevance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byRelevance) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_RELEVANCE)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

func (s byNewerToOlder) Len() int {
	return len(s)
}
func (s byNewerToOlder) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byNewerToOlder) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_NEWER_TO_OLDER)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

func (s byOlderToNewer) Len() int {
	return len(s)
}
func (s byOlderToNewer) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byOlderToNewer) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_OLDER_TO_NEWER)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

func (s bySourceFirst) Len() int {
	return len(s)
}
func (s bySourceFirst) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s bySourceFirst) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_SOURCE_FIRST)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

// TODO: All interactions with ES should be throttled to prevent downstream pressure

func NewESEngine(esc *elastic.Client, db *sql.DB, cache cache.CacheManager) *ESEngine {
	return &ESEngine{esc: esc, mdb: db, cache: cache, ExecutionTimeLog: make(map[string]time.Duration)}
}

func SuggestionHasOptions(ss elastic.SearchSuggest) bool {
	for _, v := range ss {
		for _, s := range v {
			if len(s.Options) > 0 {
				return true
			}
		}
	}
	return false
}

func (e *ESEngine) GetSuggestions(ctx context.Context, query Query, preference string) (interface{}, error) {
	e.timeTrack(time.Now(), "GetSuggestions")
	multiSearchService := e.esc.MultiSearch()
	requests := NewResultsSuggestRequests([]string{consts.ES_RESULT_TYPE_TAGS, consts.ES_RESULT_TYPE_SOURCES}, query, preference)
	multiSearchService.Add(requests...)

	// Actual call to elastic
	beforeMssDo := time.Now()
	mr, err := multiSearchService.Do(ctx)
	e.timeTrack(beforeMssDo, "GetSuggestions.MultisearchDo")
	if err != nil {
		// don't kill entire request if ctx was cancelled
		if ue, ok := err.(*url.Error); ok {
			if ue.Err == context.DeadlineExceeded || ue.Err == context.Canceled {
				if ue.Err == context.DeadlineExceeded {
					log.Warn("ESEngine.GetSuggestions - ctx cancelled - deadline.")
				}
				if ue.Err == context.Canceled {
					log.Warn("ESEngine.GetSuggestions - ctx cancelled - canceled.")
				}
				return nil, nil
			}
		}
		return nil, errors.Wrap(err, "ESEngine.GetSuggestions")
	}

	// Process response
	sRes := (*elastic.SearchResult)(nil)
	for _, r := range mr.Responses {
		if r != nil && SuggestionHasOptions(r.Suggest) {
			sRes = r
			break
		}
	}

	if sRes == nil && len(mr.Responses) > 0 {
		sRes = mr.Responses[0]
	}

	return sRes, nil
}

func (e *ESEngine) AddIntentSecondRound(h *elastic.SearchHit, intent Intent, query Query) (error, *Intent, *Query) {
	var classificationIntent es.ClassificationIntent
	if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
		return err, nil, nil
	}
	if query.Deb {
		classificationIntent.Explanation = *h.Explanation
	}
	// log.Infof("Hit: %+v %+v", *h.Score, classificationIntent)
	if h.Score != nil && *h.Score > 0 {
		classificationIntent.Score = h.Score
		// Search for specific classification by full name to evaluate max score.
		query.Term = ""
		query.ExactTerms = []string{classificationIntent.Title}
		intent.Value = classificationIntent
		// log.Infof("Potential intent: %s", classificationIntent.Title)
		return nil, &intent, &query
	}
	return nil, nil, nil
}

func (e *ESEngine) AddIntents(query *Query, preference string) error {
	if len(query.Term) == 0 && len(query.ExactTerms) == 0 {
		return nil
	}

	// Don't do intents, if sources are selected in section filter or filtering the results by media language.
	for filterKey := range query.Filters {
		if filterKey == consts.FILTERS[consts.FILTER_SECTION_SOURCES] || filterKey == consts.FILTERS[consts.FILTER_LANGUAGE] {
			return nil
		}
	}

	defer e.timeTrack(time.Now(), "DoSearch.AddIntents")

	checkContentUnitsTypes := []string{}
	if values, ok := query.Filters[consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]]; ok {
		for _, value := range values {
			if value == consts.CT_LESSON_PART {
				checkContentUnitsTypes = append(checkContentUnitsTypes, consts.CT_LESSON_PART)
			}
			if value == consts.CT_VIDEO_PROGRAM_CHAPTER {
				checkContentUnitsTypes = append(checkContentUnitsTypes, consts.CT_VIDEO_PROGRAM_CHAPTER)
			}
		}
	} else {
		checkContentUnitsTypes = append(checkContentUnitsTypes, consts.CT_LESSON_PART, consts.CT_VIDEO_PROGRAM_CHAPTER)
	}

	// Clear filters. We don't want filters on Intents. Filters should be applied to final hits.
	queryWithoutFilters := *query
	queryWithoutFilters.Filters = map[string][]string{}

	mssFirstRound := e.esc.MultiSearch()
	potentialIntents := make([]Intent, 0)
	for _, language := range query.LanguageOrder {
		// Order here provides the priority in results, i.e., tags are more importnt then sources.
		index := es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, language)
		mssFirstRound.Add(NewResultsSearchRequest(
			SearchRequestOptions{
				resultTypes:      []string{consts.ES_RESULT_TYPE_TAGS},
				index:            index,
				query:            queryWithoutFilters,
				sortBy:           consts.SORT_BY_RELEVANCE,
				from:             0,
				size:             consts.API_DEFAULT_PAGE_SIZE,
				preference:       preference,
				useHighlight:     false,
				partialHighlight: true}))
		potentialIntents = append(potentialIntents, Intent{consts.INTENT_TYPE_TAG, language, nil})
		mssFirstRound.Add(NewResultsSearchRequest(
			SearchRequestOptions{
				resultTypes:      []string{consts.ES_RESULT_TYPE_SOURCES},
				index:            index,
				query:            queryWithoutFilters,
				sortBy:           consts.SORT_BY_RELEVANCE,
				from:             0,
				size:             consts.API_DEFAULT_PAGE_SIZE,
				preference:       preference,
				useHighlight:     false,
				partialHighlight: true}))
		potentialIntents = append(potentialIntents, Intent{consts.INTENT_TYPE_SOURCE, language, nil})
	}
	beforeFirstRoundDo := time.Now()
	mr, err := mssFirstRound.Do(context.TODO())
	e.timeTrack(beforeFirstRoundDo, "DoSearch.AddIntents.FirstRoundDo")
	if err != nil {
		return errors.Wrap(err, "ESEngine.AddIntents - Error multisearch Do.")
	}

	// Build second request to evaluate how close the search is toward the full name.
	mssSecondRound := e.esc.MultiSearch()
	finalIntents := make([]Intent, 0)
	for i := 0; i < len(potentialIntents); i++ {
		res := mr.Responses[i]
		if res.Error != nil {
			log.Warnf("ESEngine.AddIntents - First Run %+v", res.Error)
			return errors.New("ESEngine.AddIntents - First Run Failed multi get (S).")
		}
		if haveHits(res) {
			for _, h := range res.Hits.Hits {
				err, intent, secondRoundQuery := e.AddIntentSecondRound(h, potentialIntents[i], queryWithoutFilters)
				// log.Infof("Adding second round for %+v %+v %+v", intent, secondRoundQuery, potentialIntents[i])
				if err != nil {
					return errors.Wrapf(err, "ESEngine.AddIntents - Error second run for intent %+v", potentialIntents[i])
				}
				if intent != nil {
					mssSecondRound.Add(NewResultsSearchRequest(
						SearchRequestOptions{
							resultTypes:      []string{consts.RESULT_TYPE_BY_INDEX_TYPE[potentialIntents[i].Type]},
							index:            es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, intent.Language),
							query:            *secondRoundQuery,
							sortBy:           consts.SORT_BY_RELEVANCE,
							from:             0,
							size:             consts.API_DEFAULT_PAGE_SIZE,
							preference:       preference,
							useHighlight:     false,
							partialHighlight: true}))
					finalIntents = append(finalIntents, *intent)
				}
			}
		}
	}

	beforeSecondRoundDo := time.Now()
	mr, err = mssSecondRound.Do(context.TODO())
	e.timeTrack(beforeSecondRoundDo, "DoSearch.AddIntents.SecondRoundDo")
	for i := 0; i < len(finalIntents); i++ {
		res := mr.Responses[i]
		if res.Error != nil {
			log.Warnf("ESEngine.AddIntents - Second Run %+v", res.Error)
			log.Warnf("ESEngine.AddIntents - Second Run %+v", res.Error.RootCause[0])
			return errors.New("ESEngine.AddIntents - Second Run Failed multi get (S).")
		}
		intentValue, intentOk := finalIntents[i].Value.(es.ClassificationIntent)
		if !intentOk {
			return errors.New(fmt.Sprintf("ESEngine.AddIntents - Unexpected intent value: %+v", finalIntents[i].Value))
		}
		if haveHits(res) {
			// log.Infof("Found Hits for %+v", intentValue)
			found := false
			for _, h := range res.Hits.Hits {
				var classificationIntent es.ClassificationIntent
				if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
					return errors.Wrap(err, "ESEngine.AddIntents - Unmarshal classification intent filed.")
				}
				if query.Deb {
					intentValue.MaxExplanation = *h.Explanation
				}
				log.Infof("%s: %+v", classificationIntent.Title, *h.Score)
				if intentValue.MDB_UID == classificationIntent.MDB_UID {
					found = true
					// log.Infof("Max Score: %+v", *h.Score)
					if h.Score != nil && *h.Score > 0 {
						intentValue.MaxScore = h.Score
						if *intentValue.MaxScore < *intentValue.Score {
							log.Warnf("ESEngine.AddIntents - Not expected score %f to be larger then max score %f for %s - %s.",
								*intentValue.Score, *intentValue.MaxScore, intentValue.MDB_UID, intentValue.Title)
						}
						query.Intents = append(query.Intents, Intent{finalIntents[i].Type, finalIntents[i].Language, intentValue})
					}
				}
			}
			if !found {
				log.Warnf("ESEngine.AddIntents - Did not find matching second run: %s - %s.",
					intentValue.MDB_UID, intentValue.Title)
			}
		}
	}

	// Set content unit type and exists for intents that are in the query, i.e., those who passed the second round.
	// If more then one content unit type exist for this intent, we will have to duplicate that intent.
	moreIntents := make([]Intent, 0)
	for intentIdx := range query.Intents {
		for _, contentUnitType := range checkContentUnitsTypes {
			if intentValue, ok := query.Intents[intentIdx].Value.(es.ClassificationIntent); ok {
				intentP := &query.Intents[intentIdx]
				intentValueP := &intentValue
				if intentValue.ContentType != "" {
					// We need to copy the intent as we have more than one existing content types for that intent.
					moreIntents = append(moreIntents, query.Intents[intentIdx])
					intentP = &moreIntents[len(moreIntents)-1]
					copyIntentValue := intentP.Value.(es.ClassificationIntent)
					intentValueP = &copyIntentValue
				}
				intentValueP.ContentType = contentUnitType
				if intentP.Type == consts.INTENT_TYPE_TAG {
					intentValueP.Exist = e.cache.SearchStats().IsTagWithUnits(intentValueP.MDB_UID, contentUnitType)
				} else if intentP.Type == consts.INTENT_TYPE_SOURCE {
					intentValueP.Exist = e.cache.SearchStats().IsSourceWithUnits(intentValueP.MDB_UID, contentUnitType)
				}
				// Assign the changed intent value, as everything is by value in golang.
				intentP.Value = *intentValueP
			}
		}
	}
	query.Intents = append(query.Intents, moreIntents...)
	return nil
}

func (e *ESEngine) IntentsToResults(query *Query) (error, map[string]*elastic.SearchResult) {
	srMap := make(map[string]*elastic.SearchResult)
	for _, lang := range query.LanguageOrder {
		sh := &elastic.SearchHits{TotalHits: 0}
		sr := &elastic.SearchResult{Hits: sh}
		srMap[lang] = sr
	}
	// log.Infof("IntentsToResults - %d intents.", len(query.Intents))
	for _, intent := range query.Intents {
		// Convert intent to result with score.
		intentValue := intent.Value.(es.ClassificationIntent)
		boostedScore := float64(0.0)
		if intentValue.Exist {
			sh := srMap[intent.Language].Hits
			sh.TotalHits++
			// Boost up to 33% for exact match, i.e., for score / max score of 1.0.
			boostedScore = *intentValue.Score * (3.0 + *intentValue.Score / *intentValue.MaxScore) / 3.0
			if sh.MaxScore != nil {
				maxScore := math.Max(*sh.MaxScore, boostedScore)
				sh.MaxScore = &maxScore
			} else {
				sh.MaxScore = &boostedScore
			}
			intentHit := &elastic.SearchHit{}
			intentHit.Explanation = &intentValue.Explanation
			intentHit.Score = &boostedScore
			intentHit.Index = consts.INTENT_INDEX_BY_TYPE[intent.Type]
			intentHit.Type = consts.INTENT_HIT_TYPE_BY_CT[intentValue.ContentType]
			source, err := json.Marshal(intentValue)
			if err != nil {
				return err, nil
			}
			intentHit.Source = (*json.RawMessage)(&source)
			sh.Hits = append(sh.Hits, intentHit)
		}
		// log.Infof("Added intent %s %s %s boost score:%f exist:%t", intentValue.Title, intent.Type, intent.Language, boostedScore, intentValue.Exist)
	}
	return nil, srMap
}

func haveHits(r *elastic.SearchResult) bool {
	return r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0
}

func score(score *float64) float64 {
	if score == nil {
		return 0
	} else {
		return *score
	}
}

func compareHits(h1 *elastic.SearchHit, h2 *elastic.SearchHit, sortBy string) (bool, error) {
	if sortBy == consts.SORT_BY_RELEVANCE {
		return score(h1.Score) > score(h2.Score), nil
	} else if sortBy == consts.SORT_BY_SOURCE_FIRST {
		var rt1, rt2 es.ResultType
		if err := json.Unmarshal(*h1.Source, &rt1); err != nil {
			return false, err
		}
		if err := json.Unmarshal(*h2.Source, &rt2); err != nil {
			return false, err
		}
		// Order by sources first, then be score.
		return rt1.ResultType == consts.ES_RESULT_TYPE_SOURCES && rt2.ResultType != consts.ES_RESULT_TYPE_SOURCES || score(h1.Score) > score(h2.Score), nil
	} else {
		var ed1, ed2 es.EffectiveDate
		if err := json.Unmarshal(*h1.Source, &ed1); err != nil {
			return false, err
		}
		if err := json.Unmarshal(*h2.Source, &ed2); err != nil {
			return false, err
		}
		if ed1.EffectiveDate == nil {
			ed1.EffectiveDate = &utils.Date{time.Time{}}
		}
		if ed2.EffectiveDate == nil {
			ed2.EffectiveDate = &utils.Date{time.Time{}}
		}
		if sortBy == consts.SORT_BY_OLDER_TO_NEWER {
			// Oder by older to newer, break ties using score.
			return ed2.EffectiveDate.Time.After(ed1.EffectiveDate.Time) ||
				ed2.EffectiveDate.Time.Equal(ed1.EffectiveDate.Time) && score(h1.Score) > score(h2.Score), nil
		} else {
			log.Infof("%+v %+v %+v %+v", ed1, ed2, h1, h2)
			// Order by newer to older, break ties using score.
			return ed2.EffectiveDate.Time.Before(ed1.EffectiveDate.Time) ||
				ed2.EffectiveDate.Time.Equal(ed1.EffectiveDate.Time) && score(h1.Score) > score(h2.Score), nil
		}
	}
}

func joinResponses(sortBy string, from int, size int, results ...*elastic.SearchResult) (*elastic.SearchResult, error) {
	if len(results) == 0 {
		return nil, nil
	}
	// Concatenate all result hits to single slice.
	concatenated := make([]*elastic.SearchHit, 0)
	for _, result := range results {
		concatenated = append(concatenated, result.Hits.Hits...)
	}

	// Apply sorting.
	if sortBy == consts.SORT_BY_RELEVANCE {
		sort.Stable(byRelevance(concatenated))
	} else if sortBy == consts.SORT_BY_OLDER_TO_NEWER {
		sort.Stable(byOlderToNewer(concatenated))
	} else if sortBy == consts.SORT_BY_NEWER_TO_OLDER {
		sort.Stable(byNewerToOlder(concatenated))
	} else if sortBy == consts.SORT_BY_SOURCE_FIRST {
		sort.Stable(bySourceFirst(concatenated))
	}

	// Filter by relevant page.
	concatenated = concatenated[from:utils.Min(from+size, len(concatenated))]

	// Take arbitrary result to use as base and set it's hits.
	// TODO: Rewrite this to be cleaner.
	result := results[0]

	// Get hits count and max score
	totalHits := int64(0)
	var maxScore float64
	if result.Hits.MaxScore != nil {
		maxScore = *result.Hits.MaxScore
	} else {
		maxScore = 0
	}
	for _, result := range results {
		totalHits += result.Hits.TotalHits
		if sortBy == consts.SORT_BY_RELEVANCE {
			if result.Hits.MaxScore != nil {
				maxScore = math.Max(maxScore, *result.Hits.MaxScore)
			}
		}
	}

	result.Hits.Hits = concatenated
	result.Hits.TotalHits = totalHits
	result.Hits.MaxScore = &maxScore

	return result, nil
}

func (e *ESEngine) addHighlights(ret *elastic.SearchResult) error {

	return errors.New("IN PROGRESS")

	hitsByTitle := make(map[string]*elastic.SearchHit)
	for _, h := range ret.Hits.Hits {
		var result es.Result
		if err := json.Unmarshal(*h.Source, &result); err != nil {
			log.Errorf("Cannot unmarshal hit result: %+v", err)
			continue
		}
		hitsByTitle[result.Title] = h
	}

	if len(hitsByTitle) > 0 {
		mssHighlights := e.esc.MultiSearch()
		for title, hit := range hitsByTitle {
			//TBD add to search only if not intent
			mssHighlights.Add(NewResultsSearchRequest(
				SearchRequestOptions{
					resultTypes: consts.ES_SEARCH_RESULT_TYPES,
					index:       hit.Index, //TBC
					query:       Query{ExactTerms: []string{title}, Term: ""},
					sortBy:      consts.SORT_BY_RELEVANCE,
					from:        0,
					size:        1, //TBC
					//preference:       preference, //TBC
					useHighlight:     true,
					partialHighlight: true}))
		}

		beforeHighlightsDoSearch := time.Now()
		mr, err := mssHighlights.Do(context.TODO())
		e.timeTrack(beforeHighlightsDoSearch, "DoSearch.MultisearcHighlightsDo")
		if err != nil {
			return errors.Wrap(err, "ESEngine.DoSearch - Error mssHighlights Do.")
		}

		for _, currentResults := range mr.Responses {
			if currentResults.Error != nil {
				log.Warnf("%+v", currentResults.Error)
				return errors.New(fmt.Sprintf("Failed multi get highlights: %+v", currentResults.Error))
			}
			if haveHits(currentResults) {
				for _, h := range currentResults.Hits.Hits {
					var result es.Result
					if err := json.Unmarshal(*h.Source, &result); err != nil {
						log.Errorf("Cannot unmarshal hit result: %+v", err)
						continue
					}
					for title, _ := range hitsByTitle {
						if result.Title == title {
							hitsByTitle[title] = h
						}
					}
				}
			}
		}
	}
	// TBC what if we have different results with same title???

	//TBD reassign ret
	return nil
}

func (e *ESEngine) timeTrack(start time.Time, operation string) {
	elapsed := time.Since(start)
	e.ExecutionTimeLog[operation] = elapsed
}

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string) (*QueryResult, error) {
	defer e.timeTrack(time.Now(), "DoSearch")

	if err := e.AddIntents(&query, preference); err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error adding intents.")
	}

	multiSearchService := e.esc.MultiSearch()
	multiSearchService.Add(NewResultsSearchRequests(
		SearchRequestOptions{
			resultTypes:      consts.ES_SEARCH_RESULT_TYPES,
			index:            "",
			query:            query,
			sortBy:           sortBy,
			from:             0,
			size:             from + size,
			preference:       preference,
			useHighlight:     false,
			partialHighlight: false})...)

	// Do search.
	beforeDoSearch := time.Now()
	mr, err := multiSearchService.Do(context.TODO())
	e.timeTrack(beforeDoSearch, "DoSearch.MultisearchDo")
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error multisearch Do.")
	}

	if len(mr.Responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
	}
	resultsByLang := make(map[string][]*elastic.SearchResult)

	// Responses are ordered by language by index, i.e., for languages [bg, ru, en].
	// We want the first matching language that has at least any result.
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		if haveHits(currentResults) {
			lang := query.LanguageOrder[i]
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], currentResults)
		}
	}

	err, intentResultsMap := e.IntentsToResults(&query)
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error adding intents to results.")
	}
	for lang, intentResults := range intentResultsMap {
		if haveHits(intentResults) {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], intentResults)
		}
	}

	results := make([]*elastic.SearchResult, 0)
	for _, lang := range query.LanguageOrder {
		if r, ok := resultsByLang[lang]; ok {
			results = r
			break
		}
	}

	ret, err := joinResponses(sortBy, from, size, results...)

	if ret != nil && ret.Hits != nil && ret.Hits.Hits != nil {

		err = e.addHighlights(ret)
		if err != nil {
			// TBD
		}

		return &QueryResult{ret, query.Intents}, err
	}

	if len(mr.Responses) > 0 {
		// This happens when there are no responses with hits.
		// Note, we don't filter here intents by language.
		return &QueryResult{mr.Responses[0], query.Intents}, err
	} else {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - No responses from multi search.")
	}
}
