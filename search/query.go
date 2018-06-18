package search

import (
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	// Content boost.
	TITLE_BOOST       = 2.0
	DESCRIPTION_BOOST = 1.2

	// Max slop.
	SLOP = 100

	// Following two boosts may be agregated.
	// Boost for standard anylyzer, i.e., without stemming.
	STANDARD_BOOST = 1.2
	// Boost for exact phrase match, without slop.
	EXACT_BOOST = 1.5
)

func createResultsQuery(result_types []string, q Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery().Must(
		elastic.NewConstantScoreQuery(
			elastic.NewTermsQuery("result_type", utils.ConvertArgsString(result_types)...),
		).Boost(0.0),
	)
	if q.Term != "" {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchQuery("title.language", q.Term),
					elastic.NewMatchQuery("description.language", q.Term),
					elastic.NewMatchQuery("content.language", q.Term),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				// Language analyzed
				elastic.NewMatchPhraseQuery("title.language", q.Term).Slop(SLOP).Boost(TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", q.Term).Slop(SLOP).Boost(DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", q.Term).Slop(SLOP),
				// Language analyzed, exact (no slop)
				elastic.NewMatchPhraseQuery("title.language", q.Term).Boost(EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", q.Term).Boost(EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", q.Term).Boost(EXACT_BOOST),
				// Standard analyzed
				elastic.NewMatchPhraseQuery("title", q.Term).Slop(SLOP).Boost(STANDARD_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Slop(SLOP).Boost(STANDARD_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Slop(SLOP).Boost(STANDARD_BOOST),
				// Standard analyzed, exact (no slop).
				elastic.NewMatchPhraseQuery("title", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchPhraseQuery("title", exactTerm),
					elastic.NewMatchPhraseQuery("description", exactTerm),
					elastic.NewMatchPhraseQuery("content", exactTerm),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				// Language analyzed, exact (no slop)
				elastic.NewMatchPhraseQuery("title.language", exactTerm).Boost(EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", exactTerm).Boost(EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", exactTerm).Boost(EXACT_BOOST),
				// Standard analyzed, exact (no slop).
				elastic.NewMatchPhraseQuery("title", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST),
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
		AddScoreFunc(elastic.NewWeightFactorFunction(2.0)).
		AddScoreFunc(elastic.NewGaussDecayFunction().FieldName("effective_date").Decay(0.6).Scale("2000d"))
}

func NewResultsSearchRequest(result_types []string, index string, query Query, sortBy string, from int, size int, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "result_type", "title")
	source := elastic.NewSearchSource().
		Query(createResultsQuery(result_types, query)).
		Highlight(
		elastic.NewHighlight().HighlighterType("unified").Fields(
			elastic.NewHighlighterField("title").NumOfFragments(0),
			elastic.NewHighlighterField("description"),
			elastic.NewHighlighterField("content"),
		),
	).
		FetchSourceContext(fetchSourceContext).
		From(from).
		Size(size).
		Explain(query.Deb)
	switch sortBy {
	case consts.SORT_BY_OLDER_TO_NEWER:
		source = source.Sort("effective_date", true)
	case consts.SORT_BY_NEWER_TO_OLDER:
		source = source.Sort("effective_date", false)
	}
	return elastic.NewSearchRequest().
		SearchSource(source).
		Index(index).
		Preference(preference)
}

func NewResultsSearchRequests(result_types []string, query Query, sortBy string, from int, size int, preference string) []*elastic.SearchRequest {
	requests := make([]*elastic.SearchRequest, 0)
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexName("prod", consts.ES_RESULTS_INDEX, query.LanguageOrder[i])
	}
	for _, index := range indices {
		request := NewResultsSearchRequest(result_types, index, query, sortBy, from, size, preference)
		requests = append(requests, request)
	}
	return requests
}

func NewResultsSuggestRequest(result_types []string, index string, query Query, preference string) *elastic.SearchRequest {
	searchSource := elastic.NewSearchSource().
		Suggester(
		elastic.NewCompletionSuggester("title_suggest").
			Field("title_suggest").
			Text(query.Term).
			ContextQuery(elastic.NewSuggesterCategoryQuery("result_type", result_types...)).
			Size(50000), // Temporary fix. Should not be a problem in v6, should set skip_duplicate = true.
	)

	return elastic.NewSearchRequest().
		SearchSource(searchSource).
		Index(index).
		Preference(preference)
}

func NewResultsSuggestRequests(result_types []string, query Query, preference string) []*elastic.SearchRequest {
	requests := make([]*elastic.SearchRequest, 0)
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexName("prod", consts.ES_RESULTS_INDEX, query.LanguageOrder[i])
	}
	for _, index := range indices {
		request := NewResultsSuggestRequest(result_types, index, query, preference)
		requests = append(requests, request)
	}
	return requests
}
