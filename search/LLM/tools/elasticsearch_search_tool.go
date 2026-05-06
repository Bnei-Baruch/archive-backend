package tools

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/search"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type ElasticsearchSearchEngine interface {
	DoSearch(
		ctx context.Context,
		query search.Query,
		sortBy string,
		from int,
		size int,
		preference string,
		checkTypo bool,
		searchTweets bool,
		searchLessonSeries bool,
		withHighlights bool,
		timeoutForHighlight time.Duration,
	) (*search.QueryResult, error)
}

type ElasticsearchSearchEngineFactory func() (ElasticsearchSearchEngine, error)

type ElasticsearchSearchTool struct {
	engine              ElasticsearchSearchEngine
	engineFactory       ElasticsearchSearchEngineFactory
	timeoutForHighlight time.Duration
}

type elasticsearchSearchToolArgs struct {
	Query       string          `json:"query,omitempty"`
	Filters     json.RawMessage `json:"filters,omitempty"`
	Language    string          `json:"language,omitempty"`
	SortBy      string          `json:"sort_by,omitempty"`
	From        int             `json:"from,omitempty"`
	Size        int             `json:"size,omitempty"`
	ExactPhrase bool            `json:"exact_phrase,omitempty"`
}

type elasticsearchSearchToolResult struct {
	Query          search.Query        `json:"query"`
	SortBy         string              `json:"sort_by"`
	From           int                 `json:"from"`
	Size           int                 `json:"size"`
	Result         *search.QueryResult `json:"result,omitempty"`
	Error          string              `json:"error,omitempty"`
	RetrySuggested bool                `json:"retry_suggested,omitempty"`
	Guidance       string              `json:"guidance,omitempty"`
}

func NewElasticsearchSearchTool(engine ElasticsearchSearchEngine, timeoutForHighlight time.Duration) *ElasticsearchSearchTool {
	return &ElasticsearchSearchTool{
		engine:              engine,
		timeoutForHighlight: timeoutForHighlight,
	}
}

func NewElasticsearchSearchToolWithFactory(engineFactory ElasticsearchSearchEngineFactory, timeoutForHighlight time.Duration) *ElasticsearchSearchTool {
	return &ElasticsearchSearchTool{
		engineFactory:       engineFactory,
		timeoutForHighlight: timeoutForHighlight,
	}
}

func (t *ElasticsearchSearchTool) Definition() llm.ReasoningToolDefinition {
	// Note: In the filter descriptions we mention that tag filter should currently be used only for holidays and observances, because current human generated tags are not efficient. In the future, we may want to update the tags using AI and remove this note.
	return llm.ReasoningToolDefinition{
		Name:        "elasticsearch_search",
		Description: "Search archive content through Elasticsearch with optional filters and exact phrase search.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search text. Optional when filters are provided.",
				},
				"filters": map[string]interface{}{
					"type":        "object",
					"description": "Optional search filters. Supported filters (keys) and their values are described in the usage explanation.",
					"additionalProperties": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Preferred UI language used for language ordering.",
				},
				"sort_by": map[string]interface{}{
					"type":        "string",
					"description": "Sort order for search results.",
					"enum":        []string{consts.SORT_BY_RELEVANCE, consts.SORT_BY_NEWER_TO_OLDER, consts.SORT_BY_OLDER_TO_NEWER, consts.SORT_BY_SOURCE_FIRST},
				},
				"from": map[string]interface{}{
					"type":        "integer",
					"description": "Zero-based result offset.",
					"minimum":     0,
				},
				"size": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return.",
					"minimum":     1,
				},
				"exact_phrase": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, treat the full query text as one exact phrase instead of general full-text search. Requires a non-empty query.",
				},
			},
			"additionalProperties": false,
		},
	}
}

