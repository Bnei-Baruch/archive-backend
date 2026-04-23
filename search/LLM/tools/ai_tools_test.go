package tools

import "testing"

func TestSelectAIQueryBatchesPrefersKeywordMatches(t *testing.T) {
	batches := [][]aiQueryChunk{
		{{Number: 1, Content: "unrelated opening text"}},
		{{Number: 2, Content: "another unrelated section"}},
		{{Number: 3, Content: "כאן מוסבר על ד בחינות דאור ישר"}},
		{{Number: 4, Content: "עוד הסבר על אור ישר ובחינות"}},
	}

	selected := selectAIQueryBatches("ד בחינות דאור ישר", batches, 2)

	if len(selected) != 2 {
		t.Fatalf("expected 2 selected batches, got %d", len(selected))
	}
	if selected[0][0].Number != 3 || selected[1][0].Number != 4 {
		t.Fatalf("expected matched batches 3 and 4, got %d and %d", selected[0][0].Number, selected[1][0].Number)
	}
}

func TestSelectAIQueryBatchesFallsBackToStartWhenNoKeywordsMatch(t *testing.T) {
	batches := [][]aiQueryChunk{
		{{Number: 1, Content: "first"}},
		{{Number: 2, Content: "second"}},
		{{Number: 3, Content: "third"}},
	}

	selected := selectAIQueryBatches("missing", batches, 2)

	if len(selected) != 2 {
		t.Fatalf("expected 2 selected batches, got %d", len(selected))
	}
	if selected[0][0].Number != 1 || selected[1][0].Number != 2 {
		t.Fatalf("expected first two batches, got %d and %d", selected[0][0].Number, selected[1][0].Number)
	}
}

func TestSelectAIQueryBatchesDoesNotFillWithUnmatchedBatches(t *testing.T) {
	batches := [][]aiQueryChunk{
		{{Number: 1, Content: "first"}},
		{{Number: 2, Content: "second"}},
		{{Number: 3, Content: "דאור ישר"}},
	}

	selected := selectAIQueryBatches("דאור ישר", batches, 2)

	if len(selected) != 1 {
		t.Fatalf("expected only matched batches, got %d", len(selected))
	}
	if selected[0][0].Number != 3 {
		t.Fatalf("expected matched batch 3, got %d", selected[0][0].Number)
	}
}

func TestAIQueryKeywordsFiltersInstructionStopWords(t *testing.T) {
	keywords := aiQueryKeywords("מצא קטעים שמגדירים או מסבירים את ד' בחינות דאור ישר")
	expected := []string{"בחינות", "דאור", "ישר"}

	if len(keywords) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, keywords)
	}
	for i := range expected {
		if keywords[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, keywords)
		}
	}
}

func TestAIQueryKeywordsFiltersInstructionStopWordsEnglish(t *testing.T) {
	keywords := aiQueryKeywords("find chunks that define or explain four phases of direct light")
	expected := []string{"four", "phases", "direct", "light"}

	if len(keywords) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, keywords)
	}
	for i := range expected {
		if keywords[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, keywords)
		}
	}
}

func TestAIQueryKeywordsFiltersInstructionStopWordsRussian(t *testing.T) {
	keywords := aiQueryKeywords("найди фрагменты которые объясняют четыре стадии прямого света")
	expected := []string{"четыре", "стадии", "прямого", "света"}

	if len(keywords) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, keywords)
	}
	for i := range expected {
		if keywords[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, keywords)
		}
	}
}

func TestAIQueryKeywordsFiltersInstructionStopWordsSpanish(t *testing.T) {
	keywords := aiQueryKeywords("encuentra fragmentos que explican cuatro fases de luz directa")
	expected := []string{"cuatro", "fases", "luz", "directa"}

	if len(keywords) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, keywords)
	}
	for i := range expected {
		if keywords[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, keywords)
		}
	}
}
