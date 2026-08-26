package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	esUtils "github.com/Bnei-Baruch/archive-backend/utils"
)

// ES9Engine is the Elasticsearch 9 counterpart of ESEngine.
// It embeds *ESEngine to reuse sub-engine methods (grammar, intents, tweets,
// lesson-series, typo-suggest) that still talk to ES6 during the transitional phase.
// Only the main results multi-search and completion-suggest calls use the ES9 client.
type ES9Engine struct {
	esc9 *elasticsearch.Client
	*ESEngine
}

// NewES9Engine creates an ES9Engine.
// esc9  - the ES9 HTTP client
// inner - the existing ESEngine; its sub-engine methods are reused via embedding.
func NewES9Engine(esc9 *elasticsearch.Client, inner *ESEngine) *ES9Engine {
	return &ES9Engine{esc9: esc9, ESEngine: inner}
}

// ---------------------------------------------------------------------------
// ES9 response parsers
// ---------------------------------------------------------------------------

// fromES9Response converts a raw ES9 JSON response map to *SearchResult.
func fromES9Response(raw map[string]interface{}) *SearchResult {
	var hits *SearchHits
	if hitsRaw, ok := raw["hits"].(map[string]interface{}); ok {
		hits = fromES9Hits(hitsRaw)
	}
	var suggest SearchSuggest
	if suggestRaw, ok := raw["suggest"].(map[string]interface{}); ok {
		suggest = fromES9Suggest(suggestRaw)
	}
	return &SearchResult{Hits: hits, Suggest: suggest}
}

// fromES9Hits converts the "hits" object of an ES9 response.
// ES9 reports total as {"value": N, "relation": "eq"} rather than a plain int64.
func fromES9Hits(hitsRaw map[string]interface{}) *SearchHits {
	result := &SearchHits{Hits: []*SearchHit{}}
	if totalRaw, ok := hitsRaw["total"].(map[string]interface{}); ok {
		if v, ok := totalRaw["value"].(float64); ok {
			result.TotalHits = int64(v)
		}
	}
	if v, ok := hitsRaw["max_score"].(float64); ok {
		result.MaxScore = &v
	}
	if hitsArr, ok := hitsRaw["hits"].([]interface{}); ok {
		result.Hits = make([]*SearchHit, 0, len(hitsArr))
		for _, hitRaw := range hitsArr {
			if hitMap, ok := hitRaw.(map[string]interface{}); ok {
				result.Hits = append(result.Hits, fromES9Hit(hitMap))
			}
		}
	}
	return result
}

// fromES9Hit converts a single ES9 hit map to *SearchHit.
func fromES9Hit(hitMap map[string]interface{}) *SearchHit {
	hit := &SearchHit{}
	if v, ok := hitMap["_index"].(string); ok {
		hit.Index = v
	}
	if v, ok := hitMap["_id"].(string); ok {
		hit.ID = v
	}
	if v, ok := hitMap["_score"].(float64); ok {
		hit.Score = &v
	}
	if src, ok := hitMap["_source"]; ok {
		if b, err := json.Marshal(src); err == nil {
			raw := json.RawMessage(b)
			hit.Source = &raw
		}
	}
	if highlightRaw, ok := hitMap["highlight"].(map[string]interface{}); ok {
		hit.Highlight = fromES9Highlight(highlightRaw)
	}
	if expRaw, ok := hitMap["_explanation"].(map[string]interface{}); ok {
		hit.Explanation = fromES9Explanation(expRaw)
	}
	return hit
}

func fromES9Highlight(raw map[string]interface{}) SearchHitHighlight {
	result := make(SearchHitHighlight, len(raw))
	for field, frags := range raw {
		if arr, ok := frags.([]interface{}); ok {
			strs := make([]string, 0, len(arr))
			for _, f := range arr {
				if s, ok := f.(string); ok {
					strs = append(strs, s)
				}
			}
			result[field] = strs
		}
	}
	return result
}

func fromES9Explanation(raw map[string]interface{}) *SearchExplanation {
	if raw == nil {
		return nil
	}
	exp := &SearchExplanation{}
	if v, ok := raw["value"].(float64); ok {
		exp.Value = v
	}
	if v, ok := raw["description"].(string); ok {
		exp.Description = v
	}
	if details, ok := raw["details"].([]interface{}); ok {
		for _, d := range details {
			if dm, ok := d.(map[string]interface{}); ok {
				if child := fromES9Explanation(dm); child != nil {
					exp.Details = append(exp.Details, *child)
				}
			}
		}
	}
	return exp
}

