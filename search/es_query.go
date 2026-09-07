package search

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// esCreateSpanNearQueryBody builds the ES9 span_near JSON map.
// Mirrors createSpanNearQuery — same fuzziness/transpositions rules.
func esCreateSpanNearQueryBody(field, term string, boost float32, slop int, inOrder bool) (map[string]interface{}, error) {
	clauses := []interface{}{}
	for _, t := range strings.Fields(term) {
		if t == "<" || t == ">" || t == "-" {
			continue
		}
		fuzziness := "AUTO"
		transpositions := true
		runes := []rune(t)
		_, intErr := strconv.Atoi(t)
		if intErr == nil || (len(runes) == 3 && runes[1] == '"') || (len(runes) == 4 && runes[2] == '"') {
			fuzziness = "0"
		} else if len(runes) == 1 && runes[0] >= 'א' && runes[0] <= 'ת' {
			fuzziness = "1"
			transpositions = false
		}
		clauses = append(clauses, map[string]interface{}{
			"span_multi": map[string]interface{}{
				"match": map[string]interface{}{
					"fuzzy": map[string]interface{}{
						field: map[string]interface{}{
							"value":          t,
							"fuzziness":      fuzziness,
							"transpositions": transpositions,
						},
					},
				},
			},
		})
	}
	return map[string]interface{}{
		"span_near": map[string]interface{}{
			"clauses":  clauses,
			"slop":     slop,
			"boost":    boost,
			"in_order": inOrder,
		},
	}, nil
}

// esAddMustNotSeries returns a must_not clause filtering out lesson-series collections,
// unless the query explicitly requests CT_LESSONS_SERIES. Mirrors addMustNotSeries.
func esAddMustNotSeries(q Query) map[string]interface{} {
	if filters, ok := q.Filters[consts.FILTER_CONTENT_TYPE]; ok {
		for _, f := range filters {
			if f == consts.CT_LESSONS_SERIES {
				return nil
			}
		}
	}
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"filter": []interface{}{
				map[string]interface{}{
					"terms": map[string]interface{}{consts.ES_RESULT_TYPE: []string{consts.ES_RESULT_TYPE_COLLECTIONS}},
				},
				map[string]interface{}{
					"terms": map[string]interface{}{
						"filter_values": []string{
							fmt.Sprintf("%s:%s", consts.FILTER_COLLECTIONS_CONTENT_TYPE, consts.CT_LESSONS_SERIES),
						},
					},
				},
			},
		},
	}
}

// mpq builds a match_phrase JSON clause with optional slop and boost.
func mpq(field, term string, slop int, boost float32) map[string]interface{} {
	inner := map[string]interface{}{"query": term, "boost": boost}
	if slop > 0 {
		inner["slop"] = slop
	}
	return map[string]interface{}{
		"match_phrase": map[string]interface{}{field: inner},
	}
}

