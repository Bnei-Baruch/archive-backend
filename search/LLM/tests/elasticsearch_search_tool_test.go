package tests

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/search"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	llmtools "github.com/Bnei-Baruch/archive-backend/search/LLM/tools"
)

type fakeElasticsearchSearchEngine struct {
	query              search.Query
	sortBy             string
	from               int
	size               int
	preference         string
	checkTypo          bool
	searchTweets       bool
	searchLessonSeries bool
	withHighlights     bool
	timeout            time.Duration

	result *search.QueryResult
	err    error
}

type elasticsearchSearchToolPayload struct {
	SortBy         string              `json:"sort_by"`
	Result         *search.QueryResult `json:"result,omitempty"`
	Error          string              `json:"error,omitempty"`
	RetrySuggested bool                `json:"retry_suggested,omitempty"`
	Guidance       string              `json:"guidance,omitempty"`
}

func (e *fakeElasticsearchSearchEngine) DoSearch(
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
) (*search.QueryResult, error) {
	_ = ctx
	e.query = query
	e.sortBy = sortBy
	e.from = from
	e.size = size
	e.preference = preference
	e.checkTypo = checkTypo
	e.searchTweets = searchTweets
	e.searchLessonSeries = searchLessonSeries
	e.withHighlights = withHighlights
	e.timeout = timeoutForHighlight
	return e.result, e.err
}

func TestElasticsearchSearchToolExecuteExactPhrase(t *testing.T) {
	engine := &fakeElasticsearchSearchEngine{
		result: &search.QueryResult{Language: consts.LANG_SPANISH},
	}
	tool := llmtools.NewElasticsearchSearchTool(engine, 2*time.Second)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"query":"\"love friends\"",
		"filters":{"content_type":"lesson","media_language":["he"]},
		"language":"es",
		"sort_by":"newertoolder",
		"from":-5,
		"size":2001,
		"exact_phrase":true
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(engine.query.ExactTerms, []string{"love friends"}) {
		t.Fatalf("unexpected exact terms: %#v", engine.query.ExactTerms)
	}
	if engine.query.Term != "" {
		t.Fatalf("expected empty term, got %q", engine.query.Term)
	}
	if !reflect.DeepEqual(engine.query.Filters[consts.FILTER_CONTENT_TYPE], []string{"lesson"}) {
		t.Fatalf("unexpected content_type filter: %#v", engine.query.Filters[consts.FILTER_CONTENT_TYPE])
	}
	if !engine.query.IncludeLikutim {
		t.Fatal("reasoning search must include LIKUTIM units")
	}
	if !reflect.DeepEqual(engine.query.Filters[consts.FILTER_MEDIA_LANGUAGE], []string{"he"}) {
		t.Fatalf("unexpected media_language filter: %#v", engine.query.Filters[consts.FILTER_MEDIA_LANGUAGE])
	}
	if len(engine.query.LanguageOrder) == 0 || engine.query.LanguageOrder[0] != consts.LANG_SPANISH {
		t.Fatalf("unexpected language order: %#v", engine.query.LanguageOrder)
	}
	if !containsString(engine.query.LanguageOrder, consts.LANG_HEBREW) {
		t.Fatalf("expected media language to be appended to language order: %#v", engine.query.LanguageOrder)
	}
	if engine.sortBy != consts.SORT_BY_NEWER_TO_OLDER {
		t.Fatalf("unexpected sort_by: %s", engine.sortBy)
	}
	if engine.from != 0 {
		t.Fatalf("unexpected from: %d", engine.from)
	}
	if engine.size != 50 {
		t.Fatalf("unexpected size: %d", engine.size)
	}
	if engine.preference == "" {
		t.Fatalf("expected non-empty preference")
	}
	if engine.checkTypo {
		t.Fatalf("expected checkTypo=false")
	}
	if !engine.searchTweets || !engine.withHighlights {
		t.Fatalf("expected searchTweets and withHighlights to be enabled")
	}
	if engine.searchLessonSeries {
		t.Fatalf("expected searchLessonSeries=false")
	}
	if engine.timeout != 2*time.Second {
		t.Fatalf("unexpected timeout: %s", engine.timeout)
	}

	payload := elasticsearchSearchToolPayload{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("failed to unmarshal tool result: %v", err)
	}
	if payload.SortBy != consts.SORT_BY_NEWER_TO_OLDER {
		t.Fatalf("unexpected payload sort_by: %s", payload.SortBy)
	}
	if payload.Result == nil || payload.Result.Language != consts.LANG_SPANISH {
		t.Fatalf("unexpected payload result: %#v", payload.Result)
	}
}