func (t *ElasticsearchSearchTool) UsageExplanation() string {
	// TBC - The "Recommendation for the agent" in the bottom may be need to be updated.
	return `Tool: elasticsearch_search
Elasticsearch holds an index of the archive content retrieved from PostgreSQL (the DB that stores various metadata about the content like content type, source, title and more) and docx/pdf files of sources (library items) and lessons/programs transcripts.
Use this tool for archive search when you need relevant results by text query, filters, or both.
Important note: Since the search engine is technically limited and based on lexical match, you should make multiple searches with the necessary variations of the query: Using similar words to the original query and different filter combinations. Sometime is reasonable to limit the text query to a simple terms combination to avoid missing relevant results. Also include plural and singular forms, different word order, and similar formulations of the same query.

The following search filters are supported and can be used in combination: 

1. content_type – Filters by type of content. Multiple types can be used. Supported values:
SOURCE – Library texts like articles or book chapters
LECTURE – General or beginner lectures
LESSONS_SERIES – Series of lessons on a topic or source
LESSON_PART – A part of the daily morning lesson (broadcast globally from Petah Tikva)
WOMEN_LESSON – Lessons primarily for women
EVENT_PART – Items from conventions or special events (e.g. Unity Day)
FRIENDS_GATHERING – Social events (Yeshivat Haverim / ישיבת חברים)
MEAL – Events with songs and intentional content
VIDEO_PROGRAM_CHAPTER – Single program of TV/video programs collection
CLIP – Video clips, sometimes lesson or program segments
ARTICLE – Articles (including external publications)
BLOG_POST – Blog posts by Dr. Laitman
R_TWEET – Tweets by Dr. Laitman
LIKUTIM - Selected Excerpts from the Sources. These are special content units that contain selected excerpts from the sources (library items) that are used in the lessons (not the full sources themselves). They are focused on a specific topic or concept. Use this content type when user is looking for excerpts (ליקוטים) or citations.
If the user is looking for a lesson, include all lesson-related content types: LECTURE, LESSONS_SERIES, LESSON_PART, WOMEN_LESSON

2. source – Filters by source (library item). Multiple sources can be used. Use get_available_books to find relevant top-level books when the book root is unknown or ambiguous, get_sources_by_source to drill down to child source ids, or get_sources_by_author when you already know the author and need sources linked to that author.
Including the sources filter in the query, means that we want to find various content that related to that sources like TV programs, lessons or the sources (library pages).
When we filter by the parent source (like book name) we mean that the content we look for should be related also to all the child sources (chapters). In that case it is enough to apply only the parent source.
The source filter accepts source ids and some author root codes. Prefer values returned by get_available_books, get_sources_by_source, or get_sources_by_author instead of guessing them.
Note: For Dr. Laitman’s content, do not use source filter ('ml' value) — use the person filter instead.

3. person – Filters by speaker/author of media content (not books). Values:
abcdefgh – Dr. Michael Laitman
KxApZ4pI – Baruch Shalom HaLevi Ashlag (Rabash). Use this filter when looking for original recordings by a specific teacher. When you add this filter, do not add other content_type filters.

4. start_date and end_date – Filter by date range. The date format is yyyy-MM-dd. If only start_date is provided, it filters from that date to the future. If only end_date is provided, it filters from the past until that date.

5. tag - Filter by topic. Currently we support topics related to Holidays and Observances - these are applied only for content units and likutim but not for sources:
- "topic id": "ksh1gGBM", "name": "אלול"
- "topic id": "k3OHIbDd", "name": "הושענא רבה"
- "topic id": "rxNl0zXg", "name": "חנוכה"
- "topic id": "8NOejqZq", "name": "ט' באב"
- "topic id": "aG35w3xs", "name": "ט\"ו באב"
- "topic id": "SuqPuYoZ", "name": "ט\"ו בשבט"
- "topic id": "XuTr8IEN", "name": "יום הזיכרון לחללי מערכות ישראל"
- "topic id": "HhQuyXga", "name": "יום הזיכרון לשואה ולגבורה"
- "topic id": "oXRmGfzj", "name": "יום הכיפורים"
- "topic id": "mnr1gzAk", "name": "יום העצמאות"
- "topic id": "2Amyg207", "name": "יום ירושלים"
- "topic id": "9BV2fKxL", "name": "י\"ז בתמוז"
- "topic id": "MkVeezxY", "name": "ימי בין המצרים"
- "topic id": "arE77pz5", "name": "ל\"ג בעומר"
- "topic id": "Q2ZFsb9a", "name": "סוכות"
- "topic id": "paN1Ehbq", "name": "ספירת העומר"
- "topic id": "ZjqGWdYE", "name": "שבת הגדול"
- "topic id": "RWqjxgkj", "name": "פסח"
- "topic id": "PkEfPB9i", "name": "ראש השנה"
- "topic id": "MyLcuAgH", "name": "שבועות"
- "topic id": "3r8kzv2E", "name": "לילה דכלה"
- "topic id": "9eCd3GLo", "name": "שבועות", "name": "שבעה באוקטובר"
- "topic id": "sDsGrrTH", "name": "שמחת תורה"
- "topic id": "n4F3bUjd", "name": "שמיני עצרת"

6. collection - Filter by collection ids. Use the get_collections tool to obtain collection ids. Collections are groups of related content units. Each daily lesson is a collection, a TV series (program) is also a collection, and there are also collections for conventions and special events.

7. mdb_uid - Exact lookup by archive result UID. Use this only when you already have a concrete mdb_uid from previous tool results and need to retrieve/check that exact result.

Arguments:
- query: optional search text. Required when exact_phrase is true.
- filters: optional structured filters as described above.
- language: optional UI language used for language ordering. Example values: en, ru, es, he. If not specified or invalid, defaults to en. Note that the search results may contain content in various languages regardless of this parameter, since it is only used for language ordering based on detected language and language filters.
- sort_by: optional result sort.
- from: optional zero-based offset.
- size: optional page size.
- exact_phrase: optional boolean. When true, the full query is treated as one exact phrase and requires a non-empty query.
Behavior:
- Returns JSON with the normalized search query, selected sort, pagination, and Elasticsearch result payload.
- If Elasticsearch fails, the tool returns an error field and retry guidance.
- Use this as the main discovery tool. If the returned highlights are not sufficient, and you need direct source or transcript evidence, follow up with query_source_ai or query_transcript_ai.
Returned data:
- query: the normalized search.Query that was actually executed. Inspect query.term, query.exact_terms, query.filters, and query.language_order to understand the final search request.
- sort_by, from, size: the resolved sort and pagination actually used.
- result: a search.QueryResult object.
- result.language: the language used for search ordering.
- result.typo_suggest: optional typo suggestion string.
- result.search_result.hits.total_hits: total number of matching results.
- result.search_result.hits.hits: the actual page of results. Each hit is a raw Elasticsearch hit.
- For each hit, inspect _source.result_type when _source is a normal object, and also inspect hit.type because some grouped/synthetic results are identified by hit.type:
  - units: individual content items such as lesson parts, lectures, clips, event parts, articles, and similar searchable media/text items.
  - sources: library/source texts such as books, articles, volumes, and chapters from the Library section.
  - collections: grouped content such as daily lessons, program series, conventions, and special events.
  - posts: blog posts.
  - tweets: tweets.
  - tags: tag/topic results.
- Important special grouped/synthetic hit types:
  - tweets_many: a grouped tweet bundle/carousel, not one concrete tweet. In this case hit._source is an array of raw tweet hits. Inspect the items inside that array and choose the most relevant individual tweet or small set of tweets.
  - lessons_series_by_source: a grouped lesson-series result. It represents lesson-series collections grouped by source. Treat it as a narrowing/grouping clue, not as one final concrete lesson item.
  - lessons_series_by_tag: a grouped lesson-series result. It represents lesson-series collections grouped by tag. Treat it as a narrowing/grouping clue, not as one final concrete lesson item.
- Important distinction:
  - units, sources, posts are usually direct content results. They normally point to one concrete item.
  - collections, tags, tweets_many, lessons_series_by_source, and lessons_series_by_tag are narrowing/grouping results, not a final concrete item. A collection usually represents a group of related content units. A tag usually represents a topic bucket that can lead to many matching items.
  - These grouping results exist to narrow the search space and avoid pushing many weak individual hits above a stronger grouped result.
- Recommendation for the agent:
  - If the user wants a concrete item to watch, read, quote, or summarize, do not stop at a collection, tag, or grouped lesson-series hit. Use that hit as a clue and run a follow-up filtered search, then return the best concrete content units or sources inside it.
  - If the user wants a concrete tweet or quote and a tweets_many hit is returned, inspect the tweets inside the hit._source array and return the most relevant individual tweet or a very small set of tweets, not the wrapper itself.
  - If the user intent is broad exploration, browsing, or discovery, it can be appropriate to present a collection, tag, grouped lesson-series hit, or grouped tweet hit as a useful answer, but explain that it is a group/bundle and not one specific content item.
  - If both direct content hits and grouping hits are relevant, prefer direct content for final recommendations and use grouping hits as supporting navigation or refinement hints.
- Useful _source fields usually include mdb_uid, title, full_title, description, content, effective_date, and typed_uids.
- Some hits may also contain highlight fragments under hit.highlight. Use those to explain why the hit matched.`
}

