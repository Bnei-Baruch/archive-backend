package search

// ES9 counterparts of the three sub-engines that still use the olivere client
// in their ESEngine implementations.  By defining these methods on *ES9Engine,
// Go's method promotion causes ES9Engine.DoSearch (which calls e.SearchTweets,
// e.LessonsSeries, e.GetTypoSuggest) to dispatch here instead of to the
// embedded *ESEngine versions.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// ---------------------------------------------------------------------------
// SearchTweets — ES9
// ---------------------------------------------------------------------------

// SearchTweets runs a tweets multi-search against ES9.
// Shadows ESEngine.SearchTweets so ES9Engine.DoSearch uses this automatically.
func (e *ES9Engine) SearchTweets(query Query, sortBy string, from int, size int, preference string) (map[string]*SearchResult, error) {
	bodies, indices, _, err := NewESResultsSearchBodies(SearchRequestOptions{
		resultTypes:      []string{consts.ES_RESULT_TYPE_TWEETS},
		query:            query,
		sortBy:           consts.SORT_BY_RELEVANCE,
		from:             0,
		size:             consts.TWEETS_SEARCH_COUNT,
		preference:       preference,
		useHighlight:     false,
		partialHighlight: false,
	})
	if err != nil {
		return nil, err
	}

	before := time.Now()
	responses, err := e.es9Msearch(context.TODO(), bodies, indices, preference)
	e.timeTrack(before, consts.LAT_DOSEARCH_MULTISEARCHTWEETSDO)
	if err != nil {
		return nil, err
	}

	if len(responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of tweet results %d, expected %d",
			len(responses), len(query.LanguageOrder)))
	}

	tweetsByLang := make(map[string]*SearchResult)
	for i, currentResults := range responses {
		if haveHits(currentResults) {
			tweetsByLang[query.LanguageOrder[i]] = currentResults
		}
	}

	return e.CombineResultsToSingleHit(tweetsByLang, consts.SEARCH_RESULT_TWEETS_MANY)
}

// ---------------------------------------------------------------------------
// LessonsSeries — ES9
// ---------------------------------------------------------------------------

// LessonsSeries runs a lessons-series multi-search against ES9.
// Shadows ESEngine.LessonsSeries so ES9Engine.DoSearch uses this automatically.
func (e *ES9Engine) LessonsSeries(query Query, preference string) (map[string]*SearchResult, error) {
	_, queryTermHasDigit := utils.HasNumeric(query.Term)
	filter := map[string][]string{consts.FILTER_CONTENT_TYPE: {consts.CT_LESSONS_SERIES}}

	var bodies []map[string]interface{}
	var indices []string
	for _, language := range query.LanguageOrder {
		index := es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, language)
		body, idx, _, err := NewESResultsSearchBody(SearchRequestOptions{
			resultTypes: []string{consts.ES_RESULT_TYPE_COLLECTIONS},
			index:       index,
			query: Query{
				Term:          query.Term,
				ExactTerms:    query.ExactTerms,
				Filters:       filter,
				LanguageOrder: query.LanguageOrder,
				Deb:           query.Deb,
			},
			sortBy:           consts.SORT_BY_RELEVANCE,
			from:             0,
			size:             100,
			preference:       preference,
			useHighlight:     false,
			partialHighlight: false,
		})
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, body)
		indices = append(indices, idx)
	}

	before := time.Now()
	responses, err := e.es9Msearch(context.TODO(), bodies, indices, preference)
	e.timeTrack(before, consts.LAT_DOSEARCH_MULTISEARCHTWEETSDO)
	if err != nil {
		return nil, err
	}

	byLang := make(map[string]*SearchResult)
	for i, res := range responses {
		if haveHits(res) {
			byLang[query.LanguageOrder[i]] = res
		}
	}

	if queryTermHasDigit {
		return byLang, nil
	}
	return combineBySourceOrTag(byLang), nil
}

// ---------------------------------------------------------------------------
// GetTypoSuggest — ES9
// ---------------------------------------------------------------------------

