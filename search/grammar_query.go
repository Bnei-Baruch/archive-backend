package search

import (
	"fmt"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	GRAMMAR_BOOST = 100.0

	GRAMMAR_SUGGEST_SIZE = 30

	GRAMMAR_SEARCH_SIZE = 2000

	GRAMMAR_PERCULATE_SIZE = 5

	PERCULATE_HIGHLIGHT_SEPERATOR = '$'
)

func createGrammarQuery(q *Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	termValue := fmt.Sprintf("%c %v %c", consts.GRAMMAR_INDEX_RULE_WRAPPER, simpleQuery(q), consts.GRAMMAR_INDEX_RULE_WRAPPER)

	if simpleQuery(q) != "" {
		boolQuery = boolQuery.Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("grammar_rule.rules.language", termValue).Slop(SLOP).Boost(GRAMMAR_BOOST),
				elastic.NewMatchPhraseQuery("grammar_rule.rules", termValue).Slop(SLOP).Boost(GRAMMAR_BOOST),
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

func NewSuggestGammarV2Request(query *Query, language string, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("grammar_rule.intent", "grammar_rule.variables", "grammar_rule.values", "grammar_rule.rules")
	source := elastic.NewSearchSource().
		Query(createGrammarQuery(query)).
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

func NewFilteredResultsSearchRequest(text string, contentType string, sources []string, from int, size int, sortBy string, resultTypes []string, language string, preference string, deb bool) ([]*elastic.SearchRequest, error) {
	if contentType == "" && len(sources) == 0 {
		return nil, fmt.Errorf("No contentType and sources provided for NewFilteredResultsSearchRequest().")
	}
	filters := map[string][]string{}
	if contentType != "" {
		var ok bool
		if filters, ok = consts.CT_VARIABLE_TO_FILTER_VALUES[contentType]; !ok {
			return nil, fmt.Errorf("Content type '%s' is not found in CT_VARIABLE_TO_FILTER_VALUES.", contentType)
		}
	}
	if len(sources) > 0 {
		filters[consts.FILTERS[consts.FILTERS[consts.FILTER_SOURCE]]] = sources
	}
	requests := []*elastic.SearchRequest{}
	if val, ok := filters[consts.FILTERS[consts.FILTER_SECTION_SOURCES]]; ok || len(sources) > 0 {
		sourceOnlyFilter := map[string][]string{
			consts.FILTERS[consts.FILTER_SECTION_SOURCES]: val,
		}
		if len(sources) > 0 {
			sourceOnlyFilter[consts.FILTERS[consts.FILTERS[consts.FILTER_SOURCE]]] = sources
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

	filtersWithoutSource := map[string][]string{}
	for key, value := range filters {
		if key != consts.FILTERS[consts.FILTER_SECTION_SOURCES] {
			filtersWithoutSource[key] = value
		}
	}
	if len(filtersWithoutSource) > 0 {
		nonSourceRequests, err := NewResultsSearchRequests(
			SearchRequestOptions{
				resultTypes:                      resultTypes,
				index:                            "",
				query:                            Query{Term: text, Filters: filtersWithoutSource, LanguageOrder: []string{language}, Deb: deb},
				sortBy:                           sortBy,
				from:                             0,
				size:                             from + size,
				preference:                       preference,
				useHighlight:                     false,
				partialHighlight:                 false,
				filterOutCUSources:               []string{},
				includeTypedUidsFromContentUnits: true})
		if err != nil {
			return nil, err
		}
		requests = append(requests, nonSourceRequests...)
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