func (t *ElasticsearchSearchTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	engine, err := t.getEngine()
	if err != nil {
		return "", err
	}

	args := elasticsearchSearchToolArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("elasticsearch_search: failed to parse arguments: %w", err)
	}
	llm.LogIfDeb(ctx, "elasticsearch_search: start query=%q language=%q sort_by=%q from=%d size=%d exact_phrase=%t", args.Query, args.Language, args.SortBy, args.From, args.Size, args.ExactPhrase)

	filters, err := normalizeElasticsearchSearchFilters(args.Filters)
	if err != nil {
		return "", err
	}
	llm.LogIfDeb(ctx, "elasticsearch_search: normalized filters=%v", filters)

	queryText := strings.TrimSpace(args.Query)
	if queryText == "" && len(filters) == 0 {
		return "", llm.NewRecoverableToolError("elasticsearch_search", "either query or filters must be provided", "Call elasticsearch_search with a non-empty query, filters, or both.")
	}
	if args.ExactPhrase && queryText == "" {
		return "", llm.NewRecoverableToolError("elasticsearch_search", "exact_phrase requires a non-empty query", "Either provide a non-empty query or set exact_phrase to false.")
	}

	query := buildElasticsearchSearchQuery(queryText, args.ExactPhrase)
	if args.ExactPhrase && len(query.ExactTerms) == 1 && query.ExactTerms[0] == "" {
		return "", llm.NewRecoverableToolError("elasticsearch_search", "exact_phrase requires a non-empty query", "Either provide a non-empty query or set exact_phrase to false.")
	}
	query.Filters = mergeElasticsearchSearchFilters(query.Filters, filters)
	query.Deb = false // Set false to avoid putting debug data into LLM context and reaching token usage limits.

	rawLanguage := strings.ToLower(strings.TrimSpace(args.Language))
	language := consts.DEFAULT_UI_LANGUAGE
	languageExplicit := false
	if rawLanguage != "" {
		if _, ok := consts.SEARCH_LANG_ORDER[rawLanguage]; ok {
			language = rawLanguage
			languageExplicit = true
		}
	}
	setElasticsearchSearchLanguageOrder(&query, language, languageExplicit)

	sortBy, err := normalizeElasticsearchSearchSortBy(args.SortBy, query.Term != "" || len(query.ExactTerms) > 0)
	if err != nil {
		return "", err
	}

	from := normalizeElasticsearchSearchFrom(args.From)
	size := normalizeElasticsearchSearchSize(args.Size)
	preference, err := buildElasticsearchSearchPreference(query, sortBy, from, size)
	if err != nil {
		return "", fmt.Errorf("elasticsearch_search: failed to build search preference: %w", err)
	}
	llm.LogIfDeb(ctx, "elasticsearch_search: prepared query term=%q exact_terms=%v filters=%v language_order=%v sort_by=%q from=%d size=%d preference=%q", query.Term, query.ExactTerms, query.Filters, query.LanguageOrder, sortBy, from, size, preference)

	result, err := engine.DoSearch(
		ctx,
		query,
		sortBy,
		from,
		size,
		preference,
		false,
		true,
		false, // searchLessonSeries is false since the synthetic result is not usable for the agent. The agent can use LESSONS_SERIES content_type filter to find real lesson series collections.
		true,
		t.timeoutForHighlight,
	)
	if err != nil {
		llm.LogIfDeb(ctx, "elasticsearch_search: runtime search failure query=%q err=%v", queryText, err)
		return marshalToolResult(elasticsearchSearchToolResult{
			Query:          query,
			SortBy:         sortBy,
			From:           from,
			Size:           size,
			Error:          fmt.Sprintf("Elasticsearch search failed: %v", err),
			RetrySuggested: true,
			Guidance:       "Try another query variation or simpler filters. If the same runtime error repeats, stop retrying the same search and continue with other available tools.",
		})
	}
	hitCount := int64(0)
	if result != nil && result.SearchResult != nil && result.SearchResult.Hits != nil {
		hitCount = result.SearchResult.Hits.TotalHits
	}
	resultLanguage := ""
	if result != nil {
		resultLanguage = result.Language
	}
	if elasticsearchSearchHasAnyResults(result) {
		llm.ReportReasoningProgressResults(ctx, elasticsearchSearchHasPotentiallyGoodResults(result))
	}
	llm.LogIfDeb(ctx, "elasticsearch_search: completed language=%q hits=%d", resultLanguage, hitCount)

	return marshalToolResult(elasticsearchSearchToolResult{
		Query:  query,
		SortBy: sortBy,
		From:   from,
		Size:   size,
		Result: result,
	})
}

