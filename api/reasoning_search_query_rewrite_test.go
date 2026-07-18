package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/spf13/viper"
)

func TestRewriteReasoningSearchQueryAddsApostropheToStandaloneHebrewLetters(t *testing.T) {
	cases := map[string]string{
		"ד בחינות דאור ישר":   "ד' בחינות דאור ישר",
		"א ב ג ד ה ו ז ח ט י": "א' ב' ג' ד' ה' ו' ז' ח' ט' י'",
		"פרק ב":               "פרק ב'",
		"אות י.":              "אות י'.",
	}

	for input, expected := range cases {
		if got := rewriteReasoningSearchQuery(input); got != expected {
			t.Fatalf("rewriteReasoningSearchQuery(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestRewriteReasoningSearchQueryKeepsExistingGereshAndWords(t *testing.T) {
	cases := map[string]string{
		"ד' בחינות דאור ישר": "ד' בחינות דאור ישר",
		"ד׳ בחינות דאור ישר": "ד׳ בחינות דאור ישר",
		"יא בחינות":          "יא בחינות",
		"דאור ישר":           "דאור ישר",
	}

	for input, expected := range cases {
		if got := rewriteReasoningSearchQuery(input); got != expected {
			t.Fatalf("rewriteReasoningSearchQuery(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestValidateReasoningSearchResponseQueryAllowsQuotePunctuationVariants(t *testing.T) {
	response := &llm.ReasoningSearchResponse{Query: "ציטוטים על ט''ו בשבט"}
	err := validateReasoningSearchResponseQuery("ציטוטים על ט'\"ו' בשבט", response)
	if err != nil {
		t.Fatalf("expected quote punctuation variant to pass, got %v", err)
	}
}

func TestReasoningSearchBlogOriginalLanguage(t *testing.T) {
	cases := []struct {
		name string
		blog *mdbmodels.Blog
		want string
	}{
		{name: "ru by type", blog: &mdbmodels.Blog{Name: "laitman-ru"}, want: consts.LANG_RUSSIAN},
		{name: "es by type", blog: &mdbmodels.Blog{Name: "laitman-es"}, want: consts.LANG_SPANISH},
		{name: "he by type", blog: &mdbmodels.Blog{Name: "laitman-co-il"}, want: consts.LANG_HEBREW},
		{name: "en by type", blog: &mdbmodels.Blog{Name: "laitman-com"}, want: consts.LANG_ENGLISH},
		{name: "unknown", blog: &mdbmodels.Blog{Name: "unknown", URL: "https://example.com"}, want: ""},
	}

	for _, tc := range cases {
		if got := reasoningSearchBlogOriginalLanguage(tc.blog); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestValidateReasoningSearchResponseQueryRejectsTopicDrift(t *testing.T) {
	response := &llm.ReasoningSearchResponse{Query: "מחשבת הבריאה"}
	err := validateReasoningSearchResponseQuery("נס", response)
	if !errors.Is(err, errReasoningSearchQueryMismatch) {
		t.Fatalf("expected query mismatch error, got %v", err)
	}
}

func TestBuildReasoningSearchHighlightsRanksAIAndRelevantESHighlights(t *testing.T) {
	result := llm.ReasoningSearchResult{
		Highlights: []llm.ReasoningSearchHighlight{
			{Field: "content", Text: "<em>משה</em>"},
			{Field: "content", Text: "רק מילת קישור <em>עם</em>"},
			{Field: "content", Text: "פתיח קצר על <em>משה</em>"},
			{Field: "content", Text: "קטע ארוך יותר שמסביר את <em>משה</em> ואת <em>תפקידו</em> בהקשר השאלה"},
		},
		LookupEvidence: []llm.ReasoningSearchResultEvidence{
			{SupportingSnippet: "עדות מכלי AI על התאמת המקור לשאלה"},
		},
	}

	highlights := buildReasoningSearchHighlightsFromEvidence(result, "משה", "תפקידו של משה")
	if len(highlights) < 3 {
		t.Fatalf("expected ranked highlights, got %#v", highlights)
	}
	if highlights[0].Field != "content" || highlights[0].Text != "עדות מכלי AI על התאמת המקור לשאלה" {
		t.Fatalf("expected AI evidence first, got %#v", highlights)
	}
	if highlights[1].Field != "content" || highlights[1].Text != "קטע ארוך יותר שמסביר את <em>משה</em> ואת <em>תפקידו</em> בהקשר השאלה" {
		t.Fatalf("expected richer ES highlight second, got %#v", highlights)
	}
	if highlights[2].Text == "רק מילת קישור <em>עם</em>" {
		t.Fatalf("expected short linking-word emphasis not to rank near the top, got %#v", highlights)
	}
}

func TestBuildReasoningSearchHighlightsPrefersReasonAnchors(t *testing.T) {
	// The final reason may cite a phrase that was not the strongest ES highlight.
	// Ranking should still surface snippets that support the selected reason.
	result := llm.ReasoningSearchResult{
		Highlights: []llm.ReasoningSearchHighlight{
			{Field: "content", Text: `נמצא מה שהאדם מתייגע בתורה ומצות, הוא מסיבת שחסר לנו את חשיבותו וגדלותו של הבורא יתברך.`},
			{Field: "content", Text: `וזה ענין שותפות, שיש להנבראים עם הבורא.`},
			{Field: "content", Text: `עמי אתה - להיות שותף עמי.`},
		},
	}

	highlights := buildReasoningSearchHighlightsFromEvidence(result, "שותפות עם הבורא", `נראה כהתאמה הטובה ביותר: "עמי אתה" - "להיות שותף עמי" וכן "וזה ענין שותפות"`)
	if len(highlights) < 2 {
		t.Fatalf("expected ranked highlights, got %#v", highlights)
	}
	if highlights[0].Text != `עמי אתה - להיות שותף עמי.` && highlights[0].Text != `וזה ענין שותפות, שיש להנבראים עם הבורא.` {
		t.Fatalf("expected reason-supporting highlight first, got %#v", highlights)
	}
	if highlights[len(highlights)-1].Text == `עמי אתה - להיות שותף עמי.` {
		t.Fatalf("expected quoted reason anchor not to sink to the bottom, got %#v", highlights)
	}
}

func TestTruncateHighlightForDisplayCentersReasonAnchor(t *testing.T) {
	text := strings.Repeat("פתיחה רחוקה ", 80) +
		`וזה ענין שותפות, שיש להנבראים עם הבורא.` +
		strings.Repeat(" המשך רחוק", 80)

	got := truncateHighlightForDisplay(text, 80, "שותפות עם הבורא", `נראה מתאים בגלל "וזה ענין שותפות"`)
	if !strings.Contains(got, "וזה ענין שותפות") {
		t.Fatalf("expected truncated highlight to include reason anchor, got %q", got)
	}
	if strings.Contains(got, "...") {
		t.Fatalf("did not expect ellipses in truncated highlight, got %q", got)
	}
}

func TestTruncateHighlightForDisplayPreservesEmphasisAroundAnchor(t *testing.T) {
	text := strings.Repeat("רקע רחוק ", 80) +
		`לפני <em>משה רבנו</em> אחרי` +
		strings.Repeat(" המשך רחוק", 80)

	got := truncateHighlightForDisplay(text, 60, "משה רבנו", "")
	if !strings.Contains(got, "<em>משה רבנו</em>") {
		t.Fatalf("expected truncated highlight to preserve emphasized anchor, got %q", got)
	}
	if strings.Contains(got, "...") {
		t.Fatalf("did not expect ellipses in truncated highlight, got %q", got)
	}
}

func TestCompactReasoningSearchHighlightsRemovesOverlappingSnippets(t *testing.T) {
	// A long lookup/source snippet can cover several shorter ES highlights.
	// The client should get distinct display snippets, not nested duplicates.
	highlights := []llm.ReasoningSearchHighlight{
		{Field: "content", Text: "תחילת קטע לכן כשיש איזו אסיפה של חברים, צריכים לזכור להעלות על השולחן את השאלה. דהיינו, שכל אחד ישאל לעצמו, כמה כבר אנו התקדמנו באהבת הזולת."},
		{Field: "content.language", Text: "לכן כשיש איזו אסיפה של <em>חברים</em>, צריכים לזכור להעלות על השולחן את השאלה."},
		{Field: "content.language", Text: "דהיינו, שכל אחד ישאל לעצמו, <em>כמה</em> כבר אנו <em>התקדמנו</em> <em>באהבת</em> הזולת."},
		{Field: "content", Text: "קטע נוסף שאינו חופף על חשיבות החברה והעבודה המשותפת."},
	}

	got := compactReasoningSearchHighlightsForDisplay(highlights, "שולחן התקדמנו אהבת חברים", "")
	if len(got) != 2 {
		t.Fatalf("expected overlapping highlights to compact to 2 distinct snippets, got %#v", got)
	}
	if !strings.Contains(visibleHighlightText(got[0].Text), "להעלות על השולחן") || !strings.Contains(visibleHighlightText(got[0].Text), "התקדמנו באהבת") {
		t.Fatalf("expected long source snippet to remain, got %#v", got)
	}
	if !strings.Contains(visibleHighlightText(got[1].Text), "קטע נוסף") {
		t.Fatalf("expected distinct highlight to fill available slot, got %#v", got)
	}
}

func TestBuildRapidVisibleResultsOmitsNotRelevantAndSortsByRelevance(t *testing.T) {
	session := &llm.ReasoningWorkflowSession{
		PartialResults: []llm.ReasoningSearchResult{
			{MDBUID: "maybe", ContentType: "SOURCE", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: "maybe"}}},
			{MDBUID: "no", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: "no"}}},
			{MDBUID: "strong", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: "strong"}}},
			{MDBUID: "program", ContentType: "VIDEO_PROGRAM_CHAPTER", Highlights: []llm.ReasoningSearchHighlight{{Field: "content", Text: "program"}}},
		},
		RapidClassifications: map[string]llm.ReasoningSearchRapidClassification{
			"maybe":   {MDBUID: "maybe", Relevance: "can_be_relevant", Reason: "maybe reason"},
			"no":      {MDBUID: "no", Relevance: "not_relevant", Reason: ""},
			"strong":  {MDBUID: "strong", Relevance: "highly_relevant", Reason: "strong reason"},
			"program": {MDBUID: "program", Relevance: "can_be_relevant", Reason: "program reason"},
		},
	}

	results, err := buildRapidVisibleResults(nil, "", session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected three visible results, got %#v", results)
	}
	if results[0].MDBUID != "strong" || results[0].Reason != "strong reason" || results[0].Relevance != "highly_relevant" {
		t.Fatalf("expected strong result first, got %#v", results)
	}
	if results[1].MDBUID != "program" || results[1].Reason != "program reason" || results[1].Relevance != "can_be_relevant" {
		t.Fatalf("expected program chapter before other same-relevance results, got %#v", results)
	}
	if results[2].MDBUID != "maybe" || results[2].Reason != "maybe reason" || results[2].Relevance != "can_be_relevant" {
		t.Fatalf("expected possible result third, got %#v", results)
	}
}

func TestBuildRapidVisibleResultsDropsPossibleResultsWhenEnoughGoodResults(t *testing.T) {
	partialResults := []llm.ReasoningSearchResult{}
	classifications := map[string]llm.ReasoningSearchRapidClassification{}
	for i := 0; i < reasoningSearchRapidGoodResultsThreshold; i++ {
		uid := fmt.Sprintf("good-%d", i)
		partialResults = append(partialResults, llm.ReasoningSearchResult{MDBUID: uid})
		classifications[uid] = llm.ReasoningSearchRapidClassification{MDBUID: uid, Relevance: "relevant", Reason: "good"}
	}
	partialResults = append(partialResults, llm.ReasoningSearchResult{MDBUID: "maybe"})
	classifications["maybe"] = llm.ReasoningSearchRapidClassification{MDBUID: "maybe", Relevance: "can_be_relevant", Reason: "maybe"}

	results, err := buildRapidVisibleResults(nil, "", &llm.ReasoningWorkflowSession{
		PartialResults:       partialResults,
		RapidClassifications: classifications,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, result := range results {
		if result.MDBUID == "maybe" {
			t.Fatalf("did not expect can_be_relevant when enough good results exist: %#v", results)
		}
	}
}

func TestSortRapidVisibleResultsCanRunAgainAfterEnrichment(t *testing.T) {
	// Rapid status can expose results before DB enrichment completes. Sorting must
	// be safe to run once with sparse metadata and again after content_type exists.
	classifications := map[string]llm.ReasoningSearchRapidClassification{
		"source":  {MDBUID: "source", Relevance: "highly_relevant"},
		"program": {MDBUID: "program", Relevance: "highly_relevant"},
	}
	results := []llm.ReasoningSearchResult{
		{MDBUID: "source", ContentType: "SOURCE"},
		{MDBUID: "program"},
	}

	sortRapidVisibleResults(results, classifications, "")
	if results[0].MDBUID != "source" {
		t.Fatalf("expected initial sort to preserve stable order without content type, got %#v", results)
	}

	results[1].ContentType = "VIDEO_PROGRAM_CHAPTER"
	sortRapidVisibleResults(results, classifications, "")
	if results[0].MDBUID != "program" {
		t.Fatalf("expected second sort after enrichment to prioritize program chapter, got %#v", results)
	}
}

func TestSortRapidVisibleResultsUsesSoftBoosts(t *testing.T) {
	// Scores are only tie-breakers inside the same classifier relevance bucket.
	// Assert the boosts directly so the test documents each backend preference.
	base := rapidResultSortScore(llm.ReasoningSearchResult{ContentType: consts.CT_SOURCE}, consts.LANG_HEBREW)

	boosted := []llm.ReasoningSearchResult{
		{ContentType: consts.CT_VIDEO_PROGRAM_CHAPTER},
		{ContentType: consts.CT_CLIP},
		{ContentType: consts.CT_LIKUTIM},
		{ContentType: consts.CT_SOURCE, BaalSulamArticle: true},
		{ContentType: consts.CT_SOURCE, ConnectingSource: true},
		{ContentType: consts.CT_SOURCE, OriginalLanguage: consts.LANG_HEBREW},
		{ContentType: consts.CT_SOURCE, Date: time.Now().Format("2006-01-02")},
		{ContentType: consts.CT_SOURCE, LookupEvidence: []llm.ReasoningSearchResultEvidence{{Content: "evidence"}}},
	}
	for _, result := range boosted {
		if score := rapidResultSortScore(result, consts.LANG_HEBREW); score <= base {
			t.Fatalf("expected boosted result %#v to score above base source score %d, got %d", result, base, score)
		}
	}

	if rapidResultSortScore(llm.ReasoningSearchResult{ContentType: consts.CT_LIKUTIM}, "") != rapidResultSortScore(llm.ReasoningSearchResult{ConnectingSource: true}, "") {
		t.Fatalf("expected LIKUTIM and Connecting to Source to get the same boost")
	}
	program := llm.ReasoningSearchResult{ContentType: consts.CT_VIDEO_PROGRAM_CHAPTER}
	world := llm.ReasoningSearchResult{ContentType: consts.CT_VIDEO_PROGRAM_CHAPTER, CollectionUID: consts.PROGRAM_COLLECTION_EL_MUNDO}
	if rapidResultSortScore(world, consts.LANG_HEBREW) >= rapidResultSortScore(program, consts.LANG_HEBREW) {
		t.Fatalf("expected El Mundo chapters to be de-prioritized outside Spanish UI")
	}
	if rapidResultSortScore(world, consts.LANG_SPANISH) != rapidResultSortScore(program, consts.LANG_SPANISH) {
		t.Fatalf("did not expect El Mundo penalty for Spanish UI")
	}
}

func TestRapidResponseCanReturnNullSummary(t *testing.T) {
	response := llm.ReasoningSearchResponse{
		Query:   "query",
		Summary: nil,
		Results: []llm.ReasoningSearchResult{},
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if _, ok := payload["summary"]; !ok {
		t.Fatalf("expected summary field in response: %s", string(raw))
	}
	if payload["summary"] != nil {
		t.Fatalf("expected null summary, got %#v in %s", payload["summary"], string(raw))
	}
}

func TestOrderRapidFinalizerResultsUsesModelOrderAndKeepsMissingCandidates(t *testing.T) {
	candidates := []llm.ReasoningSearchResult{
		{MDBUID: "a", Reason: "reason a"},
		{MDBUID: "b", Reason: "reason b"},
		{MDBUID: "c", Reason: "reason c"},
	}

	results := orderRapidFinalizerResults(candidates, []string{"c", "missing", "a", "c"})

	got := []string{}
	for _, result := range results {
		got = append(got, result.MDBUID)
	}
	expected := []string{"c", "a", "b"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected order: got %#v want %#v", got, expected)
	}
	if results[0].Reason != "reason c" {
		t.Fatalf("expected classifier reason to be preserved, got %#v", results[0])
	}
}

func TestPrepareReasoningSearchSessionAllowsFollowupWhenSnapshotExistsDespiteRunningProgress(t *testing.T) {
	// A response snapshot means a previous background run already produced a
	// user-visible result. Stale running progress must not block the follow-up.
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "openai",
		Model:             "gpt-5.4",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected follow-up state error: %v", err)
	}
	summary := "done"
	if err := workflow.SetResponseSnapshot(sessionID, &llm.ReasoningSearchResponse{
		Query:   "משה",
		Summary: &summary,
	}); err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	_, err = prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if err != nil {
		t.Fatalf("expected stale running progress not to block follow-up, got %v", err)
	}
}

func TestPrepareReasoningSearchSessionFinalizesReadyDraftForFastFollowup(t *testing.T) {
	// If the user requested finish-now and immediately sends a follow-up, finalize
	// the ready draft first so the follow-up has visible prior context.
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()
	service := llm.NewStubLLMService(nil)

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "stub-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "משה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := workflow.AddPartialResults(sessionID, []llm.ReasoningSearchResult{
		{MDBUID: "uid-1", Title: "draft result"},
	}); err != nil {
		t.Fatalf("unexpected partial results error: %v", err)
	}
	_, revision, _, ok, err := workflow.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start: ok=%t err=%v", ok, err)
	}
	summary := "draft"
	draft := &llm.ReasoningSearchResponse{
		Query:   "משה",
		Summary: &summary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "uid-1", Title: "draft result"},
		},
	}
	draft.SetSessionID(sessionID)
	if err := workflow.FinishDraft(sessionID, revision, draft); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}
	if err := workflow.RequestFinishNow(sessionID); err != nil {
		t.Fatalf("unexpected finish-now request error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	gotSessionID, err := prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
		Services: map[string]llm.Service{"stub": service},
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if err != nil {
		t.Fatalf("expected ready draft to be finalized for fast follow-up, got %v", err)
	}
	if gotSessionID != sessionID {
		t.Fatalf("unexpected session id: %q", gotSessionID)
	}
	session, err := workflow.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected workflow get error: %v", err)
	}
	if session.FinalizedFromDraft {
		t.Fatalf("expected follow-up preparation to reset finalized-from-draft")
	}
	if session.DraftFollowupSeed == nil || session.DraftFollowupSeed.Summary == nil || *session.DraftFollowupSeed.Summary != "draft" {
		t.Fatalf("expected draft follow-up seed, got %#v", session.DraftFollowupSeed)
	}
}

func TestPrepareReasoningSearchSessionBlocksFollowupWhenDraftReadyButFinishNowWasNotRequested(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "stub-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "משה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if _, _, err := workflow.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}); err != nil {
		t.Fatalf("unexpected partial results error: %v", err)
	}
	_, revision, _, ok, err := workflow.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start: ok=%t err=%v", ok, err)
	}
	summary := "draft"
	draft := &llm.ReasoningSearchResponse{Query: "משה", Summary: &summary, Results: []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}}
	draft.SetSessionID(sessionID)
	if err := workflow.FinishDraft(sessionID, revision, draft); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	_, err = prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if !errors.Is(err, errReasoningSearchAlreadyRunning) {
		t.Fatalf("expected running error without finish-now request, got %v", err)
	}
}

