package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/volatiletech/null.v6"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ESEngine struct {
	esc              *elastic.Client
	mdb              *sql.DB
	cache            cache.CacheManager
	ExecutionTimeLog *TimeLogMap
	grammars         Grammars
	TokensCache      *TokensCache
	variables        VariablesV2
}

type ClassificationIntent struct {
	// Fields from result.
	ResultType string `json:"result_type"`
	MDB_UID    string `json:"mdb_uid"`
	Title      string `json:"title"`
	FullTitle  string `json:"full_title"`

	// Intent fields.
	ContentType    string                    `json:"content_type"`
	Exist          bool                      `json:"exist"`
	Score          *float64                  `json:"score,omitempty"`
	Explanation    elastic.SearchExplanation `json:"explanation,omitempty"`
	MaxScore       *float64                  `json:"max_score,omitempty"`
	MaxExplanation elastic.SearchExplanation `json:"max_explanation,omitempty"`
}

type TimeLogMap struct {
	mx sync.Mutex
	m  map[string]time.Duration
}

func NewTimeLogMap() *TimeLogMap {
	return &TimeLogMap{
		m: make(map[string]time.Duration),
	}
}

func (c *TimeLogMap) Load(key string) (time.Duration, bool) {
	c.mx.Lock()
	defer c.mx.Unlock()
	val, ok := c.m[key]
	return val, ok
}

func (c *TimeLogMap) Store(key string, value time.Duration) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.m[key] = value
	fmt.Printf("%s - %s\n", key, value.String())
}

func (c *TimeLogMap) ToMap() map[string]time.Duration {
	c.mx.Lock()
	defer c.mx.Unlock()
	copyMap := map[string]time.Duration{}
	for k, v := range c.m {
		copyMap[k] = v
	}
	return copyMap
}

type byRelevance []*elastic.SearchHit
type byNewerToOlder []*elastic.SearchHit
type byOlderToNewer []*elastic.SearchHit
type bySourceFirst []*elastic.SearchHit

func (s byRelevance) Len() int {
	return len(s)
}
func (s byRelevance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byRelevance) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_RELEVANCE)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

func (s byNewerToOlder) Len() int {
	return len(s)
}
func (s byNewerToOlder) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byNewerToOlder) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_NEWER_TO_OLDER)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

func (s byOlderToNewer) Len() int {
	return len(s)
}
func (s byOlderToNewer) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byOlderToNewer) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_OLDER_TO_NEWER)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

func (s bySourceFirst) Len() int {
	return len(s)
}
func (s bySourceFirst) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s bySourceFirst) Less(i, j int) bool {
	res, err := compareHits(s[i], s[j], consts.SORT_BY_SOURCE_FIRST)
	if err != nil {
		panic(fmt.Sprintf("compareHits error: %s", err))
	}
	return res
}

// TODO: All interactions with ES should be throttled to prevent downstream pressure

func NewESEngine(esc *elastic.Client, db *sql.DB, cache cache.CacheManager /*, grammars Grammars*/, tc *TokensCache, variables VariablesV2) *ESEngine {
	return &ESEngine{
		esc:              esc,
		mdb:              db,
		cache:            cache,
		ExecutionTimeLog: NewTimeLogMap(),
		//grammars:         grammars,
		TokensCache: tc,
		variables:   variables,
	}
}

func SuggestionHasOptions(ss elastic.SearchSuggest) bool {
	for _, v := range ss {
		for _, s := range v {
			if len(s.Options) > 0 {
				return true
			}
		}
	}
	return false
}

