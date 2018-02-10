package search

import (
	"context"
	"database/sql"
    "encoding/json"
    "fmt"
    "math"
	"net/url"
    "time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/sync/errgroup"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
    "github.com/Bnei-Baruch/archive-backend/utils"
)

type ESEngine struct {
	esc *elastic.Client
	// TODO: Is mdb required here?
	mdb *sql.DB
}

var classTypes = [...]string{"source", "tag"}

// TODO: All interactions with ES should be throttled to prevent downstream pressure

func NewESEngine(esc *elastic.Client, db *sql.DB) *ESEngine {
	return &ESEngine{esc: esc, mdb: db}
}

func (e *ESEngine) GetSuggestions(ctx context.Context, query Query) (interface{}, error) {
	// figure out index names from language order
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexName("prod", consts.ES_CLASSIFICATIONS_INDEX, query.LanguageOrder[i])
	}

	// We call ES in parallel. Each call with a different context query
	// (classification type), i.e, tag, source, author...
	g, ctx := errgroup.WithContext(ctx)
	resp := make([]*elastic.SearchResult, 0)
	for i := range classTypes {
		classType := classTypes[i]
		g.Go(func() error {

			// Call ES
			multiSearchService := e.esc.MultiSearch()
			for _, index := range indices {
				searchSource := elastic.NewSearchSource().
					Suggester(elastic.NewCompletionSuggester("classification_name").
					Field("name_suggest").
					Text(query.Term).
					ContextQuery(elastic.NewSuggesterCategoryQuery("classification", classType))).
					Suggester(elastic.NewCompletionSuggester("classification_description").
					Field("description_suggest").
					Text(query.Term).
					ContextQuery(elastic.NewSuggesterCategoryQuery("classification", classType)))

				request := elastic.NewSearchRequest().
					SearchSource(searchSource).
					Index(index)
				multiSearchService.Add(request)
			}
			mr, err := multiSearchService.Do(ctx)

			sRes := (*elastic.SearchResult)(nil)
			for _, r := range mr.Responses {
				if r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0 {
					sRes = r
					break
				}
			}

			if sRes == nil && len(mr.Responses) > 0 {
				sRes = mr.Responses[0]
			}

			if err != nil {
				// don't kill entire request if ctx was cancelled
				if ue, ok := err.(*url.Error); ok {
					if ue.Err == context.DeadlineExceeded || ue.Err == context.Canceled {
						log.Warnf("ES search %s: ctx cancelled", classType)
						return nil
					}
				}
				return errors.Wrapf(err, "ES search %s", classType)
			}

			resp = append(resp, sRes)

			return nil
		})
	}

	// Wait for first deadly error or all goroutines to finish
	if err := g.Wait(); err != nil {
		return nil, errors.Wrap(err, "ES error")
	}

	return resp, nil
}

func createContentUnitsQuery(q Query) elastic.Query {
	query := elastic.NewBoolQuery()
	if q.Term != "" {
		query = query.Must(
			elastic.NewBoolQuery().Should(
				elastic.NewMatchQuery("name.analyzed", q.Term),
				elastic.NewMatchQuery("description.analyzed", q.Term),
				elastic.NewMatchQuery("transcript.analyzed", q.Term),
			).MinimumNumberShouldMatch(1),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		query = query.Must(
			elastic.NewBoolQuery().Should(
				elastic.NewMatchPhraseQuery("name", exactTerm),
				elastic.NewMatchPhraseQuery("description", exactTerm),
				elastic.NewMatchPhraseQuery("transcript", exactTerm),
			).MinimumNumberShouldMatch(1),
		)
	}
	contentTypeQuery := elastic.NewBoolQuery().MinimumNumberShouldMatch(1)
	filterByContentType := false
	for filter, values := range q.Filters {
		s := make([]interface{}, len(values))
		for i, v := range values {
			s[i] = v
		}
		switch filter {
		case consts.FILTERS[consts.FILTER_START_DATE]:
			query.Filter(elastic.NewRangeQuery("effective_date").Gte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_END_DATE]:
			query.Filter(elastic.NewRangeQuery("effective_date").Lte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES], consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]:
			contentTypeQuery.Should(elastic.NewTermsQuery(filter, s...))
			filterByContentType = true
		default:
			query.Filter(elastic.NewTermsQuery(filter, s...))
		}
		if filterByContentType {
			query.Filter(contentTypeQuery)
		}
	}
	return query
}