func elasticsearchSearchHasAnyResults(result *search.QueryResult) bool {
	if result == nil || result.SearchResult == nil || result.SearchResult.Hits == nil {
		return false
	}
	return len(result.SearchResult.Hits.Hits) > 0
}

func elasticsearchSearchHasPotentiallyGoodResults(result *search.QueryResult) bool {
	if !elasticsearchSearchHasAnyResults(result) {
		return false
	}

	// Current heuristic for potentially good results:
	// 1. The returned page has at least 10 visible hits.
	// 2. It includes at least one source, AND: one lesson part or one video program chapter.
	hasProgram := false
	hasSource := false
	hasLesson := false
	for _, hit := range result.SearchResult.Hits.Hits {
		if hit.Type != "result" || hit.Source == nil {
			continue
		}
		var src es.Result
		if err := json.Unmarshal(*hit.Source, &src); err != nil {
			continue
		}
		if src.ResultType == consts.ES_RESULT_TYPE_SOURCES {
			hasSource = true
		}
		for _, contentType := range elasticsearchSearchHitContentTypes(src) {
			if elasticsearchSearchIsProgramContentType(contentType) {
				hasProgram = true
			}
			if elasticsearchSearchIsLessonContentType(contentType) {
				hasLesson = true
			}
			if hasProgram && hasLesson {
				break
			}
		}
		if hasSource && (hasProgram || hasLesson) {
			return true
		}
	}

	return false
}

