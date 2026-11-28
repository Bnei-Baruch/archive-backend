package search

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/stretchr/testify/require"
	"gopkg.in/olivere/elastic.v6"
)

type searchCancellationTransport func(*http.Request) (*http.Response, error)

func (f searchCancellationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newSearchCancellationTestEngine(t *testing.T, transport searchCancellationTransport) *ESEngine {
	client, err := elastic.NewClient(
		elastic.SetURL("http://elasticsearch.test"),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(false),
		elastic.SetHttpClient(&http.Client{Transport: transport}),
	)
	require.NoError(t, err)
	t.Cleanup(client.Stop)
	return NewESEngine(client, nil, nil, nil, nil, consts.ES_SEARCH_RESULT_TYPES)
}

func searchCancellationResponse(hits string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"responses":[{"hits":{"total":1,"max_score":1,"hits":` + hits + `}}]}`)),
	}
}

func TestESEngineCancellationIsOptIn(t *testing.T) {
	for _, tc := range []struct {
		name                                             string
		enabled, parallel, highlight, deadline, failMain bool
	}{
		{name: "regular"},
		{name: "reasoning-main", enabled: true},
		{name: "reasoning-workers", enabled: true, parallel: true},
		{name: "reasoning-highlights", enabled: true, highlight: true},
		{name: "reasoning-deadline", enabled: true, deadline: true},
		{name: "reasoning-error-cleanup", enabled: true, parallel: true, failMain: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan context.Context, 4)
			exited := make(chan struct{}, 4)
			release := make(chan struct{})
			defer close(release)
			var requests int32
			engine := newSearchCancellationTestEngine(t, func(r *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}
				if atomic.AddInt32(&requests, 1) == 1 && tc.highlight {
					return searchCancellationResponse(`[{"_id":"hit","_index":"results-en","_type":"result","_score":1,"_source":{"result_type":"units","mdb_uid":"public01","title":"Example"}}]`), nil
				}
				started <- r.Context()
				defer func() { exited <- struct{}{} }()
				if tc.failMain && !strings.Contains(string(body), `"main-test"`) {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				select {
				case <-r.Context().Done():
					return nil, r.Context().Err()
				case <-release:
					if tc.failMain && strings.Contains(string(body), `"main-test"`) {
						return nil, errors.New("main search failed")
					}
					return searchCancellationResponse(`[]`), nil
				}
			})
			engine.RespectCancellation = tc.enabled
			engine.searchResultTypes = []string{"main-test"}
			timeout := time.Minute
			if tc.deadline {
				timeout = 200 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				// Mixed terms skip grammars; date sorting skips classification intents.
				query := Query{Term: "test", ExactTerms: []string{"exact"}, LanguageOrder: []string{consts.LANG_ENGLISH}}
				_, err := engine.DoSearch(ctx, query, consts.SORT_BY_NEWER_TO_OLDER, 0, 10, "test", tc.parallel, tc.parallel, tc.parallel, tc.highlight, time.Minute)
				done <- err
			}()
			count := 1
			if tc.parallel {
				count = 4 // main, tweets, series, and typo suggestion
			}
			var requestContexts []context.Context
			for i := 0; i < count; i++ {
				select {
				case requestCtx := <-started:
					requestContexts = append(requestContexts, requestCtx)
				case err := <-done:
					t.Fatalf("search exited before requests started: %v", err)
				case <-time.After(time.Second):
					t.Fatal("Elasticsearch request did not start")
				}
			}
			if tc.failMain {
				// All four calls are in flight; only the main one fails.
				// DoSearch must cancel the others and join their channel senders.
				release <- struct{}{}
			} else if tc.deadline {
				<-ctx.Done()
			} else {
				cancel()
			}
			if !tc.enabled {
				for _, requestCtx := range requestContexts {
					require.NoError(t, requestCtx.Err(), "regular search must keep ignoring request cancellation")
				}
				select {
				case err := <-done:
					t.Fatalf("regular search stopped on cancellation: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				release <- struct{}{}
			}
			select {
			case err := <-done:
				if tc.failMain {
					require.Error(t, err)
					require.Contains(t, err.Error(), "main search failed")
					require.NoError(t, ctx.Err(), "cleanup must not cancel the caller's context")
				} else if tc.enabled {
					require.ErrorIs(t, err, ctx.Err())
				} else {
					require.NoError(t, err)
				}
			case <-time.After(time.Second):
				t.Fatal("search did not exit; an HTTP request or channel sender is still blocked")
			}
			require.Len(t, exited, count, "search must join all started HTTP calls before returning")
		})
	}
}

func TestESEngineSubsearchCancellation(t *testing.T) {
	query := Query{Term: "test", LanguageOrder: []string{consts.LANG_ENGLISH}}
	for _, tc := range []struct {
		name string
		run  func(context.Context, *ESEngine) error
	}{
		{"grammar", func(ctx context.Context, engine *ESEngine) error {
			_, _, err := engine.SearchGrammarsV2(ctx, &query, 0, 10, consts.SORT_BY_RELEVANCE, consts.ES_SEARCH_RESULT_TYPES, "test")
			return err
		}},
		{"filtered-grammar", func(ctx context.Context, engine *ESEngine) error {
			intents := []Intent{{Language: consts.LANG_ENGLISH, Value: GrammarIntent{FilterValues: []FilterValue{{Name: consts.VARIABLE_TO_FILTER[consts.VAR_CONTENT_TYPE], Value: consts.VAR_CT_LESSONS}}}}}
			_, err := engine.SearchByFilterIntents(ctx, intents, nil, "test", 0, 10, consts.SORT_BY_RELEVANCE, consts.ES_SEARCH_RESULT_TYPES, "test", false)
			return err
		}},
		{"intents-first-round", func(ctx context.Context, engine *ESEngine) error {
			_, err := engine.AddIntents(ctx, &query, "test", consts.SORT_BY_RELEVANCE, true, false, nil)
			return err
		}},
		{"intents-second-round", func(ctx context.Context, engine *ESEngine) error {
			_, err := engine.AddIntents(ctx, &query, "test", consts.SORT_BY_RELEVANCE, true, false, nil)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan struct{}, 1)
			var requests int32
			engine := newSearchCancellationTestEngine(t, func(r *http.Request) (*http.Response, error) {
				if atomic.AddInt32(&requests, 1) == 1 && tc.name == "intents-second-round" {
					return searchCancellationResponse(`[{"_score":1,"_source":{"mdb_uid":"tag00001","title":"Example"}}]`), nil
				}
				started <- struct{}{}
				<-r.Context().Done()
				return nil, r.Context().Err()
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- tc.run(ctx, engine) }()
			select {
			case <-started:
			case err := <-done:
				t.Fatalf("subsearch exited before Elasticsearch request: %v", err)
			case <-time.After(time.Second):
				t.Fatal("Elasticsearch request did not start")
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("subsearch ignored cancellation")
			}
		})
	}
}
