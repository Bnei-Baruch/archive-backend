package api

import (
	"errors"
	"testing"

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
