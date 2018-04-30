package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
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

var classTypes = [...]string{consts.SOURCE_CLASSIFICATION_TYPE, consts.TAG_CLASSIFICATION_TYPE}

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
						log.Warnf("ESEngine.GetSuggestions - %s: ctx cancelled", classType)
						return nil
					}
				}
				return errors.Wrapf(err, "ESEngine.GetSuggestions - %s", classType)
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
		return nil, errors.Wrap(err, "ESEngine.GetSuggestions - ")
	}

	return resp, nil
}

func createSourcesIntentQuery(q Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	if q.Term != "" {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewMatchQuery("name", q.Term),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", q.Term).Slop(100),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewMatchPhraseQuery("name", exactTerm),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", exactTerm).Slop(100),
			),
		)
	}
	return boolQuery
}

func createTagsIntentQuery(q Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	if q.Term != "" {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewMatchQuery("name", q.Term),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", q.Term).Slop(100),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewMatchPhraseQuery("name", exactTerm),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("name", exactTerm).Slop(100),
			),
		)
	}
	return boolQuery
}

func TagsIntentRequest(query Query, language string, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "name")
	searchSource := elastic.NewSearchSource().
		Query(createTagsIntentQuery(query)).
		FetchSourceContext(fetchSourceContext).
		Explain(query.Deb)
	return elastic.NewSearchRequest().
		SearchSource(searchSource).
		Index(es.IndexName("prod", consts.ES_CLASSIFICATIONS_INDEX, language)).
		Type(consts.TAGS_INDEX_TYPE).
		Preference(preference)
}

type IntentRequestFunc func(query Query, language string, preference string) *elastic.SearchRequest

func SourcesIntentRequest(query Query, language string, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "name")
	searchSource := elastic.NewSearchSource().
		Query(createSourcesIntentQuery(query)).
		FetchSourceContext(fetchSourceContext).
		Explain(query.Deb)
	return elastic.NewSearchRequest().
		SearchSource(searchSource).
		Index(es.IndexName("prod", consts.ES_CLASSIFICATIONS_INDEX, language)).
		Type(consts.SOURCES_INDEX_TYPE).
		Preference(preference)
}

func (e *ESEngine) AddClassificationIntentSecondRound(h *elastic.SearchHit, intent Intent, query Query) (error, *Intent, *Query) {
	var classificationIntent es.ClassificationIntent
	if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
		return err, nil, nil
	}
	if query.Deb {
		classificationIntent.Explanation = *h.Explanation
	}
	if h.Score != nil && *h.Score > 0 {
		classificationIntent.Score = h.Score
		// Search for specific classification by full name to evaluate max score.
		query.Term = ""
		query.ExactTerms = []string{classificationIntent.Name}
		intent.Value = classificationIntent
		return nil, &intent, &query
	}
	return nil, nil, nil
}

