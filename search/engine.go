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
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ESEngine struct {
	esc   *elastic.Client
	mdb   *sql.DB
	cache cache.CacheManager
}

type byRelevance []*elastic.SearchHit
type byNewerToOlder []*elastic.SearchHit
type byOlderToNewer []*elastic.SearchHit

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

// var classTypes = [...]string{consts.SOURCE_CLASSIFICATION_TYPE, consts.TAG_CLASSIFICATION_TYPE}

// func debugPrintQuery(q elastic.Query) elastic.Query {
// 	i, err := q.Source()
// 	utils.Must(err)
// 	json, err := json.MarshalIndent(i, "", "  ")
// 	utils.Must(err)
// 	log.Infof("Query:\n%s\n\n", (string(json)))
// 	return q
// }

// TODO: All interactions with ES should be throttled to prevent downstream pressure

func NewESEngine(esc *elastic.Client, db *sql.DB, cache cache.CacheManager) *ESEngine {
	return &ESEngine{esc: esc, mdb: db, cache: cache}
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
	multiSearchService := e.esc.MultiSearch()
	requests := NewResultsSuggestRequests([]string{consts.ES_RESULT_TYPE_TAGS, consts.ES_RESULT_TYPE_SOURCES}, query, preference)
	multiSearchService.Add(requests...)

	// Actual call to elastic
	mr, err := multiSearchService.Do(ctx)
	if err != nil {
		// don't kill entire request if ctx was cancelled
		if ue, ok := err.(*url.Error); ok {
			if ue.Err == context.DeadlineExceeded || ue.Err == context.Canceled {
				log.Warn("ESEngine.GetSuggestions - ctx cancelled")
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

// func createSourcesIntentQuery(q Query) elastic.Query {
// 	boolQuery := elastic.NewBoolQuery()
// 	if q.Term != "" {
// 		boolQuery = boolQuery.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewMatchQuery("name.analyzed", q.Term),
// 			).Boost(0.0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100),
// 			),
// 		)
// 	}
// 	for _, exactTerm := range q.ExactTerms {
// 		boolQuery = boolQuery.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewMatchPhraseQuery("name.analyzed", exactTerm),
// 			).Boost(0.0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", exactTerm).Slop(100),
// 			),
// 		)
// 	}
// 	return elastic.NewFunctionScoreQuery().Query(boolQuery).
// 		Boost(2.0 * 3.0) // Title Boost * Time Boost
// }
//
// func createTagsIntentQuery(q Query) elastic.Query {
// 	boolQuery := elastic.NewBoolQuery()
// 	if q.Term != "" {
// 		boolQuery = boolQuery.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewMatchQuery("name.analyzed", q.Term),
// 			).Boost(0.0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100),
// 			),
// 		)
// 	}
// 	for _, exactTerm := range q.ExactTerms {
// 		boolQuery = boolQuery.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewMatchPhraseQuery("name.analyzed", exactTerm),
// 			).Boost(0.0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", exactTerm).Slop(100),
// 			),
// 		)
// 	}
// 	return elastic.NewFunctionScoreQuery().Query(boolQuery).
// 		Boost(2.0 * 3.0) // Title Boost * Time Boost
// }

// func TagsIntentRequest(query Query, language string, preference string) *elastic.SearchRequest {
// 	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "name")
// 	searchSource := elastic.NewSearchSource().
// 		Query(createTagsIntentQuery(query)).
// 		FetchSourceContext(fetchSourceContext).
// 		Explain(query.Deb)
// 	return elastic.NewSearchRequest().
// 		SearchSource(searchSource).
// 		Index(es.IndexName("prod", consts.ES_CLASSIFICATIONS_INDEX, language)).
// 		Type(consts.TAGS_INDEX_TYPE).
// 		Preference(preference)
// }

// type IntentRequestFunc func(query Query, language string, preference string) *elastic.SearchRequest

// func SourcesIntentRequest(query Query, language string, preference string) *elastic.SearchRequest {
// 	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "name")
// 	searchSource := elastic.NewSearchSource().
// 		Query(createSourcesIntentQuery(query)).
// 		FetchSourceContext(fetchSourceContext).
// 		Explain(query.Deb)
// 	return elastic.NewSearchRequest().
// 		SearchSource(searchSource).
// 		Index(es.IndexName("prod", consts.ES_CLASSIFICATIONS_INDEX, language)).
// 		Type(consts.SOURCES_INDEX_TYPE).
// 		Preference(preference)
// }

func (e *ESEngine) AddIntentSecondRound(h *elastic.SearchHit, intent Intent, query Query) (error, *Intent, *Query) {
	var classificationIntent es.ClassificationIntent
	if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
		return err, nil, nil
	}
	if query.Deb {
		classificationIntent.Explanation = *h.Explanation
	}
	log.Infof("Hit: %+v %+v", *h.Score, classificationIntent)
	if h.Score != nil && *h.Score > 0 {
		classificationIntent.Score = h.Score
		// Search for specific classification by full name to evaluate max score.
		query.Term = ""
		query.ExactTerms = []string{classificationIntent.Title}
		intent.Value = classificationIntent
		return nil, &intent, &query
	}
	return nil, nil, nil
}

func (e *ESEngine) AddIntents(query *Query, preference string) error {
	if len(query.Term) == 0 && len(query.ExactTerms) == 0 {
		return nil
	}

	mssFirstRound := e.esc.MultiSearch()
	potentialIntents := make([]Intent, 0)
	for _, language := range query.LanguageOrder {
		// Order here provides the priority in results, i.e., tags are more importnt then sources.
		index := es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, language)
		mssFirstRound.Add(NewResultsSearchRequest(
			[]string{consts.ES_RESULT_TYPE_TAGS}, index, *query,
			consts.SORT_BY_RELEVANCE, 0, consts.API_DEFAULT_PAGE_SIZE, preference))
		potentialIntents = append(potentialIntents, Intent{consts.INTENT_TYPE_TAG, language, nil})
		// mssFirstRound.Add(SourcesIntentRequest(*query, language, preference))
		// potentialIntents = append(potentialIntents, Intent{consts.INTENT_TYPE_SOURCE, language, nil})
	}
	mr, err := mssFirstRound.Do(context.TODO())
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
		// var intentRequestFunc IntentRequestFunc
		// switch potentialIntents[i].Type {
		// case consts.INTENT_TYPE_SOURCE:
		// 	intentRequestFunc = SourcesIntentRequest
		// case consts.INTENT_TYPE_TAG:
		// 	intentRequestFunc = TagsIntentRequest
		// default:
		// 	log.Errorf("ESEngine.AddIntents - First round bad type: %+v", potentialIntents[i])
		// 	continue
		// }
		if haveHits(res) {
			for _, h := range res.Hits.Hits {
				err, intent, secondRoundQuery := e.AddIntentSecondRound(h, potentialIntents[i], *query)
				log.Infof("Adding second round for %+v %+v %+v", intent, secondRoundQuery, potentialIntents[i])
				if err != nil {
					return errors.Wrapf(err, "ESEngine.AddIntents - Error second run for intent %+v", potentialIntents[i])
				}
				if intent != nil {
					mssSecondRound.Add(NewResultsSearchRequest(
						[]string{consts.RESULT_TYPE_BY_INDEX_TYPE[potentialIntents[i].Type]},
						es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, intent.Language),
						*secondRoundQuery, consts.SORT_BY_RELEVANCE, 0, consts.API_DEFAULT_PAGE_SIZE,
						preference))
					finalIntents = append(finalIntents, *intent)
				}
			}
		}
	}

	mr, err = mssSecondRound.Do(context.TODO())
	for i := 0; i < len(finalIntents); i++ {
		res := mr.Responses[i]
		if res.Error != nil {
			log.Warnf("ESEngine.AddIntents - Second Run %+v", res.Error)
			return errors.New("ESEngine.AddIntents - Second Run Failed multi get (S).")
		}
		intentValue, intentOk := finalIntents[i].Value.(es.ClassificationIntent)
		if !intentOk {
			return errors.New(fmt.Sprintf("ESEngine.AddIntents - Unexpected intent value: %+v", finalIntents[i].Value))
		}
		if haveHits(res) {
			log.Infof("Found Hits for %+v", intentValue)
			found := false
			for _, h := range res.Hits.Hits {
				var classificationIntent es.ClassificationIntent
				if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
					return errors.Wrap(err, "ESEngine.AddIntents - Unmarshal classification intent filed.")
				}
				if query.Deb {
					intentValue.MaxExplanation = *h.Explanation
				}
				if intentValue.MDB_UID == classificationIntent.MDB_UID {
					found = true
					log.Infof("Max Score: %+v", *h.Score)
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
	checkContentUnitsTypes := []string{consts.CT_LESSON_PART, consts.CT_VIDEO_PROGRAM_CHAPTER}
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
	log.Infof("IntentsToResults - %d intents.", len(query.Intents))
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
		log.Infof("Added intent %s %s %s boost score:%f exist:%t", intentValue.Title, intent.Type, intent.Language, boostedScore, intentValue.Exist)
	}
	return nil, srMap
}

// func createCollectionsQuery(q Query) elastic.Query {
// 	boolQuery := elastic.NewBoolQuery()
// 	if q.Term != "" {
// 		boolQuery = boolQuery.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewBoolQuery().Should(
// 					elastic.NewMatchQuery("name.analyzed", q.Term),
// 					elastic.NewMatchQuery("description.analyzed", q.Term),
// 				).MinimumNumberShouldMatch(1),
// 			).Boost(0.0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100).Boost(2.0),
// 				elastic.NewMatchPhraseQuery("description.analyzed", q.Term).Slop(100),
// 			),
// 		)
// 	}
// 	for _, exactTerm := range q.ExactTerms {
// 		boolQuery = boolQuery.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewBoolQuery().Should(
// 					elastic.NewMatchPhraseQuery("name.analyzed", exactTerm),
// 					elastic.NewMatchPhraseQuery("description.analyzed", exactTerm),
// 				).MinimumNumberShouldMatch(1),
// 			).Boost(0.0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", exactTerm).Slop(100).Boost(2.0),
// 				elastic.NewMatchPhraseQuery("description.analyzed", exactTerm).Slop(100),
// 			),
// 		)
// 	}
// 	contentTypeQuery := elastic.NewBoolQuery().MinimumNumberShouldMatch(1)
// 	filterByContentType := false
// 	for filter, values := range q.Filters {
// 		s := make([]interface{}, len(values))
// 		for i, v := range values {
// 			s[i] = v
// 		}
// 		switch filter {
// 		case consts.FILTERS[consts.FILTER_START_DATE]:
// 			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Gte(values[0]).Format("yyyy-MM-dd"))
// 		case consts.FILTERS[consts.FILTER_END_DATE]:
// 			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Lte(values[0]).Format("yyyy-MM-dd"))
// 		case consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]:
// 			// Skip, do nothing (filtring on content units).
// 		case consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]:
// 			contentTypeQuery.Should(elastic.NewTermsQuery("content_type", s...))
// 			filterByContentType = true
// 		default:
// 			boolQuery.Filter(elastic.NewTermsQuery(filter, s...))
// 		}
// 		if filterByContentType {
// 			boolQuery.Filter(contentTypeQuery)
// 		}
// 	}
// 	var query elastic.Query
// 	query = boolQuery
// 	if q.Term == "" && len(q.ExactTerms) == 0 {
// 		// No potential score from string matching.
// 		query = elastic.NewConstantScoreQuery(boolQuery).Boost(1.0)
// 	}
// 	return elastic.NewFunctionScoreQuery().Query(query).ScoreMode("sum").MaxBoost(100.0).
// 		Boost(1.5). // Boost collections index.
// 		AddScoreFunc(elastic.NewWeightFactorFunction(2.0)).
// 		AddScoreFunc(elastic.NewGaussDecayFunction().FieldName("effective_date").Decay(0.6).Scale("2000d"))
// }

// func GetCollectionsSearchRequests(query Query, sortBy string, from int, size int, preference string) []*elastic.SearchRequest {
// 	requests := make([]*elastic.SearchRequest, 0)
// 	collections_indices := make([]string, len(query.LanguageOrder))
// 	for i := range query.LanguageOrder {
// 		collections_indices[i] = es.IndexName("prod", consts.ES_COLLECTIONS_INDEX, query.LanguageOrder[i])
// 	}
// 	fetchSourceContext := elastic.NewFetchSourceContext(true).
// 		Include("mdb_uid", "effective_date")
// 	for _, index := range collections_indices {
// 		searchSource := elastic.NewSearchSource().
// 			Query(createCollectionsQuery(query)).
// 			Highlight(elastic.NewHighlight().HighlighterType("unified").Fields(
// 			elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
// 			elastic.NewHighlighterField("description.analyzed"),
// 			// elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
// 			// elastic.NewHighlighterField("description.analyzed"),
// 		)).
// 			FetchSourceContext(fetchSourceContext).
// 			From(from).
// 			Size(size).
// 			Explain(query.Deb)
// 		switch sortBy {
// 		case consts.SORT_BY_OLDER_TO_NEWER:
// 			searchSource = searchSource.Sort("effective_date", true)
// 		case consts.SORT_BY_NEWER_TO_OLDER:
// 			searchSource = searchSource.Sort("effective_date", false)
// 		}
// 		request := elastic.NewSearchRequest().
// 			SearchSource(searchSource).
// 			Index(index).
// 			Preference(preference)
// 		requests = append(requests, request)
// 	}
// 	return requests
// }

// func createSourcesQuery(q Query) elastic.Query {
// 	query := elastic.NewBoolQuery()
// 	if q.Term != "" {
// 		query = query.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewBoolQuery().Should(
// 					elastic.NewMatchQuery("name.analyzed", q.Term),
// 					elastic.NewMatchQuery("full_name.analyzed", q.Term),
// 					elastic.NewMatchQuery("description.analyzed", q.Term),
// 					elastic.NewMatchQuery("content.analyzed", q.Term),
// 				).MinimumNumberShouldMatch(1),
// 			).Boost(0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100).Boost(2.0),
// 				elastic.NewMatchPhraseQuery("full_name.analyzed", q.Term).Slop(100).Boost(2.0),
// 				elastic.NewMatchPhraseQuery("description.analyzed", q.Term).Slop(100).Boost(1.2),
// 				elastic.NewMatchPhraseQuery("content.analyzed", q.Term).Slop(100),
// 			),
// 		)
// 	}
// 	for _, exactTerm := range q.ExactTerms {
// 		query = query.Must(
// 			// Don't calculate score here, as we use sloped score below.
// 			elastic.NewConstantScoreQuery(
// 				elastic.NewBoolQuery().Should(
// 					elastic.NewMatchPhraseQuery("name.analyzed", exactTerm),
// 					elastic.NewMatchPhraseQuery("full_name.analyzed", exactTerm),
// 					elastic.NewMatchPhraseQuery("description.analyzed", exactTerm),
// 					elastic.NewMatchPhraseQuery("content.analyzed", exactTerm),
// 				).MinimumNumberShouldMatch(1),
// 			).Boost(0),
// 		).Should(
// 			elastic.NewDisMaxQuery().Query(
// 				elastic.NewMatchPhraseQuery("namen.analyzed", exactTerm).Slop(100).Boost(2.0),
// 				elastic.NewMatchPhraseQuery("full_name.analyzed", exactTerm).Slop(100).Boost(2.0),
// 				elastic.NewMatchPhraseQuery("description.analyzed", exactTerm).Slop(100).Boost(1.2),
// 				elastic.NewMatchPhraseQuery("content.analyzed", exactTerm).Slop(100),
// 			),
// 		)
// 	}
//
// 	for filter, values := range q.Filters {
// 		if filter == consts.FILTERS[consts.FILTER_SOURCE] {
// 			s := make([]interface{}, len(values))
// 			for i, v := range values {
// 				s[i] = v
// 			}
// 			query.Filter(elastic.NewTermsQuery(filter, s...))
// 		}
// 	}
// 	return elastic.NewFunctionScoreQuery().Query(query).ScoreMode("sum").MaxBoost(100.0).
// 		Boost(1.3). // Boost sources index.
// 		// No time decay for sources. Sources are above time and space.
// 		AddScoreFunc(elastic.NewWeightFactorFunction(3.0))
// }
//
// func GetSourcesSearchRequests(query Query, from int, size int, preference string) []*elastic.SearchRequest {
// 	sources_indices := make([]string, len(query.LanguageOrder))
// 	requests := make([]*elastic.SearchRequest, 0)
// 	for i := range query.LanguageOrder {
// 		sources_indices[i] = es.IndexName("prod", consts.ES_SOURCES_INDEX, query.LanguageOrder[i])
// 	}
// 	fetchSourceContext := elastic.NewFetchSourceContext(true).
// 		Include("mdb_uid")
// 	for _, index := range sources_indices {
// 		searchSource := elastic.NewSearchSource().
// 			Query(createSourcesQuery(query)).
// 			Highlight(elastic.NewHighlight().HighlighterType("unified").Fields(
// 			elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
// 			elastic.NewHighlighterField("description.analyzed").NumOfFragments(0),
// 			elastic.NewHighlighterField("authors.analyzed").NumOfFragments(0),
// 			elastic.NewHighlighterField("pathnames.analyzed").NumOfFragments(0),
// 			elastic.NewHighlighterField("content.analyzed"),
// 			// elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
// 			// elastic.NewHighlighterField("description.analyzed").NumOfFragments(0),
// 			// elastic.NewHighlighterField("pathnames.analyzed").NumOfFragments(0),
// 			// elastic.NewHighlighterField("content.analyzed"),
// 		)).
// 			FetchSourceContext(fetchSourceContext).
// 			From(from).
// 			Size(size).
// 			Explain(query.Deb)
// 		request := elastic.NewSearchRequest().
// 			SearchSource(searchSource).
// 			Index(index).
// 			Preference(preference)
// 		requests = append(requests, request)
// 	}
// 	return requests
// }

func haveHits(r *elastic.SearchResult) bool {
	return r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0
}

func compareHits(h1 *elastic.SearchHit, h2 *elastic.SearchHit, sortBy string) (bool, error) {
	if sortBy == consts.SORT_BY_RELEVANCE {
		return *(h1.Score) > *(h2.Score), nil
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
			return ed2.EffectiveDate.Time.After(ed1.EffectiveDate.Time), nil
		} else {
			return ed2.EffectiveDate.Time.Before(ed1.EffectiveDate.Time), nil
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

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string) (*QueryResult, error) {
	if err := e.AddIntents(&query, preference); err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error adding intents.")
	}

	log.Infof("ESEngine.DoSearch - Query: %s sort by: %s, from: %d, size: %d", query.ToString(), sortBy, from, size)

	multiSearchService := e.esc.MultiSearch()
	// requestsByIndex := make(map[string][]*elastic.SearchRequest)

	// TODO: Add support for filters for results.
	// searchFilter := consts.SEARCH_NO_FILTER
	// if len(query.Filters) > 0 {
	// 	if _, ok := query.Filters[consts.FILTERS[consts.FILTER_SECTION_SOURCES]]; ok {
	// 		searchFilter = consts.SEARCH_FILTER_ONLY_SOURCES
	// 	} else if _, ok := query.Filters[consts.FILTERS[consts.FILTER_SOURCE]]; ok {
	// 		searchFilter = consts.SEARCH_NO_FILTER
	// 	} else {
	// 		searchFilter = consts.SEARCH_FILTER_WITHOUT_SOURCES
	// 	}
	// }
	//
	// if searchFilter != consts.SEARCH_FILTER_ONLY_SOURCES {
	// 	requestsByIndex[consts.ES_UNITS_INDEX] = GetSearchRequests(query, sortBy, 0, from+size, preference)
	// 	// requestsByIndex[consts.ES_COLLECTIONS_INDEX] = GetCollectionsSearchRequests(query, sortBy, 0, from+size, preference)
	// }
	//
	// // if searchFilter != consts.SEARCH_FILTER_WITHOUT_SOURCES {
	// // 	requestsByIndex[consts.ES_SOURCES_INDEX] = GetSourcesSearchRequests(query, 0, from+size, preference)
	// // }
	//
	// for _, k := range requestsByIndex {
	// 	multiSearchService.Add(k...)
	// }

	multiSearchService.Add(NewResultsSearchRequests(consts.ES_SEARCH_RESULT_TYPES, query, sortBy, 0, from+size, preference)...)

	// Do search.
	mr, err := multiSearchService.Do(context.TODO())
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
	if ret != nil && ret.Hits != nil {
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