func (e *ESEngine) GetSuggestions(ctx context.Context, query Query, preference string) (interface{}, error) {
	beforeGetSuggest := time.Now()
	defer func() { e.timeTrack(beforeGetSuggest, consts.LAT_GETSUGGESTIONS) }()

	// Run grammar suggestions in parallel.
	grammarSuggestionsChannel := make(chan map[string][]VariablesByPhrase)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("ESEngine.GetSuggestions - Panic adding intents: %+v", err)
				grammarSuggestionsChannel <- make(map[string][]VariablesByPhrase)
			}
		}()
		beforeSuggestSuggest := time.Now()
		grammarSuggestions, err := e.SuggestGrammarsV2(&query, preference)
		if err != nil {
			log.Errorf("ESEngine.GetSuggestions - Error adding intents: %+v", err)
			grammarSuggestionsChannel <- make(map[string][]VariablesByPhrase)
		} else {
			grammarSuggestionsChannel <- grammarSuggestions
		}
		e.timeTrack(beforeSuggestSuggest, consts.LAT_SUGGEST_SUGGESTIONS)
	}()

	multiSearchService := e.esc.MultiSearch()
	requests := NewResultsSuggestRequests([]string{
		consts.ES_RESULT_TYPE_UNITS,
		consts.ES_RESULT_TYPE_COLLECTIONS,
		consts.ES_RESULT_TYPE_TAGS,
		consts.ES_RESULT_TYPE_SOURCES,
		consts.ES_RESULT_TYPE_BLOG_POSTS,
		//consts.ES_RESULT_TYPE_TWEETS,
	}, query, preference)
	multiSearchService.Add(requests...)

	// Actual call to elastic
	beforeMssDo := time.Now()
	mr, err := multiSearchService.Do(ctx)
	e.timeTrack(beforeMssDo, consts.LAT_GETSUGGESTIONS_MULTISEARCHDO)
	if err != nil {
		// don't kill entire request if ctx was cancelled
		if ue, ok := err.(*url.Error); ok {
			if ue.Err == context.DeadlineExceeded || ue.Err == context.Canceled {
				if ue.Err == context.DeadlineExceeded {
					log.Warn("ESEngine.GetSuggestions - ctx cancelled - deadline.")
				}
				if ue.Err == context.Canceled {
					log.Warn("ESEngine.GetSuggestions - ctx cancelled - canceled.")
				}
				return nil, nil
			}
		}
		return nil, errors.Wrap(err, "ESEngine.GetSuggestions")
	}

	//  Nativize response to client - Replace title with full title
	for _, r := range mr.Responses {
		for key := range r.Suggest {
			for j := range r.Suggest[key] {
				for opIdx, op := range r.Suggest[key][j].Options {
					var src es.Result
					err = json.Unmarshal(*op.Source, &src)
					if err != nil {
						log.Errorf("ESEngine.GetSuggestions - cannot unmarshal source.")
						continue
					}
					if src.ResultType == consts.ES_RESULT_TYPE_SOURCES && src.FullTitle != "" {
						src.Title = src.FullTitle
						src.FullTitle = ""
						nsrc, err := json.Marshal(src)
						if err != nil {
							log.Errorf("ESEngine.GetSuggestions - cannot marshal source with title correction.")
							continue
						}
						r.Suggest[key][j].Options[opIdx].Source = (*json.RawMessage)(&nsrc)
					}
				}
			}
		}
	}

	// Merge with grammar suggestions.
	var grammarSuggestions map[string][]VariablesByPhrase
	grammarSuggestions = <-grammarSuggestionsChannel

	for i, lang := range query.LanguageOrder {
		if langSuggestions, ok := grammarSuggestions[lang]; ok && len(langSuggestions) > 0 && mr != nil && len(mr.Responses) > i {
			r := mr.Responses[i]
			if r.Suggest == nil {
				r.Suggest = make(map[string][]elastic.SearchSuggestion)
			}
			if len(r.Suggest) == 0 {
				r.Suggest["title_suggest"] = []elastic.SearchSuggestion{}
			}
			for key := range r.Suggest {
				for j := range r.Suggest[key] {
					for _, variablesByPhrase := range langSuggestions {
						for suggestion, _ := range variablesByPhrase {
							source := struct {
								Title      string `json:"title"`
								ResultType string `json:"result_type"`
							}{Title: suggestion, ResultType: consts.GRAMMAR_TYPE_LANDING_PAGE}
							sourceRawMessage, err := json.Marshal(source)
							if err != nil {
								return nil, err
							}
							raw := json.RawMessage(sourceRawMessage)
							option := elastic.SearchSuggestionOption{
								Text:   suggestion,
								Source: &raw,
							}
							r.Suggest[key][j].Options = append([]elastic.SearchSuggestionOption{option}, r.Suggest[key][j].Options...)
						}
					}
				}
			}
		}
	}

	// Debug. Tokens stats: fmt.Printf("--------------Count: %d, First: %d\n", countVM, countVMFirst)

	// Process response
	sRes := (*elastic.SearchResult)(nil)
	for _, r := range mr.Responses {
		if r != nil && SuggestionHasOptions(r.Suggest) {
			sRes = r
			break
		}
	}

	if sRes == nil && len(mr.Responses) > 0 {
		sRes = mr.Responses[0]
	}

	return sRes, nil
}