// fromES9Suggest parses the "suggest" field of an ES9 response.
func fromES9Suggest(raw map[string]interface{}) SearchSuggest {
	result := make(SearchSuggest, len(raw))
	for key, suggestionsRaw := range raw {
		suggestionsArr, ok := suggestionsRaw.([]interface{})
		if !ok {
			continue
		}
		suggestions := make([]SearchSuggestion, 0, len(suggestionsArr))
		for _, sRaw := range suggestionsArr {
			sMap, ok := sRaw.(map[string]interface{})
			if !ok {
				continue
			}
			sugg := SearchSuggestion{}
			if v, ok := sMap["text"].(string); ok {
				sugg.Text = v
			}
			if v, ok := sMap["offset"].(float64); ok {
				sugg.Offset = int(v)
			}
			if v, ok := sMap["length"].(float64); ok {
				sugg.Length = int(v)
			}
			if optionsArr, ok := sMap["options"].([]interface{}); ok {
				for _, oRaw := range optionsArr {
					oMap, ok := oRaw.(map[string]interface{})
					if !ok {
						continue
					}
					opt := SearchSuggestionOption{}
					if v, ok := oMap["text"].(string); ok {
						opt.Text = v
					}
					if v, ok := oMap["_score"].(float64); ok {
						opt.Score = v
					}
					if src, ok := oMap["_source"]; ok {
						if b, err := json.Marshal(src); err == nil {
							raw := json.RawMessage(b)
							opt.Source = &raw
						}
					}
					sugg.Options = append(sugg.Options, opt)
				}
			}
			suggestions = append(suggestions, sugg)
		}
		result[key] = suggestions
	}
	return result
}

// ---------------------------------------------------------------------------
// Low-level ES9 search helpers
// ---------------------------------------------------------------------------

// es9Msearch runs a multi-search request against ES9.
// bodies and indices must be the same length.
func (e *ES9Engine) es9Msearch(ctx context.Context, bodies []map[string]interface{}, indices []string, preference string) ([]*SearchResult, error) {
	var buf bytes.Buffer
	for i, body := range bodies {
		header := map[string]interface{}{"index": indices[i]}
		if preference != "" {
			header["preference"] = preference
		}
		if err := json.NewEncoder(&buf).Encode(header); err != nil {
			return nil, fmt.Errorf("es9Msearch: encode header[%d]: %w", i, err)
		}
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("es9Msearch: encode body[%d]: %w", i, err)
		}
	}

	res, err := e.esc9.Msearch(
		&buf,
		e.esc9.Msearch.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("es9Msearch: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("es9Msearch: %s", res.String())
	}

	var msearchResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&msearchResult); err != nil {
		return nil, fmt.Errorf("es9Msearch: decode: %w", err)
	}

	responsesRaw, ok := msearchResult["responses"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("es9Msearch: no responses field")
	}

	results := make([]*SearchResult, 0, len(responsesRaw))
	for _, r := range responsesRaw {
		rMap, ok := r.(map[string]interface{})
		if !ok {
			results = append(results, nil)
			continue
		}
		if errObj, ok := rMap["error"]; ok {
			return nil, fmt.Errorf("es9Msearch: response error: %v", errObj)
		}
		results = append(results, fromES9Response(rMap))
	}
	return results, nil
}

// es9SingleSearch runs a single search against ES9 and returns *SearchResult.
func (e *ES9Engine) es9SingleSearch(ctx context.Context, body map[string]interface{}, index, preference string) (*SearchResult, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("es9SingleSearch: encode: %w", err)
	}

	res, err := e.esc9.Search(
		e.esc9.Search.WithContext(ctx),
		e.esc9.Search.WithIndex(index),
		e.esc9.Search.WithBody(&buf),
		e.esc9.Search.WithPreference(preference),
	)
	if err != nil {
		return nil, fmt.Errorf("es9SingleSearch: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("es9SingleSearch: %s", res.String())
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("es9SingleSearch: decode: %w", err)
	}
	return fromES9Response(raw), nil
}

// ---------------------------------------------------------------------------
// GetSuggestions
// ---------------------------------------------------------------------------

