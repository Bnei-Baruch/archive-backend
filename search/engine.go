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
	"github.com/golang/sync/errgroup"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ESEngine struct {
	esc *elastic.Client
	// TODO: Is mdb required here?
	mdb *sql.DB
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

var classTypes = [...]string{"source", "tag"}

// TODO: All interactions with ES should be throttled to prevent downstream pressure

func NewESEngine(esc *elastic.Client, db *sql.DB) *ESEngine {
	return &ESEngine{esc: esc, mdb: db}
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

func (e *ESEngine) GetSuggestions(ctx context.Context, query Query) (interface{}, error) {
	// Figure out index names from language order.
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexName("prod", consts.ES_CLASSIFICATIONS_INDEX, query.LanguageOrder[i])
	}

	// We call ES in parallel. Each call with a different context query
	// (classification type), i.e, tag, source, author...
	g, ctx := errgroup.WithContext(ctx)
	resp := make([]*elastic.SearchResult, 0)
	for i := range classTypes {
		classType := classTypes[i]
		g.Go(func() error {

			// Create MultiSearch request
			multiSearchService := e.esc.MultiSearch()
			for _, index := range indices {
				searchSource := elastic.NewSearchSource().
					Suggester(elastic.NewCompletionSuggester("classification_name").
						Field("name_suggest").
						Text(query.Term).
						ContextQuery(elastic.NewSuggesterCategoryQuery("classification", classType))).
					Suggester(elastic.NewCompletionSuggester("classification_description").
						Field("description_suggest").
						Text(query.Term).
						ContextQuery(elastic.NewSuggesterCategoryQuery("classification", classType)))

				request := elastic.NewSearchRequest().
					SearchSource(searchSource).
					Index(index)
				multiSearchService.Add(request)
			}

			// Actual call to elastic
			mr, err := multiSearchService.Do(ctx)
			if err != nil {
				// don't kill entire request if ctx was cancelled
				if ue, ok := err.(*url.Error); ok {
					if ue.Err == context.DeadlineExceeded || ue.Err == context.Canceled {
						log.Warnf("ES suggestions %s: ctx cancelled", classType)
						return nil
					}
				}
				return errors.Wrapf(err, "ES suggestions %s", classType)
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

			resp = append(resp, sRes)

			return nil
		})
	}

	// Wait for first deadly error or all goroutines to finish
	if err := g.Wait(); err != nil {
		return nil, errors.Wrap(err, "ES error")
	}

	return resp, nil
}

func createContentUnitsQuery(q Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	if q.Term != "" {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchQuery("name.analyzed", q.Term),
					elastic.NewMatchQuery("description.analyzed", q.Term),
					elastic.NewMatchQuery("transcript.analyzed", q.Term),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100).Boost(2.0),
				elastic.NewMatchPhraseQuery("description.analyzed", q.Term).Slop(100).Boost(1.2),
				elastic.NewMatchPhraseQuery("transcript.analyzed", q.Term).Slop(100),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchPhraseQuery("name", exactTerm),
					elastic.NewMatchPhraseQuery("description", exactTerm),
					elastic.NewMatchPhraseQuery("transcript", exactTerm),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", exactTerm).Slop(100).Boost(2.0),
				elastic.NewMatchPhraseQuery("description", exactTerm).Slop(100).Boost(1.2),
				elastic.NewMatchPhraseQuery("transcript", exactTerm).Slop(100),
			),
		)
	}
	contentTypeQuery := elastic.NewBoolQuery().MinimumNumberShouldMatch(1)
	filterByContentType := false
	for filter, values := range q.Filters {
		s := make([]interface{}, len(values))
		for i, v := range values {
			s[i] = v
		}
		switch filter {
		case consts.FILTERS[consts.FILTER_START_DATE]:
			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Gte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_END_DATE]:
			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Lte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES], consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]:
			contentTypeQuery.Should(elastic.NewTermsQuery(filter, s...))
			filterByContentType = true
		default:
			boolQuery.Filter(elastic.NewTermsQuery(filter, s...))
		}
		if filterByContentType {
			boolQuery.Filter(contentTypeQuery)
		}
	}
	var query elastic.Query
	query = boolQuery
	if q.Term == "" && len(q.ExactTerms) == 0 {
		// No potential score from string matching.
		query = elastic.NewConstantScoreQuery(boolQuery).Boost(1.0)
	}
	return elastic.NewFunctionScoreQuery().Query(query).ScoreMode("sum").MaxBoost(100.0).
		AddScoreFunc(elastic.NewWeightFactorFunction(3.0)).
		AddScoreFunc(elastic.NewGaussDecayFunction().FieldName("effective_date").Decay(0.9).Scale("300d"))
}