func elasticsearchSearchHitContentTypes(src es.Result) []string {
	contentTypes, err := es.KeyValuesToValues("content_type", src.FilterValues)
	if err != nil {
		return nil
	}
	collectionContentTypes, err := es.KeyValuesToValues(consts.FILTER_COLLECTIONS_CONTENT_TYPE, src.FilterValues)
	if err == nil {
		contentTypes = append(contentTypes, collectionContentTypes...)
	}
	return contentTypes
}

func elasticsearchSearchIsProgramContentType(contentType string) bool {
	return contentType == consts.CT_VIDEO_PROGRAM || contentType == consts.CT_VIDEO_PROGRAM_CHAPTER
}

func elasticsearchSearchIsLessonContentType(contentType string) bool {
	switch contentType {
	case consts.CT_DAILY_LESSON, consts.CT_FULL_LESSON, consts.CT_LESSON_PART:
		return true
	default:
		return false
	}
}

func (t *ElasticsearchSearchTool) getEngine() (ElasticsearchSearchEngine, error) {
	if t.engineFactory != nil {
		engine, err := t.engineFactory()
		if err != nil {
			return nil, fmt.Errorf("elasticsearch_search: failed to build engine: %w", err)
		}
		if engine == nil {
			return nil, fmt.Errorf("elasticsearch_search: engine factory returned nil")
		}
		return engine, nil
	}
	if t.engine == nil {
		return nil, fmt.Errorf("elasticsearch_search: engine is nil")
	}
	return t.engine, nil
}