// esCreateResultsQueryBody builds the ES9 JSON "query" object for a results search.
// This is the ES9 counterpart of createResultsQuery in query.go.
func esCreateResultsQueryBody(resultTypes []string, q Query, docIds []string, filterOutCUSources []string, titlesOnly bool) (map[string]interface{}, error) {
	// Result-type filter (zero-boost constant_score so it doesn't affect scoring)
	rtValues := make([]interface{}, len(resultTypes))
	for i, rt := range resultTypes {
		rtValues[i] = rt
	}
	mustClauses := []interface{}{
		map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": map[string]interface{}{
					"terms": map[string]interface{}{"result_type": rtValues},
				},
				"boost": 0.0,
			},
		},
	}

	filterClauses := []interface{}{}
	mustNotClauses := []interface{}{}
	shouldClauses := []interface{}{}

	if len(docIds) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"ids": map[string]interface{}{"values": docIds},
		})
	}

	for _, src := range filterOutCUSources {
		mustNotClauses = append(mustNotClauses, map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{
					map[string]interface{}{
						"terms": map[string]interface{}{
							"typed_uids": []string{fmt.Sprintf("%s:%s", consts.FILTER_SOURCE, src)},
						},
					},
					map[string]interface{}{
						"terms": map[string]interface{}{
							consts.ES_RESULT_TYPE: []string{consts.ES_RESULT_TYPE_UNITS},
						},
					},
				},
			},
		})
	}

	if mustNot := esAddMustNotSeries(q); mustNot != nil {
		mustNotClauses = append(mustNotClauses, mustNot)
	}

	appendDescription := !titlesOnly || (len(resultTypes) == 1 && resultTypes[0] == consts.ES_RESULT_TYPE_SOURCES)

	if q.Term != "" {
		// Constant-score presence filter (must match at least one field)
		presenceShould := []interface{}{
			map[string]interface{}{"match": map[string]interface{}{"title.language": q.Term}},
			map[string]interface{}{"match": map[string]interface{}{"full_title.language": q.Term}},
		}
		if appendDescription {
			presenceShould = append(presenceShould,
				map[string]interface{}{"match": map[string]interface{}{"description.language": q.Term}},
			)
		}
		if !titlesOnly {
			presenceShould = append(presenceShould,
				map[string]interface{}{"match": map[string]interface{}{"content.language": q.Term}},
			)
		}
		mustClauses = append(mustClauses, map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": map[string]interface{}{
					"bool": map[string]interface{}{
						"should":               presenceShould,
						"minimum_should_match": 1,
					},
				},
				"boost": 0.0,
			},
		})

		// Scoring dis_max: phrase queries + span_near queries
		disMax := []interface{}{
			mpq("title.language", q.Term, SLOP, TITLE_BOOST),
			mpq("full_title.language", q.Term, SLOP, FULL_TITLE_BOOST),
			mpq("title.language", q.Term, 0, EXACT_BOOST*TITLE_BOOST),
			mpq("full_title.language", q.Term, 0, EXACT_BOOST*FULL_TITLE_BOOST),
			mpq("title", q.Term, SLOP, STANDARD_BOOST*TITLE_BOOST),
			mpq("full_title", q.Term, SLOP, STANDARD_BOOST*FULL_TITLE_BOOST),
			mpq("title", q.Term, 0, STANDARD_BOOST*EXACT_BOOST*TITLE_BOOST),
			mpq("full_title", q.Term, 0, STANDARD_BOOST*EXACT_BOOST*FULL_TITLE_BOOST),
		}
		type snqSpec struct {
			field   string
			boost   float32
			slop    int
			inOrder bool
		}
		titleSnqs := []snqSpec{
			{"title.language", TITLE_BOOST * SPAN_NEAR_BOOST, SLOP, true},
			{"full_title.language", FULL_TITLE_BOOST * SPAN_NEAR_BOOST, SLOP, false},
			{"title.language", EXACT_BOOST * TITLE_BOOST * SPAN_NEAR_BOOST, 0, true},
			{"full_title.language", EXACT_BOOST * FULL_TITLE_BOOST * SPAN_NEAR_BOOST, 0, true},
			{"title", STANDARD_BOOST * TITLE_BOOST * SPAN_NEAR_BOOST, SLOP, true},
			{"full_title", STANDARD_BOOST * FULL_TITLE_BOOST * SPAN_NEAR_BOOST, SLOP, false},
			{"title", STANDARD_BOOST * EXACT_BOOST * TITLE_BOOST * SPAN_NEAR_BOOST, 0, true},
			{"full_title", STANDARD_BOOST * EXACT_BOOST * FULL_TITLE_BOOST * SPAN_NEAR_BOOST, 0, true},
		}
		for _, s := range titleSnqs {
			snq, err := esCreateSpanNearQueryBody(s.field, q.Term, s.boost, s.slop, s.inOrder)
			if err != nil {
				return nil, err
			}
			disMax = append(disMax, snq)
		}

		if appendDescription {
			disMax = append(disMax,
				mpq("description.language", q.Term, SLOP, DESCRIPTION_BOOST),
				mpq("description.language", q.Term, 0, EXACT_BOOST*DESCRIPTION_BOOST),
				mpq("description", q.Term, SLOP, STANDARD_BOOST*DESCRIPTION_BOOST),
				mpq("description", q.Term, 0, STANDARD_BOOST*EXACT_BOOST*DESCRIPTION_BOOST),
			)
			descSnqs := []snqSpec{
				{"description.language", DESCRIPTION_BOOST * SPAN_NEAR_BOOST, SLOP, true},
				{"description.language", EXACT_BOOST * DESCRIPTION_BOOST * SPAN_NEAR_BOOST, 0, true},
				{"description", STANDARD_BOOST * DESCRIPTION_BOOST * SPAN_NEAR_BOOST, SLOP, true},
				{"description", STANDARD_BOOST * EXACT_BOOST * DESCRIPTION_BOOST * SPAN_NEAR_BOOST, 0, true},
			}
			for _, s := range descSnqs {
				snq, err := esCreateSpanNearQueryBody(s.field, q.Term, s.boost, s.slop, s.inOrder)
				if err != nil {
					return nil, err
				}
				disMax = append(disMax, snq)
			}
		}

		if !titlesOnly {
			disMax = append(disMax,
				mpq("content.language", q.Term, SLOP, DEFAULT_BOOST),
				mpq("content.language", q.Term, 0, EXACT_BOOST),
				mpq("content", q.Term, SLOP, STANDARD_BOOST),
				mpq("content", q.Term, 0, STANDARD_BOOST*EXACT_BOOST),
			)
			contentSnqs := []snqSpec{
				{"content.language", DEFAULT_BOOST * SPAN_NEAR_BOOST, SLOP, true},
				{"content.language", EXACT_BOOST * SPAN_NEAR_BOOST, 0, true},
				{"content", STANDARD_BOOST * SPAN_NEAR_BOOST, SLOP, true},
				{"content", STANDARD_BOOST * EXACT_BOOST * SPAN_NEAR_BOOST, 0, true},
			}
			for _, s := range contentSnqs {
				snq, err := esCreateSpanNearQueryBody(s.field, q.Term, s.boost, s.slop, s.inOrder)
				if err != nil {
					return nil, err
				}
				disMax = append(disMax, snq)
			}
		}

		shouldClauses = append(shouldClauses, map[string]interface{}{
			"dis_max": map[string]interface{}{"queries": disMax},
		})
	}

	// Exact terms
	for _, exactTerm := range q.ExactTerms {
		presenceShould := []interface{}{
			map[string]interface{}{"match_phrase": map[string]interface{}{"title": exactTerm}},
			map[string]interface{}{"match_phrase": map[string]interface{}{"full_title": exactTerm}},
		}
		if appendDescription {
			presenceShould = append(presenceShould,
				map[string]interface{}{"match_phrase": map[string]interface{}{"description": exactTerm}},
			)
		}
		if !titlesOnly {
			presenceShould = append(presenceShould,
				map[string]interface{}{"match_phrase": map[string]interface{}{"content": exactTerm}},
			)
		}
		mustClauses = append(mustClauses, map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": map[string]interface{}{
					"bool": map[string]interface{}{
						"should":               presenceShould,
						"minimum_should_match": 1,
					},
				},
				"boost": 0.0,
			},
		})

		disMaxExact := []interface{}{
			mpq("title.language", exactTerm, 0, EXACT_BOOST*TITLE_BOOST),
			mpq("full_title.language", exactTerm, 0, EXACT_BOOST*FULL_TITLE_BOOST),
			mpq("title", exactTerm, 0, STANDARD_BOOST*EXACT_BOOST*TITLE_BOOST),
			mpq("full_title", exactTerm, 0, STANDARD_BOOST*EXACT_BOOST*FULL_TITLE_BOOST),
		}
		if appendDescription {
			disMaxExact = append(disMaxExact,
				mpq("description.language", exactTerm, 0, EXACT_BOOST*DESCRIPTION_BOOST),
				mpq("description", exactTerm, 0, STANDARD_BOOST*EXACT_BOOST*DESCRIPTION_BOOST),
			)
		}
		if !titlesOnly {
			disMaxExact = append(disMaxExact,
				mpq("content.language", exactTerm, 0, EXACT_BOOST),
				mpq("content", exactTerm, 0, STANDARD_BOOST*EXACT_BOOST),
			)
		}
		shouldClauses = append(shouldClauses, map[string]interface{}{
			"dis_max": map[string]interface{}{"queries": disMaxExact},
		})
	}

	// Filters
	for filter, values := range q.Filters {
		switch filter {
		case consts.FILTER_START_DATE:
			filterClauses = append(filterClauses, map[string]interface{}{
				"range": map[string]interface{}{
					"effective_date": map[string]interface{}{
						"gte":    values[0],
						"format": "yyyy-MM-dd",
					},
				},
			})
		case consts.FILTER_END_DATE:
			filterClauses = append(filterClauses, map[string]interface{}{
				"range": map[string]interface{}{
					"effective_date": map[string]interface{}{
						"lte":    values[0],
						"format": "yyyy-MM-dd",
					},
				},
			})
		case consts.FILTER_CONTENT_TYPE:
			shouldForCT := []interface{}{}
			collectionCTs := utils.FilterStringSlice(values, func(ct string) bool {
				return utils.StringInSlice(ct, consts.COLLECTIONS_CONTENT_TYPES)
			})
			if len(collectionCTs) > 0 {
				shouldForCT = append(shouldForCT, map[string]interface{}{
					"terms": map[string]interface{}{"filter_values": es.KeyIValues(consts.FILTER_COLLECTIONS_CONTENT_TYPE, collectionCTs)},
				})
			}
			unitsCTs := utils.FilterStringSlice(values, func(ct string) bool {
				return ct != consts.CT_SOURCE && !utils.StringInSlice(ct, consts.COLLECTIONS_CONTENT_TYPES)
			})
			if len(unitsCTs) > 0 {
				shouldForCT = append(shouldForCT, map[string]interface{}{
					"terms": map[string]interface{}{"filter_values": es.KeyIValues(consts.FILTER_CONTENT_TYPE, unitsCTs)},
				})
			}
			if utils.StringInSlice(consts.CT_SOURCE, values) {
				shouldForCT = append(shouldForCT, map[string]interface{}{
					"terms": map[string]interface{}{"result_type": []string{consts.ES_RESULT_TYPE_SOURCES}},
				})
			}
			filterClauses = append(filterClauses, map[string]interface{}{
				"bool": map[string]interface{}{
					"should":               shouldForCT,
					"minimum_should_match": 1,
				},
			})
		case consts.FILTER_COLLECTION:
			filterClauses = append(filterClauses, map[string]interface{}{
				"terms": map[string]interface{}{
					"typed_uids": []string{fmt.Sprintf("%s:%s", consts.ES_UID_TYPE_COLLECTION, values[0])},
				},
			})
		default:
			filterClauses = append(filterClauses, map[string]interface{}{
				"terms": map[string]interface{}{"filter_values": es.KeyIValues(filter, values)},
			})
		}
	}

	// Assemble inner bool query
	boolBody := map[string]interface{}{
		"must": mustClauses,
	}
	if len(filterClauses) > 0 {
		boolBody["filter"] = filterClauses
	}
	if len(mustNotClauses) > 0 {
		boolBody["must_not"] = mustNotClauses
	}
	if len(shouldClauses) > 0 {
		boolBody["should"] = shouldClauses
	}

	var innerQuery map[string]interface{}
	innerQuery = map[string]interface{}{"bool": boolBody}

	if q.Term == "" && len(q.ExactTerms) == 0 {
		// No text scoring possible — wrap in constant_score to give a uniform base score.
		innerQuery = map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": innerQuery,
				"boost":  1.0,
			},
		}
	}

	// function_score: per-result-type weight boosts + clip penalty
	scoreFunctions := []interface{}{}
	for _, resultType := range resultTypes {
		weight := 1.0
		switch resultType {
		case consts.ES_RESULT_TYPE_UNITS:
			weight = 1.1
		case consts.ES_RESULT_TYPE_TAGS:
			weight = 2.3
		case consts.ES_RESULT_TYPE_SOURCES:
			weight = 1.8
		case consts.ES_RESULT_TYPE_COLLECTIONS:
			weight = 2.0
		}
		scoreFunctions = append(scoreFunctions, map[string]interface{}{
			"filter": map[string]interface{}{
				"terms": map[string]interface{}{"result_type": []string{resultType}},
			},
			"weight": weight,
		})
	}
	scoreFunctions = append(scoreFunctions, map[string]interface{}{
		"filter": map[string]interface{}{
			"terms": map[string]interface{}{
				"filter_values": []string{es.KeyValue("content_type", consts.CT_CLIP)},
			},
		},
		"weight": 0.7,
	})

	innerFunctionScore := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query":      innerQuery,
			"functions":  scoreFunctions,
			"score_mode": "multiply",
			"min_score":  MIN_SCORE_FOR_RESULTS,
		},
	}

	// Outer function_score: static weight boost + gauss date decay
	outerFunctions := []interface{}{
		map[string]interface{}{"weight": 2.0},
		map[string]interface{}{
			"gauss": map[string]interface{}{
				"effective_date": map[string]interface{}{
					"scale": "2000d",
					"decay": 0.6,
				},
			},
		},
	}

	return map[string]interface{}{
		"function_score": map[string]interface{}{
			"query":      innerFunctionScore,
			"functions":  outerFunctions,
			"score_mode": "sum",
			"max_boost":  100.0,
		},
	}, nil
}