func TestPrepareReasoningSearchSessionBlocksFollowupAfterFailedFinishNowAttempt(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "stub-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "משה", ""); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if err := workflow.RequestFinishNow(sessionID); err != nil {
		t.Fatalf("unexpected finish-now request error: %v", err)
	}
	if err := workflow.ClearFinishNowRequest(sessionID); err != nil {
		t.Fatalf("unexpected clear finish-now request error: %v", err)
	}
	if _, _, err := workflow.AddPartialResults(sessionID, []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}); err != nil {
		t.Fatalf("unexpected partial results error: %v", err)
	}
	_, revision, _, ok, err := workflow.TryStartDraft(sessionID, 0, 1)
	if err != nil || !ok {
		t.Fatalf("unexpected draft start: ok=%t err=%v", ok, err)
	}
	summary := "draft"
	draft := &llm.ReasoningSearchResponse{Query: "משה", Summary: &summary, Results: []llm.ReasoningSearchResult{{MDBUID: "uid-1"}}}
	draft.SetSessionID(sessionID)
	if err := workflow.FinishDraft(sessionID, revision, draft); err != nil {
		t.Fatalf("unexpected finish draft error: %v", err)
	}
	progress.Reserve(sessionID)
	progress.Thinking(sessionID, 1, false)

	requestSessionID := sessionID
	_, err = prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:     "follow-up",
		SessionID: &requestSessionID,
	})
	if !errors.Is(err, errReasoningSearchAlreadyRunning) {
		t.Fatalf("expected running error after failed finish-now attempt, got %v", err)
	}
}

