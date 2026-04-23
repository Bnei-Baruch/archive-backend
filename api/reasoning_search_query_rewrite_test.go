package api

import "testing"

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