func (e *ESEngine) IntentsToResults(query *Query) (error, map[string]*elastic.SearchResult) {
	srMap := make(map[string]*elastic.SearchResult)
	for _, lang := range query.LanguageOrder {
		sh := &elastic.SearchHits{TotalHits: 0}
		sr := &elastic.SearchResult{Hits: sh}
		srMap[lang] = sr
	}

	// Limit ClassificationIntents to top MAX_CLASSIFICATION_INTENTS
	boostClassificationScore := func(intentValue *ClassificationIntent) float64 {
		// Boost up to 33% for exact match, i.e., for score / max score of 1.0.
		return *intentValue.Score * (3.0 + *intentValue.Score / *intentValue.MaxScore) / 3.0
	}
	scores := []float64{}
	for i := range query.Intents {
		// Convert intent to result with score.
		if intentValue, ok := query.Intents[i].Value.(ClassificationIntent); ok && intentValue.Exist {
			scores = append(scores, boostClassificationScore(&intentValue))
		}
	}
	sort.Float64s(scores)
	minClassificationScore := float64(0)
	if len(scores) > 0 {
		scores = scores[utils.MaxInt(0, len(scores)-consts.MAX_CLASSIFICATION_INTENTS):]
		minClassificationScore = scores[0]
	}

	// log.Infof("IntentsToResults - %d intents.", len(query.Intents))
	for _, intent := range query.Intents {
		// Convert intent to result with score.
		if intentValue, ok := intent.Value.(ClassificationIntent); ok {
			boostedScore := float64(0.0)
			if intentValue.Exist {
				sh := srMap[intent.Language].Hits
				sh.TotalHits++
				boostedScore = boostClassificationScore(&intentValue)
				if boostedScore < minClassificationScore {
					continue // Skip classificaiton intents with score lower then first MAX_CLASSIFICATION_INTENTS
				}
				if sh.MaxScore != nil {
					maxScore := math.Max(*sh.MaxScore, boostedScore)
					sh.MaxScore = &maxScore
				} else {
					sh.MaxScore = &boostedScore
				}
				intentHit := &elastic.SearchHit{}
				intentHit.Explanation = &intentValue.Explanation
				intentHit.Score = &boostedScore
				intentHit.Index = consts.INTENT_INDEX_BY_TYPE[intent.Type]
				intentHit.Type = consts.INTENT_HIT_TYPE_BY_CT[intentValue.ContentType]
				source, err := json.Marshal(intentValue)
				if err != nil {
					return err, nil
				}
				intentHit.Source = (*json.RawMessage)(&source)
				sh.Hits = append(sh.Hits, intentHit)
			}
			// log.Infof("Added intent %s %s %s boost score:%f exist:%t", intentValue.Title, intent.Type, intent.Language, boostedScore, intentValue.Exist)
		}
		if intentValue, ok := intent.Value.(GrammarIntent); ok {
			sh := srMap[intent.Language].Hits
			sh.TotalHits++
			boostedScore := float64(2000.0)
			if intentValue.Score > 0 {
				boostedScore = intentValue.Score
			}
			if sh.MaxScore != nil {
				maxScore := math.Max(*sh.MaxScore, boostedScore)
				sh.MaxScore = &maxScore
			} else {
				sh.MaxScore = &boostedScore
			}
			intentHit := &elastic.SearchHit{}
			if intentValue.Explanation != nil {
				intentHit.Explanation = intentValue.Explanation
			}
			intentHit.Score = &boostedScore
			intentHit.Index = consts.GRAMMAR_INDEX
			intentHit.Type = intent.Type
			source, err := json.Marshal(intentValue)
			if err != nil {
				return err, nil
			}
			intentHit.Source = (*json.RawMessage)(&source)
			sh.Hits = append(sh.Hits, intentHit)
		}
	}
	return nil, srMap
}

func haveHits(r *elastic.SearchResult) bool {
	return r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0
}

func score(score *float64) float64 {
	if score == nil {
		return 0
	} else {
		return *score
	}
}

