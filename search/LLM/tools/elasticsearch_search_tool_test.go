package tools

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/search"
	"gopkg.in/olivere/elastic.v6"
)

func TestSetElasticsearchSearchLanguageOrderUsesExplicitLanguage(t *testing.T) {
	query := search.Query{Term: "miracle"}

	setElasticsearchSearchLanguageOrder(&query, consts.LANG_HEBREW, true)

	expected := []string{consts.LANG_HEBREW, consts.LANG_ENGLISH}
	if !reflect.DeepEqual(query.LanguageOrder, expected) {
		t.Fatalf("unexpected language order: got %v, want %v", query.LanguageOrder, expected)
	}
}

func TestElasticsearchSearchHasPotentiallyGoodResultsWithFourDirectHitsAndTwoCategories(t *testing.T) {
	result := elasticsearchSearchQueryResultWithHits(
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_SOURCES},
		elasticsearchSearchTestUnitHit(consts.CT_VIDEO_PROGRAM_CHAPTER),
		elasticsearchSearchTestUnitHit(consts.CT_VIDEO_PROGRAM_CHAPTER),
		elasticsearchSearchTestUnitHit(consts.CT_ARTICLE),
	)

	if !elasticsearchSearchHasPotentiallyGoodResults(result) {
		t.Fatalf("expected four direct hits across two categories to be potentially good")
	}
}

func TestElasticsearchSearchHasPotentiallyGoodResultsRequiresFourDirectHits(t *testing.T) {
	result := elasticsearchSearchQueryResultWithHits(
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_SOURCES},
		elasticsearchSearchTestUnitHit(consts.CT_VIDEO_PROGRAM_CHAPTER),
		elasticsearchSearchTestUnitHit(consts.CT_LESSON_PART),
	)

	if elasticsearchSearchHasPotentiallyGoodResults(result) {
		t.Fatalf("expected fewer than four direct hits to be insufficient")
	}
}

func TestElasticsearchSearchHasPotentiallyGoodResultsRequiresTwoCategories(t *testing.T) {
	result := elasticsearchSearchQueryResultWithHits(
		elasticsearchSearchTestUnitHit(consts.CT_LESSON_PART),
		elasticsearchSearchTestUnitHit(consts.CT_LESSON_PART),
		elasticsearchSearchTestUnitHit(consts.CT_LESSON_PART),
		elasticsearchSearchTestUnitHit(consts.CT_LESSON_PART),
	)

	if elasticsearchSearchHasPotentiallyGoodResults(result) {
		t.Fatalf("expected one useful category to be insufficient")
	}
}

func TestElasticsearchSearchHasPotentiallyGoodResultsIgnoresGroupingHits(t *testing.T) {
	result := elasticsearchSearchQueryResultWithHits(
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_SOURCES},
		elasticsearchSearchTestUnitHit(consts.CT_VIDEO_PROGRAM_CHAPTER),
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_COLLECTIONS},
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_COLLECTIONS},
	)

	if elasticsearchSearchHasPotentiallyGoodResults(result) {
		t.Fatalf("expected grouping hits not to count as direct results")
	}
}

func TestElasticsearchSearchHasPotentiallyGoodResultsOnlyChecksTopSix(t *testing.T) {
	result := elasticsearchSearchQueryResultWithHits(
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_SOURCES},
		elasticsearchSearchTestUnitHit(consts.CT_VIDEO_PROGRAM_CHAPTER),
		elasticsearchSearchTestUnitHit(consts.CT_VIDEO_PROGRAM_CHAPTER),
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_COLLECTIONS},
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_COLLECTIONS},
		elasticsearchSearchTestHit{ResultType: consts.ES_RESULT_TYPE_COLLECTIONS},
		elasticsearchSearchTestUnitHit(consts.CT_ARTICLE),
		elasticsearchSearchTestUnitHit(consts.CT_ARTICLE),
	)

	if elasticsearchSearchHasPotentiallyGoodResults(result) {
		t.Fatalf("expected direct hits after top six not to count")
	}
}

type elasticsearchSearchTestHit struct {
	ResultType  string
	ContentType string
}

func elasticsearchSearchTestUnitHit(contentType string) elasticsearchSearchTestHit {
	return elasticsearchSearchTestHit{
		ResultType:  consts.ES_RESULT_TYPE_UNITS,
		ContentType: contentType,
	}
}

func elasticsearchSearchQueryResultWithHits(testHits ...elasticsearchSearchTestHit) *search.QueryResult {
	hits := make([]*elastic.SearchHit, 0, len(testHits))
	for _, testHit := range testHits {
		sourceMap := map[string]interface{}{"result_type": testHit.ResultType}
		if testHit.ContentType != "" {
			sourceMap["filter_values"] = []string{es.KeyValue("content_type", testHit.ContentType)}
		}
		source, _ := json.Marshal(sourceMap)
		raw := json.RawMessage(source)
		hits = append(hits, &elastic.SearchHit{Type: "result", Source: &raw})
	}
	return &search.QueryResult{
		SearchResult: &elastic.SearchResult{
			Hits: &elastic.SearchHits{
				TotalHits: int64(len(hits)),
				Hits:      hits,
			},
		},
	}
}
