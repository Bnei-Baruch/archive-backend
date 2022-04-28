package search

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

type ConstantTerms struct {
	pattern string
	terms   []string
}

func (c *ConstantTerms) RememberTerms(s string) {
	if len(c.pattern) > 0 {
		rg, err := regexp.Compile(c.pattern)
		if err != nil {
			return
		}

		t := rg.FindAllStringSubmatch(s, -1)

		for _, match := range t {
			c.terms = append(c.terms, match[1])
		}
	}
}

func (c *ConstantTerms) ReplaceTerms(s string) string {
	if len(c.terms) > 0 {
		terms := strings.Fields(s)

		for i, term := range terms {
			if res, err := regexp.MatchString(c.pattern, term); err == nil && res && len(c.terms) > 0 {
				terms[i], c.terms = c.terms[0], c.terms[1:]
			}
		}

		s = strings.Join(terms, ` `)
	}

	return s
}

func (e *ESEngine) GetTypoSuggest(query Query, filterIntents []Intent) (null.String, error) {
	srv := e.esc.Search()
	suggestText := null.String{"", false}
	constantTerms := ConstantTerms{pattern: consts.TERMS_PATTERN_DIGITS}

	if _, err := strconv.Atoi(query.Term); err == nil {
		//  ignore numbers
		return suggestText, nil
	}

	checkTerm := query.Term
	considerGrammarTextValue := false
	if filterIntents != nil && len(filterIntents) > 0 {
		// Check typos for the "free text" value from the detected grammar rule.
		// Currently we support the check for only one appeareance of "free text" value.
		for _, filterIntent := range filterIntents {
			if intentValue, ok := filterIntent.Value.(GrammarIntent); ok {
				for _, fv := range intentValue.FilterValues {
					if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_TEXT] {
						checkTerm = fv.Value
						considerGrammarTextValue = true
						break
					}
				}
				if considerGrammarTextValue {
					break
				}
			} else {
				return suggestText, errors.Errorf("ESEngine.DoSearch - Intent is not GrammarIntent. Intent: %+v", filterIntent)
			}
		}
	}

	var hasHebrew bool
	var hasRussian bool
	var hasEnglish bool

	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		if query.LanguageOrder[i] == consts.LANG_HEBREW {
			hasHebrew = true
		} else if query.LanguageOrder[i] == consts.LANG_RUSSIAN {
			hasRussian = true
		} else if query.LanguageOrder[i] == consts.LANG_ENGLISH {
			hasEnglish = true
		}
		indices[i] = es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, query.LanguageOrder[i])
	}
	srv.Index(indices...)

	var suggestorField string
	var candidateField1 null.String
	var candidateField2 null.String
	var addMaxEdits bool

	if hasHebrew {
		suggestorField = "title"
		candidateField1.SetValid("title")
		candidateField2.SetValid("content")
		addMaxEdits = true
	} else if hasRussian {
		suggestorField = "content"
		candidateField1.SetValid("content.language")
		candidateField2.SetValid("title.language")
		addMaxEdits = false
	} else if hasEnglish {
		suggestorField = "content.language"
		candidateField1.SetValid("content.language")
		candidateField2 = null.StringFromPtr(nil)
		addMaxEdits = true
	} else {
		//  default settings for all languages
		suggestorField = "content.language"
		candidateField1.SetValid("content.language")
		candidateField2 = null.StringFromPtr(nil)
		addMaxEdits = true
	}

	constantTerms.RememberTerms(checkTerm)

	suggester := elastic.NewPhraseSuggester("pharse-suggest").
		Text(checkTerm).
		Field(suggestorField).
		Size(1).
		GramSize(1).
		Confidence(1).
		SmoothingModel(elastic.NewLaplaceSmoothingModel(0.7))

	if candidateField1.Valid {
		can1 := elastic.NewDirectCandidateGenerator(candidateField1.String).SuggestMode("popular")
		if addMaxEdits {
			can1.MaxEdits(1)
		}
		suggester.CandidateGenerator(can1)
	}
	if candidateField2.Valid {
		can2 := elastic.NewDirectCandidateGenerator(candidateField2.String).SuggestMode("popular")
		if addMaxEdits {
			can2.MaxEdits(1)
		}
		suggester.CandidateGenerator(can2)
	}

	srv.Suggester(suggester)
	beforeDoSearch := time.Now()
	r, err := srv.Do(context.TODO())
	e.timeTrack(beforeDoSearch, "DoSearch.TypoSuggestDo")
	if err != nil {
		return suggestText, errors.Wrap(err, "ESEngine.DoSearch - Error TypoSuggestDo Do.")
	}

	if sp, ok := r.Suggest["pharse-suggest"]; ok {
		if len(sp) > 0 && sp[0].Options != nil && len(sp[0].Options) > 0 {
			suggested := sp[0].Options[0].Text
			suggested = constantTerms.ReplaceTerms(suggested)
			if considerGrammarTextValue {
				suggested = strings.Replace(query.Term, checkTerm, suggested, -1)
			}
			suggestText = null.String{suggested, true}
		}
	}

	return suggestText, nil
}
