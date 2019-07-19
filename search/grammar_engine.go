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

func (e *ESEngine) SearchGrammars(query *Query) ([]Intent, error) {
	intents := []Intent{}
	for _, language := range query.LanguageOrder {
		if grammarByIntent, ok := e.grammars[language]; ok {
			for _, grammar := range grammarByIntent {
				intent, err := grammar.SearchGrammar(query)
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

func (g *Grammar) SearchGrammar(query *Query) (*Intent, error) {
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return nil, nil
	}

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
	simpleQueryTokens, err := MakeTokensFromPhrase(simpleQuery, g.Language, g.Esc)
	if err != nil {
		return nil, errors.Wrapf(err, "Error tokenizing simpleQuery: [%s] in %s.", simpleQuery, g.Language)
	}
	if TokensMatch(g.Patterns, simpleQueryTokens) {
		// Uncomment for debug:
		// log.Infof("Matched search [%s] for grammar %s, intent %s for %s. Pattern: %s", query.Original, g.HitType, g.Intent, g.Language, pattern)
		return &Intent{Type: consts.GRAMMAR_TYPE_LANDING_PAGE, Language: g.Language, Value: GrammarIntent{LandingPage: g.Intent}}, nil
	}
	return nil, nil
}