func GetContentUnitsSearchRequests(query Query, sortBy string, from int, size int, preference string) []*elastic.SearchRequest {
	requests := make([]*elastic.SearchRequest, 0)
	content_units_indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		content_units_indices[i] = es.IndexName("prod", consts.ES_UNITS_INDEX, query.LanguageOrder[i])
	}
	fetchSourceContext := elastic.NewFetchSourceContext(true).
		Include("mdb_uid", "effective_date")
	for _, index := range content_units_indices {
		searchSource := elastic.NewSearchSource().
			Query(createContentUnitsQuery(query)).
			Highlight(elastic.NewHighlight().HighlighterType("unified").Fields(
				elastic.NewHighlighterField("name").NumOfFragments(0),
				elastic.NewHighlighterField("description"),
				elastic.NewHighlighterField("transcript"),
				elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
				elastic.NewHighlighterField("description.analyzed"),
				elastic.NewHighlighterField("transcript.analyzed"),
			)).
			FetchSourceContext(fetchSourceContext).
			From(from).
			Size(size).
			Explain(query.Deb)
		switch sortBy {
		case consts.SORT_BY_OLDER_TO_NEWER:
			searchSource = searchSource.Sort("effective_date", true)
		case consts.SORT_BY_NEWER_TO_OLDER:
			searchSource = searchSource.Sort("effective_date", false)
		}
		request := elastic.NewSearchRequest().
			SearchSource(searchSource).
			Index(index).
			Preference(preference)
		requests = append(requests, request)
	}
	return requests
}

func createCollectionsQuery(q Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	if q.Term != "" {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchQuery("name.analyzed", q.Term),
					elastic.NewMatchQuery("description.analyzed", q.Term),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100).Boost(2.0),
				elastic.NewMatchPhraseQuery("description.analyzed", q.Term).Slop(100),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchPhraseQuery("name", exactTerm),
					elastic.NewMatchPhraseQuery("description", exactTerm),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", exactTerm).Slop(100).Boost(2.0),
				elastic.NewMatchPhraseQuery("description", exactTerm).Slop(100),
			),
		)
	}
	contentTypeQuery := elastic.NewBoolQuery().MinimumNumberShouldMatch(1)
	filterByContentType := false
	for filter, values := range q.Filters {
		s := make([]interface{}, len(values))
		for i, v := range values {
			s[i] = v
		}
		switch filter {
		case consts.FILTERS[consts.FILTER_START_DATE]:
			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Gte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_END_DATE]:
			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Lte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]:
			// Skip, do nothing (filtring on content units).
		case consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]:
			contentTypeQuery.Should(elastic.NewTermsQuery("content_type", s...))
			filterByContentType = true
		default:
			boolQuery.Filter(elastic.NewTermsQuery(filter, s...))
		}
		if filterByContentType {
			boolQuery.Filter(contentTypeQuery)
		}
	}
	var query elastic.Query
	query = boolQuery
	if q.Term == "" && len(q.ExactTerms) == 0 {
		// No potential score from string matching.
		query = elastic.NewConstantScoreQuery(boolQuery).Boost(1.0)
	}
	return elastic.NewFunctionScoreQuery().Query(query).ScoreMode("sum").MaxBoost(100.0).
		Boost(1.4). // Boost collections index.
		AddScoreFunc(elastic.NewWeightFactorFunction(3.0)).
		AddScoreFunc(elastic.NewGaussDecayFunction().FieldName("effective_date").Decay(0.9).Scale("300d"))
}

