package search

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/pkg/errors"
)

func (e *ESEngine) SearchTweets(query Query, sortBy string, from int, size int, preference string) (map[string]*elastic.SearchResult, error) {
	tweetsByLang := make(map[string]*elastic.SearchResult)
	mssTweets := e.esc.MultiSearch()
	requests, err := NewResultsSearchRequests(
		// Inside the carousel, the tweets are always sorted by relevance.
		//The EffectiveDate of the carousel itself will be equal to the EffectiveDate of the most relevant tweet.
		SearchRequestOptions{
			resultTypes:      []string{consts.ES_RESULT_TYPE_TWEETS},
			index:            "",
			query:            query,
			sortBy:           consts.SORT_BY_RELEVANCE,
			from:             0,
			size:             consts.TWEETS_SEARCH_COUNT,
			preference:       preference,
			useHighlight:     false,
			partialHighlight: false})
	if err != nil {
		return nil, err
	}
	mssTweets.Add(requests...)

	beforeTweetsSearch := time.Now()
	mr, err := mssTweets.Do(context.TODO())
	e.timeTrack(beforeTweetsSearch, consts.LAT_DOSEARCH_MULTISEARCHTWEETSDO)
	if err != nil {
		return nil, err
	}

	if len(mr.Responses) != len(query.LanguageOrder) {
		err := errors.New(fmt.Sprintf("Unexpected number of tweet results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
		return nil, err
	}

	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			err := errors.New(fmt.Sprintf("Failed tweets multi get: %+v", currentResults.Error))
			return nil, err
		}
		if haveHits(currentResults) {
			lang := query.LanguageOrder[i]
			tweetsByLang[lang] = currentResults
		}
	}

	combinedToSingleHit, err := e.CombineResultsToSingleHit(tweetsByLang, consts.SEARCH_RESULT_TWEETS_MANY)
	if err != nil {
		return nil, err
	}

	return combinedToSingleHit, nil
}

func (e *ESEngine) CombineResultsToSingleHit(resultsByLang map[string]*elastic.SearchResult, hitType string) (map[string]*elastic.SearchResult, error) {

	//  Create single hit result for each language.
	//  Set the score as the highest score of all hits per language.

	for _, result := range resultsByLang {
		hitsClone := *result.Hits

		innerHitsMap := make(map[string]*elastic.SearchHitInnerHits)
		innerHitsMap[hitType] = &elastic.SearchHitInnerHits{
			Hits: &hitsClone,
		}

		hit := &elastic.SearchHit{
			Source:    result.Hits.Hits[0].Source,
			Type:      hitType,
			Score:     result.Hits.Hits[0].Score,
			InnerHits: innerHitsMap,
		}

		result.Hits.Hits = []*elastic.SearchHit{hit}
		result.Hits.TotalHits = 1
		result.Hits.MaxScore = result.Hits.Hits[0].Score
	}

	return resultsByLang, nil
}

// Moving data from InnerHits to Source (as marshaled json) (this is for client).
func (e *ESEngine) NativizeTweetsHitForClient(hit *elastic.SearchHit, innerHitsKey string) error {
	if hit.InnerHits == nil {
		return errors.New("NativizeHitForClient - InnerHits is nil.")
	}
	if _, ok := hit.InnerHits[innerHitsKey]; !ok {
		return errors.New(fmt.Sprintf("NativizeHitForClient - %s key is not present in InnerHits.", innerHitsKey))
	}
	if hit.InnerHits[innerHitsKey].Hits == nil {
		return errors.New(fmt.Sprintf("hit.InnerHits[%s].Hits is nil.", innerHitsKey))
	}

	hits := hit.InnerHits[innerHitsKey].Hits.Hits
	source, err := json.Marshal(hits)
	if err != nil {
		return err
	}

	hit.Source = (*json.RawMessage)(&source)
	hit.InnerHits = nil

	return nil
}
