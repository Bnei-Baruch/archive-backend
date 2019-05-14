package search

import (
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	GRAMMAR_TYPE_LANDING_PAGE = "landing-page"
)

type GrammarIntent struct {
	LandingPage string `json:"landing_page,omitempty"`
}

func (e *ESEngine) SearchGrammars(query *Query) ([]Intent, error) {
	intents := []Intent{}
	for _, language := range query.LanguageOrder {
		log.Infof("LanguageOrder: %s Grammars: %+v", language, e.grammars)
		if grammarByIntent, ok := e.grammars[language]; ok {
			log.Infof("grammarByIntent: %+v", grammarByIntent)
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

	log.Infof("Mached grammars: %+v", intents)

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
	for _, pattern := range g.Patterns {
		if pattern == simpleQuery {
			log.Infof("Matched search [%s] for grammar %s, intent %s for %s. Pattern: %s", query.Original, g.HitType, g.Intent, g.Language, pattern)
			return &Intent{Type: GRAMMAR_TYPE_LANDING_PAGE, Language: g.Language, Value: GrammarIntent{LandingPage: g.Intent}}, nil
		}
	}
	return nil, nil
}
