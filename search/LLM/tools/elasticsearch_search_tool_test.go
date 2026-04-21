package tools

import (
	"reflect"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/search"
)

func TestSetElasticsearchSearchLanguageOrderUsesExplicitLanguage(t *testing.T) {
	query := search.Query{Term: "miracle"}

	setElasticsearchSearchLanguageOrder(&query, consts.LANG_HEBREW, true)

	expected := []string{consts.LANG_HEBREW, consts.LANG_ENGLISH}
	if !reflect.DeepEqual(query.LanguageOrder, expected) {
		t.Fatalf("unexpected language order: got %v, want %v", query.LanguageOrder, expected)
	}
}