// GetSuggestions implements the same logic as ESEngine.GetSuggestions but uses
// ES9 for the completion-suggest multi-search.
func (e *ES9Engine) GetSuggestions(ctx context.Context, query Query, preference string) (interface{}, error) {
	beforeGetSuggest := time.Now()
	defer func() { e.timeTrack(beforeGetSuggest, consts.LAT_GETSUGGESTIONS) }()

	// Grammar suggestions run in parallel (V2 grammar has no ES client dependency).
	grammarSuggestionsChannel := make(chan map[string][]VariablesByPhrase)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("ES9Engine.GetSuggestions - Panic adding intents: %+v", err)
				grammarSuggestionsChannel <- make(map[string][]VariablesByPhrase)
			}
		}()
		beforeSuggestSuggest := time.Now()
		grammarSuggestions, err := e.SuggestGrammarsV2(&query, preference)
		if err != nil {
			log.Errorf("ES9Engine.GetSuggestions - Error adding intents: %+v", err)
			grammarSuggestionsChannel <- make(map[string][]VariablesByPhrase)
		} else {
			grammarSuggestionsChannel <- grammarSuggestions
		}
		e.timeTrack(beforeSuggestSuggest, consts.LAT_SUGGEST_SUGGESTIONS)
	}()

	// Build ES9 suggest bodies and run msearch.
	suggestResultTypes := []string{
		consts.ES_RESULT_TYPE_UNITS,
		consts.ES_RESULT_TYPE_COLLECTIONS,
		consts.ES_RESULT_TYPE_TAGS,
		consts.ES_RESULT_TYPE_SOURCES,
		consts.ES_RESULT_TYPE_BLOG_POSTS,
	}
	bodies, indices, _ := NewESResultsSuggestBodies(suggestResultTypes, query, preference)

	beforeMssDo := time.Now()
	responses, err := e.es9Msearch(ctx, bodies, indices, preference)
	e.timeTrack(beforeMssDo, consts.LAT_GETSUGGESTIONS_MULTISEARCHDO)
	if err != nil {
		if isContextError(err) {
			log.Warn("ES9Engine.GetSuggestions - ctx cancelled.")
			return nil, nil
		}
		return nil, errors.Wrap(err, "ES9Engine.GetSuggestions")
	}

	// Nativize response — replace title with full_title for sources.
	for _, r := range responses {
		if r == nil {
			continue
		}
		for key := range r.Suggest {
			for j := range r.Suggest[key] {
				for opIdx, op := range r.Suggest[key][j].Options {
					var src es.Result
					if op.Source == nil {
						continue
					}
					err = json.Unmarshal(*op.Source, &src)
					if err != nil {
						log.Errorf("ES9Engine.GetSuggestions - cannot unmarshal source.")
						continue
					}
					if src.ResultType == consts.ES_RESULT_TYPE_SOURCES && src.FullTitle != "" {
						src.Title = src.FullTitle
						src.FullTitle = ""
						nsrc, err := json.Marshal(src)
						if err != nil {
							log.Errorf("ES9Engine.GetSuggestions - cannot marshal source with title correction.")
							continue
						}
						r.Suggest[key][j].Options[opIdx].Source = (*json.RawMessage)(&nsrc)
					}
				}
			}
		}
	}

	// Merge grammar suggestions.
	grammarSuggestions := <-grammarSuggestionsChannel
	for i, lang := range query.LanguageOrder {
		if langSuggestions, ok := grammarSuggestions[lang]; ok && len(langSuggestions) > 0 && len(responses) > i {
			r := responses[i]
			if r == nil {
				continue
			}
			if r.Suggest == nil {
				r.Suggest = make(map[string][]SearchSuggestion)
			}
			if len(r.Suggest) == 0 {
				r.Suggest["title_suggest"] = []SearchSuggestion{}
			}
			for key := range r.Suggest {
				for j := range r.Suggest[key] {
					for _, variablesByPhrase := range langSuggestions {
						for suggestion := range variablesByPhrase {
							source := struct {
								Title      string `json:"title"`
								ResultType string `json:"result_type"`
							}{Title: suggestion, ResultType: consts.GRAMMAR_TYPE_LANDING_PAGE}
							sourceRawMessage, err := json.Marshal(source)
							if err != nil {
								return nil, err
							}
							raw := json.RawMessage(sourceRawMessage)
							option := SearchSuggestionOption{
								Text:   suggestion,
								Source: &raw,
							}
							r.Suggest[key][j].Options = append([]SearchSuggestionOption{option}, r.Suggest[key][j].Options...)
						}
					}
				}
			}
		}
	}

	// Return the first response that has suggestions, else the first response.
	var sRes *SearchResult
	for _, r := range responses {
		if r != nil && SuggestionHasOptions(r.Suggest) {
			sRes = r
			break
		}
	}
	if sRes == nil && len(responses) > 0 {
		sRes = responses[0]
	}
	return sRes, nil
}

// ---------------------------------------------------------------------------
// DoSearch
// ---------------------------------------------------------------------------