func compareHits(h1 *elastic.SearchHit, h2 *elastic.SearchHit, sortBy string) (bool, error) {
	if sortBy == consts.SORT_BY_RELEVANCE {
		return score(h1.Score) > score(h2.Score), nil
	} else if sortBy == consts.SORT_BY_SOURCE_FIRST {
		var rt1, rt2 es.ResultType
		if err := json.Unmarshal(*h1.Source, &rt1); err != nil {
			return false, err
		}
		if err := json.Unmarshal(*h2.Source, &rt2); err != nil {
			return false, err
		}
		// Order by sources first, then be score.
		return rt1.ResultType == consts.ES_RESULT_TYPE_SOURCES && rt2.ResultType != consts.ES_RESULT_TYPE_SOURCES || score(h1.Score) > score(h2.Score), nil
	} else {
		var ed1, ed2 es.EffectiveDate
		if err := json.Unmarshal(*h1.Source, &ed1); err != nil {
			return false, err
		}
		if err := json.Unmarshal(*h2.Source, &ed2); err != nil {
			return false, err
		}
		if ed1.EffectiveDate == nil {
			ed1.EffectiveDate = &utils.Date{time.Time{}}
		}
		if ed2.EffectiveDate == nil {
			ed2.EffectiveDate = &utils.Date{time.Time{}}
		}
		if sortBy == consts.SORT_BY_OLDER_TO_NEWER {
			// Oder by older to newer, break ties using score.
			return ed2.EffectiveDate.Time.After(ed1.EffectiveDate.Time) ||
				ed2.EffectiveDate.Time.Equal(ed1.EffectiveDate.Time) && score(h1.Score) > score(h2.Score), nil
		} else {
			//log.Infof("%+v %+v %+v %+v", ed1, ed2, h1, h2)
			// Order by newer to older, break ties using score.
			return ed2.EffectiveDate.Time.Before(ed1.EffectiveDate.Time) ||
				ed2.EffectiveDate.Time.Equal(ed1.EffectiveDate.Time) && score(h1.Score) > score(h2.Score), nil
		}
	}
}

func joinResponses(sortBy string, from int, size int, results ...*elastic.SearchResult) (*elastic.SearchResult, error) {
	if len(results) == 0 {
		return nil, nil
	}
	// Concatenate all result hits to single slice.
	concatenated := make([]*elastic.SearchHit, 0)
	for _, result := range results {
		concatenated = append(concatenated, result.Hits.Hits...)
	}

	// Apply sorting.
	if sortBy == consts.SORT_BY_RELEVANCE {
		sort.Stable(byRelevance(concatenated))
	} else if sortBy == consts.SORT_BY_OLDER_TO_NEWER {
		sort.Stable(byOlderToNewer(concatenated))
	} else if sortBy == consts.SORT_BY_NEWER_TO_OLDER {
		sort.Stable(byNewerToOlder(concatenated))
	} else if sortBy == consts.SORT_BY_SOURCE_FIRST {
		sort.Stable(bySourceFirst(concatenated))
	}

	// Filter by relevant page.
	concatenated = concatenated[from:utils.Min(from+size, len(concatenated))]

	// Take arbitrary result to use as base and set it's hits.
	// TODO: Rewrite this to be cleaner.
	result := results[0]

	// Get hits count and max score
	totalHits := int64(0)
	var maxScore float64
	if result.Hits.MaxScore != nil {
		maxScore = *result.Hits.MaxScore
	} else {
		maxScore = 0
	}
	for _, result := range results {
		totalHits += result.Hits.TotalHits
		if sortBy == consts.SORT_BY_RELEVANCE {
			if result.Hits.MaxScore != nil {
				maxScore = math.Max(maxScore, *result.Hits.MaxScore)
			}
		}
	}

	result.Hits.Hits = concatenated
	result.Hits.TotalHits = totalHits
	result.Hits.MaxScore = &maxScore

	return result, nil
}