func AddContentUnitsSearchRequests(mss *elastic.MultiSearchService, query Query, sortBy string, from int, size int, preference string) {
	content_units_indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		content_units_indices[i] = es.IndexName("prod", consts.ES_UNITS_INDEX, query.LanguageOrder[i])
	}
	fetchSourceContext := elastic.NewFetchSourceContext(true).
		Include("mdb_uid", "effective_date")
	for _, index := range content_units_indices {
		searchSource := elastic.NewSearchSource().
			Query(createContentUnitsQuery(query)).
			Highlight(elastic.NewHighlight().Fields(
			elastic.NewHighlighterField("name"),
			elastic.NewHighlighterField("description"),
			elastic.NewHighlighterField("transcript"),
			elastic.NewHighlighterField("name.analyzed"),
			elastic.NewHighlighterField("description.analyzed"),
			elastic.NewHighlighterField("transcript.analyzed"),
		)).
			FetchSourceContext(fetchSourceContext).
			From(from).
			Size(size)
		switch sortBy {
		case consts.SORT_BY_OLDER_TO_NEWER:
			searchSource = searchSource.Sort("effective_date", true)
		case consts.SORT_BY_NEWER_TO_OLDER:
			searchSource = searchSource.Sort("effective_date", false)
		}
		request := elastic.NewSearchRequest().
			SearchSource(searchSource).
			Index(index).
			Preference(preference)
		mss.Add(request)
	}
}

func createCollectionsQuery(q Query) elastic.Query {
	query := elastic.NewBoolQuery()
	if q.Term != "" {
		query = query.Must(
			elastic.NewBoolQuery().Should(
				elastic.NewMatchQuery("name.analyzed", q.Term),
				elastic.NewMatchQuery("description.analyzed", q.Term),
			).MinimumNumberShouldMatch(1),
		)
	}
	for _, exactTerm := range q.ExactTerms {
		query = query.Must(
			elastic.NewBoolQuery().Should(
				elastic.NewMatchPhraseQuery("name", exactTerm),
				elastic.NewMatchPhraseQuery("description", exactTerm),
			).MinimumNumberShouldMatch(1),
		)
	}
	contentTypeQuery := elastic.NewBoolQuery().MinimumNumberShouldMatch(1)
	filterByContentType := false
	for filter, values := range q.Filters {
		s := make([]interface{}, len(values))
		for i, v := range values {
			s[i] = v
		}
		switch filter {
		case consts.FILTERS[consts.FILTER_START_DATE]:
			query.Filter(elastic.NewRangeQuery("effective_date").Gte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_END_DATE]:
			query.Filter(elastic.NewRangeQuery("effective_date").Lte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES], consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]:
			contentTypeQuery.Should(elastic.NewTermsQuery(filter, s...))
			filterByContentType = true
		default:
			query.Filter(elastic.NewTermsQuery(filter, s...))
		}
		if filterByContentType {
			query.Filter(contentTypeQuery)
		}
	}
	return query
}

func AddCollectionsSearchRequests(mss *elastic.MultiSearchService, query Query, sortBy string, from int, size int, preference string) {
	collections_indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		collections_indices[i] = es.IndexName("prod", consts.ES_COLLECTIONS_INDEX, query.LanguageOrder[i])
	}
	fetchSourceContext := elastic.NewFetchSourceContext(true).
		Include("mdb_uid", "effective_date")
	for _, index := range collections_indices {
		searchSource := elastic.NewSearchSource().
			Query(createCollectionsQuery(query)).
			Highlight(elastic.NewHighlight().Fields(
			elastic.NewHighlighterField("name"),
			elastic.NewHighlighterField("description"),
			elastic.NewHighlighterField("name.analyzed"),
			elastic.NewHighlighterField("description.analyzed"),
		)).
			FetchSourceContext(fetchSourceContext).
			From(from).
			Size(size)
		switch sortBy {
		case consts.SORT_BY_OLDER_TO_NEWER:
			searchSource = searchSource.Sort("effective_date", true)
		case consts.SORT_BY_NEWER_TO_OLDER:
			searchSource = searchSource.Sort("effective_date", false)
		}
		request := elastic.NewSearchRequest().
			SearchSource(searchSource).
			Index(index).
			Preference(preference)
		mss.Add(request)
	}
}