// esSourceFields returns the _source include list for the given result types.
// Mirrors the FetchSourceContext logic in NewResultsSearchRequest.
func esSourceFields(resultTypes []string) []string {
	fields := []string{"mdb_uid", "result_type", "effective_date", "typed_uids"}
	titleAdded, fullTitleAdded, contentAdded := false, false, false
	for _, rt := range resultTypes {
		if rt == consts.ES_RESULT_TYPE_TWEETS && !contentAdded {
			fields = append(fields, "content")
			contentAdded = true
		} else if rt == consts.ES_RESULT_TYPE_SOURCES && !fullTitleAdded {
			fields = append(fields, "full_title")
			fullTitleAdded = true
		}
		if !titleAdded && rt != consts.ES_RESULT_TYPE_TWEETS {
			fields = append(fields, "title")
			titleAdded = true
		}
		if contentAdded && titleAdded && fullTitleAdded {
			break
		}
	}
	return fields
}

// esCreateHighlightBody builds the ES9 "highlight" JSON object.
// Mirrors createHighlightQuery.
func esCreateHighlightBody(terms []string, numFragments int, partialHighlight bool) map[string]interface{} {
	fieldsMap := map[string]interface{}{}
	for _, term := range terms {
		hq := map[string]interface{}{"simple_query_string": map[string]interface{}{"query": term}}
		fieldsMap["title"] = map[string]interface{}{"number_of_fragments": 0, "highlight_query": hq}
		fieldsMap["full_title"] = map[string]interface{}{"number_of_fragments": 0, "highlight_query": hq}
		fieldsMap["description"] = map[string]interface{}{"highlight_query": hq}
		fieldsMap["description.language"] = map[string]interface{}{"highlight_query": hq}
		fieldsMap["content"] = map[string]interface{}{"number_of_fragments": numFragments, "highlight_query": hq}
		fieldsMap["content.language"] = map[string]interface{}{"number_of_fragments": numFragments, "highlight_query": hq}
		if !partialHighlight {
			fieldsMap["title.language"] = map[string]interface{}{"number_of_fragments": 0, "highlight_query": hq}
		}
	}
	// ES >= 7 rejects highlighting fields longer than index.highlight.max_analyzed_offset
	// (default 1,000,000) with a 400. Some content fields exceed that (e.g. a 1.8M-char
	// Hebrew transcript). Setting max_analyzed_offset just under the index limit makes ES
	// truncate long fields instead of failing the whole search (ES6/6.8 had no such guard).
	return map[string]interface{}{
		"fields":              fieldsMap,
		"max_analyzed_offset": 999999,
	}
}