func GetCollectionsSearchRequests(query Query, sortBy string, from int, size int, preference string) []*elastic.SearchRequest {
	requests := make([]*elastic.SearchRequest, 0)
	collections_indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		collections_indices[i] = es.IndexName("prod", consts.ES_COLLECTIONS_INDEX, query.LanguageOrder[i])
	}
	fetchSourceContext := elastic.NewFetchSourceContext(true).
		Include("mdb_uid", "effective_date")
	for _, index := range collections_indices {
		searchSource := elastic.NewSearchSource().
			Query(createCollectionsQuery(query)).
			Highlight(elastic.NewHighlight().HighlighterType("unified").Fields(
				elastic.NewHighlighterField("name").NumOfFragments(0),
				elastic.NewHighlighterField("description"),
				elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
				elastic.NewHighlighterField("description.analyzed"),
			)).
			FetchSourceContext(fetchSourceContext).
			From(from).
			Size(size).
			Explain(query.Deb)
		switch sortBy {
		case consts.SORT_BY_OLDER_TO_NEWER:
			searchSource = searchSource.Sort("effective_date", true)
		case consts.SORT_BY_NEWER_TO_OLDER:
			searchSource = searchSource.Sort("effective_date", false)
		}
		request := elastic.NewSearchRequest().
			SearchSource(searchSource).
			Index(index).
			Preference(preference)
		requests = append(requests, request)
	}
	return requests
}

func createSourcesQuery(q Query) elastic.Query {
	query := elastic.NewBoolQuery()
	if q.Term != "" {
		query = query.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchQuery("name.analyzed", q.Term),
					elastic.NewMatchQuery("description.analyzed", q.Term),
					elastic.NewMatchQuery("content.analyzed", q.Term),
					elastic.NewMatchQuery("authors.analyzed", q.Term),
				).MinimumNumberShouldMatch(1)).Boost(1),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name.analyzed", q.Term).Slop(100).Boost(2.0),
				elastic.NewMatchPhraseQuery("description.analyzed", q.Term).Slop(100).Boost(1.2),
				elastic.NewMatchPhraseQuery("content.analyzed", q.Term).Slop(100),
				elastic.NewMatchPhraseQuery("authors.analyzed", q.Term).Slop(100),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		query = query.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchPhraseQuery("name", exactTerm),
					elastic.NewMatchPhraseQuery("description", exactTerm),
					elastic.NewMatchPhraseQuery("content", exactTerm),
					elastic.NewMatchPhraseQuery("authors", exactTerm),
				).MinimumNumberShouldMatch(1)).Boost(1),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", exactTerm).Slop(100).Boost(2.0),
				elastic.NewMatchPhraseQuery("description", exactTerm).Slop(100).Boost(1.2),
				elastic.NewMatchPhraseQuery("content.analyzed", exactTerm).Slop(100),
				elastic.NewMatchPhraseQuery("authors.analyzed", exactTerm).Slop(100),
			),
		)
	}

	for filter, values := range q.Filters {
		if filter == consts.FILTERS[consts.FILTER_SOURCE] {
			s := make([]interface{}, len(values))
			for i, v := range values {
				s[i] = v
			}
			query.Filter(elastic.NewTermsQuery(filter, s...))
		}
	}
	return elastic.NewFunctionScoreQuery().Query(query).ScoreMode("sum").MaxBoost(100.0).
		Boost(1.2). // Boost sources index.
		// No time decay for sources. Sources are above time and space.
		AddScoreFunc(elastic.NewWeightFactorFunction(4.0))
}