func TestPrepareReasoningSearchSessionSwitchesNonRapidFollowupToRapid(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()
	service := llm.NewStubLLMService(nil)

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "non-rapid-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "non-rapid-provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "initial query", "he"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if err := workflow.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected follow-up state error: %v", err)
	}
	summary := "non-rapid result"
	if err := workflow.SetResponseSnapshot(sessionID, &llm.ReasoningSearchResponse{
		Query:   "initial query",
		Summary: &summary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "non-rapid-uid", Title: "non-rapid result"},
		},
	}); err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}

	requestSessionID := sessionID
	gotSessionID, err := prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
		Services: map[string]llm.Service{"stub": service},
		RapidConfig: &llm.ReasoningSearchRapidConfig{
			Gather: llm.ReasoningSearchRapidStageConfig{
				Provider:      "stub",
				Model:         "rapid-gather-model",
				Effort:        "low",
				MaxTokens:     1000,
				MaxIterations: 3,
			},
			Classifier: llm.ReasoningSearchRapidStageConfig{
				Provider:  "stub",
				Model:     "rapid-classifier-model",
				MaxTokens: 1000,
			},
		},
	}, &ReasoningSearchRequest{
		Query:      "rapid follow-up",
		SessionID:  &requestSessionID,
		UILanguage: "he",
		IsRapid:    true,
	})
	if err != nil {
		t.Fatalf("unexpected mode switch error: %v", err)
	}
	if gotSessionID != sessionID {
		t.Fatalf("unexpected session id: %q", gotSessionID)
	}

	session, err := workflow.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected workflow get error: %v", err)
	}
	if !session.Rapid {
		t.Fatalf("expected session to switch to rapid mode")
	}
	if session.FollowupCount != 1 {
		t.Fatalf("unexpected follow-up count: %d", session.FollowupCount)
	}
	if len(session.ResponseSnapshotJSON) != 0 {
		t.Fatalf("expected previous snapshot to be cleared")
	}
	if session.RapidFollowupSeed == nil || session.RapidFollowupSeed.Summary == nil || *session.RapidFollowupSeed.Summary != "non-rapid result" {
		t.Fatalf("expected previous response as rapid seed, got %#v", session.RapidFollowupSeed)
	}
	reasoningStage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	if reasoningStage.ProviderSessionID == "" || reasoningStage.ProviderSessionID == "non-rapid-provider-session" {
		t.Fatalf("expected fresh rapid provider session, got %q", reasoningStage.ProviderSessionID)
	}
	if reasoningStage.Model != "rapid-gather-model" {
		t.Fatalf("unexpected rapid gather model: %q", reasoningStage.Model)
	}
}