func TestElasticsearchSearchToolExecuteMergesParsedFilters(t *testing.T) {
	engine := &fakeElasticsearchSearchEngine{
		result: &search.QueryResult{Language: consts.LANG_ENGLISH},
	}
	tool := llmtools.NewElasticsearchSearchTool(engine, time.Second)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"query":"tag:daily author:rav transcript",
		"filters":{"person":["p1"]}
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if engine.query.Term != "transcript" {
		t.Fatalf("unexpected term: %q", engine.query.Term)
	}
	if !reflect.DeepEqual(engine.query.Filters[consts.FILTER_TAG], []string{"daily"}) {
		t.Fatalf("unexpected tag filter: %#v", engine.query.Filters[consts.FILTER_TAG])
	}
	if !reflect.DeepEqual(engine.query.Filters[consts.FILTER_SOURCE], []string{"rav"}) {
		t.Fatalf("unexpected source filter: %#v", engine.query.Filters[consts.FILTER_SOURCE])
	}
	if !reflect.DeepEqual(engine.query.Filters[consts.FILTER_PERSON], []string{"p1"}) {
		t.Fatalf("unexpected person filter: %#v", engine.query.Filters[consts.FILTER_PERSON])
	}
	if engine.sortBy != consts.SORT_BY_RELEVANCE {
		t.Fatalf("unexpected default sort_by: %s", engine.sortBy)
	}
	if engine.from != 0 || engine.size != 50 {
		t.Fatalf("unexpected paging: from=%d size=%d", engine.from, engine.size)
	}
}

func TestElasticsearchSearchToolPageSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		size, from, wantSize int
	}{
		{"omitted", 0, 0, 50},
		{"zero", 0, 0, 50},
		{"negative", -1, 0, 50},
		{"small", 10, 0, 10},
		{"maximum", 50, 0, 50},
		{"over-maximum", 51, 0, 50},
		{"large", 1000, 0, 50},
		{"next-page", 50, 50, 50},
		{"next-page-clamped", 1000, 100, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &fakeElasticsearchSearchEngine{}
			tool := llmtools.NewElasticsearchSearchTool(engine, 0)
			args := map[string]interface{}{"query": "test", "from": tc.from}
			if tc.name != "omitted" {
				args["size"] = tc.size
			}
			arguments, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			result, err := tool.Execute(context.Background(), arguments)
			if err != nil {
				t.Fatal(err)
			}
			if engine.size != tc.wantSize || engine.from != tc.from {
				t.Fatalf("unexpected paging: from=%d size=%d", engine.from, engine.size)
			}
			var payload struct {
				From int `json:"from"`
				Size int `json:"size"`
			}
			if err := json.Unmarshal([]byte(result), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Size != tc.wantSize || payload.From != tc.from {
				t.Fatalf("unexpected paging metadata: %#v", payload)
			}
		})
	}
}

func TestElasticsearchSearchToolDocumentsPagingLimit(t *testing.T) {
	tool := llmtools.NewElasticsearchSearchTool(&fakeElasticsearchSearchEngine{}, 0)
	parameters := tool.Definition().Parameters.(map[string]interface{})
	properties := parameters["properties"].(map[string]interface{})
	size := properties["size"].(map[string]interface{})
	if size["maximum"] != 50 {
		t.Fatalf("expected schema maximum 50, got %v", size["maximum"])
	}
	usage := tool.UsageExplanation()
	if !strings.Contains(usage, "maximum 50 results per call") || !strings.Contains(usage, "from=50") {
		t.Fatal("model instructions must explain the page size limit and from pagination")
	}
}