func buildElasticsearchSearchQuery(queryText string, exactPhrase bool) search.Query {
	queryText = strings.TrimSpace(queryText)
	if exactPhrase {
		return search.Query{
			ExactTerms: []string{trimElasticsearchSearchOuterQuotes(queryText)},
			Original:   queryText,
			Filters:    map[string][]string{},
		}
	}
	if queryText == "" {
		return search.Query{
			Original: queryText,
			Filters:  map[string][]string{},
		}
	}
	query := search.ParseQuery(queryText)
	if query.Filters == nil {
		query.Filters = map[string][]string{}
	}
	return query
}

func normalizeElasticsearchSearchFilters(raw json.RawMessage) (map[string][]string, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return map[string][]string{}, nil
	}

	values := map[string]interface{}{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("elasticsearch_search: failed to parse filters: %w", err)
	}

	ret := map[string][]string{}
	for rawKey, rawValue := range values {
		key := normalizeElasticsearchSearchFilterName(rawKey)
		if !isAllowedElasticsearchSearchFilter(key) {
			return nil, llm.NewRecoverableToolError("elasticsearch_search", fmt.Sprintf("unsupported filter '%s'", rawKey), "Use only supported filters: content_type, source, person, start_date, end_date, tag, collection, or mdb_uid.")
		}

		stringValues, err := normalizeElasticsearchSearchFilterValues(rawValue)
		if err != nil {
			return nil, llm.NewRecoverableToolError("elasticsearch_search", fmt.Sprintf("invalid values for filter '%s': %v", rawKey, err), "Filter values must be strings or arrays of strings.")
		}
		if len(stringValues) == 0 {
			return nil, llm.NewRecoverableToolError("elasticsearch_search", fmt.Sprintf("filter '%s' requires at least one non-empty value", rawKey), "Remove the empty filter or provide at least one non-empty value.")
		}

		if key == consts.FILTER_AUTHOR {
			key = consts.FILTER_SOURCE
		}
		ret[key] = appendUniqueStrings(ret[key], stringValues...)
	}

	return ret, nil
}

func normalizeElasticsearchSearchFilterValues(rawValue interface{}) ([]string, error) {
	switch value := rawValue.(type) {
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, nil
		}
		return []string{trimmed}, nil
	case []interface{}:
		ret := make([]string, 0, len(value))
		for _, rawItem := range value {
			item, ok := rawItem.(string)
			if !ok {
				return nil, fmt.Errorf("expected string array values")
			}
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			ret = appendUniqueStrings(ret, trimmed)
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("expected string or array of strings")
	}
}

func mergeElasticsearchSearchFilters(base map[string][]string, extra map[string][]string) map[string][]string {
	if base == nil {
		base = map[string][]string{}
	}
	for rawKey, values := range extra {
		key := normalizeElasticsearchSearchFilterName(rawKey)
		if key == consts.FILTER_AUTHOR {
			key = consts.FILTER_SOURCE
		}
		base[key] = appendUniqueStrings(base[key], values...)
	}
	return base
}

