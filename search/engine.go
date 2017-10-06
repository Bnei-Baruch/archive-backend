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

			// call ES
			sRes, err := e.esc.
				Search(indices...).
				Suggester(elastic.NewCompletionSuggester("classification_name").
					Field("name_suggest").
					Text(query.Term).
					ContextQuery(elastic.NewSuggesterCategoryQuery("classification", classType))).
				Suggester(elastic.NewCompletionSuggester("classification_description").
					Field("description_suggest").
					Text(query.Term).
					ContextQuery(elastic.NewSuggesterCategoryQuery("classification", classType))).
				Do(ctx)

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

func (e *ESEngine) DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error) {
	// figure out index names from language order
	indices := make([]string, len(query.LanguageOrder))
	for i := range query.LanguageOrder {
		indices[i] = es.IndexName(consts.ES_UNITS_INDEX, query.LanguageOrder[i])
	}

	resp, err := e.esc.Search(indices...).
		Query(
			elastic.NewBoolQuery().Should(
				elastic.NewMatchQuery("name", query.Term),
				elastic.NewMatchQuery("description", query.Term),
			)).
		Highlight(
			elastic.NewHighlight().Fields(
				elastic.NewHighlighterField("name"),
				elastic.NewHighlighterField("description"),
			)).
		From(from).
		Size(size).
		Preference(preference).
		Do(context.TODO())

	if err != nil {
		return nil, errors.Wrap(err, "ES error")
	}

	return resp, nil
}
