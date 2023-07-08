package search

import (
	"fmt"
	"sort"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	GRAMMAR_BOOST = 100.0

	GRAMMAR_BOOST_KEYWORD = 300.0

	GRAMMAR_SUGGEST_SIZE = 30

	GRAMMAR_SEARCH_SIZE = 2000

	GRAMMAR_PERCULATE_SIZE = 5

	PERCULATE_HIGHLIGHT_SEPERATOR = '$'
)

func createGrammarQueryOfSpecificHitType(q *Query, hitType string) elastic.Query {
	boolQuery := elastic.NewBoolQuery().Must(
		elastic.NewMatchQuery("grammar_rule.hit_type", hitType),
		createGrammarQuery(q),
	)
	return boolQuery
}

func createGrammarQuery(q *Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	if simpleQuery(q) != "" {
		boolQuery = boolQuery.Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("grammar_rule.rules.keyword", simpleQuery(q)).Slop(SLOP).Boost(GRAMMAR_BOOST_KEYWORD),
				elastic.NewMatchPhraseQuery("grammar_rule.rules.language", simpleQuery(q)).Slop(SLOP).Boost(GRAMMAR_BOOST),
				elastic.NewMatchPhraseQuery("grammar_rule.rules", simpleQuery(q)).Slop(SLOP).Boost(GRAMMAR_BOOST),
			),
		)
	}
	return boolQuery
}

func createPerculateQuery(q *Query) elastic.Query {
	if len(q.ExactTerms) > 0 { // TBD consider support for query with partly exact term
		return elastic.NewMatchNoneQuery()
	}
	query := elastic.NewPercolatorQuery().Field("query").Document(
		struct {
			SearchText string `json:"search_text"`
		}{SearchText: simpleQuery(q)})
	return query
}

func NewGammarPerculateRequest(query *Query, language string, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("grammar_rule.intent", "grammar_rule.variables", "grammar_rule.values", "grammar_rule.rules")
	source := elastic.NewSearchSource().
		Query(createPerculateQuery(query)).
		Highlight(elastic.NewHighlight().Field("search_text").
			PreTags(string(PERCULATE_HIGHLIGHT_SEPERATOR)).
			PostTags(string(PERCULATE_HIGHLIGHT_SEPERATOR))).
		FetchSourceContext(fetchSourceContext).
		Size(GRAMMAR_PERCULATE_SIZE).
		Explain(query.Deb)
	return elastic.NewSearchRequest().
		SearchSource(source).
		Index(GrammarIndexNameForServing(language)).
		Preference(preference)
}

func NewSuggestGammarV2Request(query *Query, language string, preference string, hitType *string) *elastic.SearchRequest {
	var grammarQuery elastic.Query
	if hitType != nil {
		grammarQuery = createGrammarQueryOfSpecificHitType(query, *hitType)
	} else {
		grammarQuery = createGrammarQuery(query)
	}
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("grammar_rule.intent", "grammar_rule.variables", "grammar_rule.values", "grammar_rule.rules")
	source := elastic.NewSearchSource().
		Query(grammarQuery).
		FetchSourceContext(fetchSourceContext).
		Size(GRAMMAR_SEARCH_SIZE).
		Explain(query.Deb)
	return elastic.NewSearchRequest().
		SearchSource(source).
		Index(GrammarIndexNameForServing(language)).
		Preference(preference)
}

func NewResultsSuggestGrammarV2CompletionRequest(query *Query, language string, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("grammar_rule.intent", "grammar_rule.variables", "grammar_rule.values", "grammar_rule.rules")
	source := elastic.NewSearchSource().
		FetchSourceContext(fetchSourceContext).
		Suggester(
			elastic.NewCompletionSuggester("rules_suggest").
				Field("grammar_rule.rules_suggest").
				Text(simpleQuery(query)).
				Size(GRAMMAR_SUGGEST_SIZE).
				SkipDuplicates(true)).
		Suggester(
			elastic.NewCompletionSuggester("rules_suggest.language").
				Field("grammar_rule.rules_suggest.language").
				Text(simpleQuery(query)).
				Size(GRAMMAR_SUGGEST_SIZE).
				SkipDuplicates(true))

	return elastic.NewSearchRequest().
		SearchSource(source).
		Index(GrammarIndexNameForServing(language)).
		Preference(preference)
}