// DoSearch implements the same orchestration as ESEngine.DoSearch but uses
// ES9 for the main results multi-search and the per-hit highlight searches.
// Sub-engines (grammar, intents, tweets, lesson-series, typo) still use the
// embedded ESEngine's ES6 client until those are ported in Tasks #6.
func (e *ES9Engine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string, checkTypo bool, searchTweets bool, searchLessonSeries bool, withHighlights bool, timeoutForHighlight time.Duration) (*QueryResult, error) {
	defer e.timeTrack(time.Now(), consts.LAT_DOSEARCH)

	suggestChannel := make(chan null.String)
	grammarsSingleHitIntentsChannel := make(chan []Intent, 1)
	grammarsFilterIntentsChannel := make(chan []Intent, 1)
	grammarsFilteredResultsByLangChannel := make(chan map[string][]FilteredSearchResult)
	tweetsByLangChannel := make(chan map[string]*SearchResult)
	seriesLangChannel := make(chan map[string]*SearchResult)

	filterIntents := []Intent{}
	filteredByLang := map[string][]FilteredSearchResult{}
	tweetsByLang := map[string]*SearchResult{}
	seriesByLang := map[string]*SearchResult{}

	var resultTypes []string
	if sortBy == consts.SORT_BY_NEWER_TO_OLDER || sortBy == consts.SORT_BY_OLDER_TO_NEWER {
		resultTypes = make([]string, 0)
		for _, str := range e.searchResultTypes {
			if str != consts.ES_RESULT_TYPE_COLLECTIONS {
				resultTypes = append(resultTypes, str)
			}
		}
	} else {
		resultTypes = e.searchResultTypes
	}

	// Search grammars in parallel (uses inner ESEngine's ES6 client).
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("ES9Engine.DoSearch - Panic searching grammars: %+v", err)
				grammarsSingleHitIntentsChannel <- []Intent{}
				grammarsFilterIntentsChannel <- []Intent{}
				grammarsFilteredResultsByLangChannel <- map[string][]FilteredSearchResult{}
			}
		}()
		if singleHitIntents, filterIntents, err := e.SearchGrammarsV2(&query, from, size, sortBy, resultTypes, preference); err != nil {
			log.Errorf("ES9Engine.DoSearch - Error searching grammars: %+v", err)
			grammarsSingleHitIntentsChannel <- []Intent{}
			grammarsFilterIntentsChannel <- []Intent{}
			grammarsFilteredResultsByLangChannel <- map[string][]FilteredSearchResult{}
		} else {
			grammarsSingleHitIntentsChannel <- singleHitIntents
			grammarsFilterIntentsChannel <- filterIntents
			if filtered, err := e.SearchByFilterIntents(filterIntents, query.Filters, query.Term, from, size, sortBy, resultTypes, preference, query.Deb); err != nil {
				log.Errorf("ES9Engine.DoSearch - Error searching filtered results by grammars: %+v", err)
				grammarsFilteredResultsByLangChannel <- map[string][]FilteredSearchResult{}
			} else {
				grammarsFilteredResultsByLangChannel <- filtered
			}
		}
	}()

	if searchTweets {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("ES9Engine.DoSearch - Panic searching tweets: %+v", err)
					tweetsByLangChannel <- map[string]*SearchResult{}
				}
			}()
			if tweetsByLang, err := e.SearchTweets(query, sortBy, from, size, preference); err != nil {
				log.Errorf("ES9Engine.DoSearch - Error searching tweets: %+v", err)
				tweetsByLangChannel <- map[string]*SearchResult{}
			} else {
				tweetsByLangChannel <- tweetsByLang
			}
		}()
	}

	if searchLessonSeries {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("ES9Engine.DoSearch - Panic searching lesson series: %+v", err)
					seriesLangChannel <- map[string]*SearchResult{}
				}
			}()
			if byLang, err := e.LessonsSeries(query, preference); err != nil {
				log.Errorf("ES9Engine.DoSearch - Error searching lesson series: %+v", err)
				seriesLangChannel <- map[string]*SearchResult{}
			} else {
				seriesLangChannel <- byLang
			}
		}()
	}

	filterIntents = <-grammarsFilterIntentsChannel
	LogIfDeb(&query, IntentsToStringDebug("GRAMMAR FILTER INTENTS", filterIntents))

	if checkTypo {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("ES9Engine.GetTypoSuggest - Panic getting typo suggest: %+v", err)
					suggestChannel <- null.String{"", false}
				}
			}()
			if suggestText, err := e.GetTypoSuggest(query, filterIntents); err != nil {
				log.Errorf("ES9Engine.GetTypoSuggest - Error getting typo suggest: %+v", err)
				suggestChannel <- null.String{"", false}
			} else {
				suggestChannel <- suggestText
			}
		}()
	}

	LogIfDeb(&query, fmt.Sprintf("query.Intents: %d", len(query.Intents)))
	query.Intents = append(query.Intents, <-grammarsSingleHitIntentsChannel...)
	LogIfDeb(&query, IntentsToStringDebug("GRAMMARS SINGLE HIT INTENTS", query.Intents))

	hasClassificationIntentFromGrammar := false
	for _, intent := range query.Intents {
		if intentValue, ok := intent.Value.(ClassificationIntent); ok && intentValue.Exist {
			hasClassificationIntentFromGrammar = true
			break
		}
	}
	LogIfDeb(&query, fmt.Sprintf("Has classification intent from grammar: %s", fmt.Sprintf("%t", hasClassificationIntentFromGrammar)))

	intents, err := e.AddIntents(&query, preference, sortBy, true, !hasClassificationIntentFromGrammar, filterIntents)
	if err != nil {
		log.Errorf("ES9Engine.DoSearch - Error adding intents: %+v", err)
	}
	LogIfDeb(&query, IntentsToStringDebug("ADD INTENTS", intents))
	query.Intents = append(query.Intents, intents...)

	filterOutCUSources := make([]string, 0)
	for _, intent := range query.Intents {
		if intent.Type == consts.INTENT_TYPE_SOURCE {
			if intentValue, ok := intent.Value.(ClassificationIntent); ok && intentValue.Exist {
				filterOutCUSources = append(filterOutCUSources, intentValue.MDB_UID)
				LogIfDeb(&query, fmt.Sprintf("MDB_UID added to filterOutCUSources: %s.", intentValue.MDB_UID))
			}
		}
	}

	// --- Main results search via ES9 ---
	bodies, indices, _, err := NewESResultsSearchBodies(SearchRequestOptions{
		resultTypes:        resultTypes,
		query:              query,
		sortBy:             sortBy,
		from:               0,
		size:               from + size,
		preference:         preference,
		useHighlight:       false,
		filterOutCUSources: filterOutCUSources,
	})
	if err != nil {
		return nil, errors.Wrap(err, "ES9Engine.DoSearch - Error creating search bodies.")
	}

	beforeDoSearch := time.Now()
	mainResponses, err := e.es9Msearch(context.TODO(), bodies, indices, preference)
	e.timeTrack(beforeDoSearch, consts.LAT_DOSEARCH_MULTISEARCHDO)
	if err != nil {
		return nil, errors.Wrap(err, "ES9Engine.DoSearch - Error msearch.")
	}

	shouldMergeResults := false
	for _, lang := range query.LanguageOrder {
		if lang == consts.LANG_SPANISH {
			shouldMergeResults = true
			break
		}
	}
	if len(mainResponses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mainResponses), len(query.LanguageOrder)))
	}

	resultsByLang := make(map[string][]*SearchResult)

	var maxRegularScore *float64
	programsToReplaceWithGrammarResults := []struct {
		hitId        string
		score        float64
		grammarHitId *string
	}{}

	for i, currentResults := range mainResponses {
		if haveHits(currentResults) {
			if len(filterIntents) > 0 {
				var programCollectionUid *string
				for _, fi := range filterIntents {
					if intentValue, ok := fi.Value.(GrammarIntent); ok {
						freeText := getFilterValue(intentValue.FilterValues, consts.VARIABLE_TO_FILTER[consts.VAR_TEXT])
						if freeText == nil {
							programCollectionUid = getFilterValue(intentValue.FilterValues, consts.VARIABLE_TO_FILTER[consts.VAR_PROGRAM])
							if programCollectionUid != nil {
								break
							}
						}
					}
				}
				LogIfDeb(&query, fmt.Sprintf("Program collection uid: %+v", programCollectionUid))
				if programCollectionUid != nil {
					for _, hit := range currentResults.Hits.Hits {
						if hit.Score == nil {
							continue
						}
						var src es.Result
						if err = json.Unmarshal(*hit.Source, &src); err != nil {
							log.Errorf("ES9Engine.DoSearch - cannot unmarshal source for hit '%v'.", hit.ID)
							continue
						}
						if src.ResultType == consts.ES_RESULT_TYPE_UNITS {
							if esUtils.Contains(esUtils.Is(src.TypedUids), es.KeyValue(consts.ES_UID_TYPE_COLLECTION, *programCollectionUid)) {
								programsToReplaceWithGrammarResults = append(programsToReplaceWithGrammarResults,
									struct {
										hitId        string
										score        float64
										grammarHitId *string
									}{hit.ID, *hit.Score, nil})
							}
						}
					}
				}
			}
			if len(programsToReplaceWithGrammarResults) > 0 {
				sort.SliceStable(programsToReplaceWithGrammarResults, func(i, j int) bool {
					return programsToReplaceWithGrammarResults[i].score > programsToReplaceWithGrammarResults[j].score
				})
			}

			if currentResults.Hits.MaxScore != nil {
				if maxRegularScore == nil {
					maxRegularScore = new(float64)
					*maxRegularScore = *currentResults.Hits.MaxScore
				}
				if shouldMergeResults && *currentResults.Hits.MaxScore > *maxRegularScore {
					*maxRegularScore = *currentResults.Hits.MaxScore
				}
			}
			lang := query.LanguageOrder[i]
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], currentResults)
		}
	}

	LogIfDeb(&query, ResultsSliceMapToStringDebug("RESULTS BY LANG", resultsByLang, 5))

	err, intentResultsMap := e.IntentsToResults(&query)
	if err != nil {
		return nil, errors.Wrap(err, "ES9Engine.DoSearch - Error adding intents to results.")
	}
	LogIfDeb(&query, ResultsMapToStringDebug("INTENTS TO RESULTS", intentResultsMap, 3))
	for lang, intentResults := range intentResultsMap {
		if haveHits(intentResults) {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], intentResults)
		}
	}

	if searchTweets {
		tweetsByLang = <-tweetsByLangChannel
		LogIfDeb(&query, ResultsMapToStringDebug("TWEETS", tweetsByLang, 3))
		for lang, tweets := range tweetsByLang {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], tweets)
		}
	}

	if searchLessonSeries {
		seriesByLang = <-seriesLangChannel
		LogIfDeb(&query, ResultsMapToStringDebug("SERIES", seriesByLang, 3))
		for lang, s := range seriesByLang {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], s)
		}
	}

	filteredByLang = <-grammarsFilteredResultsByLangChannel
	LogIfDeb(&query, "---- GRAMMAR SEARCH FILTERED ----")
	for k, v := range filteredByLang {
		LogIfDeb(&query, fmt.Sprintf("\t%+v:", k))
		for i := range v {
			for j := range v[i].Results {
				LogIfDeb(&query, fmt.Sprintf("\t\t%d (%d hits)", j, len(v[i].Results[j].Hits.Hits)))
				LogIfDeb(&query, ResultToStringDebug(v[i].Results[j], 3))
			}
		}
	}
	LogIfDeb(&query, "---- END GRAMMAR SEARCH FILTERED ----")

	var programToReplaceIndex int
	if len(programsToReplaceWithGrammarResults) > 0 {
		for lang, filtered := range filteredByLang {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*SearchResult, 0)
			}
			for _, fr := range filtered {
				if fr.ProgramCollection != nil {
					for _, result := range fr.Results {
						for _, hit := range result.Hits.Hits {
							var src es.Result
							if err = json.Unmarshal(*hit.Source, &src); err != nil {
								log.Errorf("ES9Engine.DoSearch - cannot unmarshal source for hit '%v'.", hit.ID)
								continue
							}
							if src.ResultType == consts.ES_RESULT_TYPE_UNITS {
								if esUtils.Contains(esUtils.Is(src.TypedUids), es.KeyValue(consts.ES_UID_TYPE_COLLECTION, *fr.ProgramCollection)) {
									if programToReplaceIndex < len(programsToReplaceWithGrammarResults) {
										hit.Score = &programsToReplaceWithGrammarResults[programToReplaceIndex].score
										programsToReplaceWithGrammarResults[programToReplaceIndex].grammarHitId = &hit.ID
										programToReplaceIndex++
									} else {
										zero := 0.0
										hit.Score = &zero
									}
								}
							}
						}
					}
				}
			}
		}
	}

	for lang, filtered := range filteredByLang {
		if _, ok := resultsByLang[lang]; !ok {
			resultsByLang[lang] = make([]*SearchResult, 0)
		}
		for _, fr := range filtered {
			for _, result := range fr.Results {
				sort.Strings(filterOutCUSources)
				withoutCarouselDuplications := []*SearchHit{}
				var maxScore float64
				for _, hit := range result.Hits.Hits {
					var src es.Result
					if err = json.Unmarshal(*hit.Source, &src); err != nil {
						log.Errorf("ES9Engine.DoSearch - cannot unmarshal source for hit '%v'.", hit.ID)
						continue
					}
					if src.ResultType == consts.ES_RESULT_TYPE_UNITS {
						hitSources, err := es.KeyValuesToValues(consts.ES_UID_TYPE_SOURCE, src.TypedUids)
						if err != nil {
							log.Errorf("ES9Engine.DoSearch - cannot read TypedUids for hit '%v'.", hit.ID)
							continue
						}
						sort.Strings(hitSources)
						if len(esUtils.IntersectSortedStringSlices(hitSources, filterOutCUSources)) > 0 {
							log.Infof("Remove CU hit from 'filter grammar' that duplicates carousels source: %v", src.MDB_UID)
						} else {
							if hit.Score != nil {
								maxScore = math.Max(*hit.Score, maxScore)
							}
							withoutCarouselDuplications = append(withoutCarouselDuplications, hit)
						}
					} else {
						withoutCarouselDuplications = append(withoutCarouselDuplications, hit)
					}
				}
				result.Hits.Hits = withoutCarouselDuplications
				result.Hits.MaxScore = &maxScore
				result.Hits.TotalHits = int64(len(withoutCarouselDuplications))
			}
		}

		if maxRegularScore != nil && *maxRegularScore >= 15 {
			var filteredMaxScore float64
			for _, fr := range filtered {
				for _, result := range fr.Results {
					for _, hit := range result.Hits.Hits {
						if hit.Score != nil {
							filteredMaxScore = math.Max(*hit.Score, filteredMaxScore)
						}
					}
				}
			}
			boost := ((*maxRegularScore * 0.9) + 10) / filteredMaxScore
			LogIfDeb(&query, fmt.Sprintf("--- NORMALIZE FILTERED RESULTS SCORES --- maxRegularScore: %.2f filteredMaxScore: %.2f, boost: %.2f",
				*maxRegularScore, filteredMaxScore, boost))
			for _, fr := range filtered {
				for _, result := range fr.Results {
					var maxScore float64
					for _, hit := range result.Hits.Hits {
						replaced := false
						for _, p := range programsToReplaceWithGrammarResults {
							if p.grammarHitId != nil && *p.grammarHitId == hit.ID {
								replaced = true
								break
							}
						}
						if !replaced && hit.Score != nil {
							*hit.Score *= boost
						}
						maxScore = math.Max(*hit.Score, maxScore)
						result.Hits.MaxScore = &maxScore
					}
				}
			}
		}
		for _, result := range resultsByLang[lang] {
			for _, hit := range result.Hits.Hits {
				if hit.Score != nil {
					if len(programsToReplaceWithGrammarResults) > 0 {
						for i := 0; i < programToReplaceIndex; i++ {
							if hit.ID == programsToReplaceWithGrammarResults[i].hitId {
								LogIfDeb(&query, fmt.Sprintf("Setting zero score for %s.", hit.ID))
								zero := 0.0
								hit.Score = &zero
								break
							}
						}
					}
					for _, fr := range filtered {
						if _, hasId := fr.HitIdsMap[hit.ID]; hasId {
							LogIfDeb(&query, fmt.Sprintf("Same hit found for both regular and grammar filtered results: %v", hit.ID))
							if hit.Score != nil && *hit.Score > 5 {
								*hit.Score += consts.FILTER_GRAMMAR_INCREMENT_FOR_MATCH_TO_FULL_TERM
							}
							if !fr.PreserveTermForHighlight {
								delete(fr.HitIdsMap, hit.ID)
							}
						}
					}
				}
			}
		}
		for _, fr := range filtered {
			resultsByLang[lang] = append(resultsByLang[lang], fr.Results...)
		}
	}

	var currentLang string
	results := make([]*SearchResult, 0)
	for _, lang := range query.LanguageOrder {
		if r, ok := resultsByLang[lang]; ok {
			if shouldMergeResults {
				results = append(results, resultsByLang[lang]...)
			} else {
				if len(r) > 0 {
					results = r
					currentLang = lang
					break
				}
			}
		}
	}

	ret, err := joinResponses(sortBy, from, size, results...)
	LogIfDeb(&query, "--- AFTER JOIN ---")
	LogIfDeb(&query, ResultToStringDebug(ret, 20))
	LogIfDeb(&query, "--- END AFTER JOIN ---")

	suggestText := null.String{"", false}

	if ret != nil && ret.Hits != nil && ret.Hits.Hits != nil {
		if withHighlights {
			// --- Highlights via ES9 single searches ---
			highlightsLangs := query.LanguageOrder
			if !shouldMergeResults {
				highlightsLangs = []string{currentLang}
			}

			type highlightJob struct {
				opts SearchRequestOptions
			}
			var highlightJobs []highlightJob

			for _, h := range ret.Hits.Hits {
				if h.Type == consts.SEARCH_RESULT_TWEETS_MANY && h.InnerHits != nil {
					if tweetHits, ok := h.InnerHits[consts.SEARCH_RESULT_TWEETS_MANY]; ok {
						for _, th := range tweetHits.Hits.Hits {
							highlightJobs = append(highlightJobs, highlightJob{opts: SearchRequestOptions{
								resultTypes:          []string{consts.ES_RESULT_TYPE_TWEETS},
								docIds:               []string{th.ID},
								index:                th.Index,
								query:                Query{ExactTerms: query.ExactTerms, Term: query.Term, Filters: query.Filters, LanguageOrder: highlightsLangs, Deb: query.Deb},
								sortBy:               consts.SORT_BY_RELEVANCE,
								from:                 0,
								size:                 1,
								preference:           preference,
								useHighlight:         true,
								highlightFullContent: true,
								partialHighlight:     true,
							}})
						}
					}
					continue
				}
				if h.ID == "" || strings.HasPrefix(h.Index, "intent-") {
					continue
				}

				term := query.Term
				for _, lang := range highlightsLangs {
					if filtered, ok := filteredByLang[lang]; ok {
						for _, fr := range filtered {
							if _, hasId := fr.HitIdsMap[h.ID]; hasId {
								term = fr.Term
								break
							}
						}
					}
				}

				highlightJobs = append(highlightJobs, highlightJob{opts: SearchRequestOptions{
					resultTypes:      resultTypes,
					docIds:           []string{h.ID},
					index:            h.Index,
					query:            Query{ExactTerms: query.ExactTerms, Term: term, Filters: query.Filters, LanguageOrder: highlightsLangs, Deb: query.Deb},
					sortBy:           consts.SORT_BY_RELEVANCE,
					from:             0,
					size:             1,
					preference:       preference,
					useHighlight:     true,
					partialHighlight: true,
				}})
			}

			if len(highlightJobs) > 0 {
				log.Debug("Searching for highlights and replacing original results with highlighted results.")

				var wg sync.WaitGroup
				wg.Add(len(highlightJobs))
				hlErrors := make([]error, len(highlightJobs))
				hlResults := make([]*SearchResult, len(highlightJobs))

				beforeHighlightsDoSearch := time.Now()
				for i, job := range highlightJobs {
					go func(opts SearchRequestOptions, idx int) {
						defer wg.Done()
						hlCtx, cancel := context.WithTimeout(context.TODO(), timeoutForHighlight)
						defer cancel()
						body, index, pref, err := NewESResultsSearchBody(opts)
						if err != nil {
							hlErrors[idx] = err
							return
						}
						sr, err := e.es9SingleSearch(hlCtx, body, index, pref)
						if hlCtx.Err() != nil {
							hlErrors[idx] = hlCtx.Err()
						} else {
							hlErrors[idx] = err
						}
						hlResults[idx] = sr
					}(job.opts, i)
				}
				wg.Wait()
				e.timeTrack(beforeHighlightsDoSearch, consts.LAT_DOSEARCH_MULTISEARCHHIGHLIGHTSDO)

				for i, hlResult := range hlResults {
					if hlErrors[i] == context.DeadlineExceeded {
						continue
					}
					if hlErrors[i] != nil {
						return nil, errors.Wrap(hlErrors[i], "ES9Engine.DoSearch - Error highlight search.")
					}
					if !haveHits(hlResult) {
						continue
					}
					for _, hr := range hlResult.Hits.Hits {
						for i2, h := range ret.Hits.Hits {
							if h.ID == hr.ID {
								ret.Hits.Hits[i2] = hr
								ret.Hits.Hits[i2].Score = h.Score
							} else if h.Type == consts.SEARCH_RESULT_TWEETS_MANY && h.InnerHits != nil {
								if tweetHits, ok := h.InnerHits[consts.SEARCH_RESULT_TWEETS_MANY]; ok {
									for k, th := range tweetHits.Hits.Hits {
										if th.ID == hr.ID {
											tweetHits.Hits.Hits[k] = hr
										}
									}
								}
							}
						}
					}
				}
			}
		}

		// Prepare results for client.
		for _, hit := range ret.Hits.Hits {
			if hit.Type == consts.SEARCH_RESULT_TWEETS_MANY {
				err = e.NativizeTweetsHitForClient(hit, consts.SEARCH_RESULT_TWEETS_MANY)
			} else if hit.Type != consts.GRAMMAR_TYPE_LANDING_PAGE {
				var src es.Result
				if err = json.Unmarshal(*hit.Source, &src); err != nil {
					log.Errorf("ES9Engine.DoSearch - cannot unmarshal source.")
					continue
				}
				src.TypedUids = nil
				if src.ResultType == consts.ES_RESULT_TYPE_SOURCES {
					if src.FullTitle != "" {
						src.Title = src.FullTitle
						src.FullTitle = ""
					}
					if hit.Highlight != nil {
						if ft, ok := hit.Highlight["full_title"]; ok {
							if len(ft) > 0 && ft[0] != "" {
								hit.Highlight["title"] = ft
								hit.Highlight["full_title"] = nil
							}
						}
					}
				}
				nsrc, err := json.Marshal(src)
				if err != nil {
					log.Errorf("ES9Engine.DoSearch - cannot marshal source with title correction.")
					continue
				}
				hit.Source = (*json.RawMessage)(&nsrc)
			}
			if hit.Highlight == nil {
				hit.Highlight = SearchHitHighlight{}
			}
		}

		if checkTypo && (ret.Hits.MaxScore == nil || *ret.Hits.MaxScore < consts.MIN_RESULTS_SCORE_TO_IGNOGRE_TYPO_SUGGEST) {
			suggestText = <-suggestChannel
		}
		return &QueryResult{ret, suggestText, currentLang, nil}, err
	}

	if checkTypo {
		suggestText = <-suggestChannel
	}
	if len(mainResponses) > 0 {
		return &QueryResult{mainResponses[0], suggestText, currentLang, nil}, err
	}
	return nil, errors.Wrap(err, "ES9Engine.DoSearch - No responses from search.")
}

// isContextError returns true if the error is a context cancellation or deadline.
func isContextError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, context.DeadlineExceeded.Error()) ||
		strings.Contains(s, context.Canceled.Error())
}
