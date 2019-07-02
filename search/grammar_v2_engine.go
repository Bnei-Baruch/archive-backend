package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

func (e *ESEngine) SuggestGrammarsV2(query *Query, preference string) (map[string][]VariablesByPhrase, error) {
	start := time.Now()
	// Map from lang => Original Full Phrase => $Var => values
	suggests := make(map[string][]VariablesByPhrase)

	if query.Term == "" || len(query.ExactTerms) > 0 {
		log.Infof("Term is empty of exact term exists, should not trigger grammar: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return suggests, nil
	}

	multiSearchService := e.esc.MultiSearch()

	for _, language := range query.LanguageOrder {
		// Suggester:
		multiSearchService.Add(NewResultsSuggestGrammarV2CompletionRequest(query, language, preference))
		// Search (will not match part of words): multiSearchService.Add(NewSuggestGammarV2Request(query, language, preference))
	}

	mr, err := multiSearchService.Do(context.TODO())
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("multiSearchService.Do - %s\n\n", elapsed.String())
	}
	if err != nil {
		return nil, errors.Wrap(err, "Error loking for grammar suggest.")
	}

	if len(mr.Responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
	}

	start = time.Now()
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		// Suggester
		if SuggestionHasOptions(currentResults.Suggest) {
			language := query.LanguageOrder[i]
			if suggests[language], err = e.suggestOptionsToVariablesByPhrases(query, &currentResults.Suggest); err != nil {
				return nil, err
			}
		}

		// Searcher: <=== will not match part of words.
		//if haveHits(currentResults) {
		//	language := query.LanguageOrder[i]
		//	if suggests[language], err = e.suggestResultsToVariablesByPhrases(query, currentResults); err != nil {
		//		return nil, err
		//	}
		//}

	}
	elapsed = time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("build suggests - %s\n\n", elapsed.String())
	}

	return suggests, nil
}

func (e *ESEngine) suggestOptionsToVariablesByPhrases(query *Query, suggest *elastic.SearchSuggest) ([]VariablesByPhrase, error) {
	ret := []VariablesByPhrase(nil)
	for _, v := range *suggest {
		for _, s := range v {
			if len(s.Options) > 0 {
				for _, option := range s.Options {
					var rule GrammarRule
					if err := json.Unmarshal(*option.Source, &rule); err != nil {
						return nil, err
					}
					// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", option.Score, option.Index, option.Type, option.Id, rule)
					if len(rule.Values) != len(rule.Variables) {
						return nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
					}
					vMap := make(map[string][]string)
					for i := range rule.Variables {
						vMap[rule.Variables[i]] = []string{rule.Values[i]}
					}
					if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
						//log.Infof("Chosen: %s", chosen)
						//log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", option.Score, option.Index, option.Type, option.Id, rule)
						//log.Infof("Options: %+v", option)
						//log.Infof("vMap: [%+v]", vMap)
						// Map from lang => Original Full Phrase => $Var => values
						variablesByPhrase := make(VariablesByPhrase)
						variablesByPhrase[chooseRule(query, rule.Rules)] = vMap
						ret = append(ret, variablesByPhrase)
					}
				}
			}
		}
	}

	return ret, nil
}

func (e *ESEngine) suggestResultsToVariablesByPhrases(query *Query, result *elastic.SearchResult) ([]VariablesByPhrase, error) {
	ret := []VariablesByPhrase(nil)
	if haveHits(result) {
		// log.Infof("Total Hits: %d, Max Score: %.2f", result.Hits.TotalHits, *result.Hits.MaxScore)
		for _, hit := range result.Hits.Hits {
			var rule GrammarRule
			if err := json.Unmarshal(*hit.Source, &rule); err != nil {
				return nil, err
			}
			// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", *hit.Score, hit.Index, hit.Type, hit.Id, rule)
			if len(rule.Values) != len(rule.Variables) {
				return nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
			}
			vMap := make(map[string][]string)
			for i := range rule.Variables {
				vMap[rule.Variables[i]] = []string{rule.Values[i]}
			}
			if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
				// Map from lang => Original Full Phrase => $Var => values
				variablesByPhrase := make(VariablesByPhrase)
				variablesByPhrase[chooseRule(query, rule.Rules)] = vMap
				ret = append(ret, variablesByPhrase)
			}
		}
	}
	return ret, nil
}

func (e *ESEngine) SearchGrammarsV2(query *Query, preferences string) ([]Intent, error) {
	intents := []Intent{}
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return intents, nil
	}

	multiSearchService := e.esc.MultiSearch()
	for _, language := range query.LanguageOrder {
		multiSearchService.Add(NewSuggestGammarV2Request(query, language, preferences))
	}
	mr, err := multiSearchService.Do(context.TODO())
	if err != nil {
		return nil, errors.Wrap(err, "Error loking for grammar suggest.")
	}

	if len(mr.Responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
	}

	start := time.Now()
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		language := query.LanguageOrder[i]

		if haveHits(currentResults) {
			if langIntents, err := e.searchResultsToIntents(query, language, currentResults); err != nil {
				return nil, err
			} else {
				intents = append(intents, langIntents...)
			}
		}

	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("build grammar intent - %s\n\n", elapsed.String())
	}

	return intents, nil
}

func (e *ESEngine) VariableMapToFilterValues(vMap map[string][]string, language string) []FilterValue {
	ret := []FilterValue{}
	for name, values := range vMap {
		// TODO: Actually map the variable names to filter names and variable values to filter values.
		// Maybe this should be done in frontend...
		filterName, ok := consts.VARIABLE_TO_FILTER[name]
		if !ok {
			filterName = name
		}
		for _, value := range values {
			ret = append(ret, FilterValue{
				Name:       filterName,
				Value:      value,
				Origin:     e.variables[name][language][value][0],
				OriginFull: e.variables[name][language][value][0],
			})
		}
	}
	return ret
}

func (e *ESEngine) searchResultsToIntents(query *Query, language string, result *elastic.SearchResult) ([]Intent, error) {
	// log.Infof("Total Hits: %d, Max Score: %.2f", result.Hits.TotalHits, *result.Hits.MaxScore)
	intents := []Intent(nil)
	for _, hit := range result.Hits.Hits {
		var rule GrammarRule
		if err := json.Unmarshal(*hit.Source, &rule); err != nil {
			return nil, err
		}
		// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", *hit.Score, hit.Index, hit.Type, hit.Id, rule)
		if len(rule.Values) != len(rule.Variables) {
			return nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
		}
		vMap := make(map[string][]string)
		for i := range rule.Variables {
			vMap[rule.Variables[i]] = []string{rule.Values[i]}
		}
		if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
			intents = append(intents, Intent{
				Type:     consts.GRAMMAR_TYPE_LANDING_PAGE,
				Language: language,
				Value: GrammarIntent{
					LandingPage:  rule.Intent,
					FilterValues: e.VariableMapToFilterValues(vMap, language),
					Score:        *hit.Score,
				},
			})
		}
	}
	return intents, nil
}