func NewFilteredResultsSearchRequest(text string, filters map[string][]string, contentType string, programCollection string, sources []string, from int, size int, sortBy string, resultTypes []string, language string, preference string, deb bool) ([]*elastic.SearchRequest, error) {
	// THOSE CONSTRAINTS ARE NO LONGER TRUE...
	if contentType == "" && programCollection == "" && len(sources) == 0 {
		return nil, fmt.Errorf("No contentType or programCollection or sources provided for NewFilteredResultsSearchRequest().")
	}
	if contentType != "" && len(sources) > 0 {
		return nil, fmt.Errorf("Filter by source and content type combination is not currently supported.")
	}
	var searchSources bool
	filtersCopy := map[string][]string{}
	for k, v := range filters {
		filtersCopy[k] = v
	}
	filters = filtersCopy // Reassign pointer to a copy in order to keep the original query filters
	isSectionSources := utils.StringInSlice(consts.CT_SOURCE, filters[consts.FILTER_CONTENT_TYPE])
	if contentType != "" || len(sources) > 0 {
		searchSources = len(filters) == 0 || isSectionSources // Search for sources only on the main section (without filters) or on the sources section.
		if contentType != "" {
			// by content type filter
			if len(filters) > 0 && !hasCommonFilter(filters, consts.CT_VARIABLE_TO_FILTER_VALUES[contentType]) {
				return nil, fmt.Errorf("No common query filters with filters by content type operation.")
			}
			filters, _ = consts.CT_VARIABLE_TO_FILTER_VALUES[contentType] // We override the given query filters. Consider merging filters.
			_, enableSourcesSearch := consts.CT_VARIABLES_ENABLE_SOURCES_SEARCH[contentType]
			if len(filters) == 0 && !enableSourcesSearch {
				return nil, fmt.Errorf("Content type '%s' is not found in CT_VARIABLE_TO_FILTER_VALUES and not in CT_VARIABLES_ENABLE_SOURCES_SEARCH.", contentType)
			}
			searchSources = searchSources && enableSourcesSearch
		}
		if len(sources) > 0 {
			// by source filter
			filters[consts.FILTER_SOURCE] = sources
		}
	}
	if programCollection != "" {
		// by program
		filters[consts.FILTER_COLLECTION] = []string{programCollection}
	}
	requests := []*elastic.SearchRequest{}
	if searchSources {
		sourceOnlyFilter := map[string][]string{consts.FILTER_CONTENT_TYPE: []string{consts.CT_SOURCE}}
		if len(sources) > 0 {
			sourceOnlyFilter[consts.FILTER_SOURCE] = sources
		}
		titlesOnly := contentType == consts.VAR_CT_BOOK_TITLES
		sourceRequests, err := NewResultsSearchRequests(
			SearchRequestOptions{
				resultTypes:        []string{consts.ES_RESULT_TYPE_SOURCES},
				index:              "",
				query:              Query{Term: text, Filters: sourceOnlyFilter, LanguageOrder: []string{language}, Deb: deb},
				sortBy:             sortBy,
				from:               0,
				size:               from + size,
				preference:         preference,
				useHighlight:       false,
				partialHighlight:   false,
				filterOutCUSources: []string{},
				titlesOnly:         titlesOnly})
		if err != nil {
			return nil, err
		}
		requests = append(requests, sourceRequests...)
	}
	if !isSectionSources {
		if len(filters) > 0 {
			nonSourceRequests, err := NewResultsSearchRequests(
				SearchRequestOptions{
					resultTypes:        resultTypes,
					index:              "",
					query:              Query{Term: text, Filters: filters, LanguageOrder: []string{language}, Deb: deb},
					sortBy:             sortBy,
					from:               0,
					size:               from + size,
					preference:         preference,
					useHighlight:       false,
					partialHighlight:   false,
					filterOutCUSources: []string{},
					filterOutCUTypes:   consts.ES_CONTENT_UNIT_TYPES_TO_FILTER_IN_MAIN_SEARCH})
			if err != nil {
				return nil, err
			}
			requests = append(requests, nonSourceRequests...)
		}
	}

	return requests, nil
}

func wordToHist(word string) map[rune]int {
	ret := make(map[rune]int)
	for _, r := range word {
		ret[r]++
	}
	return ret
}

func simpleQuery(q *Query) string {
	if q.Term == "" && len(q.ExactTerms) == 1 {
		return q.ExactTerms[0]
	}
	return q.Term
}

func cmpWordHist(a, b map[rune]int) float64 {
	common := 0
	diff := 0
	for r, countA := range a {
		if countB, ok := b[r]; ok {
			min, max := utils.MinMax(countA, countB)
			common += min
			diff += max - min
		} else {
			diff += countA
		}
	}
	for r, countB := range b {
		if _, ok := a[r]; !ok {
			diff += countB
		}
	}
	return float64(common) / float64(common+diff)
}

func chooseRule(query *Query, rules []string) string {
	if len(rules) == 0 {
		return ""
	}
	queryHist := wordToHist(simpleQuery(query))
	max := float64(0)
	maxIndex := 0
	for i := range rules {
		ruleHist := wordToHist(rules[i])
		if cur := cmpWordHist(queryHist, ruleHist); cur > max {
			max = cur
			maxIndex = i
		}
	}
	return rules[maxIndex]
}

func hasCommonFilter(a map[string][]string, b map[string][]string) bool {
	for filterName, values := range a {
		if grammarValues, ok := b[filterName]; ok {
			sort.Strings(values)
			sort.Strings(grammarValues)
			if len(utils.IntersectSortedStringSlices(values, grammarValues)) > 0 {
				return true
			}
		}
	}
	return false
}