func (e *ESEngine) AddIntents(query *Query, preference string) error {
    // Dark launch intents, only if query.Deb is true.
    // Remove query.Deb check when intents quality is good.
	if len(query.Term) == 0 && len(query.ExactTerms) == 0 || !query.Deb {
		return nil
	}
	mssFirstRound := e.esc.MultiSearch()
	potentialIntents := make([]Intent, 0)
	for _, language := range query.LanguageOrder {
		// Order here provides the priority in results, i.e., tags are more importnt then sources.
		mssFirstRound.Add(TagsIntentRequest(*query, language, preference))
		potentialIntents = append(potentialIntents, Intent{I_TAG, language, nil})
		mssFirstRound.Add(SourcesIntentRequest(*query, language, preference))
		potentialIntents = append(potentialIntents, Intent{I_SOURCE, language, nil})
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
		var intentRequestFunc IntentRequestFunc
		switch potentialIntents[i].Type {
		case I_SOURCE:
			intentRequestFunc = SourcesIntentRequest
		case I_TAG:
			intentRequestFunc = TagsIntentRequest
		default:
			log.Errorf("ESEngine.AddIntents - First round bad type: %+v", potentialIntents[i])
			continue
		}
		if haveHits(res) {
			for _, h := range res.Hits.Hits {
				err, intent, secondRoundQuery := e.AddClassificationIntentSecondRound(h, potentialIntents[i], *query)
				if err != nil {
					return errors.Wrapf(err, "ESEngine.AddIntents - Error second run for intent %+v", potentialIntents[i])
				}
				if intent != nil {
					mssSecondRound.Add(intentRequestFunc(*secondRoundQuery, intent.Language, preference))
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
		intentValue := finalIntents[i].Value.(es.ClassificationIntent)
		if haveHits(res) {
			found := false
			for _, h := range res.Hits.Hits {
				var classificationIntent es.ClassificationIntent
				if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
					return err
				}
				if query.Deb {
					intentValue.MaxExplanation = *h.Explanation
				}
				if intentValue.MDB_UID == classificationIntent.MDB_UID {
					found = true
					if h.Score != nil && *h.Score > 0 {
						intentValue.MaxScore = h.Score
						if *intentValue.Score / *intentValue.MaxScore >= 0.2 {
							if *intentValue.MaxScore < *intentValue.Score {
								log.Warnf("ESEngine.AddIntents - Not expected score %f to be larger then max score %f for %s - %s.",
									*intentValue.Score, *intentValue.MaxScore, intentValue.MDB_UID, intentValue.Name)
							}
							query.Intents = append(query.Intents, Intent{finalIntents[i].Type, finalIntents[i].Language, intentValue})
						}
					}
				}
			}
			if !found {
				log.Warnf("ESEngine.AddIntents - Did not find matching second run: %s - %s.",
					intentValue.MDB_UID, intentValue.Name)
			}
		}
	}
	return nil
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

func AddContentUnitsSearchRequests(mss *elastic.MultiSearchService, query Query, sortBy string, from int, size int, preference string) {
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
		mss.Add(request)
	}
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

func AddCollectionsSearchRequests(mss *elastic.MultiSearchService, query Query, sortBy string, from int, size int, preference string) {
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
		mss.Add(request)
	}
}

func haveHits(r *elastic.SearchResult) bool {
	return r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0
}

func compareHits(h1 *elastic.SearchHit, h2 *elastic.SearchHit, sortBy string) (bool, error) {
	if sortBy == consts.SORT_BY_RELEVANCE {
		return *(h1.Score) >= *(h2.Score), nil
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
			return !ed1.EffectiveDate.Time.After(ed2.EffectiveDate.Time), nil
		} else {
			return ed1.EffectiveDate.Time.After(ed2.EffectiveDate.Time), nil
		}
	}
}

func joinResponses(r1 *elastic.SearchResult, r2 *elastic.SearchResult, sortBy string, from int, size int) (*elastic.SearchResult, error) {
	if r1.Hits.TotalHits == 0 {
		r2.Hits.Hits = r2.Hits.Hits[from:utils.Min(from+size, len(r2.Hits.Hits))]
		return r2, nil
	} else if r2.Hits.TotalHits == 0 {
		r1.Hits.Hits = r1.Hits.Hits[from:utils.Min(from+size, len(r1.Hits.Hits))]
		return r1, nil
	}
	result := elastic.SearchResult(*r1)
	result.Hits.TotalHits += r2.Hits.TotalHits
	if sortBy == consts.SORT_BY_RELEVANCE {
		result.Hits.MaxScore = new(float64)
		*result.Hits.MaxScore = math.Max(*result.Hits.MaxScore, *r2.Hits.MaxScore)
	}
	var hits []*elastic.SearchHit

	// Merge using compareHits
	i1, i2 := int(0), int(0)
	for i1 < len(r1.Hits.Hits) || i2 < len(r2.Hits.Hits) {
		if i1 == len(r1.Hits.Hits) {
			hits = append(hits, r2.Hits.Hits[i2:]...)
			break
		}
		if i2 == len(r2.Hits.Hits) {
			hits = append(hits, r1.Hits.Hits[i1:]...)
			break
		}
		h1Larger, err := compareHits(r1.Hits.Hits[i1], r2.Hits.Hits[i2], sortBy)
		if err != nil {
			return nil, err
		}
		if h1Larger {
			hits = append(hits, r1.Hits.Hits[i1])
			i1++
		} else {
			hits = append(hits, r2.Hits.Hits[i2])
			i2++
		}
	}

	result.Hits.Hits = hits[from:utils.Min(from+size, len(hits))]
	return &result, nil
}

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string) (*QueryResult, error) {
	if err := e.AddIntents(&query, preference); err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error adding intents.")
	}
	log.Infof("ESEngine.DoSearch - Query: %+v sort by: %s, from: %d, size: %d", query, sortBy, from, size)
	multiSearchService := e.esc.MultiSearch()
	AddContentUnitsSearchRequests(multiSearchService, query, sortBy, 0, from+size, preference)
	AddCollectionsSearchRequests(multiSearchService, query, sortBy, 0, from+size, preference)

	// Do search.
	mr, err := multiSearchService.Do(context.TODO())
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error multisearch Do.")
	}

	if len(mr.Responses) != 2*len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("ESEngine.DoSearch - Unexpected number of results %d, expected %d",
			len(mr.Responses), 2*len(query.LanguageOrder)))
	}

	// Interleave content units and collection results by language.
	// Then go over responses and choose first not empty retults list.
	for i := 0; i < len(query.LanguageOrder); i++ {
		cuR := mr.Responses[i]
		cR := mr.Responses[i+len(query.LanguageOrder)]
		if cuR.Error != nil {
			log.Warnf("ESEngine.DoSearch - %+v", cuR.Error)
			return nil, errors.New("ESEngine.DoSearch - Failed multi get (CU).")
		}
		if cR.Error != nil {
			log.Warnf("ESEngine.DoSearch - %+v", cR.Error)
			return nil, errors.New("ESEngine.DoSearch - Failed multi get (C).")
		}
		if haveHits(cuR) || haveHits(cR) {
			ret, err := joinResponses(cuR, cR, sortBy, from, size)
			// Filter intents by language
			langIntents := make([]Intent, 0)
			for _, intent := range query.Intents {
				if intent.Language == query.LanguageOrder[i] {
					langIntents = append(langIntents, intent)
				}
			}
			return &QueryResult{ret, langIntents}, errors.Wrap(err, "ESEngine.DoSearch - Failed joinResponses.")
		}
	}

	if len(mr.Responses) > 0 {
		// This happens when there are no responses with hits.
		// Note, we don't filter here intents by language.
		return &QueryResult{mr.Responses[0], query.Intents}, nil
	} else {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - No responses from multi search.")
	}
}
