package search

import (
	"context"
	"strconv"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"
	null "gopkg.in/volatiletech/null.v6"
)

func (e *ESEngine) GetTypoSuggest(query Query) (TypoSuggestResponse, error) {
	srv := e.esc.Search()
	result := TypoSuggestResponse{null.String{"", false}, 1}

	if _, err := strconv.Atoi(query.Term); err == nil {
		//  ignore numbers
		return result, nil
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

	suggester := elastic.NewPhraseSuggester("pharse-suggest").
		Text(query.Term).
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
		return result, errors.Wrap(err, "ESEngine.DoSearch - Error TypoSuggestDo Do.")
	}

	if sp, ok := r.Suggest["pharse-suggest"]; ok {
		if len(sp) > 0 && sp[0].Options != nil && len(sp[0].Options) > 0 {
			result.Text = null.String{sp[0].Options[0].Text, true}
			result.Score = sp[0].Options[0].Score
		}
	}

	return result, nil
}
