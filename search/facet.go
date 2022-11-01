package search

import (
	"fmt"
	"time"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func NewFacetSearchRequest(q Query, options CreateFacetAggregationOptions) (*elastic.SearchRequest, error) {
	index := es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, q.LanguageOrder[0])

	resultQuery, err := createResultsQuery(
		consts.ES_ALL_RESULT_TYPES, q,
		[]string{} /*docIds=*/, []string{} /*filterOutCUSources=*/, false /*titleOnly=*/)
	if err != nil {
		fmt.Printf("Error creating results query: %s", err.Error())
		return nil, err
	}
	source := elastic.NewSearchSource().
		Query(resultQuery).
		FetchSource(false).
		Size(0)

	aggQueries := createFacetAggregationQueries(options)

	for aggName, aggQuery := range aggQueries {
		source.Aggregation(aggName, aggQuery)
	}

	return elastic.NewSearchRequest().SearchSource(source).Index(index), nil
}

func createFacetAggregationQueries(options CreateFacetAggregationOptions) map[string]elastic.Query {
	queries := map[string]elastic.Query{}

	if len(options.tagUIDs) > 0 {
		queries[consts.FILTER_TAG] = createFacetAggregationQuery(options.tagUIDs, consts.FILTER_TAG)
	}

	if len(options.contentTypeValues) > 0 {
		queries[consts.FILTER_CONTENT_TYPE] = createFacetAggregationQuery(
			options.contentTypeValues,
			consts.FILTER_CONTENT_TYPE,
		)
	}

	if len(options.mediaLanguageValues) > 0 {
		queries[consts.FILTER_MEDIA_LANGUAGE] = createFacetAggregationQuery(options.mediaLanguageValues, consts.FILTER_MEDIA_LANGUAGE)
	}

	if len(options.originalLanguageValues) > 0 {
		queries[consts.FILTER_ORIGINAL_LANGUAGE] = createFacetAggregationQuery(options.originalLanguageValues, consts.FILTER_ORIGINAL_LANGUAGE)
	}

	if len(options.sourceUIDs) > 0 {
		queries[consts.FILTER_SOURCE] = createFacetAggregationQuery(options.sourceUIDs, consts.FILTER_SOURCE)
	}

	if len(options.dateRanges) > 0 {
		now := time.Now()
		today := now.Format("2006-01-02")
		agg := elastic.NewFiltersAggregation()
		for _, dateRange := range options.dateRanges {
			to := today
			from := ""
			switch dateRange {
			case consts.DATE_FILTER_TODAY:
				from = today
			case consts.DATE_FILTER_YESTERDAY:
				yesterday := now.Add(-7 * 24 * time.Hour).Format("2006-01-02")
				from = yesterday
				to = yesterday
			case consts.DATE_FILTER_LAST_7_DAYS:
				from = now.Add(-7 * 24 * time.Hour).Format("2006-01-02")
			case consts.DATE_FILTER_LAST_30_DAYS:
				from = now.Add(-30 * 24 * time.Hour).Format("2006-01-02")
			}
			agg.FilterWithName(dateRange, elastic.NewRangeQuery("effective_date").Gte(from).Lte(to).Format("yyyy-MM-dd"))
		}
		queries[consts.AGG_FILTER_DATES] = agg
	}

	if len(options.personUIDs) > 0 {
		queries[consts.FILTER_PERSON] = createFacetAggregationQuery(options.personUIDs, consts.FILTER_PERSON)
	}

	return queries
}

func createFacetAggregationQuery(values []string, filter string) elastic.Query {
	agg := elastic.NewFiltersAggregation()
	for _, value := range values {
		agg.FilterWithName(value, elastic.NewTermQuery(
			"filter_values",
			fmt.Sprintf("%s:%s", filter, value),
		))
	}

	// Handle special cases where we want to aggregate content type for source, blog or tweet.
	// Sources, blogs and Tweets are indexed with a different result type.
	if filter == consts.FILTER_CONTENT_TYPE {
		if utils.StringInSlice(consts.CT_SOURCE, values) {
			agg.FilterWithName(consts.CT_SOURCE, elastic.NewTermsQuery("result_type", consts.ES_RESULT_TYPE_SOURCES))
		}
		if utils.StringInSlice(consts.CT_BLOG_POST, values) {
			agg.FilterWithName(consts.CT_BLOG_POST, elastic.NewTermsQuery("result_type", consts.ES_RESULT_TYPE_BLOG_POSTS))
		}
		if utils.StringInSlice(consts.SCT_TWEET, values) {
			agg.FilterWithName(consts.SCT_TWEET, elastic.NewTermsQuery("result_type", consts.ES_RESULT_TYPE_TWEETS))
		}
	}

	return agg
}