func TestPrepareReasoningSearchSessionSwitchesRapidFollowupToNonRapid(t *testing.T) {
	oldProvider := viper.Get("llm.provider")
	oldModel := viper.Get("stub.reasoning-search-model")
	oldEffort := viper.Get("stub.reasoning-search-effort")
	oldPlanningEnabled := viper.Get("llm.reasoning-search-planning-enabled")
	oldVerificationEnabled := viper.Get("llm.reasoning-search-verification-enabled")
	t.Cleanup(func() {
		viper.Set("llm.provider", oldProvider)
		viper.Set("stub.reasoning-search-model", oldModel)
		viper.Set("stub.reasoning-search-effort", oldEffort)
		viper.Set("llm.reasoning-search-planning-enabled", oldPlanningEnabled)
		viper.Set("llm.reasoning-search-verification-enabled", oldVerificationEnabled)
	})
	viper.Set("llm.provider", "stub")
	viper.Set("stub.reasoning-search-model", "non-rapid-model")
	viper.Set("stub.reasoning-search-effort", "low")
	viper.Set("llm.reasoning-search-planning-enabled", false)
	viper.Set("llm.reasoning-search-verification-enabled", false)

	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()
	service := llm.NewStubLLMService(nil)

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "rapid-gather-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "rapid-provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetRapid(sessionID, true); err != nil {
		t.Fatalf("unexpected set rapid error: %v", err)
	}
	if err := workflow.SetStage(sessionID, llm.ReasoningWorkflowStageRapidClassifier, llm.ReasoningWorkflowStageSession{
		Provider: "stub",
		Model:    "rapid-classifier-model",
	}); err != nil {
		t.Fatalf("unexpected classifier stage error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "initial query", "he"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if err := workflow.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected follow-up state error: %v", err)
	}
	summary := "rapid result"
	if err := workflow.SetResponseSnapshot(sessionID, &llm.ReasoningSearchResponse{
		Query:   "initial query",
		Summary: &summary,
		Results: []llm.ReasoningSearchResult{
			{MDBUID: "rapid-uid", Title: "rapid result"},
		},
	}); err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}

	requestSessionID := sessionID
	gotSessionID, err := prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
		Services: map[string]llm.Service{"stub": service},
	}, &ReasoningSearchRequest{
		Query:      "non-rapid follow-up",
		SessionID:  &requestSessionID,
		UILanguage: "he",
		IsRapid:    false,
	})
	if err != nil {
		t.Fatalf("unexpected mode switch error: %v", err)
	}
	if gotSessionID != sessionID {
		t.Fatalf("unexpected session id: %q", gotSessionID)
	}

	session, err := workflow.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected workflow get error: %v", err)
	}
	if session.Rapid {
		t.Fatalf("expected session to switch to non-rapid reasoning search mode")
	}
	if session.FollowupCount != 1 {
		t.Fatalf("unexpected follow-up count: %d", session.FollowupCount)
	}
	if len(session.ResponseSnapshotJSON) != 0 {
		t.Fatalf("expected previous snapshot to be cleared")
	}
	if session.DraftFollowupSeed == nil || session.DraftFollowupSeed.Summary == nil || *session.DraftFollowupSeed.Summary != "rapid result" {
		t.Fatalf("expected previous response as regular seed, got %#v", session.DraftFollowupSeed)
	}
	if _, ok := session.Stages[llm.ReasoningWorkflowStageRapidClassifier]; ok {
		t.Fatalf("expected stale rapid classifier stage to be removed")
	}
	reasoningStage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	if reasoningStage.ProviderSessionID == "" || reasoningStage.ProviderSessionID == "rapid-provider-session" {
		t.Fatalf("expected fresh non-rapid provider session, got %q", reasoningStage.ProviderSessionID)
	}
	if reasoningStage.Model != "non-rapid-model" {
		t.Fatalf("unexpected non-rapid reasoning search model: %q", reasoningStage.Model)
	}
}

