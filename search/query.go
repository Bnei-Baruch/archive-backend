package search

import (
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

const (
    // Content boost.
    TITLE_BOOST = 2.0
    DESCRIPTION_BOOST = 1.2

    // Max slop.
    SLOP = 100

    // Following two boosts may be agregated.
    // Boost for standard anylyzer, i.e., without stemming.
    STANDARD_BOOST = 1.2
    // Boost for exact phrase match, without slop.
    EXACT_BOOST = 1.5
)

func createResultsQuery(q Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
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
				elastic.NewMatchPhraseQuery("title.language", q.Term).Boost(EXACT_BOOST * TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", q.Term).Boost(EXACT_BOOST * DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", q.Term).Boost(EXACT_BOOST),
                // Standard analyzed
				elastic.NewMatchPhraseQuery("title", q.Term).Slop(SLOP).Boost(STANDARD_BOOST * TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Slop(SLOP).Boost(STANDARD_BOOST * DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Slop(SLOP).Boost(STANDARD_BOOST),
                // Standard analyzed, exact (no slop).
				elastic.NewMatchPhraseQuery("title", q.Term).Boost(STANDARD_BOOST * EXACT_BOOST * TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Boost(STANDARD_BOOST * EXACT_BOOST * DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Boost(STANDARD_BOOST * EXACT_BOOST),
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
				elastic.NewMatchPhraseQuery("title.language", q.Term).Boost(EXACT_BOOST * TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", q.Term).Boost(EXACT_BOOST * DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", q.Term).Boost(EXACT_BOOST),
                // Standard analyzed, exact (no slop).
				elastic.NewMatchPhraseQuery("title", q.Term).Boost(STANDARD_BOOST * EXACT_BOOST * TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Boost(STANDARD_BOOST * EXACT_BOOST * DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Boost(STANDARD_BOOST * EXACT_BOOST),
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

func ResultsSearchSource(query Query, sortBy string, from int, size int) *elastic.SearchSource {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "result_type")
    searchSource := elastic.NewSearchSource().
        Query(createResultsQuery(query)).
        Highlight(elastic.NewHighlight().HighlighterType("unified").Fields(
        elastic.NewHighlighterField("title").NumOfFragments(0),
        elastic.NewHighlighterField("description"),
        elastic.NewHighlighterField("content"),
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
    return searchSource
}
