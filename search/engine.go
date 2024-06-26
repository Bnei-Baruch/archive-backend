package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ESEngine struct {
	esc               *elastic.Client
	mdb               *sql.DB
	cache             cache.CacheManager
	ExecutionTimeLog  *TimeLogMap
	grammars          Grammars
	TokensCache       *TokensCache
	variables         VariablesV2
	searchResultTypes []string
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

type FilteredSearchResult struct {
	Term                     string
	PreserveTermForHighlight bool //Use the term as highlight term even if we have same hit result from regular search
	HitIdsMap                map[string]bool
	Results                  []*elastic.SearchResult
	MaxScore                 *float64
	ProgramCollection        *string
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

func NewESEngine(esc *elastic.Client, db *sql.DB, cache cache.CacheManager /*, grammars Grammars*/, tc *TokensCache, variables VariablesV2, searchResultTypes []string) *ESEngine {
	return &ESEngine{
		esc:              esc,
		mdb:              db,
		cache:            cache,
		ExecutionTimeLog: NewTimeLogMap(),
		//grammars:         grammars,
		TokensCache:       tc,
		variables:         variables,
		searchResultTypes: searchResultTypes,
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

func FloatOrNil(f *float64) string {
	if f == nil {
		return "nil"
	}
	return fmt.Sprintf("%.2f", *f)
}

func LogIfDeb(q *Query, l string) {
	if q.Deb {
		log.Info(l)
	}
}

func IntentsToStringDebug(label string, intents []Intent) string {
	parts := []string{fmt.Sprintf("--- %s ----", label), fmt.Sprintf("%d items", len(intents))}
	for k := range intents {
		parts = append(parts, fmt.Sprintf("%d: %+v", k, IntentToStringDebug(intents[k])))
	}
	parts = append(parts, fmt.Sprintf("--- END %s ----", label))
	return strings.Join(parts, "\n")
}

func ResultsSliceMapToStringDebug(label string, m map[string][]*elastic.SearchResult, limit int) string {
	parts := []string{fmt.Sprintf("--- %s ----", label), fmt.Sprintf("%d items", len(m))}
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s: %d", k, len(v)))
		for j, r := range v {
			parts = append(parts, fmt.Sprintf("\t%d: %s", j, ResultToStringDebug(r, limit)))
		}
	}
	parts = append(parts, fmt.Sprintf("--- END %s ----", label))
	return strings.Join(parts, "\n")
}

func ResultsMapToStringDebug(label string, m map[string]*elastic.SearchResult, limit int) string {
	parts := []string{fmt.Sprintf("--- %s ----", label), fmt.Sprintf("%d items", len(m))}
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s: %s", k, ResultToStringDebug(v, limit)))
	}
	parts = append(parts, fmt.Sprintf("--- END %s ----", label))
	return strings.Join(parts, "\n")
}

func IntentToStringDebug(intent Intent) string {
	str := fmt.Sprintf("INTENT: Type: %s Language: %s ValueType: %s", intent.Type, intent.Language, reflect.TypeOf(intent.Value).Name())
	if v, ok := intent.Value.(ClassificationIntent); ok {
		str = fmt.Sprintf(
			"%s (%s - %s - [%s]) (%s Exist:%s Score:%s Max:%s)",
			str, v.ResultType, v.MDB_UID, v.Title, v.ContentType,
			strconv.FormatBool(v.Exist), FloatOrNil(v.Score), FloatOrNil(v.MaxScore))
	} else if v, ok := intent.Value.(GrammarIntent); ok {
		singleHitMdbUid := "mdb uid nil"
		if v.SingleHitMdbUid != nil {
			singleHitMdbUid = fmt.Sprintf("mdb uid %s", *v.SingleHitMdbUid)
		}
		str = fmt.Sprintf(
			"%.2f %s %s %s %+v",
			v.Score, v.LandingPage, singleHitMdbUid, HitToStringDebug("", v.SingleHit), v.FilterValues)
	}
	return str
}

func ResultToStringDebug(r *elastic.SearchResult, limit int) string {
	if r == nil || r.Hits == nil {
		return "Results or Hits are nil."
	}
	parts := []string{fmt.Sprintf("%d hits.", len(r.Hits.Hits))}
	// log.Infof("\t\t%d: %+v", j, r.Hits)
	for k := range r.Hits.Hits {
		if k >= limit {
			parts = append(parts, "\t\t\t...")
			break
		}
		parts = append(parts, HitToStringDebug(fmt.Sprintf("\t\t\t%d: ", k), r.Hits.Hits[k]))
	}
	return strings.Join(parts, "\n")
}

func HitToStringDebug(prefix string, h *elastic.SearchHit) string {
	if h == nil {
		return "nil"
	}
	parts := []string(nil)
	var src es.Result
	if err := json.Unmarshal(*h.Source, &src); err != nil {
		parts = append(parts, fmt.Sprintf("%s%s %s %s %s failed unmarshling source.", prefix, FloatOrNil(h.Score), h.Index, h.Id, h.Type))
	} else {
		parts = append(parts, fmt.Sprintf("%s%s %s %s %s %+v", prefix, FloatOrNil(h.Score), h.Index, h.Id, h.Type, src))
	}
	return strings.Join(parts, "\n")
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
		if intentValue.MaxScore != nil {
			// Boost up to 33% for exact match, i.e., for score / max score of 1.0.
			return *intentValue.Score * (3.0 + *intentValue.Score / *intentValue.MaxScore) / 3.0
		}
		return *intentValue.Score
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
			var intentHit *elastic.SearchHit
			convertedToSingleHit := false
			if intentValue.SingleHit != nil {
				intentHit = intentValue.SingleHit
				convertedToSingleHit = true
			} else {
				intentHit = &elastic.SearchHit{}
			}
			if intentValue.Explanation != nil {
				intentHit.Explanation = intentValue.Explanation
			}
			intentHit.Score = &boostedScore
			if !convertedToSingleHit {
				intentHit.Index = consts.GRAMMAR_INDEX
				intentHit.Type = intent.Type
				source, err := json.Marshal(intentValue)
				if err != nil {
					return err, nil
				}
				intentHit.Source = (*json.RawMessage)(&source)
			}
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

	// Keep only unique results by MDB_UID (additional results with a duplicate MDB_UID might be added by Grammar).
	unique := uniqueHitsByMdbUid(
		concatenated,
		[]string{
			consts.INTENT_INDEX_TAG,
			consts.INTENT_INDEX_SOURCE,
		},
		[]string{
			consts.HT_LESSONS_SERIES,
			consts.SEARCH_RESULT_LESSONS_SERIES_BY_TAG,
			consts.SEARCH_RESULT_LESSONS_SERIES_BY_SOURCE,
		},
	)

	// Apply sorting.
	if sortBy == consts.SORT_BY_RELEVANCE {
		sort.Stable(byRelevance(unique))
	} else if sortBy == consts.SORT_BY_OLDER_TO_NEWER {
		sort.Stable(byOlderToNewer(unique))
	} else if sortBy == consts.SORT_BY_NEWER_TO_OLDER {
		sort.Stable(byNewerToOlder(unique))
	} else if sortBy == consts.SORT_BY_SOURCE_FIRST {
		sort.Stable(bySourceFirst(unique))
	}

	if from >= len(unique) {
		// Edge case when we cannot calculate totalHits correctly due to many duplications of grammar and regular results (that we filter out only when loading a specific page).
		unique = []*elastic.SearchHit{}
	} else {
		// Filter by relevant page.
		unique = unique[from:utils.Min(from+size, len(unique))]
	}

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

	result.Hits.Hits = unique
	result.Hits.TotalHits = totalHits
	result.Hits.MaxScore = &maxScore

	return result, nil
}

func uniqueHitsByMdbUid(hits []*elastic.SearchHit, indexesToIgnore []string, typesToIgnore []string) []*elastic.SearchHit {
	unique := make([]*elastic.SearchHit, 0)
	mdbMap := make(map[string]*elastic.SearchHit)
	for _, hit := range hits {
		var mdbUid es.MdbUid
		if hit.Score != nil && !utils.Contains(utils.Is(indexesToIgnore), hit.Index) && !utils.Contains(utils.Is(typesToIgnore), hit.Type) {
			if err := json.Unmarshal(*hit.Source, &mdbUid); err == nil {
				if mdbUid.MDB_UID != "" {
					// Uncomment for debug
					// if dupHit, ok := mdbMap[mdbUid.MDB_UID]; ok {
					// 	log.Infof("Found duplicate of \n%f: %+v \nand \n%f: %+v", *hit.Score, hit, *dupHit.Score, dupHit)
					// }
					// We keep the result with a higher score.
					if _, ok := mdbMap[mdbUid.MDB_UID]; !ok || *hit.Score > *mdbMap[mdbUid.MDB_UID].Score {
						mdbMap[mdbUid.MDB_UID] = hit
					}
				} else {
					unique = append(unique, hit)
				}
			} else {
				log.Warnf("Unable to unmarshal source for hit ''%s.", hit.Id)
				unique = append(unique, hit)
			}
		} else {
			unique = append(unique, hit)
		}
	}
	for _, hit := range mdbMap {
		unique = append(unique, hit)
	}
	return unique
}

func (e *ESEngine) timeTrack(start time.Time, operation string) {
	elapsed := time.Since(start)
	e.ExecutionTimeLog.Store(operation, elapsed)
}

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string, checkTypo bool, searchTweets bool, searchLessonSeries bool, withHighlights bool, timeoutForHighlight time.Duration) (*QueryResult, error) {
	defer e.timeTrack(time.Now(), consts.LAT_DOSEARCH)

	// Initializing all channels.
	suggestChannel := make(chan null.String)
	grammarsSingleHitIntentsChannel := make(chan []Intent, 1)
	grammarsFilterIntentsChannel := make(chan []Intent, 1)
	grammarsFilteredResultsByLangChannel := make(chan map[string][]FilteredSearchResult)
	tweetsByLangChannel := make(chan map[string]*elastic.SearchResult)
	seriesLangChannel := make(chan map[string]*elastic.SearchResult)

	filterIntents := []Intent{}
	filteredByLang := map[string][]FilteredSearchResult{}
	tweetsByLang := map[string]*elastic.SearchResult{}
	seriesByLang := map[string]*elastic.SearchResult{}

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

	// Search grammars in parallel to native search.
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("ESEngine.DoSearch - Panic searching grammars: %+v", err)
				grammarsSingleHitIntentsChannel <- []Intent{}
				grammarsFilterIntentsChannel <- []Intent{}
				grammarsFilteredResultsByLangChannel <- map[string][]FilteredSearchResult{}
			}
		}()
		if singleHitIntents, filterIntents, err := e.SearchGrammarsV2(&query, from, size, sortBy, resultTypes, preference); err != nil {
			log.Errorf("ESEngine.DoSearch - Error searching grammars: %+v", err)
			grammarsSingleHitIntentsChannel <- []Intent{}
			grammarsFilterIntentsChannel <- []Intent{}
			grammarsFilteredResultsByLangChannel <- map[string][]FilteredSearchResult{}
		} else {
			grammarsSingleHitIntentsChannel <- singleHitIntents
			grammarsFilterIntentsChannel <- filterIntents
			if filtered, err := e.SearchByFilterIntents(filterIntents, query.Filters, query.Term, from, size, sortBy, resultTypes, preference, query.Deb); err != nil {
				log.Errorf("ESEngine.DoSearch - Error searching filtered results by grammars: %+v", err)
				grammarsFilteredResultsByLangChannel <- map[string][]FilteredSearchResult{}
			} else {
				grammarsFilteredResultsByLangChannel <- filtered
			}
		}
	}()

	if searchTweets {
		// Search tweets in parallel to native search.
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
	}

	if searchLessonSeries {
		// Search lesson series
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("ESEngine.DoSearch - Panic searching lesson series: %+v", err)
					seriesLangChannel <- map[string]*elastic.SearchResult{}
				}
			}()
			if byLang, err := e.LessonsSeries(query, preference); err != nil {
				log.Errorf("ESEngine.DoSearch - Error searching lesson series: %+v", err)
				seriesLangChannel <- map[string]*elastic.SearchResult{}
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
					log.Errorf("ESEngine.GetTypoSuggest - Panic getting typo suggest: %+v", err)
					suggestChannel <- null.String{"", false}
				}
			}()
			if suggestText, err := e.GetTypoSuggest(query, filterIntents); err != nil {
				log.Errorf("ESEngine.GetTypoSuggest - Error getting typo suggest: %+v", err)
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
	LogIfDeb(&query, fmt.Sprintf("Has classification intent from grammar: %s", strconv.FormatBool(hasClassificationIntentFromGrammar)))

	LogIfDeb(&query, fmt.Sprintf("query.Intents: %d", len(query.Intents)))
	// Grammar engine is currently support a search for classification intents according to 'by_content_type_and_source' rule only.
	// If we have classification intents from Grammar, IntentsEngine will search for intents only by tag.
	intents, err := e.AddIntents(&query, preference, sortBy, true, !hasClassificationIntentFromGrammar, filterIntents)
	if err != nil {
		log.Errorf("ESEngine.DoSearch - Error adding intents: %+v", err)
	}
	LogIfDeb(&query, IntentsToStringDebug("ADD INTENTS", intents))
	query.Intents = append(query.Intents, intents...)

	// When we have a lessons carousel we filter out the regular results that are also exist in the carousel.
	filterOutCUSources := make([]string, 0)
	for _, intent := range query.Intents {
		if intent.Type == consts.INTENT_TYPE_SOURCE {
			if intentValue, ok := intent.Value.(ClassificationIntent); ok && intentValue.Exist {
				// This is not a perfect solution since we dont know yet what is the currentLang and we filter by all languages
				// Also: it is possible that we may filter regular lesson results even if the carousel is not on the first page.
				filterOutCUSources = append(filterOutCUSources, intentValue.MDB_UID)
				LogIfDeb(&query, fmt.Sprintf("MDB_UID added to filterOutCUSources: %s.", intentValue.MDB_UID))
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

	// Do regular search.
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
	var maxRegularScore *float64 // max score for regular result - not intent, grammar or tweet
	programsToReplaceWithGrammarResults := []struct {
		hitId        string
		score        float64
		grammarHitId *string
	}{}
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
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
						err = json.Unmarshal(*hit.Source, &src)
						if err != nil {
							log.Errorf("ESEngine.DoSearch - cannot unmarshal source for hit '%v'.", hit.Id)
							continue
						}
						if src.ResultType == consts.ES_RESULT_TYPE_UNITS {
							if utils.Contains(utils.Is(src.TypedUids), es.KeyValue(consts.ES_UID_TYPE_COLLECTION, *programCollectionUid)) {
								programsToReplaceWithGrammarResults = append(programsToReplaceWithGrammarResults,
									struct {
										hitId        string
										score        float64
										grammarHitId *string
									}{
										hit.Id,
										*hit.Score,
										nil,
									})
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
				if shouldMergeResults {
					if *currentResults.Hits.MaxScore > *maxRegularScore {
						*maxRegularScore = *currentResults.Hits.MaxScore
					}
				}
			}
			lang := query.LanguageOrder[i]
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], currentResults)
		}
	}

	LogIfDeb(&query, ResultsSliceMapToStringDebug("RESULTS BY LANG", resultsByLang, 5))
	LogIfDeb(&query, fmt.Sprintf("Program to replace with grammar results: %d", len(programsToReplaceWithGrammarResults)))
	for _, p := range programsToReplaceWithGrammarResults {
		LogIfDeb(&query, fmt.Sprintf("\t%+v", p))
	}

	// Convert intents and grammars to results.
	err, intentResultsMap := e.IntentsToResults(&query)
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.DoSearch - Error adding intents to results.")
	}
	LogIfDeb(&query, ResultsMapToStringDebug("INTENTS TO RESULTS", intentResultsMap, 3))
	for lang, intentResults := range intentResultsMap {
		if haveHits(intentResults) {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], intentResults)
		}
	}

	if searchTweets {
		tweetsByLang = <-tweetsByLangChannel
		LogIfDeb(&query, ResultsMapToStringDebug("TWEETS", tweetsByLang, 3))
		for lang, tweets := range tweetsByLang {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], tweets)
		}
	}

	if searchLessonSeries {
		seriesByLang = <-seriesLangChannel
		LogIfDeb(&query, ResultsMapToStringDebug("SERIES", seriesByLang, 3))
		for lang, s := range seriesByLang {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			resultsByLang[lang] = append(resultsByLang[lang], s)
		}
	}

	filteredByLang = <-grammarsFilteredResultsByLangChannel
	LogIfDeb(&query, fmt.Sprintf("---- GRAMMAR SEARCH FILTERED ----"))
	for k, v := range filteredByLang {
		LogIfDeb(&query, fmt.Sprintf("\t%+v:", k))
		for i := range v {
			for j := range v[i].Results {
				LogIfDeb(&query, fmt.Sprintf("\t\t%d (%d hits)", j, len(v[i].Results[j].Hits.Hits)))
				LogIfDeb(&query, fmt.Sprintf(ResultToStringDebug(v[i].Results[j], 3)))
			}
		}
	}
	LogIfDeb(&query, fmt.Sprintf("---- END GRAMMAR SEARCH FILTERED ----"))

	var programToReplaceIndex int
	if len(programsToReplaceWithGrammarResults) > 0 {
		for lang, filtered := range filteredByLang {
			if _, ok := resultsByLang[lang]; !ok {
				resultsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			for _, fr := range filtered {
				if fr.ProgramCollection != nil {
					for _, result := range fr.Results {
						for _, hit := range result.Hits.Hits {
							var src es.Result
							err = json.Unmarshal(*hit.Source, &src)
							if err != nil {
								log.Errorf("ESEngine.DoSearch - cannot unmarshal source for hit '%v'.", hit.Id)
								continue
							}
							if src.ResultType == consts.ES_RESULT_TYPE_UNITS {
								if utils.Contains(utils.Is(src.TypedUids), es.KeyValue(consts.ES_UID_TYPE_COLLECTION, *fr.ProgramCollection)) {
									if programToReplaceIndex < len(programsToReplaceWithGrammarResults) {
										hit.Score = &programsToReplaceWithGrammarResults[programToReplaceIndex].score
										programsToReplaceWithGrammarResults[programToReplaceIndex].grammarHitId = &hit.Id
										// TBD update hit explanation
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

	// Loop over grammar filtered results to apply the score logic for combination with regular results
	for lang, filtered := range filteredByLang {
		if _, ok := resultsByLang[lang]; !ok {
			resultsByLang[lang] = make([]*elastic.SearchResult, 0)
		}
		for _, fr := range filtered {
			for _, result := range fr.Results {
				sort.Strings(filterOutCUSources)
				withoutCarouselDuplications := []*elastic.SearchHit{}
				var maxScore float64
				for _, hit := range result.Hits.Hits {
					var src es.Result
					err = json.Unmarshal(*hit.Source, &src)
					if err != nil {
						log.Errorf("ESEngine.DoSearch - cannot unmarshal source for hit '%v'.", hit.Id)
						continue
					}
					if src.ResultType == consts.ES_RESULT_TYPE_UNITS {
						hitSources, err := es.KeyValuesToValues(consts.ES_UID_TYPE_SOURCE, src.TypedUids)
						if err != nil {
							log.Errorf("ESEngine.DoSearch - cannot read TypedUids for hit '%v'.", hit.Id)
							continue
						}
						sort.Strings(hitSources)
						if len(utils.IntersectSortedStringSlices(hitSources, filterOutCUSources)) > 0 {
							// We remove the hits we recieved from 'filter grammar' that are duplicate the existed items inside carousels
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

		// Note:
		// Below we handle 2 result types from a different elastic queries: grammar based results and regular results.
		// Changes we made to the scores that is based on the reliance to the results of another type
		//  can potentially break in some way the uniqueness of results for each page (page 2 may contain a result from page 1).
		// Also the logic of boosting results that identical to both types has a limited effect
		//  since we we are not checking identification in all results but only in the results we received according to page filter.
		// Ideal solution for these issues is to handle all score calculations for both types within a single elastic query.
		if maxRegularScore != nil && *maxRegularScore >= 15 { // if we have big enough regular scores, we should increase or decrease the filtered results scores
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
					// Why we add +10 to the formula:
					// In some cases we have several regular results with a very close scores that above 90% of the maxRegularScore.
					// Since the top score for the best 'filter grammar' result is 90% of the maxRegularScore,
					//	we have cases where the best 'filter grammar' result will be below the high regular results with a VERY SMALL GAP between them.
					// To minimize this gap, we add +10 to the formula.
					// e.g. search of term "ביטול קטעי מקור" without adding 10 bring the relevant result in position #4. With adding 10, the relevant result is the first.
					for _, hit := range result.Hits.Hits {
						replaced := false
						for _, p := range programsToReplaceWithGrammarResults {
							if p.grammarHitId != nil && *p.grammarHitId == hit.Id {
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
						// Assign results score to zero if the results are to be replaced by program grammar
						for i := 0; i < programToReplaceIndex; i++ {
							if hit.Id == programsToReplaceWithGrammarResults[i].hitId {
								LogIfDeb(&query, fmt.Sprintf("Setting zero score for %s.", hit.Id))
								zero := 0.0
								hit.Score = &zero
								break
							}
						}
					}
					for _, fr := range filtered {
						if _, hasId := fr.HitIdsMap[hit.Id]; hasId {
							LogIfDeb(&query, fmt.Sprintf("Same hit found for both regular and grammar filtered results: %v", hit.Id))
							if hit.Score != nil && *hit.Score > 5 { // We will increment the score only if the result is relevant enough (score > 5)
								*hit.Score += consts.FILTER_GRAMMAR_INCREMENT_FOR_MATCH_TO_FULL_TERM
							}
							if !fr.PreserveTermForHighlight {
								// We remove this hit id from HitIdsMap in order to highlight the original search term and not $Text val.
								delete(fr.HitIdsMap, hit.Id)
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
	results := make([]*elastic.SearchResult, 0)
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

			// Preparing highlights search.
			// Since some highlight queries are acting like bottlenecks (in cases of scanning large documents)
			// and may hold the overall search duration for a few tens of seconds,
			// we prefer to execute several ES calls in parallel with a timeout limit for each call.
			highlightRequests := []*elastic.SearchRequest{}

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
							highlightRequests = append(highlightRequests, req)
						}
					}
					continue
				}
				if h.Id == "" || strings.HasPrefix(h.Index, "intent-") {
					// Bypass intent
					continue
				}

				term := query.Term

				for _, lang := range highlightsLangs {
					if filtered, ok := filteredByLang[lang]; ok {
						for _, fr := range filtered {
							if _, hasId := fr.HitIdsMap[h.Id]; hasId {
								// set highlight search term as the grammar filter search term
								term = fr.Term
								break
							}
						}
					}
				}

				// We use multiple search request because we saw that a single request
				// filtered by id's list take more time than multiple requests.
				req, err := NewResultsSearchRequest(
					SearchRequestOptions{
						resultTypes:      resultTypes,
						docIds:           []string{h.Id},
						index:            h.Index,
						query:            Query{ExactTerms: query.ExactTerms, Term: term, Filters: query.Filters, LanguageOrder: highlightsLangs, Deb: query.Deb},
						sortBy:           consts.SORT_BY_RELEVANCE,
						from:             0,
						size:             1,
						preference:       preference,
						useHighlight:     true,
						partialHighlight: true})
				if err != nil {
					return nil, errors.Wrap(err, "ESEngine.DoSearch - Error creating highlight request in multisearch Do.")
				}
				highlightRequests = append(highlightRequests, req)
			}

			if len(highlightRequests) > 0 {

				log.Debug("Searching for highlights and replacing original results with highlighted results.")

				var wg sync.WaitGroup
				wg.Add(len(highlightRequests))
				mhErrors := make([]error, len(highlightRequests))
				mhResults := make([]*elastic.MultiSearchResult, len(highlightRequests))

				beforeHighlightsDoSearch := time.Now()
				for i, hr := range highlightRequests {
					go func(req *elastic.SearchRequest, idx int) {
						highlightCtx, cancelFn := context.WithTimeout(context.TODO(), timeoutForHighlight)
						defer cancelFn()
						mssHighlights := e.esc.MultiSearch().Add(req)
						mr, err := mssHighlights.Do(highlightCtx)
						if highlightCtx.Err() != nil {
							mhErrors[idx] = highlightCtx.Err()
						} else {
							mhErrors[idx] = err
						}
						mhResults[idx] = mr
						wg.Done()
					}(hr, i)
				}
				wg.Wait()
				e.timeTrack(beforeHighlightsDoSearch, consts.LAT_DOSEARCH_MULTISEARCHHIGHLIGHTSDO)
				responses := []*elastic.SearchResult{}
				for i, mhResult := range mhResults {
					if mhErrors[i] == context.DeadlineExceeded {
						continue
					}
					if mhErrors[i] != nil {
						return nil, errors.Wrap(mhErrors[i], "ESEngine.DoSearch - Error mssHighlights Do.")
					}
					responses = append(responses, mhResult.Responses...)
				}
				for _, highlightedResults := range responses {
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
									//  Keep the score of the original hit (possibly incr. by grammar)
									ret.Hits.Hits[i].Score = h.Score
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
		}

		// Prepare results for client
		for _, hit := range ret.Hits.Hits {
			if hit.Type == consts.SEARCH_RESULT_TWEETS_MANY {
				err = e.NativizeTweetsHitForClient(hit, consts.SEARCH_RESULT_TWEETS_MANY)
			} else if hit.Type != consts.GRAMMAR_TYPE_LANDING_PAGE {
				var src es.Result
				err = json.Unmarshal(*hit.Source, &src)
				if err != nil {
					log.Errorf("ESEngine.DoSearch - cannot unmarshal source.")
					continue
				}
				src.TypedUids = nil // Client has no need for TypedUids list
				if src.ResultType == consts.ES_RESULT_TYPE_SOURCES {
					//  Replace title with full title
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
					log.Errorf("ESEngine.DoSearch - cannot marshal source with title correction.")
					continue
				}
				hit.Source = (*json.RawMessage)(&nsrc)
			}

			//  Temp. workround until client could handle null values in Highlight fields (WIP by David)
			if hit.Highlight == nil {
				hit.Highlight = elastic.SearchHitHighlight{}
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

	if len(mr.Responses) > 0 {
		// This happens when there are no responses with hits.
		// Note, we don't filter here intents by language.
		return &QueryResult{mr.Responses[0], suggestText, currentLang, nil}, err
	}
	return nil, errors.Wrap(err, "ESEngine.DoSearch - No responses from multi search.")
}

func (e *ESEngine) GetCounts(ctx context.Context, query Query, allSourceUIDs []string, allTagUIDs []string) (*FacetSearchResults, error) {
	defer e.timeTrack(time.Now(), consts.LAT_GET_SOURCE_COUNTS)

	options := CreateFacetAggregationOptions{
		sourceUIDs:             allSourceUIDs,
		tagUIDs:                allTagUIDs,
		mediaLanguageValues:    consts.ALL_KNOWN_LANGS[:],
		originalLanguageValues: consts.ALL_KNOWN_LANGS[:],
		contentTypeValues:      consts.SECTION_CT_TYPES[:],
		dateRanges:             []string{consts.DATE_FILTER_TODAY, consts.DATE_FILTER_YESTERDAY, consts.DATE_FILTER_LAST_7_DAYS, consts.DATE_FILTER_LAST_30_DAYS},
		personUIDs:             []string{mdb.PERSONS_REGISTRY.ByPattern[consts.P_RAV].UID, mdb.PERSONS_REGISTRY.ByPattern[consts.P_RABASH].UID},
	}

	multiSearchService := e.esc.MultiSearch()
	for _, countFilter := range []string{
		consts.FILTER_CONTENT_TYPE,
		consts.FILTER_SOURCE,
		consts.FILTER_TAG,
		consts.FILTER_MEDIA_LANGUAGE,
		consts.FILTER_ORIGINAL_LANGUAGE,
		consts.AGG_FILTER_DATES,
		consts.FILTER_PERSON,
	} {
		// If filter exist in query, count it with separate facet request.
		// Handle dates in special way as the AGG_FILTER_DATES is applied via
		// two different filters FILTER_START_DATE and FILTER_END_DATE.
		if (countFilter == consts.AGG_FILTER_DATES &&
			(len(query.Filters[consts.FILTER_START_DATE]) > 0 || len(query.Filters[consts.FILTER_END_DATE]) > 0)) ||
			len(query.Filters[countFilter]) > 0 {
			// Shallow copy query without the relevant filter.
			filterQuery := query
			filterQuery.Filters = make(map[string][]string)
			for k, v := range query.Filters {
				if k != countFilter || (countFilter == consts.AGG_FILTER_DATES && (k != consts.FILTER_START_DATE && k != consts.FILTER_END_DATE)) {
					filterQuery.Filters[k] = v
				}
			}

			filterOptions := CreateFacetAggregationOptions{}
			switch countFilter {
			case consts.FILTER_CONTENT_TYPE:
				filterOptions.contentTypeValues = options.contentTypeValues
				options.contentTypeValues = []string(nil)
			case consts.FILTER_SOURCE:
				filterOptions.sourceUIDs = options.sourceUIDs
				options.sourceUIDs = []string(nil)
			case consts.FILTER_TAG:
				filterOptions.tagUIDs = options.tagUIDs
				options.tagUIDs = []string(nil)
			case consts.FILTER_MEDIA_LANGUAGE:
				filterOptions.mediaLanguageValues = options.mediaLanguageValues
				options.mediaLanguageValues = []string(nil)
			case consts.FILTER_ORIGINAL_LANGUAGE:
				filterOptions.originalLanguageValues = options.originalLanguageValues
				options.originalLanguageValues = []string(nil)
			case consts.AGG_FILTER_DATES:
				filterOptions.dateRanges = options.dateRanges
				options.dateRanges = []string(nil)
			case consts.FILTER_PERSON:
				filterOptions.personUIDs = options.personUIDs
				options.personUIDs = []string(nil)
			}

			request, err := NewFacetSearchRequest(filterQuery, filterOptions)
			if err != nil {
				return nil, errors.Wrap(err, "ESEngine.GetSourceCounts - Error creating facet search source.")
			}
			multiSearchService.Add(request)
		}
	}
	// If some filters were not set, use one search to count them all.
	// In most cases where filters are not used, only this will be applied.
	if len(options.contentTypeValues) > 0 || len(options.sourceUIDs) > 0 ||
		len(options.tagUIDs) > 0 || len(options.mediaLanguageValues) > 0 ||
		len(options.originalLanguageValues) > 0 || len(options.dateRanges) > 0 ||
		len(options.personUIDs) > 0 {
		request, err := NewFacetSearchRequest(query, options)
		if err != nil {
			return nil, errors.Wrap(err, "ESEngine.GetSourceCounts - Error creating facet search source.")
		}
		multiSearchService.Add(request)
	}
	mr, err := multiSearchService.Do(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.GetSourceCounts - Error execute facet multi search request.")
	}

	facetSearchResults, err := parseFacetAggregationsResult(mr.Responses)
	if err != nil {
		return nil, errors.Wrap(err, "ESEngine.GetSourceCounts - Error parce facet search result.")
	}

	return facetSearchResults, nil
}

func parseFacetAggregationsResult(results []*elastic.SearchResult) (*FacetSearchResults, error) {
	r := new(FacetSearchResults)
	for _, result := range results {
		agg := &result.Aggregations
		if agg == nil {
			return nil, errors.New("ESEngine.GetSourceCounts - No search aggregations")
		}

		if tags, err := parseFacetAggregationForName(agg, consts.FILTER_TAG); err != nil {
			return nil, err
		} else if len(tags) > 0 {
			if r.Tags != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating tags filter twice")
			}
			r.Tags = tags
		}

		if contentTypes, err := parseFacetAggregationForName(agg, consts.FILTER_CONTENT_TYPE); err != nil {
			return nil, err
		} else if len(contentTypes) > 0 {
			if r.ContentTypes != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating content types filter twice")
			}
			r.ContentTypes = contentTypes
		}

		if mediaLanguages, err := parseFacetAggregationForName(agg, consts.FILTER_MEDIA_LANGUAGE); err != nil {
			return nil, err
		} else if len(mediaLanguages) > 0 {
			if r.MediaLanguages != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating media language filter twice")
			}
			r.MediaLanguages = mediaLanguages
		}

		if originalLanguages, err := parseFacetAggregationForName(agg, consts.FILTER_ORIGINAL_LANGUAGE); err != nil {
			return nil, err
		} else if len(originalLanguages) > 0 {
			if r.OriginalLanguages != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating original language filter twice")
			}
			r.OriginalLanguages = originalLanguages
		}

		if sources, err := parseFacetAggregationForName(agg, consts.FILTER_SOURCE); err != nil {
			return nil, err
		} else if len(sources) > 0 {
			if r.Sources != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating source filter twice")
			}
			r.Sources = sources
		}

		if dates, err := parseFacetAggregationForName(agg, consts.AGG_FILTER_DATES); err != nil {
			return nil, err
		} else if len(dates) > 0 {
			if r.Dates != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating same filter twice")
			}
			r.Dates = dates
		}

		if persons, err := parseFacetAggregationForName(agg, consts.FILTER_PERSON); err != nil {
			return nil, err
		} else if len(persons) > 0 {
			if r.Persons != nil {
				return nil, errors.New("ESEngine.GetSourceCounts - Aggregating same filter twice")
			}
			r.Persons = persons
		}
	}

	if len(r.Tags) == 0 ||
		len(r.ContentTypes) == 0 ||
		len(r.MediaLanguages) == 0 ||
		len(r.OriginalLanguages) == 0 ||
		len(r.Sources) == 0 ||
		len(r.Dates) == 0 ||
		len(r.Persons) == 0 {
		return nil, errors.New("ESEngine.GetSourceCounts - One of the aggregations is empty, expected all to be non-empty.")
	}

	return r, nil
}

func parseFacetAggregationForName(agg *elastic.Aggregations, aggName string) (map[string]int64, error) {
	aggFilters, _ := agg.Filters(aggName)

	if aggFilters == nil || len(aggFilters.NamedBuckets) <= 0 {
		// We expect several searches which split the aggregations between them.
		return nil, nil
	}

	result := map[string]int64{}

	for name, data := range aggFilters.NamedBuckets {
		result[name] = data.DocCount
	}

	return result, nil
}
