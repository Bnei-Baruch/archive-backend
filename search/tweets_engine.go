package search

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
	log "github.com/Sirupsen/logrus"
	"time"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/pkg/errors"
)

func (e *ESEngine) SearchTweets(query Query, sortBy string, from int, size int, preference string) (map[string]*elastic.SearchResult, error) {
	tweetsByLang := make(map[string]*elastic.SearchResult)
	mssTweets := e.esc.MultiSearch()
	mssTweets.Add(NewResultsSearchRequests(
		SearchRequestOptions{
			resultTypes:      []string{consts.ES_RESULT_TYPE_TWEETS},
			index:            "",
			query:            query,
			sortBy:           consts.SORT_BY_RELEVANCE,
			from:             0,
			size:             consts.TWEETS_SEARCH_COUNT,
			preference:       preference,
			useHighlight:     false,
			partialHighlight: false})...)

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

	combinedToSingleHit, err := e.CombineResultsToSingleHit(tweetsByLang, consts.SEARCH_RESULT_TWEETS_MANY, sortBy)
	if err != nil {
		return nil, err
	}

	return combinedToSingleHit, nil
}

func (e *ESEngine) CombineResultsToSingleHit(resultsByLang map[string]*elastic.SearchResult, hitType, sortBy string) (map[string]*elastic.SearchResult, error) {

	//  Create single hit result for each language.
	//  Set the score as the highest score of all hits per language.

	for _, result := range resultsByLang {
		edMax := es.EffectiveDate{EffectiveDate: &utils.Date{time.Time{}}}
		source, marshalErr := json.Marshal(edMax)
		if marshalErr != nil {
			return nil, marshalErr
		}

		//get source of combined hit from latest hit. Relevant when sorted by time
		if sortBy == consts.SORT_BY_NEWER_TO_OLDER || sortBy == consts.SORT_BY_OLDER_TO_NEWER {
			for _, hit := range result.Hits.Hits {
				var ed es.EffectiveDate
				err := json.Unmarshal(*hit.Source, &ed)
				if err != nil {
					log.Error("Can't unmarshal tweet result to EffectiveDate", err)
				}
				if err == nil && ed.EffectiveDate != nil && ed.EffectiveDate.Time.After(edMax.EffectiveDate.Time) {
					edMax = ed
					source = *hit.Source
				}
			}
		}

		hitsClone := *result.Hits

		innerHitsMap := make(map[string]*elastic.SearchHitInnerHits)
		innerHitsMap[hitType] = &elastic.SearchHitInnerHits{
			Hits: &hitsClone,
		}

		hit := &elastic.SearchHit{
			Source:    (*json.RawMessage)(&source),
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
