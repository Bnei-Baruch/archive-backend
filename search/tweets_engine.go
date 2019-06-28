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

func (e *ESEngine) SearchTweets(query Query, sortBy string, from int, size int, preference string) (map[string][]*elastic.SearchResult, error) {
	tweetsByLang := make(map[string][]*elastic.SearchResult)
	mssTweets := e.esc.MultiSearch()
	mssTweets.Add(NewResultsSearchRequests(
		SearchRequestOptions{
			resultTypes:      []string{consts.ES_RESULT_TYPE_TWEETS},
			index:            "",
			query:            query,
			sortBy:           sortBy,
			from:             0,
			size:             from + consts.TWEETS_SEARCH_COUNT,
			preference:       preference,
			useHighlight:     false,
			partialHighlight: false})...)

	beforeTweetsSearch := time.Now()
	mr, err := mssTweets.Do(context.TODO())
	e.timeTrack(beforeTweetsSearch, "DoSearch.MultisearcTweetsDo")
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
			if _, ok := tweetsByLang[lang]; !ok {
				tweetsByLang[lang] = make([]*elastic.SearchResult, 0)
			}
			tweetsByLang[lang] = append(tweetsByLang[lang], currentResults)
		}
	}

	//  Create single tweet result for each language.
	//  Set the score as the highest score of all tweets per language.

	for _, tweetResults := range tweetsByLang {

		for _, result := range tweetResults {

			var maxScore float64
			for _, hit := range result.Hits.Hits {
				if *hit.Score > maxScore {
					maxScore = *hit.Score
				}
			}

			source, err := json.Marshal(result.Hits.Hits)
			if err != nil {
				return nil, err
			}

			hit := &elastic.SearchHit{
				Type:   consts.ES_RESULT_TYPE_TWEETS,
				Source: (*json.RawMessage)(&source),
				Score:  &maxScore,
			}

			result.Hits.Hits = []*elastic.SearchHit{hit}
			result.Hits.TotalHits = 1
			result.Hits.MaxScore = &maxScore
		}

	}
	return tweetsByLang, nil
}
