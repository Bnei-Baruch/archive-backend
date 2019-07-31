package search

import (
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type GrammarIntent struct {
	LandingPage string `json:"landing_page,omitempty"`
}

func (e *ESEngine) SuggestGrammars(query *Query) (map[string][]string, error) {
	suggests := make(map[string][]string)
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return suggests, nil
	}
	for _, language := range query.LanguageOrder {
		if grammarByIntent, ok := e.grammars[language]; ok {
			for _, grammar := range grammarByIntent {
				grammarSuggest, err := grammar.SuggestGrammar(query, e.TokensCache)
				if err != nil {
					return nil, err
				}
				if grammarSuggest != "" {
					suggests[language] = append(suggests[language], grammarSuggest)
				}
			}
		}
	}
	return suggests, nil
}

func (g *Grammar) SuggestGrammar(query *Query, tc *TokensCache) (string, error) {
	simpleQuery := query.Term
	if simpleQuery == "" && len(query.ExactTerms) == 1 {
		simpleQuery = query.ExactTerms[0]
	}
	// TODO: Tokenization is call to elastic. We can do this in parallel for all languages.
	// Consider extracting up the generation of Tokens.
	simpleQueryTokens, err := MakeTokensFromPhrase(simpleQuery, g.Language, g.Esc, tc)
	if err != nil {
		return "", errors.Wrapf(err, "Error tokenizing simpleQuery: [%s] in %s.", simpleQuery, g.Language)
	}
	return TokensSearch(simpleQueryTokens, g.Patterns)
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
				intent, err := grammar.SearchGrammar(query, e.TokensCache)
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

func (g *Grammar) SearchGrammar(query *Query, tc *TokensCache) (*Intent, error) {
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
	if TokensMatch(simpleQueryTokens, g.Patterns) {
		// Uncomment for debug:
		// log.Infof("Matched search [%s] for grammar %s, intent %s for %s. Pattern: %s", query.Original, g.HitType, g.Intent, g.Language, pattern)
		return &Intent{Type: consts.GRAMMAR_TYPE_LANDING_PAGE, Language: g.Language, Value: GrammarIntent{LandingPage: g.Intent}}, nil
	}
	return nil, nil
}