func haveHits(r *elastic.SearchResult) bool {
    return r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0
}

func compareHits(h1* elastic.SearchHit, h2* elastic.SearchHit, sortBy string) (bool, error) {
    if sortBy == consts.SORT_BY_RELEVANCE {
        return *(h1.Score) >= *(h2.Score), nil
    } else {
        var ed1, ed2 es.EffectiveDate
        if err := json.Unmarshal(*h1.Source, &ed1); err != nil {
            return false, err
        }
        if err := json.Unmarshal(*h2.Source, &ed2); err != nil {
            return false, err
        }
        if ed1.EffectiveDate == nil{
            ed1.EffectiveDate = &utils.Date{time.Time{}}
        }
        if ed2.EffectiveDate == nil {
            ed2.EffectiveDate = &utils.Date{time.Time{}}
        }
        if sortBy == consts.SORT_BY_OLDER_TO_NEWER {
            return !ed1.EffectiveDate.Time.After(ed2.EffectiveDate.Time), nil
        } else {
            return ed1.EffectiveDate.Time.After(ed2.EffectiveDate.Time), nil
        }
    }
}

func joinResponses(r1 *elastic.SearchResult, r2 *elastic.SearchResult, sortBy string, from int, size int) (*elastic.SearchResult, error) {
    if r1.Hits.TotalHits == 0 {
        return r2, nil
    } else if r2.Hits.TotalHits == 0 {
        return r1, nil
    }
    result := elastic.SearchResult(*r1)
    result.Hits.TotalHits += r2.Hits.TotalHits
    if sortBy == consts.SORT_BY_RELEVANCE {
        result.Hits.MaxScore = new(float64)
        *result.Hits.MaxScore = math.Max(*result.Hits.MaxScore, *r2.Hits.MaxScore)
    }
    var hits []*elastic.SearchHit

    // Merge by score
    i1, i2 := int(0), int(0)
    for i1 < len(r1.Hits.Hits) || i2 < len(r2.Hits.Hits) {
        if i1 == len(r1.Hits.Hits) {
            hits = append(hits, r2.Hits.Hits[i2:]...)
            break
        }
        if i2 == len(r2.Hits.Hits) {
            hits = append(hits, r1.Hits.Hits[i1:]...)
            break
        }
        h1Larger, err := compareHits(r1.Hits.Hits[i1], r2.Hits.Hits[i2], sortBy)
        if err != nil {
            return nil, err
        }
        if h1Larger {
            hits = append(hits, r1.Hits.Hits[i1])
            i1++
        } else {
            hits = append(hits, r2.Hits.Hits[i2])
            i2++
        }
    }

    result.Hits.Hits = hits[from:utils.Min(from + size, len(hits))]
    return &result, nil
}

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string) (interface{}, error) {
    log.Infof("Query: %+v", query)
	multiSearchService := e.esc.MultiSearch()
	// Content Units
    AddContentUnitsSearchRequests(multiSearchService, query, sortBy, 0, from + size, preference)
    // Collections
    AddCollectionsSearchRequests(multiSearchService, query, sortBy, 0, from + size, preference)

	// Do search.
	mr, err := multiSearchService.Do(context.TODO())

	if err != nil {
		return nil, errors.Wrap(err, "ES error.")
	}

    if len(mr.Responses) != 2*len(query.LanguageOrder) {
        return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
            len(mr.Responses), 2*len(query.LanguageOrder)))
    }

    // Interleave content units and collection results by language.
    // Then go over responses and choose first not empty retults list.
    for i := 0; i < len(query.LanguageOrder); i++ {
        cuR := mr.Responses[i]
        cR := mr.Responses[i+len(query.LanguageOrder)]
        if haveHits(cuR) || haveHits(cR) {
            return joinResponses(cuR, cR, sortBy, from, size)
        }
    }

	if len(mr.Responses) > 0 {
		return mr.Responses[0], nil
	} else {
		return nil, errors.Wrap(err, "No responses from multi search.")
	}
}