func GetSourcesSearchRequests(query Query, from int, size int, preference string) []*elastic.SearchRequest {
	sources_indices := make([]string, len(query.LanguageOrder))
	requests := make([]*elastic.SearchRequest, 0)
	for i := range query.LanguageOrder {
		sources_indices[i] = es.IndexName("prod", consts.ES_SOURCES_INDEX, query.LanguageOrder[i])
	}
	fetchSourceContext := elastic.NewFetchSourceContext(true).
		Include("mdb_uid")
	for _, index := range sources_indices {
		searchSource := elastic.NewSearchSource().
			Query(createSourcesQuery(query)).
			Highlight(elastic.NewHighlight().HighlighterType("unified").Fields(
				elastic.NewHighlighterField("name").NumOfFragments(0),
				elastic.NewHighlighterField("description").NumOfFragments(0),
				elastic.NewHighlighterField("authors").NumOfFragments(0),
				elastic.NewHighlighterField("content"),
				elastic.NewHighlighterField("name.analyzed").NumOfFragments(0),
				elastic.NewHighlighterField("description.analyzed").NumOfFragments(0),
				elastic.NewHighlighterField("authors.analyzed").NumOfFragments(0),
				elastic.NewHighlighterField("content.analyzed"),
			)).
			FetchSourceContext(fetchSourceContext).
			From(from).
			Size(size).
			Explain(query.Deb)
		request := elastic.NewSearchRequest().
			SearchSource(searchSource).
			Index(index).
			Preference(preference)
		requests = append(requests, request)
	}
	return requests
}

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

	// Concatenate all result hits to single slice.
	concatenated := make([]*elastic.SearchHit, 0)
	for _, result := range results {
		concatenated = append(concatenated, result.Hits.Hits...)
	}

	fmt.Printf("Hit Results: ")
	for _, sr := range concatenated {
		fmt.Printf("uid: %s score: %f || ", sr.Uid, *sr.Score)
	}
	fmt.Println()

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

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string) (*elastic.SearchResult, error) {
	log.Infof("Query: %+v sort by: %s, from: %d, size: %d", query, sortBy, from, size)
	multiSearchService := e.esc.MultiSearch()
	requests := make([]*elastic.SearchRequest, 0)
	requestsByIndex := make(map[string][]*elastic.SearchRequest)

	status := consts.NO_FILTER
	if len(query.Filters) > 0 {
		if _, ok := query.Filters[consts.FILTERS[consts.FILTER_SECTION_SOURCES]]; ok {
			status = consts.ONLY_SOURCES
		} else if _, ok := query.Filters[consts.FILTERS[consts.FILTER_SOURCE]]; ok {
			status = consts.NO_FILTER
		} else {
			status = consts.WITHOUT_SOURCES
		}
	}

	if status != consts.ONLY_SOURCES {
		requestsByIndex[consts.ES_UNITS_INDEX] = append(requests, GetContentUnitsSearchRequests(query, sortBy, 0, from+size, preference)...)
		requestsByIndex[consts.ES_COLLECTIONS_INDEX] = append(requests, GetCollectionsSearchRequests(query, sortBy, 0, from+size, preference)...)
	}

	if status != consts.WITHOUT_SOURCES {
		requestsByIndex[consts.ES_SOURCES_INDEX] = append(requests, GetSourcesSearchRequests(query, 0, from+size, preference)...)
	}

	for _, k := range requestsByIndex {
		multiSearchService.Add(k...)
	}

	// Do search.
	mr, err := multiSearchService.Do(context.TODO())

	if err != nil {
		return nil, errors.Wrap(err, "ES error.")
	}

	if len(mr.Responses) != len(requestsByIndex)*len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(requestsByIndex)*len(query.LanguageOrder)))
	}

	var results []*elastic.SearchResult

	for i := 0; i < len(mr.Responses); i += len(query.LanguageOrder) {

		currentResults := mr.Responses[i]
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New("Failed multi get.")
		}

		if haveHits(currentResults) {
			results = append(results, currentResults)
		}
	}

	ret, err := joinResponses(sortBy, from, size, results...)

	if ret != nil && ret.Hits != nil {
		log.Infof("Res: %+v", ret.Hits)
		return ret, err
	}

	if len(mr.Responses) > 0 {
		return mr.Responses[0], err
	} else {
		return nil, errors.Wrap(err, "No responses from multi search.")
	}
}
