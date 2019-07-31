package search

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	log "github.com/Sirupsen/logrus"
)

func (e *ESEngine) AddIntentSecondRound(h *elastic.SearchHit, intent Intent, query Query) (error, *Intent, *Query) {
	var classificationIntent ClassificationIntent
	if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
		return err, nil, nil
	}
	if query.Deb {
		classificationIntent.Explanation = *h.Explanation
	}
	// log.Infof("Hit: %+v %+v", *h.Score, classificationIntent)
	if h.Score != nil && *h.Score > 0 {
		classificationIntent.Score = h.Score
		// Search for specific classification by full name to evaluate max score.
		query.Term = ""
		query.ExactTerms = []string{classificationIntent.Title}
		intent.Value = classificationIntent
		// log.Infof("Potential intent: %s", classificationIntent.Title)
		return nil, &intent, &query
	}
	return nil, nil, nil
}

func (e *ESEngine) AddIntents(query *Query, preference string, size int, sortBy string) ([]Intent, error) {

	intents := make([]Intent, 0)

	if (len(query.Term) == 0 && len(query.ExactTerms) == 0) ||
		sortBy == consts.SORT_BY_NEWER_TO_OLDER ||
		sortBy == consts.SORT_BY_OLDER_TO_NEWER {
		return intents, nil
	}

	for filterKey := range query.Filters {
		if _, ok := consts.ES_INTENT_SUPPORTED_FILTERS[filterKey]; !ok {
			return intents, nil
		}
	}

	if contentTypes, ok := query.Filters[consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]]; ok {
		for _, contentType := range contentTypes {
			if _, ok := consts.ES_INTENT_SUPPORTED_CONTENT_TYPES[contentType]; !ok {
				return intents, nil
			}
		}
	}

	defer e.timeTrack(time.Now(), "DoSearch.AddIntents")

	checkContentUnitsTypes := []string{}
	if values, ok := query.Filters[consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]]; ok {
		for _, value := range values {
			if value == consts.CT_LESSON_PART {
				checkContentUnitsTypes = append(checkContentUnitsTypes, consts.CT_LESSON_PART)
			}
			if value == consts.CT_VIDEO_PROGRAM_CHAPTER {
				checkContentUnitsTypes = append(checkContentUnitsTypes, consts.CT_VIDEO_PROGRAM_CHAPTER)
			}
		}
	} else {
		checkContentUnitsTypes = append(checkContentUnitsTypes, consts.CT_LESSON_PART, consts.CT_VIDEO_PROGRAM_CHAPTER)
	}

	queryWithoutFilters := *query
	queryWithoutFilters.Filters = make(map[string][]string)
	for filterName, values := range query.Filters {
		queryWithoutFilters.Filters[filterName] = values
	}
	//  Keep only source and tag filters.
	if _, ok := queryWithoutFilters.Filters[consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]]; ok {
		delete(queryWithoutFilters.Filters, consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES])
	}
	if _, ok := queryWithoutFilters.Filters[consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES]]; ok {
		delete(queryWithoutFilters.Filters, consts.FILTERS[consts.FILTER_COLLECTIONS_CONTENT_TYPES])
	}

	mssFirstRound := e.esc.MultiSearch()
	potentialIntents := make([]Intent, 0)
	for _, language := range query.LanguageOrder {
		// Order here provides the priority in results, i.e., tags are more importnt then sources.
		index := es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, language)
		mssFirstRound.Add(NewResultsSearchRequest(
			SearchRequestOptions{
				resultTypes:      []string{consts.ES_RESULT_TYPE_TAGS},
				index:            index,
				query:            queryWithoutFilters,
				sortBy:           consts.SORT_BY_RELEVANCE,
				from:             0,
				size:             size,
				preference:       preference,
				useHighlight:     false,
				partialHighlight: true}))
		potentialIntents = append(potentialIntents, Intent{consts.INTENT_TYPE_TAG, language, nil})
		mssFirstRound.Add(NewResultsSearchRequest(
			SearchRequestOptions{
				resultTypes:      []string{consts.ES_RESULT_TYPE_SOURCES},
				index:            index,
				query:            queryWithoutFilters,
				sortBy:           consts.SORT_BY_RELEVANCE,
				from:             0,
				size:             size,
				preference:       preference,
				useHighlight:     false,
				partialHighlight: true}))
		potentialIntents = append(potentialIntents, Intent{consts.INTENT_TYPE_SOURCE, language, nil})
	}
	beforeFirstRoundDo := time.Now()
	mr, err := mssFirstRound.Do(context.TODO())
	e.timeTrack(beforeFirstRoundDo, "DoSearch.AddIntents.FirstRoundDo")
	if err != nil {
		return intents, errors.Wrap(err, "ESEngine.AddIntents - Error multisearch Do.")
	}

	// Build second request to evaluate how close the search is toward the full name.
	mssSecondRound := e.esc.MultiSearch()
	finalIntents := make([]Intent, 0)
	for i := 0; i < len(potentialIntents); i++ {
		res := mr.Responses[i]
		if res.Error != nil {
			log.Warnf("ESEngine.AddIntents - First Run %+v", res.Error)
			return intents, errors.New("ESEngine.AddIntents - First Run Failed multi get (S).")
		}
		if haveHits(res) {
			for _, h := range res.Hits.Hits {
				err, intent, secondRoundQuery := e.AddIntentSecondRound(h, potentialIntents[i], queryWithoutFilters)
				// log.Infof("Adding second round for %+v %+v %+v", intent, secondRoundQuery, potentialIntents[i])
				if err != nil {
					return intents, errors.Wrapf(err, "ESEngine.AddIntents - Error second run for intent %+v", potentialIntents[i])
				}
				if intent != nil {
					mssSecondRound.Add(NewResultsSearchRequest(
						SearchRequestOptions{
							resultTypes:      []string{consts.RESULT_TYPE_BY_INDEX_TYPE[potentialIntents[i].Type]},
							index:            es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, intent.Language),
							query:            *secondRoundQuery,
							sortBy:           consts.SORT_BY_RELEVANCE,
							from:             0,
							size:             size,
							preference:       preference,
							useHighlight:     false,
							partialHighlight: true}))
					finalIntents = append(finalIntents, *intent)
				}
			}
		}
	}

	beforeSecondRoundDo := time.Now()
	mr, err = mssSecondRound.Do(context.TODO())
	e.timeTrack(beforeSecondRoundDo, "DoSearch.AddIntents.SecondRoundDo")
	for i := 0; i < len(finalIntents); i++ {
		res := mr.Responses[i]
		if res.Error != nil {
			log.Warnf("ESEngine.AddIntents - Second Run %+v", res.Error)
			log.Warnf("ESEngine.AddIntents - Second Run %+v", res.Error.RootCause[0])
			return intents, errors.New("ESEngine.AddIntents - Second Run Failed multi get (S).")
		}
		intentValue, intentOk := finalIntents[i].Value.(ClassificationIntent)
		if !intentOk {
			return intents, errors.New(fmt.Sprintf("ESEngine.AddIntents - Unexpected intent value: %+v", finalIntents[i].Value))
		}
		if haveHits(res) {
			// log.Infof("Found Hits for %+v", intentValue)
			found := false
			for _, h := range res.Hits.Hits {
				var classificationIntent ClassificationIntent
				if err := json.Unmarshal(*h.Source, &classificationIntent); err != nil {
					return intents, errors.Wrap(err, "ESEngine.AddIntents - Unmarshal classification intent filed.")
				}
				if query.Deb {
					intentValue.MaxExplanation = *h.Explanation
				}
				log.Debugf("%s: %+v", classificationIntent.Title, *h.Score)
				if intentValue.MDB_UID == classificationIntent.MDB_UID {
					found = true
					// log.Infof("Max Score: %+v", *h.Score)
					if h.Score != nil && *h.Score > 0 {
						intentValue.MaxScore = h.Score
						if *intentValue.MaxScore < *intentValue.Score {
							log.Warnf("ESEngine.AddIntents - Not expected score %f to be larger then max score %f for %s - %s.",
								*intentValue.Score, *intentValue.MaxScore, intentValue.MDB_UID, intentValue.Title)
						}
						intents = append(intents, Intent{finalIntents[i].Type, finalIntents[i].Language, intentValue})
					}
				}
			}
			if !found {
				log.Warnf("ESEngine.AddIntents - Did not find matching second run: %s - %s.",
					intentValue.MDB_UID, intentValue.Title)
			}
		}
	}

	// Set content unit type and exists for intents that are in the query, i.e., those who passed the second round.
	// If more then one content unit type exist for this intent, we will have to duplicate that intent.
	moreIntents := make([]Intent, 0)
	for intentIdx := range intents {
		for _, contentUnitType := range checkContentUnitsTypes {
			if intentValue, ok := intents[intentIdx].Value.(ClassificationIntent); ok {
				intentP := &intents[intentIdx]
				intentValueP := &intentValue
				if intentValue.ContentType != "" {
					// We need to copy the intent as we have more than one existing content types for that intent.
					moreIntents = append(moreIntents, intents[intentIdx])
					intentP = &moreIntents[len(moreIntents)-1]
					copyIntentValue := intentP.Value.(ClassificationIntent)
					intentValueP = &copyIntentValue
				}
				intentValueP.ContentType = contentUnitType
				if intentP.Type == consts.INTENT_TYPE_TAG {
					intentValueP.Exist = e.cache.SearchStats().IsTagWithEnoughUnits(intentValueP.MDB_UID, consts.INTENTS_MIN_UNITS, contentUnitType)
				} else if intentP.Type == consts.INTENT_TYPE_SOURCE {
					intentValueP.Exist = e.cache.SearchStats().IsSourceWithEnoughUnits(intentValueP.MDB_UID, consts.INTENTS_MIN_UNITS, contentUnitType)
				}
				// Assign the changed intent value, as everything is by value in golang.
				intentP.Value = *intentValueP
			}
		}
	}
	intents = append(intents, moreIntents...)
	return intents, nil
}