func TestPrepareReasoningSearchSessionKeepsProviderSessionForNonRapidFollowup(t *testing.T) {
	workflow := llm.NewReasoningWorkflowSessionStore(time.Hour)
	defer workflow.Close()
	progress := llm.NewReasoningProgressStore(time.Hour)
	defer progress.Close()

	sessionID, err := workflow.Create(llm.ReasoningWorkflowStageReasoning, llm.ReasoningWorkflowStageSession{
		Provider:          "stub",
		Model:             "non-rapid-model",
		ReasoningEffort:   "low",
		MaxFollowups:      2,
		ProviderSessionID: "existing-provider-session",
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := workflow.SetQuery(sessionID, "initial query", "he"); err != nil {
		t.Fatalf("unexpected set query error: %v", err)
	}
	if err := workflow.SetFollowupState(sessionID, true, 0); err != nil {
		t.Fatalf("unexpected follow-up state error: %v", err)
	}
	summary := "previous result"
	if err := workflow.SetResponseSnapshot(sessionID, &llm.ReasoningSearchResponse{
		Query:   "initial query",
		Summary: &summary,
	}); err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}

	requestSessionID := sessionID
	gotSessionID, err := prepareReasoningSearchSession(nil, &llm.Runtime{
		Workflow: workflow,
		Progress: progress,
	}, &ReasoningSearchRequest{
		Query:      "non-rapid follow-up",
		SessionID:  &requestSessionID,
		UILanguage: "he",
		IsRapid:    false,
	})
	if err != nil {
		t.Fatalf("unexpected follow-up error: %v", err)
	}
	if gotSessionID != sessionID {
		t.Fatalf("unexpected session id: %q", gotSessionID)
	}

	session, err := workflow.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected workflow get error: %v", err)
	}
	if session.Rapid {
		t.Fatalf("expected session to stay in non-rapid reasoning search mode")
	}
	if session.FollowupCount != 1 {
		t.Fatalf("unexpected follow-up count: %d", session.FollowupCount)
	}
	if session.DraftFollowupSeed != nil || session.RapidFollowupSeed != nil {
		t.Fatalf("expected same-mode non-rapid follow-up not to seed from snapshot")
	}
	reasoningStage := session.Stages[llm.ReasoningWorkflowStageReasoning]
	if reasoningStage.ProviderSessionID != "existing-provider-session" {
		t.Fatalf("expected existing provider session to continue, got %q", reasoningStage.ProviderSessionID)
	}
}
