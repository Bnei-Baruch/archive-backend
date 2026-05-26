package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
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

func TestValidateReasoningSearchResponseQueryRejectsTopicDrift(t *testing.T) {
	response := &llm.ReasoningSearchResponse{Query: "מחשבת הבריאה"}
	err := validateReasoningSearchResponseQuery("נס", response)
	if !errors.Is(err, errReasoningSearchQueryMismatch) {
		t.Fatalf("expected query mismatch error, got %v", err)
	}
}

func TestBuildReasoningSearchHighlightsRanksAIAndRelevantESHighlights(t *testing.T) {
	result := llm.ReasoningSearchResult{
		Highlights: []string{
			"<em>משה</em>",
			"רק מילת קישור <em>עם</em>",
			"פתיח קצר על <em>משה</em>",
			"קטע ארוך יותר שמסביר את <em>משה</em> ואת <em>תפקידו</em> בהקשר השאלה",
		},
		LookupEvidence: []llm.ReasoningSearchResultEvidence{
			{SupportingSnippet: "עדות מכלי AI על התאמת המקור לשאלה"},
		},
	}

	highlights := buildReasoningSearchHighlightsFromEvidence(result, "משה", "תפקידו של משה")
	if len(highlights) < 3 {
		t.Fatalf("expected ranked highlights, got %#v", highlights)
	}
	if highlights[0] != "עדות מכלי AI על התאמת המקור לשאלה" {
		t.Fatalf("expected AI evidence first, got %#v", highlights)
	}
	if highlights[1] != "קטע ארוך יותר שמסביר את <em>משה</em> ואת <em>תפקידו</em> בהקשר השאלה" {
		t.Fatalf("expected richer ES highlight second, got %#v", highlights)
	}
	if highlights[2] == "רק מילת קישור <em>עם</em>" {
		t.Fatalf("expected short linking-word emphasis not to rank near the top, got %#v", highlights)
	}
}

func TestBuildReasoningSearchHighlightsPrefersReasonAnchors(t *testing.T) {
	result := llm.ReasoningSearchResult{
		Highlights: []string{
			`נמצא מה שהאדם מתייגע בתורה ומצות, הוא מסיבת שחסר לנו את חשיבותו וגדלותו של הבורא יתברך.`,
			`וזה ענין שותפות, שיש להנבראים עם הבורא.`,
			`עמי אתה - להיות שותף עמי.`,
		},
	}

	highlights := buildReasoningSearchHighlightsFromEvidence(result, "שותפות עם הבורא", `נראה כהתאמה הטובה ביותר: "עמי אתה" - "להיות שותף עמי" וכן "וזה ענין שותפות"`)
	if len(highlights) < 2 {
		t.Fatalf("expected ranked highlights, got %#v", highlights)
	}
	if highlights[0] != `עמי אתה - להיות שותף עמי.` && highlights[0] != `וזה ענין שותפות, שיש להנבראים עם הבורא.` {
		t.Fatalf("expected reason-supporting highlight first, got %#v", highlights)
	}
	if highlights[len(highlights)-1] == `עמי אתה - להיות שותף עמי.` {
		t.Fatalf("expected quoted reason anchor not to sink to the bottom, got %#v", highlights)
	}
}

func TestBuildRapidVisibleResultsOmitsNotRelevantAndSortsByRelevance(t *testing.T) {
	session := &llm.ReasoningWorkflowSession{
		PartialResults: []llm.ReasoningSearchResult{
			{MDBUID: "maybe", ContentType: "SOURCE", Highlights: []string{"maybe"}},
			{MDBUID: "no", Highlights: []string{"no"}},
			{MDBUID: "strong", Highlights: []string{"strong"}},
			{MDBUID: "program", ContentType: "VIDEO_PROGRAM_CHAPTER", Highlights: []string{"program"}},
		},
		RapidClassifications: map[string]llm.ReasoningSearchRapidClassification{
			"maybe":   {MDBUID: "maybe", Relevance: "can_be_relevant", Reason: "maybe reason"},
			"no":      {MDBUID: "no", Relevance: "not_relevant", Reason: ""},
			"strong":  {MDBUID: "strong", Relevance: "highly_relevant", Reason: "strong reason"},
			"program": {MDBUID: "program", Relevance: "can_be_relevant", Reason: "program reason"},
		},
	}

	results, err := buildRapidVisibleResults(nil, "", session, true)
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

	results, err = buildRapidVisibleResults(nil, "", session, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, result := range results {
		if result.Relevance != "" {
			t.Fatalf("expected relevance to be hidden outside debug mode, got %#v", results)
		}
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
	}, true)
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
	classifications := map[string]llm.ReasoningSearchRapidClassification{
		"source":  {MDBUID: "source", Relevance: "highly_relevant"},
		"clip":    {MDBUID: "clip", Relevance: "highly_relevant"},
		"baal":    {MDBUID: "baal", Relevance: "highly_relevant"},
		"likutim": {MDBUID: "likutim", Relevance: "highly_relevant"},
		"connect": {MDBUID: "connect", Relevance: "highly_relevant"},
		"native":  {MDBUID: "native", Relevance: "highly_relevant"},
		"world":   {MDBUID: "world", Relevance: "highly_relevant"},
		"program": {MDBUID: "program", Relevance: "highly_relevant"},
	}
	results := []llm.ReasoningSearchResult{
		{MDBUID: "source", ContentType: consts.CT_SOURCE},
		{MDBUID: "clip", ContentType: consts.CT_CLIP},
		{MDBUID: "baal", ContentType: consts.CT_SOURCE, BaalSulamArticle: true},
		{MDBUID: "likutim", ContentType: consts.CT_LIKUTIM},
		{MDBUID: "connect", ContentType: consts.CT_SOURCE, ConnectingSource: true},
		{MDBUID: "native", ContentType: consts.CT_SOURCE, OriginalLanguage: consts.LANG_HEBREW},
		{MDBUID: "world", ContentType: consts.CT_VIDEO_PROGRAM_CHAPTER, CollectionUID: consts.PROGRAM_COLLECTION_EL_MUNDO},
		{MDBUID: "program", ContentType: consts.CT_VIDEO_PROGRAM_CHAPTER},
	}

	sortRapidVisibleResults(results, classifications, consts.LANG_HEBREW)

	if results[0].MDBUID != "clip" {
		t.Fatalf("expected clip to keep the top boost and win on stable ordering, got %#v", results)
	}
	if results[len(results)-1].MDBUID != "source" {
		t.Fatalf("expected unboosted source last, got %#v", results)
	}
	if rapidResultSortScore(llm.ReasoningSearchResult{ContentType: consts.CT_LIKUTIM}, "") != rapidResultSortScore(llm.ReasoningSearchResult{ConnectingSource: true}, "") {
		t.Fatalf("expected LIKUTIM and Connecting to Source to get the same boost")
	}
	for i, result := range results {
		if result.MDBUID == "world" && i < 3 {
			t.Fatalf("expected El Mundo program to be de-prioritized outside Spanish UI, got %#v", results)
		}
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
