package llm

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/search"
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

type ElasticsearchSearchTool struct {
	engine              ElasticsearchSearchEngine
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
	Query  search.Query        `json:"query"`
	SortBy string              `json:"sort_by"`
	From   int                 `json:"from"`
	Size   int                 `json:"size"`
	Result *search.QueryResult `json:"result,omitempty"`
}

func NewElasticsearchSearchTool(engine ElasticsearchSearchEngine, timeoutForHighlight time.Duration) *ElasticsearchSearchTool {
	return &ElasticsearchSearchTool{
		engine:              engine,
		timeoutForHighlight: timeoutForHighlight,
	}
}

func (t *ElasticsearchSearchTool) Definition() ReasoningToolDefinition {
	return ReasoningToolDefinition{
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
					"description": "Optional search filters. Keys should use archive filter names such as content_type, source, tag, media_language, original_language, person, start_date, end_date, or collection.",
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

func (t *ElasticsearchSearchTool) Execute(arguments json.RawMessage) (string, error) {
	if t.engine == nil {
		return "", fmt.Errorf("elasticsearch_search: engine is nil")
	}

	args := elasticsearchSearchToolArgs{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("elasticsearch_search: failed to parse arguments: %w", err)
	}

	filters, err := normalizeElasticsearchSearchFilters(args.Filters)
	if err != nil {
		return "", err
	}

	queryText := strings.TrimSpace(args.Query)
	if queryText == "" && len(filters) == 0 {
		return "", fmt.Errorf("elasticsearch_search: either query or filters must be provided")
	}
	if args.ExactPhrase && queryText == "" {
		return "", fmt.Errorf("elasticsearch_search: exact_phrase requires a non-empty query")
	}

	query := buildElasticsearchSearchQuery(queryText, args.ExactPhrase)
	if args.ExactPhrase && len(query.ExactTerms) == 1 && query.ExactTerms[0] == "" {
		return "", fmt.Errorf("elasticsearch_search: exact_phrase requires a non-empty query")
	}
	query.Filters = mergeElasticsearchSearchFilters(query.Filters, filters)

	language := normalizeElasticsearchSearchLanguage(args.Language)
	setElasticsearchSearchLanguageOrder(&query, language)

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

	result, err := t.engine.DoSearch(
		context.Background(),
		query,
		sortBy,
		from,
		size,
		preference,
		false,
		true,
		true,
		true,
		t.timeoutForHighlight,
	)
	if err != nil {
		return "", fmt.Errorf("elasticsearch_search: DoSearch failed: %w", err)
	}

	return marshalPostgreSQLToolResult(elasticsearchSearchToolResult{
		Query:  query,
		SortBy: sortBy,
		From:   from,
		Size:   size,
		Result: result,
	})
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
			return nil, fmt.Errorf("elasticsearch_search: unsupported filter '%s'", rawKey)
		}

		stringValues, err := normalizeElasticsearchSearchFilterValues(rawValue)
		if err != nil {
			return nil, fmt.Errorf("elasticsearch_search: invalid values for filter '%s': %w", rawKey, err)
		}
		if len(stringValues) == 0 {
			return nil, fmt.Errorf("elasticsearch_search: filter '%s' requires at least one non-empty value", rawKey)
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

func normalizeElasticsearchSearchLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return consts.DEFAULT_UI_LANGUAGE
	}
	if _, ok := consts.SEARCH_LANG_ORDER[language]; !ok {
		return consts.DEFAULT_UI_LANGUAGE
	}
	return language
}

func setElasticsearchSearchLanguageOrder(query *search.Query, language string) {
	detectTerms := append([]string{}, query.ExactTerms...)
	if query.Term != "" {
		detectTerms = append(detectTerms, query.Term)
	}

	query.LanguageOrder = utils.DetectLanguage(strings.Join(detectTerms, " "), language, "", nil)

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
