package search

import (
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	// Content boost.
	TITLE_BOOST       = 2.0
	DESCRIPTION_BOOST = 1.2
	FULL_TITLE_BOOST  = 1.1

	// Max slop.
	SLOP = 100

	// Following two boosts may be agregated.
	// Boost for standard anylyzer, i.e., without stemming.
	STANDARD_BOOST = 1.2
	// Boost for exact phrase match, without slop.
	EXACT_BOOST = 1.5

	NUM_SUGGESTS = 30
)

type Query struct {
	Term          string              `json:"term,omitempty"`
	ExactTerms    []string            `json:"exact_terms,omitempty"`
	Original      string              `json:"original,omitempty"`
	Filters       map[string][]string `json:"filters,omitempty"`
	LanguageOrder []string            `json:"language_order,omitempty"`
	Deb           bool                `json:"deb,omitempty"`
	Intents       []Intent            `json:"intents,omitempty"`
}

func isTokenStart(i int, runes []rune, lastQuote rune) bool {
	return i == 0 && !unicode.IsSpace(runes[0]) ||
		(i > 0 && !unicode.IsSpace(runes[i]) && unicode.IsSpace(runes[i-1]))
}

func isTokenEnd(i int, runes []rune, lastQuote rune, lastQuoteIdx int) bool {
	return i == len(runes)-1 ||
		(i < len(runes)-1 && unicode.IsSpace(runes[i+1]) &&
			(lastQuote == rune(0) || runes[i] == lastQuote && lastQuoteIdx >= 0 && lastQuoteIdx < i))
}

func isRuneQuotationMark(r rune) bool {
	return unicode.In(r, unicode.Quotation_Mark) || r == rune(1523) || r == rune(1524)
}

// Tokenizes string to work with user friendly escapings of quotes (see tests).
func tokenize(str string) []string {
	runes := []rune(str)
	start := -1
	lastQuote := rune(0)
	lastQuoteIdx := -1
	parts := 0
	var tokens []string
	for i, r := range runes {
		if start == -1 && isTokenStart(i, runes, lastQuote) {
			start = i
		}
		if i == start && lastQuote == rune(0) && isRuneQuotationMark(r) {
			lastQuote = r
			lastQuoteIdx = i
		}
		if start >= 0 && isTokenEnd(i, runes, lastQuote, lastQuoteIdx) {
			tokens = append(tokens, string(runes[start:i+1]))
			lastQuote = rune(0)
			lastQuoteIdx = -1
			start = -1
			parts += 1
		}
	}

	return tokens
}

// Parses query and extracts terms and filters.
func ParseQuery(q string) Query {
	filters := make(map[string][]string)
	var terms []string
	var exactTerms []string
	for _, t := range tokenize(q) {
		isFilter := false
		for filter := range consts.FILTERS {
			prefix := fmt.Sprintf("%s:", filter)
			if isFilter = strings.HasPrefix(t, prefix); isFilter {
				filters[consts.FILTERS[filter]] = strings.Split(strings.TrimPrefix(t, prefix), ",")
				break
			}
		}
		if !isFilter {
			// Not clear what kind of decoding is happening here, utf-8?!
			runes := []rune(t)
			// For debug
			// for _, c := range runes {
			//     fmt.Printf("%04x %s\n", c, string(c))
			// }
			if len(runes) >= 2 && isRuneQuotationMark(runes[0]) && runes[0] == runes[len(runes)-1] {
				exactTerms = append(exactTerms, string(runes[1:len(runes)-1]))
			} else {
				terms = append(terms, t)
			}
		}
	}
	return Query{Term: strings.Join(terms, " "), ExactTerms: exactTerms, Original: q, Filters: filters}
}

