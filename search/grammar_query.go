package search

import (
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	GRAMMAR_BOOST = 100.0

	GRAMMAR_SUGGEST_SIZE = 30

	GRAMMAR_SEARCH_SIZE = 2000
)

func createGrammarQuery(q *Query) elastic.Query {
	boolQuery := elastic.NewBoolQuery()
	if simpleQuery(q) != "" {
		boolQuery = boolQuery.Should(
			elastic.NewDisMaxQuery().Query(
				elastic.NewMatchPhraseQuery("rules.language", simpleQuery(q)).Slop(SLOP).Boost(GRAMMAR_BOOST),
				elastic.NewMatchPhraseQuery("rules", simpleQuery(q)).Slop(SLOP).Boost(GRAMMAR_BOOST),
			),
		)
	}
	return boolQuery
}

func NewSuggestGammarV2Request(query *Query, language string, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("intent", "variables", "values", "rules")
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
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("intent", "variables", "values", "rules")

	source := elastic.NewSearchSource().
		FetchSourceContext(fetchSourceContext).
		Suggester(
			elastic.NewCompletionSuggester("rules_suggest").
				Field("rules_suggest").
				Text(simpleQuery(query)).
				Size(GRAMMAR_SUGGEST_SIZE).
				Fuzziness(1).
				SkipDuplicates(true)).
		Suggester(
			elastic.NewCompletionSuggester("rules_suggest.language").
				Field("rules_suggest.language").
				Text(simpleQuery(query)).
				Size(GRAMMAR_SUGGEST_SIZE).
				Fuzziness(2).
				SkipDuplicates(true)).
		Explain(query.Deb)

	return elastic.NewSearchRequest().
		SearchSource(source).
		Index(GrammarIndexNameForServing(language)).
		Preference(preference)
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