func setElasticsearchSearchLanguageOrder(query *search.Query, language string, languageExplicit bool) {
	if languageExplicit {
		query.LanguageOrder = appendUniqueStrings([]string{}, consts.SEARCH_LANG_ORDER[language]...)
	} else {
		detectTerms := append([]string{}, query.ExactTerms...)
		if query.Term != "" {
			detectTerms = append(detectTerms, query.Term)
		}
		query.LanguageOrder = utils.DetectLanguage(strings.Join(detectTerms, " "), language, "", nil)
	}

	if mediaLanguages, ok := query.Filters[consts.FILTER_MEDIA_LANGUAGE]; ok {
		query.LanguageOrder = appendUniqueStrings(query.LanguageOrder, mediaLanguages...)
	}

	if language == consts.LANG_SPANISH {
		reordered := []string{consts.LANG_SPANISH}
		for _, current := range query.LanguageOrder {
			if current != consts.LANG_SPANISH {
				reordered = append(reordered, current)
			}
		}
		query.LanguageOrder = appendUniqueStrings([]string{}, reordered...)
	}
}

func normalizeElasticsearchSearchSortBy(sortBy string, hasTextQuery bool) (string, error) {
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	if sortBy == "" {
		if hasTextQuery {
			return consts.SORT_BY_RELEVANCE, nil
		}
		return consts.SORT_BY_SOURCE_FIRST, nil
	}
	if sortBy == consts.SORT_BY_SOURCE_FIRST || consts.SORT_BY_VALUES[sortBy] {
		return sortBy, nil
	}
	return "", fmt.Errorf("elasticsearch_search: unsupported sort_by '%s'", sortBy)
}

func normalizeElasticsearchSearchFrom(from int) int {
	if from < 0 {
		return 0
	}
	return from
}

func normalizeElasticsearchSearchSize(size int) int {
	if size <= 0 {
		return consts.API_DEFAULT_PAGE_SIZE
	}
	if size > consts.API_MAX_PAGE_SIZE {
		return consts.API_MAX_PAGE_SIZE
	}
	return size
}

func buildElasticsearchSearchPreference(query search.Query, sortBy string, from int, size int) (string, error) {
	payload, err := json.Marshal(struct {
		Query  search.Query `json:"query"`
		SortBy string       `json:"sort_by"`
		From   int          `json:"from"`
		Size   int          `json:"size"`
	}{
		Query:  query,
		SortBy: sortBy,
		From:   from,
		Size:   size,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(payload)), nil
}

func normalizeElasticsearchSearchFilterName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isAllowedElasticsearchSearchFilter(filter string) bool {
	switch filter {
	case consts.FILTER_TAG,
		consts.FILTER_START_DATE,
		consts.FILTER_END_DATE,
		consts.FILTER_SOURCE,
		consts.FILTER_AUTHOR,
		consts.FILTER_CONTENT_TYPE,
		consts.FILTER_COLLECTIONS_CONTENT_TYPE,
		consts.FILTER_MDB_UID,
		consts.FILTER_MEDIA_LANGUAGE,
		consts.FILTER_ORIGINAL_LANGUAGE,
		consts.FILTER_PERSON,
		consts.FILTER_COLLECTION:
		return true
	default:
		return false
	}
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := map[string]bool{}
	ret := make([]string, 0, len(existing)+len(values))
	for _, value := range existing {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		ret = append(ret, value)
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		ret = append(ret, value)
	}
	return ret
}

func trimElasticsearchSearchOuterQuotes(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) >= 2 && isElasticsearchSearchQuotationMark(runes[0]) && runes[0] == runes[len(runes)-1] {
		return strings.TrimSpace(string(runes[1 : len(runes)-1]))
	}
	return string(runes)
}

func isElasticsearchSearchQuotationMark(r rune) bool {
	return unicode.In(r, unicode.Quotation_Mark) || r == rune(1523) || r == rune(1524)
}