func TestElasticsearchSearchToolExecuteFilterOnlySearch(t *testing.T) {
	engine := &fakeElasticsearchSearchEngine{
		result: &search.QueryResult{Language: consts.LANG_ENGLISH},
	}
	tool := llmtools.NewElasticsearchSearchTool(engine, 0)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"filters":{"content_type":["lesson"]}
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if engine.query.Term != "" || len(engine.query.ExactTerms) != 0 {
		t.Fatalf("expected filter-only query, got term=%q exact=%#v", engine.query.Term, engine.query.ExactTerms)
	}
	if engine.sortBy != consts.SORT_BY_SOURCE_FIRST {
		t.Fatalf("unexpected sort_by for filter-only search: %s", engine.sortBy)
	}
}

func TestElasticsearchSearchToolExecuteAllowsMDBUIDFilter(t *testing.T) {
	engine := &fakeElasticsearchSearchEngine{
		result: &search.QueryResult{Language: consts.LANG_HEBREW},
	}
	tool := llmtools.NewElasticsearchSearchTool(engine, 0)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"filters":{"mdb_uid":["4AN1qAqj"]},
		"language":"he",
		"size":1
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(engine.query.Filters[consts.FILTER_MDB_UID], []string{"4AN1qAqj"}) {
		t.Fatalf("unexpected mdb_uid filter: %#v", engine.query.Filters[consts.FILTER_MDB_UID])
	}
	if engine.size != 1 {
		t.Fatalf("unexpected size: %d", engine.size)
	}
}

func TestElasticsearchSearchToolExecuteRejectsUnknownFilter(t *testing.T) {
	tool := llmtools.NewElasticsearchSearchTool(&fakeElasticsearchSearchEngine{}, 0)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"filters":{"unsupported":["x"]}
	}`))
	if err == nil {
		t.Fatalf("expected unsupported filter error")
	}
	if !strings.Contains(err.Error(), "unsupported filter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestElasticsearchSearchToolExecuteReturnsRecoverableErrorForMalformedArguments(t *testing.T) {
	tool := llmtools.NewElasticsearchSearchTool(&fakeElasticsearchSearchEngine{}, 0)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"אהבה" ×}`))
	if err == nil {
		t.Fatalf("expected malformed argument error")
	}
	var recoverable *llm.RecoverableToolError
	if !errors.As(err, &recoverable) {
		t.Fatalf("expected recoverable error, got %T %v", err, err)
	}
	if !strings.Contains(recoverable.Message, "valid JSON") {
		t.Fatalf("unexpected recoverable message: %q", recoverable.Message)
	}
}

func TestElasticsearchSearchToolExecuteReturnsRetryPayloadOnSearchRuntimeError(t *testing.T) {
	engine := &fakeElasticsearchSearchEngine{
		err: errors.New("es timeout"),
	}
	tool := llmtools.NewElasticsearchSearchTool(engine, 0)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"query":"ד' בחינות דאור ישר",
		"language":"he"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := elasticsearchSearchToolPayload{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("failed to unmarshal tool result: %v", err)
	}
	if payload.Error == "" {
		t.Fatalf("expected error payload, got %#v", payload)
	}
	if !payload.RetrySuggested {
		t.Fatalf("expected retry guidance, got %#v", payload)
	}
	if !strings.Contains(payload.Guidance, "Try another query variation") {
		t.Fatalf("unexpected guidance: %q", payload.Guidance)
	}
	if payload.Result != nil {
		t.Fatalf("expected nil result on runtime error, got %#v", payload.Result)
	}
}

func TestElasticsearchSearchToolCancellationDoesNotSuggestRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tool := llmtools.NewElasticsearchSearchTool(&fakeElasticsearchSearchEngine{err: context.Canceled}, 0)
	result, err := tool.Execute(ctx, json.RawMessage(`{"query":"test"}`))
	if !errors.Is(err, context.Canceled) || result != "" {
		t.Fatalf("expected cancellation error without retry payload, got result=%q err=%v", result, err)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