// NewESResultsSearchBody builds the ES9 request body + index + preference for a results search.
// Returns (body, index, preference, error). Mirrors NewResultsSearchRequest.
func NewESResultsSearchBody(options SearchRequestOptions) (map[string]interface{}, string, string, error) {
	queryBody, err := esCreateResultsQueryBody(
		options.resultTypes, options.query, options.docIds,
		options.filterOutCUSources, options.titlesOnly,
	)
	if err != nil {
		return nil, "", "", fmt.Errorf("NewESResultsSearchBody: %w", err)
	}

	body := map[string]interface{}{
		"query":   queryBody,
		"from":    options.from,
		"size":    options.size,
		"_source": map[string]interface{}{"includes": esSourceFields(options.resultTypes)},
	}

	if options.query.Deb {
		body["explain"] = true
	}

	if options.Timeout != nil {
		body["timeout"] = *options.Timeout
	}

	if options.useHighlight {
		var terms []string
		if options.query.Term != "" {
			terms = []string{options.query.Term}
		} else {
			terms = options.query.ExactTerms
		}
		contentFragments := 5
		if options.highlightFullContent {
			contentFragments = 0
		}
		body["highlight"] = esCreateHighlightBody(terms, contentFragments, options.partialHighlight)
	}

	switch options.sortBy {
	case consts.SORT_BY_OLDER_TO_NEWER:
		body["sort"] = []interface{}{map[string]interface{}{"effective_date": map[string]interface{}{"order": "asc"}}}
	case consts.SORT_BY_NEWER_TO_OLDER:
		body["sort"] = []interface{}{map[string]interface{}{"effective_date": map[string]interface{}{"order": "desc"}}}
	}

	return body, options.index, options.preference, nil
}

