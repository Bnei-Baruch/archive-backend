package search

import (
	"context"
	"database/sql"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/sync/errgroup"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

type ESEngine struct {
	esc *elastic.Client
	mdb *sql.DB
}

var classTypes = [...]string{"source", "tag"}

// TODO: all interactions with ES should be throttled to prevent downstream pressure

func NewESEngine(esc *elastic.Client, db *sql.DB) *ESEngine {
	return &ESEngine{esc: esc, mdb: db}
}

func (e *ESEngine) GetSuggestions(ctx context.Context, query Query) (interface{}, error) {
	// figure out index names from language order
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexName(consts.ES_CLASSIFICATIONS_INDEX, query.LanguageOrder[i])
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
				elastic.NewMatchQuery("name", q.Term),
				elastic.NewMatchQuery("description", q.Term),
				elastic.NewMatchQuery("transcript", q.Term),
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
	for filter, values := range q.Filters {
		switch filter {
		case consts.FILTER_START_DATE:
			query.Filter(elastic.NewRangeQuery("film_date").Gte(values[0]).Format("yyyy-MM-dd"))
		case consts.FILTER_END_DATE:
			query.Filter(elastic.NewRangeQuery("film_date").Lte(values[0]).Format("yyyy-MM-dd"))
		default:
			for _, value := range values {
				query.Filter(elastic.NewTermsQuery(filter, value))
			}
		}
	}
	return query
}

func (e *ESEngine) DoSearch(ctx context.Context, query Query, sortBy string, from int, size int, preference string) (interface{}, error) {
	multiSearchService := e.esc.MultiSearch()
	// Content Units
	content_units_indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		content_units_indices[i] = es.IndexName(consts.ES_UNITS_INDEX, query.LanguageOrder[i])
	}
	for _, index := range content_units_indices {
		searchSource := elastic.NewSearchSource().
			Query(createContentUnitsQuery(query)).
			Highlight(elastic.NewHighlight().Fields(
			elastic.NewHighlighterField("name"),
			elastic.NewHighlighterField("description"),
			elastic.NewHighlighterField("transcript"),
		)).
			From(from).
			Size(size)
		switch sortBy {
		case consts.SORT_BY_OLDER_TO_NEWER:
			searchSource = searchSource.Sort("film_date", true)
		case consts.SORT_BY_NEWER_TO_OLDER:
			searchSource = searchSource.Sort("film_date", false)
		}
		request := elastic.NewSearchRequest().
			SearchSource(searchSource).
			Index(index).
			Preference(preference)
		multiSearchService.Add(request)
	}
	// Do search.
	mr, err := multiSearchService.Do(context.TODO())

	if err != nil {
		return nil, errors.Wrap(err, "ES error")
	}

	for _, r := range mr.Responses {
		if r != nil && r.Hits != nil && r.Hits.Hits != nil && len(r.Hits.Hits) > 0 {
			return r, nil
		}
	}

	if len(mr.Responses) > 0 {
		return mr.Responses[0], nil
	} else {
		return nil, errors.Wrap(err, "No responses from multi search.")
	}
}
