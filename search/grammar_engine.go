package search

import (
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type FilterValue struct {
	Name       string `json:"name,omitempty"`
	Value      string `json:"value,omitempty"`
	Origin     string `json:"origin,omitempty"`
	OriginFull string `json:"origin_full,omitempty"`
}

type GrammarIntent struct {
	LandingPage  string        `json:"landing_page,omitempty"`
	FilterValues []FilterValue `json:"filter_values,omitempty"`
	Score        float64       `json:"score,omitempty"`
}

func (e *ESEngine) SuggestGrammars(query *Query) (map[string][]VariablesByPhrase, error) {
	suggests := make(map[string][]VariablesByPhrase)
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return suggests, nil
	}
	for _, language := range query.LanguageOrder {
		if grammarByIntent, ok := e.grammars[language]; ok {
			for grammarKey, grammar := range grammarByIntent {
				start := time.Now()
				grammarSuggest, err := grammar.SuggestGrammar(query, e.TokensCache, e.cache)
				elapsed := time.Since(start)
				if elapsed > 10*time.Millisecond {
					fmt.Printf("%s-%s - %s\n", language, grammarKey, elapsed.String())
				}
				if err != nil {
					return nil, err
				}
				if len(grammarSuggest) > 0 {
					suggests[language] = append(suggests[language], grammarSuggest)
				}
			}
		}
	}
	return suggests, nil
}

func (g *Grammar) SuggestGrammar(query *Query, tc *TokensCache, cm cache.CacheManager) (VariablesByPhrase, error) {
	simpleQuery := query.Term
	if simpleQuery == "" && len(query.ExactTerms) == 1 {
		simpleQuery = query.ExactTerms[0]
	}
	// TODO: Tokenization is call to elastic. We can do this in parallel for all languages.
	// Consider extracting up the generation of Tokens.
	simpleQueryTokens, err := MakeTokensFromPhrase(simpleQuery, g.Language, g.Esc, tc)
	if err != nil {
		return VariablesByPhrase(nil), errors.Wrapf(err, "Error tokenizing simpleQuery: [%s] in %s.", simpleQuery, g.Language)
	}
	// Should filter variables by phrase here! By fetching from DB and checking item
	// existense.
	start := time.Now()
	variablesByPhrase, err := TokensSearch(simpleQueryTokens, g.Patterns, g.Variables)
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("TokenSearch - %s\n", elapsed.String())
	}
	if err != nil {
		return variablesByPhrase, err
	}
	GrammarFilterVariablesMatch(g.Intent, variablesByPhrase, cm)
	return variablesByPhrase, nil
}

func (e *ESEngine) SearchGrammars(query *Query) ([]Intent, error) {
	intents := []Intent{}
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return intents, nil
	}
	for _, language := range query.LanguageOrder {
		if grammarByIntent, ok := e.grammars[language]; ok {
			for _, grammar := range grammarByIntent {
				intent, err := grammar.SearchGrammar(query, e.TokensCache, e.cache)
				if err != nil {
					return []Intent{}, err
				}
				if intent != nil {
					intents = append(intents, *intent)
				}
			}
		}
	}

	return intents, nil
}

func VariableValuesToFilterValues(values []VariableValue) []FilterValue {
	ret := []FilterValue{}
	for i := range values {
		// TODO: Actually map the variable names to filter names and variable values to filter values.
		// Maybe this should be done in frontend...
		filterName, ok := consts.VARIABLE_TO_FILTER[values[i].Name]
		if !ok {
			filterName = values[i].Name
		}
		ret = append(ret, FilterValue{
			Name:       filterName,
			Value:      values[i].Value,
			Origin:     values[i].Origin,
			OriginFull: values[i].OriginFull,
		})
	}
	return ret
}

func (g *Grammar) SearchGrammar(query *Query, tc *TokensCache, cm cache.CacheManager) (*Intent, error) {
	// Check filters match, i.e., existing query filter match at least one supported grammar intent filter.
	if len(query.Filters) > 0 {
		common := false
		for filterName, values := range query.Filters {
			sort.Strings(values)
			if grammarValues, ok := g.Filters[filterName]; ok {
				sort.Strings(grammarValues)
				if len(utils.IntersectSortedStringSlices(values, grammarValues)) > 0 {
					common = true
					break
				}
			}
		}
		if !common {
			// No matching filter found, should not trigger intent.
			log.Infof("No common filters for intent %s: %+v vs %+v", g.Intent, query.Filters, g.Filters)
			return nil, nil
		}
	}

	simpleQuery := query.Term
	if simpleQuery == "" && len(query.ExactTerms) == 1 {
		simpleQuery = query.ExactTerms[0]
	}
	// TODO: Tokenization is call to elastic. We can do this in parallel for all languages.
	// Consider extracting up the generation of Tokens.
	simpleQueryTokens, err := MakeTokensFromPhrase(simpleQuery, g.Language, g.Esc, tc)
	if err != nil {
		return nil, errors.Wrapf(err, "Error tokenizing simpleQuery: [%s] in %s.", simpleQuery, g.Language)
	}
	if match, values, _, err := TokensMatch(simpleQueryTokens, g.Patterns, false, g.Variables); err != nil {
		return nil, err
	} else if match {
		vMap := make(map[string][]string)
		for _, variableValue := range values {
			vMap[variableValue.Name] = append(vMap[variableValue.Name], variableValue.Value)
		}
		if GrammarVariablesMatch(g.Intent, vMap, cm) {
			// If this varibale map match any data, return intent.
			// Uncomment for debug:
			// log.Infof("Matched search [%s] for grammar %s, intent %s for %s. Pattern: %s", query.Original, g.HitType, g.Intent, g.Language, pattern)
			return &Intent{
				Type:     consts.GRAMMAR_TYPE_LANDING_PAGE,
				Language: g.Language,
				Value: GrammarIntent{
					LandingPage:  g.Intent,
					FilterValues: VariableValuesToFilterValues(values),
				},
			}, nil
		}
	}
	return nil, nil
}