// GetTypoSuggest runs a phrase-suggester query against ES9.
// Shadows ESEngine.GetTypoSuggest so ES9Engine.DoSearch uses this automatically.
func (e *ES9Engine) GetTypoSuggest(query Query, filterIntents []Intent) (null.String, error) {
	suggestText := null.String{"", false}
	constantTerms := ConstantTerms{pattern: consts.TERMS_PATTERN_DIGITS}

	if _, err := strconv.Atoi(query.Term); err == nil {
		return suggestText, nil
	}

	checkTerm := query.Term
	considerGrammarTextValue := false
	if len(filterIntents) > 0 {
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
				return suggestText, errors.Errorf("ES9Engine.GetTypoSuggest - Intent is not GrammarIntent. Intent: %+v", filterIntent)
			}
		}
	}

	var hasHebrew, hasRussian, hasEnglish bool
	indices := make([]string, len(query.LanguageOrder))
	for i, lang := range query.LanguageOrder {
		switch lang {
		case consts.LANG_HEBREW:
			hasHebrew = true
		case consts.LANG_RUSSIAN:
			hasRussian = true
		case consts.LANG_ENGLISH:
			hasEnglish = true
		}
		indices[i] = es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, lang)
	}

	var suggestorField string
	var candidateField1, candidateField2 null.String
	var addMaxEdits bool

	switch {
	case hasHebrew:
		suggestorField = "title"
		candidateField1.SetValid("title")
		candidateField2.SetValid("content")
		addMaxEdits = true
	case hasRussian:
		suggestorField = "content"
		candidateField1.SetValid("content.language")
		candidateField2.SetValid("title.language")
		addMaxEdits = false
	case hasEnglish:
		suggestorField = "content.language"
		candidateField1.SetValid("content.language")
		addMaxEdits = true
	default:
		suggestorField = "content.language"
		candidateField1.SetValid("content.language")
		addMaxEdits = true
	}

	constantTerms.RememberTerms(checkTerm)

	// Build direct_generator array
	generators := []interface{}{}
	if candidateField1.Valid {
		gen := map[string]interface{}{
			"field":        candidateField1.String,
			"suggest_mode": "popular",
		}
		if addMaxEdits {
			gen["max_edits"] = 1
		}
		generators = append(generators, gen)
	}
	if candidateField2.Valid {
		gen := map[string]interface{}{
			"field":        candidateField2.String,
			"suggest_mode": "popular",
		}
		if addMaxEdits {
			gen["max_edits"] = 1
		}
		generators = append(generators, gen)
	}

	phraseBody := map[string]interface{}{
		"field":      suggestorField,
		"size":       1,
		"gram_size":  1,
		"confidence": 1,
		"smoothing": map[string]interface{}{
			"laplace": map[string]interface{}{"alpha": 0.7},
		},
	}
	if len(generators) > 0 {
		phraseBody["direct_generator"] = generators
	}

	body := map[string]interface{}{
		"_source": false,
		"size":    0,
		"suggest": map[string]interface{}{
			"pharse-suggest": map[string]interface{}{
				"text":   checkTerm,
				"phrase": phraseBody,
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return suggestText, fmt.Errorf("ES9Engine.GetTypoSuggest: encode: %w", err)
	}

	before := time.Now()
	res, err := e.esc9.Search(
		e.esc9.Search.WithContext(context.TODO()),
		e.esc9.Search.WithIndex(indices...),
		e.esc9.Search.WithBody(&buf),
	)
	e.timeTrack(before, "DoSearch.TypoSuggestDo")
	if err != nil {
		return suggestText, errors.Wrap(err, "ES9Engine.GetTypoSuggest - search failed")
	}
	defer res.Body.Close()
	if res.IsError() {
		return suggestText, fmt.Errorf("ES9Engine.GetTypoSuggest: %s", res.String())
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return suggestText, fmt.Errorf("ES9Engine.GetTypoSuggest: decode: %w", err)
	}

	// Parse suggest response
	suggestRaw, ok := raw["suggest"].(map[string]interface{})
	if !ok {
		return suggestText, nil
	}
	spArr, ok := suggestRaw["pharse-suggest"].([]interface{})
	if !ok || len(spArr) == 0 {
		return suggestText, nil
	}
	spMap, ok := spArr[0].(map[string]interface{})
	if !ok {
		return suggestText, nil
	}
	optionsArr, ok := spMap["options"].([]interface{})
	if !ok || len(optionsArr) == 0 {
		return suggestText, nil
	}
	opt0, ok := optionsArr[0].(map[string]interface{})
	if !ok {
		return suggestText, nil
	}
	suggested, ok := opt0["text"].(string)
	if !ok || suggested == "" {
		return suggestText, nil
	}

	suggested = constantTerms.ReplaceTerms(suggested)
	if considerGrammarTextValue {
		suggested = strings.Replace(query.Term, checkTerm, suggested, -1)
	}
	return null.String{suggested, true}, nil
}
