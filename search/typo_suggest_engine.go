package search

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"
	null "gopkg.in/volatiletech/null.v6"
)

type QueryFormatter struct {
	pattern string
	sources map[int]string
}

func (f *QueryFormatter) ToRequest(s string) string {
	if len(f.pattern) > 0 {
		stringSplit := strings.Split(s, ` `)
		result := make([]string, 0)
		f.sources = make(map[int]string)

		for i, word := range stringSplit {
			if res, err := regexp.MatchString(f.pattern, word); err == nil && res {
				f.sources[i] = word
			} else {
				result = append(result, word)
			}
		}

		s = strings.Join(result, ` `)
	}

	return s
}

func (f *QueryFormatter) ToResponse(s string) string {
	if len(f.sources) > 0 {
		stringSplit := strings.Split(s, ` `)
		result := make([]string, len(stringSplit)+len(f.sources))

		for i, source := range f.sources {
			result[i] = source
		}

		var item string
		for i, res := range result {
			if res == `` {
				item, stringSplit = stringSplit[0], stringSplit[1:]
				result[i] = item
			}
		}

		s = strings.Join(result, ` `)
	}

	return s
}

func (e *ESEngine) GetTypoSuggest(query Query, filterIntents []Intent) (null.String, error) {
	srv := e.esc.Search()
	suggestText := null.String{"", false}
	queryFormatter := QueryFormatter{pattern: `^\d+$`}

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

	checkTerm = queryFormatter.ToRequest(checkTerm)

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
			if considerGrammarTextValue {
				suggested = strings.Replace(query.Term, checkTerm, suggested, -1)
			}
			suggested = queryFormatter.ToResponse(suggested)
			suggestText = null.String{suggested, true}
		}
	}

	return suggestText, nil
}