// NewESResultsSearchBodies builds one body per language index.
// Mirrors NewResultsSearchRequests.
func NewESResultsSearchBodies(options SearchRequestOptions) ([]map[string]interface{}, []string, string, error) {
	var bodies []map[string]interface{}
	var indices []string
	for _, lang := range options.query.LanguageOrder {
		options.index = es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, lang)
		body, index, preference, err := NewESResultsSearchBody(options)
		if err != nil {
			return nil, nil, "", err
		}
		bodies = append(bodies, body)
		indices = append(indices, index)
		_ = preference // same for all; caller uses options.preference directly
	}
	return bodies, indices, options.preference, nil
}

// NewESResultsSuggestBody builds the ES9 suggest request body.
// Mirrors NewResultsSuggestRequest.
func NewESResultsSuggestBody(resultTypes []string, query Query) map[string]interface{} {
	contextQuery := make([]interface{}, len(resultTypes))
	for i, rt := range resultTypes {
		contextQuery[i] = rt
	}
	makeSuggester := func(field string) map[string]interface{} {
		return map[string]interface{}{
			"text": query.Term,
			"completion": map[string]interface{}{
				"field":           field,
				"size":            NUM_SUGGESTS,
				"skip_duplicates": true,
				"contexts": map[string]interface{}{
					"result_type": contextQuery,
				},
			},
		}
	}
	return map[string]interface{}{
		"_source": map[string]interface{}{
			"includes": []string{"mdb_uid", "result_type", "title", "full_title"},
		},
		"suggest": map[string]interface{}{
			"title_suggest":          makeSuggester("title_suggest"),
			"title_suggest.language": makeSuggester("title_suggest.language"),
		},
	}
}

// NewESResultsSuggestBodies builds one suggest body per language index.
// Mirrors NewResultsSuggestRequests.
func NewESResultsSuggestBodies(resultTypes []string, query Query, preference string) ([]map[string]interface{}, []string, string) {
	var bodies []map[string]interface{}
	var indices []string
	for _, lang := range query.LanguageOrder {
		index := es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, lang)
		bodies = append(bodies, NewESResultsSuggestBody(resultTypes, query))
		indices = append(indices, index)
	}
	return bodies, indices, preference
}