func createResultsQuery(resultTypes []string, q Query, docIds []string, filterOutCUSources []string) elastic.Query {
	boolQuery := elastic.NewBoolQuery().Must(
		elastic.NewConstantScoreQuery(
			elastic.NewTermsQuery("result_type", utils.ConvertArgsString(resultTypes)...),
		).Boost(0.0),
	)
	if docIds != nil && len(docIds) > 0 {
		idsQuery := elastic.NewIdsQuery().Ids(docIds...)
		boolQuery.Filter(idsQuery)
	}
	if len(filterOutCUSources) > 0 {
		rtForMustNotQuery := elastic.NewTermsQuery(consts.ES_RESULT_TYPE, consts.ES_RESULT_TYPE_UNITS)
		//ctForMustNotQuery := elastic.NewTermsQuery("typed_uids", fmt.Sprintf("content_type:%s", consts.CT_LESSON_PART))
		//  TBC if filtering out CT_VIDEO_PROGRAM_CHAPTER is also relevant
		//  PROBLEM - CT_LESSON_PART is not added to content_type. Why? For example: 3Qb2dWQj
		for _, src := range filterOutCUSources {
			sourceForMustNotQuery := elastic.NewTermsQuery("typed_uids", fmt.Sprintf("%s:%s", consts.FILTER_SOURCE, src))
			innerBoolQuery := elastic.NewBoolQuery().Filter(sourceForMustNotQuery, rtForMustNotQuery)
			boolQuery.MustNot(innerBoolQuery)
		}
	}
	if q.Term != "" {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchQuery("title.language", q.Term),
					elastic.NewMatchQuery("full_title.language", q.Term),
					elastic.NewMatchQuery("description.language", q.Term),
					elastic.NewMatchQuery("content.language", q.Term),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				// Language analyzed
				elastic.NewMatchPhraseQuery("title.language", q.Term).Slop(SLOP).Boost(TITLE_BOOST),
				elastic.NewMatchPhraseQuery("full_title.language", q.Term).Slop(SLOP).Boost(FULL_TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", q.Term).Slop(SLOP).Boost(DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", q.Term).Slop(SLOP),
				// Language analyzed, exact (no slop)
				elastic.NewMatchPhraseQuery("title.language", q.Term).Boost(EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("full_title.language", q.Term).Boost(EXACT_BOOST*FULL_TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", q.Term).Boost(EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", q.Term).Boost(EXACT_BOOST),
				// Standard analyzed
				elastic.NewMatchPhraseQuery("title", q.Term).Slop(SLOP).Boost(STANDARD_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("full_title", q.Term).Slop(SLOP).Boost(STANDARD_BOOST*FULL_TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Slop(SLOP).Boost(STANDARD_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Slop(SLOP).Boost(STANDARD_BOOST),
				// Standard analyzed, exact (no slop).
				elastic.NewMatchPhraseQuery("title", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("full_title", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST*FULL_TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", q.Term).Boost(STANDARD_BOOST*EXACT_BOOST),
			),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		boolQuery = boolQuery.Must(
			// Don't calculate score here, as we use sloped score below.
			elastic.NewConstantScoreQuery(
				elastic.NewBoolQuery().Should(
					elastic.NewMatchPhraseQuery("title", exactTerm),
					elastic.NewMatchPhraseQuery("full_title", exactTerm),
					elastic.NewMatchPhraseQuery("description", exactTerm),
					elastic.NewMatchPhraseQuery("content", exactTerm),
				).MinimumNumberShouldMatch(1),
			).Boost(0.0),
		).Should(
			elastic.NewDisMaxQuery().Query(
				// Language analyzed, exact (no slop)
				elastic.NewMatchPhraseQuery("title.language", exactTerm).Boost(EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("full_title.language", exactTerm).Boost(EXACT_BOOST*FULL_TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description.language", exactTerm).Boost(EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content.language", exactTerm).Boost(EXACT_BOOST),
				// Standard analyzed, exact (no slop).
				elastic.NewMatchPhraseQuery("title", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST*TITLE_BOOST),
				elastic.NewMatchPhraseQuery("full_title", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST*FULL_TITLE_BOOST),
				elastic.NewMatchPhraseQuery("description", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST*DESCRIPTION_BOOST),
				elastic.NewMatchPhraseQuery("content", exactTerm).Boost(STANDARD_BOOST*EXACT_BOOST),
			),
		)
	}
	contentTypeQuery := elastic.NewBoolQuery().MinimumNumberShouldMatch(1)
	filterByContentType := false
	for filter, values := range q.Filters {
		s := make([]string, len(values))
		for i, v := range values {
			s[i] = v
		}
		switch filter {
		case consts.FILTERS[consts.FILTER_START_DATE]:
			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Gte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_END_DATE]:
			boolQuery.Filter(elastic.NewRangeQuery("effective_date").Lte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES], consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]:
			contentTypeQuery.Should(elastic.NewTermsQuery("filter_values", es.KeyIValues(filter, s)...))
			filterByContentType = true
		case consts.FILTERS[consts.FILTER_SECTION_SOURCES]:
			boolQuery.Filter(elastic.NewTermsQuery("result_type", consts.ES_RESULT_TYPE_SOURCES))
		default:
			boolQuery.Filter(elastic.NewTermsQuery("filter_values", es.KeyIValues(filter, s)...))
		}
		if filterByContentType {
			boolQuery.Filter(contentTypeQuery)
		}
	}
	var query elastic.Query
	query = boolQuery
	if q.Term == "" && len(q.ExactTerms) == 0 {
		// No potential score from string matching.
		query = elastic.NewConstantScoreQuery(boolQuery).Boost(1.0)
	}
	scoreQuery := elastic.NewFunctionScoreQuery().ScoreMode("multiply")
	for _, resultType := range resultTypes {
		weight := 1.0
		if resultType == consts.ES_RESULT_TYPE_UNITS {
			weight = 1.1
		} else if resultType == consts.ES_RESULT_TYPE_TAGS {
			weight = 2.0 // We use tags for intents only
		} else if resultType == consts.ES_RESULT_TYPE_SOURCES {
			weight = 1.5
		} else if resultType == consts.ES_RESULT_TYPE_COLLECTIONS {
			weight = 2.0
		}
		scoreQuery.Add(elastic.NewTermsQuery("result_type", resultType), elastic.NewWeightFactorFunction(weight))
	}
	// Reduce score for clips.
	scoreQuery.Add(elastic.NewTermsQuery("filter_values", es.KeyValue("content_type", consts.CT_CLIP)), elastic.NewWeightFactorFunction(0.7))
	return elastic.NewFunctionScoreQuery().Query(scoreQuery.Query(query)).ScoreMode("sum").MaxBoost(100.0).
		AddScoreFunc(elastic.NewWeightFactorFunction(2.0)).
		AddScoreFunc(elastic.NewGaussDecayFunction().FieldName("effective_date").Decay(0.6).Scale("2000d"))
}

func NewResultsSearchRequest(options SearchRequestOptions) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "result_type")

	titleAdded := false
	fullTitleAdded := false
	contentAdded := false
	typedUidsAdded := false
	//	This is a generic imp. that supports searching tweets together with other results.
	//	Currently we are not searching for tweets together with other results but in parallel.
	for _, rt := range options.resultTypes {
		if !contentAdded && rt == consts.ES_RESULT_TYPE_TWEETS {
			fetchSourceContext.Include("content")
			contentAdded = true
		} else if !fullTitleAdded && rt == consts.ES_RESULT_TYPE_SOURCES {
			fetchSourceContext.Include("full_title")
			fullTitleAdded = true
		} else if !typedUidsAdded && rt == consts.ES_RESULT_TYPE_UNITS {
			fetchSourceContext.Include("typed_uids")
			typedUidsAdded = true
		} else if !titleAdded {
			fetchSourceContext.Include("title")
			titleAdded = true
		}
		if contentAdded && titleAdded && fullTitleAdded {
			break
		}
	}

	source := elastic.NewSearchSource().
		Query(createResultsQuery(options.resultTypes, options.query, options.docIds, options.filterOutCUSources)).
		FetchSourceContext(fetchSourceContext).
		From(options.from).
		Size(options.size).
		Explain(options.query.Deb)

	if options.useHighlight {
		terms := make([]string, 1)
		if options.query.Term != "" {
			terms = append(terms, options.query.Term)
		} else {
			terms = options.query.ExactTerms
		}

		contentNumOfFragments := 5 //  elastic default
		if options.highlightFullContent {
			contentNumOfFragments = 0
		}
		highlightQuery := createHighlightQuery(terms, contentNumOfFragments, options.partialHighlight)

		source = source.Highlight(highlightQuery)
	}

	switch options.sortBy {
	case consts.SORT_BY_OLDER_TO_NEWER:
		source = source.Sort("effective_date", true)
	case consts.SORT_BY_NEWER_TO_OLDER:
		source = source.Sort("effective_date", false)
	}
	return elastic.NewSearchRequest().
		SearchSource(source).
		Index(options.index).
		Preference(options.preference)
}

func createHighlightQuery(terms []string, n int, partialHighlight bool) *elastic.Highlight {
	//  We use special HighlightQuery with SimpleQueryStringQuery to
	//	 solve elastic issue with synonyms and highlights.

	query := elastic.NewHighlight()
	for _, term := range terms {
		query.Fields(
			elastic.NewHighlighterField("title").NumOfFragments(0).HighlightQuery(elastic.NewSimpleQueryStringQuery(term)),
			elastic.NewHighlighterField("full_title").NumOfFragments(0).HighlightQuery(elastic.NewSimpleQueryStringQuery(term)),
			elastic.NewHighlighterField("description").HighlightQuery(elastic.NewSimpleQueryStringQuery(term)),
			elastic.NewHighlighterField("description.language").HighlightQuery(elastic.NewSimpleQueryStringQuery(term)),
			elastic.NewHighlighterField("content").NumOfFragments(n).HighlightQuery(elastic.NewSimpleQueryStringQuery(term)),
			elastic.NewHighlighterField("content.language").NumOfFragments(n).HighlightQuery(elastic.NewSimpleQueryStringQuery(term)))

		if !partialHighlight {
			// Following field not used in intents to solve elastic bug with highlight.
			query.Fields(
				elastic.NewHighlighterField("title.language").NumOfFragments(0).HighlightQuery(elastic.NewSimpleQueryStringQuery(term)))
		}
	}
	return query
}

func NewResultsSearchRequests(options SearchRequestOptions) []*elastic.SearchRequest {
	requests := make([]*elastic.SearchRequest, 0)
	indices := make([]string, len(options.query.LanguageOrder))
	for i := range options.query.LanguageOrder {
		indices[i] = es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, options.query.LanguageOrder[i])
	}
	for _, index := range indices {
		options.index = index
		request := NewResultsSearchRequest(options)
		requests = append(requests, request)
	}
	return requests
}

func NewResultsSuggestRequest(resultTypes []string, index string, query Query, preference string) *elastic.SearchRequest {
	fetchSourceContext := elastic.NewFetchSourceContext(true).Include("mdb_uid", "result_type", "title", "full_title")
	searchSource := elastic.NewSearchSource().
		FetchSourceContext(fetchSourceContext).
		Suggester(
			elastic.NewCompletionSuggester("title_suggest").
				Field("title_suggest").
				Text(query.Term).
				ContextQuery(elastic.NewSuggesterCategoryQuery("result_type", resultTypes...)).
				Size(NUM_SUGGESTS).
				SkipDuplicates(true)).
		Suggester(
			elastic.NewCompletionSuggester("title_suggest.language").
				Field("title_suggest.language").
				Text(query.Term).
				ContextQuery(elastic.NewSuggesterCategoryQuery("result_type", resultTypes...)).
				Size(NUM_SUGGESTS).
				SkipDuplicates(true))

	return elastic.NewSearchRequest().
		SearchSource(searchSource).
		Index(index).
		Preference(preference)
}

func NewResultsSuggestRequests(resultTypes []string, query Query, preference string) []*elastic.SearchRequest {
	requests := make([]*elastic.SearchRequest, 0)
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexNameForServing("prod", consts.ES_RESULTS_INDEX, query.LanguageOrder[i])
	}
	for _, index := range indices {
		request := NewResultsSuggestRequest(resultTypes, index, query, preference)
		requests = append(requests, request)
	}
	return requests
}