func (e *ESEngine) timeTrack(start time.Time, operation string) {
	elapsed := time.Since(start)
	e.ExecutionTimeLog.Store(operation, elapsed)
}

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string, checkTypo bool, timeoutForHighlight time.Duration) (*QueryResult, error) {
	defer e.timeTrack(time.Now(), consts.LAT_DOSEARCH)

	suggestChannel := make(chan null.String)

	// Search grammars in parallel to native search.
	grammarsChannel := make(chan []Intent)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("ESEngine.DoSearch - Panic searching grammars: %+v", err)
				grammarsChannel <- []Intent{}
			}
		}()
		if grammars, err := e.SearchGrammarsV2(&query, preference); err != nil {
			log.Errorf("ESEngine.DoSearch - Error searching grammars: %+v", err)
			grammarsChannel <- []Intent{}
		} else {
			grammarsChannel <- grammars
		}
	}()

	// Search tweets in parallel to native search.
	tweetsByLangChannel := make(chan map[string]*elastic.SearchResult)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("ESEngine.DoSearch - Panic searching tweets: %+v", err)
				tweetsByLangChannel <- map[string]*elastic.SearchResult{}
			}
		}()
		if tweetsByLang, err := e.SearchTweets(query, sortBy, from, size, preference); err != nil {
			log.Errorf("ESEngine.DoSearch - Error searching tweets: %+v", err)
			tweetsByLangChannel <- map[string]*elastic.SearchResult{}
		} else {
			tweetsByLangChannel <- tweetsByLang
		}
	}()

	if checkTypo {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("ESEngine.GetTypoSuggest - Panic getting typo suggest: %+v", err)
					suggestChannel <- null.String{"", false}
				}
			}()
			if suggestText, err := e.GetTypoSuggest(query); err != nil {
				log.Errorf("ESEngine.GetTypoSuggest - Error getting typo suggest: %+v", err)
				suggestChannel <- null.String{"", false}
			} else {
				suggestChannel <- suggestText
			}
		}()
	}

	var resultTypes []string
	if sortBy == consts.SORT_BY_NEWER_TO_OLDER || sortBy == consts.SORT_BY_OLDER_TO_NEWER {
		resultTypes = make([]string, 0)
		for _, str := range consts.ES_SEARCH_RESULT_TYPES {
			if str != consts.ES_RESULT_TYPE_COLLECTIONS {
				resultTypes = append(resultTypes, str)
			}
		}
	} else {
		resultTypes = consts.ES_SEARCH_RESULT_TYPES
	}

	intents, err := e.AddIntents(&query, preference, consts.INTENTS_SEARCH_COUNT, sortBy)
	if err != nil {
		log.Errorf("ESEngine.DoSearch - Error adding intents: %+v", err)
	}

	// Filter out duplicates of regular results and intents carousel results
	filterOutCUSources := make([]string, 0)
	for _, intent := range intents {
		if intent.Type == consts.INTENT_TYPE_SOURCE {
			if intentValue, ok := intent.Value.(ClassificationIntent); ok && intentValue.Exist {
				// This is not a perfect solution since we dont know yet what is the currentLang and we filter by all languages
				filterOutCUSources = append(filterOutCUSources, intentValue.MDB_UID)
				log.Infof("MDB_UID added to filterOutCUSources: %s.", intentValue.MDB_UID)
			}
		}
	}

	multiSearchService := e.esc.MultiSearch()
	requests, err := NewResultsSearchRequests(
		SearchRequestOptions{
			resultTypes:        resultTypes,
			index:              "",
			query:              query,
			sortBy:             sortBy,
			from:               0,
			size:               from + size,
			preference:         preference,
			useHighlight:       false,
			partialHighlight:   false,
			filterOutCUSources: filterOutCUSources})
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error multisearch Do on creating requests.")
	}
	multiSearchService.Add(requests...)

	// Do search.
	beforeDoSearch := time.Now()
	mr, err := multiSearchService.Do(context.TODO())
	e.timeTrack(beforeDoSearch, consts.LAT_DOSEARCH_MULTISEARCHDO)
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error multisearch Do.")
	}
	shouldMergeResults := false
	//  Right now we are testing the language results merge for Spanish UI only
	for _, lang := range query.LanguageOrder {
		if lang == consts.LANG_SPANISH {
			shouldMergeResults = true
			break
		}
	}
	if len(mr.Responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
	}
	resultsByLang := make(map[string][]*elastic.SearchResult)

	// Responses are ordered by language by index, i.e., for languages [bg, ru, en].
	// We want the first matching language that has at least any result.
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		if haveHits(currentResults) {
			lang := query.LanguageOrder[i]
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], currentResults)
		}
	}

	query.Intents = append(query.Intents, intents...)
	query.Intents = append(query.Intents, <-grammarsChannel...)

	log.Debugf("Intents: %+v", query.Intents)

	// Convert intents and grammars to results.
	err, intentResultsMap := e.IntentsToResults(&query)
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error adding intents to results.")
	}
	for lang, intentResults := range intentResultsMap {
		if haveHits(intentResults) {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], intentResults)
		}
	}

	tweetsByLang := <-tweetsByLangChannel
	for lang, tweets := range tweetsByLang {
		if _, ok := resultsByLang[lang]; !ok {
			resultsByLang[lang] = make([]*elastic.SearchResult, 0)
		}
		resultsByLang[lang] = append(resultsByLang[lang], tweets)
	}

	var currentLang string
	results := make([]*elastic.SearchResult, 0)
	for _, lang := range query.LanguageOrder {
		if r, ok := resultsByLang[lang]; ok {
			if shouldMergeResults {
				results = append(results, resultsByLang[lang]...)
			} else {
				results = r
				currentLang = lang
				break
			}
		}
	}

	ret, err := joinResponses(sortBy, from, size, results...)

	suggestText := null.String{"", false}

	if ret != nil && ret.Hits != nil && ret.Hits.Hits != nil {

		// Preparing highlights search.
		mssHighlights := e.esc.MultiSearch()
		highlightRequestAdded := false

		highlightsLangs := query.LanguageOrder
		if !shouldMergeResults {
			highlightsLangs = []string{currentLang}
		}

		for _, h := range ret.Hits.Hits {

			if h.Type == consts.SEARCH_RESULT_TWEETS_MANY && h.InnerHits != nil {
				if tweetHits, ok := h.InnerHits[consts.SEARCH_RESULT_TWEETS_MANY]; ok {
					for _, th := range tweetHits.Hits.Hits {
						req, err := NewResultsSearchRequest(
							SearchRequestOptions{
								resultTypes:          []string{consts.ES_RESULT_TYPE_TWEETS},
								docIds:               []string{th.Id},
								index:                th.Index,
								query:                Query{ExactTerms: query.ExactTerms, Term: query.Term, Filters: query.Filters, LanguageOrder: highlightsLangs, Deb: query.Deb},
								sortBy:               consts.SORT_BY_RELEVANCE,
								from:                 0,
								size:                 1,
								preference:           preference,
								useHighlight:         true,
								highlightFullContent: true,
								partialHighlight:     true})
						if err != nil {
							return nil, errors.Wrap(err, "ESEngine.DoSearch - Error creating tweets request in multisearch Do.")
						}
						mssHighlights.Add(req)

						highlightRequestAdded = true
					}
				}
				continue
			}
			if h.Id == "" || strings.HasPrefix(h.Index, "intent-") {
				// Bypass intent
				continue
			}

			// We use multiple search request because we saw that a single request
			// filtered by id's list take more time than multiple requests.
			req, err := NewResultsSearchRequest(
				SearchRequestOptions{
					resultTypes:      resultTypes,
					docIds:           []string{h.Id},
					index:            h.Index,
					query:            Query{ExactTerms: query.ExactTerms, Term: query.Term, Filters: query.Filters, LanguageOrder: highlightsLangs, Deb: query.Deb},
					sortBy:           consts.SORT_BY_RELEVANCE,
					from:             0,
					size:             1,
					preference:       preference,
					useHighlight:     true,
					partialHighlight: true})
			if err != nil {
				return nil, errors.Wrap(err, "ESEngine.DoSearch - Error creating highlight request in multisearch Do.")
			}
			mssHighlights.Add(req)

			highlightRequestAdded = true
		}

		if highlightRequestAdded {

			log.Debug("Searching for highlights and replacing original results with highlighted results.")

			beforeHighlightsDoSearch := time.Now()
			highlightCtx, cancelFn := context.WithTimeout(context.Background(), timeoutForHighlight)
			defer cancelFn()
			mr, err := mssHighlights.Do(highlightCtx)
			e.timeTrack(beforeHighlightsDoSearch, consts.LAT_DOSEARCH_MULTISEARCHHIGHLIGHTSDO)
			if err != nil {
				switch highlightCtx.Err() {
				case context.DeadlineExceeded:
					log.Error(err, "ESEngine.DoSearch - DeadlineExceeded mssHighlights Do.")
					mr = new(elastic.MultiSearchResult)
				default:
					return nil, errors.Wrap(err, "ESEngine.DoSearch - Error mssHighlights Do.")
				}
			}

			for _, highlightedResults := range mr.Responses {
				if highlightedResults.Error != nil {
					log.Warnf("%+v", highlightedResults.Error)
					return nil, errors.New(fmt.Sprintf("Failed multi get highlights: %+v", highlightedResults.Error))
				}
				if haveHits(highlightedResults) {
					for _, hr := range highlightedResults.Hits.Hits {
						for i, h := range ret.Hits.Hits {
							if h.Id == hr.Id {
								//  Replacing original search result with highlighted result.
								ret.Hits.Hits[i] = hr
							} else if h.Type == consts.SEARCH_RESULT_TWEETS_MANY && h.InnerHits != nil {
								if tweetHits, ok := h.InnerHits[consts.SEARCH_RESULT_TWEETS_MANY]; ok {
									for k, th := range tweetHits.Hits.Hits {
										if th.Id == hr.Id {
											//  Replacing original tweet result with highlighted tweet result.
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

		for i := 0; i < len(ret.Hits.Hits); i++ {
			hit := ret.Hits.Hits[i]
			if hit.Type == consts.SEARCH_RESULT_TWEETS_MANY {
				err = e.NativizeTweetsHitForClient(hit, consts.SEARCH_RESULT_TWEETS_MANY)
			} else if hit.Type == consts.GRAMMAR_TYPE_LANDING_PAGE {
				// TBD same logic for locations
				var grammarIntent GrammarIntent
				err = json.Unmarshal(*hit.Source, &grammarIntent)
				if err != nil {
					log.Errorf("ESEngine.DoSearch - cannot unmarshal source to es.GrammarIntent.")
					continue
				}
				if grammarIntent.LandingPage == consts.GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS && grammarIntent.FilterValues != nil {
					var year string
					var holiday string
					for _, fv := range grammarIntent.FilterValues {
						if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_HOLIDAYS] {
							holiday = fv.Value

						} else if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_YEAR] {
							year = fv.Value
						}
						if year != "" && holiday != "" {
							break
						}
					}
					if year != "" && holiday != "" { // TBD improve decision - base ALSO on cache!
						// Since the LandingPage has only one collection item, convert the LandingPage result to the single collection hit
						log.Infof("Converting LandingPage of %s %s to a single collection.", holiday, year)
						ret.Hits.Hits[i], err = e.HolidaysLandingPageToCollectionHit(year, holiday, grammarIntent.Score)
						if err != nil {
							log.Warnf("%+v", err)
							return nil, errors.New(fmt.Sprintf("HolidaysLandingPageToCollectionHit Failed: %+v", err))
						}
					}
				}
				// TBD ConventionsLandingPageToCollectionHit
			} else {
				var src es.Result
				err = json.Unmarshal(*hit.Source, &src)
				if err != nil {
					log.Errorf("ESEngine.DoSearch - cannot unmarshal source to es.Result.")
					continue
				}
				if src.ResultType == consts.ES_RESULT_TYPE_SOURCES {
					//  Replace title with full title
					if src.FullTitle != "" {
						src.Title = src.FullTitle
						src.FullTitle = ""
						nsrc, err := json.Marshal(src)
						if err != nil {
							log.Errorf("ESEngine.DoSearch - cannot marshal source with title correction.")
							continue
						}
						hit.Source = (*json.RawMessage)(&nsrc)
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
			}

			//  Temp. workround until client could handle null values in Highlight fields (WIP by David)
			if hit.Highlight == nil {
				hit.Highlight = elastic.SearchHitHighlight{}
			}
		}

		if checkTypo && (ret.Hits.MaxScore == nil || *ret.Hits.MaxScore < consts.MIN_RESULTS_SCORE_TO_IGNOGRE_TYPO_SUGGEST) {
			suggestText = <-suggestChannel
		}
		return &QueryResult{ret, suggestText, currentLang}, err
	}

	if checkTypo {
		suggestText = <-suggestChannel
	}

	if len(mr.Responses) > 0 {
		// This happens when there are no responses with hits.
		// Note, we don't filter here intents by language.
		return &QueryResult{mr.Responses[0], suggestText, currentLang}, err
	}
	return nil, errors.Wrap(err, "ESEngine.DoSearch - No responses from multi search.")
}
