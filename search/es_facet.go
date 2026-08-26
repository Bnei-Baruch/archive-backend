package search

import (
	"fmt"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// NewESFacetSearchBody builds the ES9 JSON request body for a facet/aggregation search.
// Returns the body map and index name.
func NewESFacetSearchBody(q Query, options CreateFacetAggregationOptions) (map[string]interface{}, string, error) {
	index := es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, q.LanguageOrder[0])

	queryBody, err := esCreateResultsQueryBody(consts.ES_ALL_RESULT_TYPES, q, nil, nil, false)
	if err != nil {
		return nil, "", fmt.Errorf("NewESFacetSearchBody: create results query: %w", err)
	}

	body := map[string]interface{}{
		"query":   queryBody,
		"size":    0,
		"_source": false,
		"aggs":    esFacetAggregations(options),
	}

	return body, index, nil
}

// esFacetAggregations builds the "aggs" map for all requested facet options.
func esFacetAggregations(options CreateFacetAggregationOptions) map[string]interface{} {
	aggs := map[string]interface{}{}

	if len(options.tagUIDs) > 0 {
		aggs[consts.FILTER_TAG] = esFiltersAgg(options.tagUIDs, consts.FILTER_TAG)
	}
	if len(options.contentTypeValues) > 0 {
		aggs[consts.FILTER_CONTENT_TYPE] = esFiltersAgg(options.contentTypeValues, consts.FILTER_CONTENT_TYPE)
	}
	if len(options.mediaLanguageValues) > 0 {
		aggs[consts.FILTER_MEDIA_LANGUAGE] = esFiltersAgg(options.mediaLanguageValues, consts.FILTER_MEDIA_LANGUAGE)
	}
	if len(options.originalLanguageValues) > 0 {
		aggs[consts.FILTER_ORIGINAL_LANGUAGE] = esFiltersAgg(options.originalLanguageValues, consts.FILTER_ORIGINAL_LANGUAGE)
	}
	if len(options.sourceUIDs) > 0 {
		aggs[consts.FILTER_SOURCE] = esFiltersAgg(options.sourceUIDs, consts.FILTER_SOURCE)
	}
	if len(options.dateRanges) > 0 {
		aggs[consts.AGG_FILTER_DATES] = esDateRangeFiltersAgg(options.dateRanges)
	}
	if len(options.personUIDs) > 0 {
		aggs[consts.FILTER_PERSON] = esFiltersAgg(options.personUIDs, consts.FILTER_PERSON)
	}

	return aggs
}

// esFiltersAgg builds a named-filters aggregation for a list of values under a given filter key.
// Mirrors olivere's FiltersAggregation with FilterWithName.
func esFiltersAgg(values []string, filter string) map[string]interface{} {
	filters := map[string]interface{}{}

	for _, value := range values {
		filters[value] = map[string]interface{}{
			"term": map[string]interface{}{
				"filter_values": fmt.Sprintf("%s:%s", filter, value),
			},
		}
	}

	// Sources, blog posts, and tweets are indexed with a dedicated result_type
	// rather than a filter_values entry — mirror the special-casing in facet.go.
	if filter == consts.FILTER_CONTENT_TYPE {
		if utils.StringInSlice(consts.CT_SOURCE, values) {
			filters[consts.CT_SOURCE] = map[string]interface{}{
				"terms": map[string]interface{}{"result_type": []string{consts.ES_RESULT_TYPE_SOURCES}},
			}
		}
		if utils.StringInSlice(consts.CT_BLOG_POST, values) {
			filters[consts.CT_BLOG_POST] = map[string]interface{}{
				"terms": map[string]interface{}{"result_type": []string{consts.ES_RESULT_TYPE_BLOG_POSTS}},
			}
		}
		if utils.StringInSlice(consts.SCT_TWEET, values) {
			filters[consts.SCT_TWEET] = map[string]interface{}{
				"terms": map[string]interface{}{"result_type": []string{consts.ES_RESULT_TYPE_TWEETS}},
			}
		}
	}

	return map[string]interface{}{
		"filters": map[string]interface{}{
			"filters": filters,
		},
	}
}

// esDateRangeFiltersAgg builds a named-filters aggregation for date range buckets.
func esDateRangeFiltersAgg(dateRanges []string) map[string]interface{} {
	now := time.Now()
	today := now.Format("2006-01-02")

	filters := map[string]interface{}{}
	for _, dateRange := range dateRanges {
		from := ""
		to := today
		switch dateRange {
		case consts.DATE_FILTER_TODAY:
			from = today
		case consts.DATE_FILTER_YESTERDAY:
			yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
			from = yesterday
			to = yesterday
		case consts.DATE_FILTER_LAST_7_DAYS:
			from = now.Add(-7 * 24 * time.Hour).Format("2006-01-02")
		case consts.DATE_FILTER_LAST_30_DAYS:
			from = now.Add(-30 * 24 * time.Hour).Format("2006-01-02")
		}
		filters[dateRange] = map[string]interface{}{
			"range": map[string]interface{}{
				"effective_date": map[string]interface{}{
					"gte":    from,
					"lte":    to,
					"format": "yyyy-MM-dd",
				},
			},
		}
	}

	return map[string]interface{}{
		"filters": map[string]interface{}{
			"filters": filters,
		},
	}
}
