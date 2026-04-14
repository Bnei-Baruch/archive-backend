package tests

import (
	"strings"
	"testing"
	"time"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
)

func TestReasoningSearchCacheKeyForQueryNormalizesPlusSeparatedQuery(t *testing.T) {
	key, ok := llm.ReasoningSearchCacheKeyForQuery("חיים חדשים+ברית הנישואין")
	if !ok {
		t.Fatalf("expected query to be cacheable")
	}
	if key != "חיים חדשים ברית הנישואין" {
		t.Fatalf("unexpected cache key: %q", key)
	}
}

func TestReasoningSearchCacheKeyForQueryRejectsTemporalQueries(t *testing.T) {
	testCases := []string{
		"today lesson",
		"שיעור של היום",
		"היום בשיעור",
		"בשיעור היום",
		"מאתמול",
		"מסוף השבוע",
		"בסוף השבוע",
		"בשבת",
		"מיום ראשון",
		"sunday lesson",
		"в воскресенье",
		"субботний урок",
		"domingo",
		"сегодняшний урок",
		"lección de hoy",
	}

	for _, query := range testCases {
		if key, ok := llm.ReasoningSearchCacheKeyForQuery(query); ok {
			t.Fatalf("expected temporal query %q to bypass cache, got key %q", query, key)
		}
	}
}

func TestReasoningSearchCacheKeyForQueryRejectsLongQueries(t *testing.T) {
	if key, ok := llm.ReasoningSearchCacheKeyForQuery("one two three four five"); ok {
		t.Fatalf("expected query to bypass cache, got key %q", key)
	}
}

func TestBuildReasoningSearchCacheEntryFromResponseStripsMetadata(t *testing.T) {
	entry := llm.BuildReasoningSearchCacheEntryFromResponse(&llm.ReasoningSearchResponse{
		Query:   "ד' בחינות דאור ישר",
		Summary: "summary",
		Results: []llm.ReasoningSearchResult{
			{
				MDBUID:           "abc",
				ResultType:       "units",
				Title:            "Title",
				Description:      "Desc",
				ContentType:      "VIDEO_PROGRAM_CHAPTER",
				Date:             "2024-01-01",
				Reason:           "why",
				Highlights:       []string{"match"},
				IsGroupingResult: true,
			},
		},
	})

	if entry == nil {
		t.Fatalf("expected cache entry")
	}
	if len(entry.Results) != 1 {
		t.Fatalf("unexpected results length: %d", len(entry.Results))
	}
	if entry.Results[0].Title != "" || entry.Results[0].Description != "" || entry.Results[0].ContentType != "" || entry.Results[0].Date != "" {
		t.Fatalf("expected metadata fields to be stripped: %#v", entry.Results[0])
	}
	if entry.Results[0].Reason != "why" {
		t.Fatalf("unexpected reason: %#v", entry.Results[0])
	}
}

func TestBuildReasoningSearchCacheEntryFromResponseSkipsEmptyResults(t *testing.T) {
	entry := llm.BuildReasoningSearchCacheEntryFromResponse(&llm.ReasoningSearchResponse{
		Query:   "query",
		Summary: "summary",
		Results: []llm.ReasoningSearchResult{},
	})
	if entry != nil {
		t.Fatalf("expected nil cache entry for empty results, got %#v", entry)
	}
}

func TestReasoningWorkflowSessionStoreCachedInitialResponseLifecycle(t *testing.T) {
	store := llm.NewReasoningWorkflowSessionStore(5 * time.Minute)
	defer store.Close()

	sessionID, err := store.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:        "openrouter",
		Model:           "model",
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	entry := &llm.ReasoningSearchCacheEntry{
		Query:   "חיים חדשים נישואין פרק ב",
		Summary: "summary",
		Results: []llm.ReasoningSearchResult{{MDBUID: "abc", ResultType: "units", Reason: "why"}},
	}
	if err := store.SetCachedInitialResponse(sessionID, entry); err != nil {
		t.Fatalf("unexpected cached response set error: %v", err)
	}

	session, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if session.CachedInitialResponse == nil {
		t.Fatalf("expected cached initial response")
	}
	if session.CachedInitialResponse.Query != entry.Query {
		t.Fatalf("unexpected cached query: %q", session.CachedInitialResponse.Query)
	}

	session.CachedInitialResponse.Summary = "changed"

	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if session.CachedInitialResponse.Summary != "summary" {
		t.Fatalf("expected cached seed copy to be isolated, got %q", session.CachedInitialResponse.Summary)
	}

	if err := store.SetCachedInitialResponse(sessionID, nil); err != nil {
		t.Fatalf("unexpected clear error: %v", err)
	}
	session, err = store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected third get error: %v", err)
	}
	if session.CachedInitialResponse != nil {
		t.Fatalf("expected cached initial response to be cleared")
	}
}

func TestBuildReasoningSearchCacheSeedAssistantContent(t *testing.T) {
	content := llm.BuildReasoningSearchCacheSeedAssistantContent(&llm.ReasoningSearchCacheEntry{
		Query:   "query",
		Summary: "summary",
		Results: []llm.ReasoningSearchResult{{MDBUID: "abc", ResultType: "units", Reason: "why"}},
	})
	if !strings.Contains(content, "Cached initial reasoning search response:") {
		t.Fatalf("unexpected seed content: %q", content)
	}
	if !strings.Contains(content, "\"mdb_uid\":\"abc\"") {
		t.Fatalf("expected cached seed to include result identifiers: %q", content)
	}
}
