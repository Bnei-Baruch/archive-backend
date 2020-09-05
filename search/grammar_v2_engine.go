package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func (e *ESEngine) SuggestGrammarsV2(query *Query, preference string) (map[string][]VariablesByPhrase, error) {
	start := time.Now()
	// Map from lang => Original Full Phrase => $Var => values
	suggests := make(map[string][]VariablesByPhrase)

	if query.Term == "" || len(query.ExactTerms) > 0 {
		log.Infof("Term is empty of exact term exists, should not trigger grammar: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return suggests, nil
	}

	multiSearchService := e.esc.MultiSearch()

	for _, language := range query.LanguageOrder {
		// Suggester:
		multiSearchService.Add(NewResultsSuggestGrammarV2CompletionRequest(query, language, preference))
		// Search (will not match part of words): multiSearchService.Add(NewSuggestGammarV2Request(query, language, preference))
	}

	mr, err := multiSearchService.Do(context.TODO())
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("multiSearchService.Do - %s\n\n", elapsed.String())
	}
	if err != nil {
		return nil, errors.Wrap(err, "Error loking for grammar suggest.")
	}

	if len(mr.Responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
	}

	start = time.Now()
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		// Suggester
		if SuggestionHasOptions(currentResults.Suggest) {
			language := query.LanguageOrder[i]
			if suggests[language], err = e.suggestOptionsToVariablesByPhrases(query, &currentResults.Suggest); err != nil {
				return nil, err
			}
		}

		// Searcher: <=== will not match part of words.
		//if haveHits(currentResults) {
		//	language := query.LanguageOrder[i]
		//	if suggests[language], err = e.suggestResultsToVariablesByPhrases(query, currentResults); err != nil {
		//		return nil, err
		//	}
		//}

	}
	elapsed = time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("build suggests - %s\n\n", elapsed.String())
	}

	return suggests, nil
}

func (e *ESEngine) suggestOptionsToVariablesByPhrases(query *Query, suggest *elastic.SearchSuggest) ([]VariablesByPhrase, error) {
	ret := []VariablesByPhrase(nil)
	for _, v := range *suggest {
		for _, s := range v {
			if len(s.Options) > 0 {
				for _, option := range s.Options {
					var rule GrammarRule
					if err := json.Unmarshal(*option.Source, &rule); err != nil {
						return nil, err
					}
					// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", option.Score, option.Index, option.Type, option.Id, rule)
					if len(rule.Values) != len(rule.Variables) {
						return nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
					}
					vMap := make(map[string][]string)
					for i := range rule.Variables {
						vMap[rule.Variables[i]] = []string{rule.Values[i]}
					}
					if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
						//log.Infof("Chosen: %s", chosen)
						//log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", option.Score, option.Index, option.Type, option.Id, rule)
						//log.Infof("Options: %+v", option)
						//log.Infof("vMap: [%+v]", vMap)
						// Map from lang => Original Full Phrase => $Var => values
						variablesByPhrase := make(VariablesByPhrase)
						variablesByPhrase[chooseRule(query, rule.Rules)] = vMap
						ret = append(ret, variablesByPhrase)
					}
				}
			}
		}
	}

	return ret, nil
}

func (e *ESEngine) suggestResultsToVariablesByPhrases(query *Query, result *elastic.SearchResult) ([]VariablesByPhrase, error) {
	ret := []VariablesByPhrase(nil)
	if haveHits(result) {
		// log.Infof("Total Hits: %d, Max Score: %.2f", result.Hits.TotalHits, *result.Hits.MaxScore)
		for _, hit := range result.Hits.Hits {
			var rule GrammarRule
			if err := json.Unmarshal(*hit.Source, &rule); err != nil {
				return nil, err
			}
			// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", *hit.Score, hit.Index, hit.Type, hit.Id, rule)
			if len(rule.Values) != len(rule.Variables) {
				return nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
			}
			vMap := make(map[string][]string)
			for i := range rule.Variables {
				vMap[rule.Variables[i]] = []string{rule.Values[i]}
			}
			if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
				// Map from lang => Original Full Phrase => $Var => values
				variablesByPhrase := make(VariablesByPhrase)
				variablesByPhrase[chooseRule(query, rule.Rules)] = vMap
				ret = append(ret, variablesByPhrase)
			}
		}
	}
	return ret, nil
}

func (e *ESEngine) SearchGrammarsV2(query *Query, preferences string) ([]Intent, error) {
	intents := []Intent{}
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return intents, nil
	}

	multiSearchService := e.esc.MultiSearch()
	for _, language := range query.LanguageOrder {
		multiSearchService.Add(NewSuggestGammarV2Request(query, language, preferences))
		multiSearchService.Add(NewGammarPerculateRequest(query, language, preferences))
	}
	mr, err := multiSearchService.Do(context.TODO())
	if err != nil {
		return nil, errors.Wrap(err, "Error loking for grammar suggest.")
	}

	if len(mr.Responses) != len(query.LanguageOrder) {
		return nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)))
	}

	start := time.Now()
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		language := query.LanguageOrder[i]

		if haveHits(currentResults) {
			if langIntents, err := e.searchResultsToIntents(query, language, currentResults); err != nil {
				return nil, err
			} else {
				intents = append(intents, langIntents...)
			}
		}

	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("build grammar intent - %s\n\n", elapsed.String())
	}

	return intents, nil
}

func (e *ESEngine) VariableMapToFilterValues(vMap map[string][]string, language string) []FilterValue {
	ret := []FilterValue{}
	for name, values := range vMap {
		// TODO: Actually map the variable names to filter names and variable values to filter values.
		// Maybe this should be done in frontend...
		filterName, ok := consts.VARIABLE_TO_FILTER[name]
		if !ok {
			filterName = name
		}
		for _, value := range values {
			ret = append(ret, FilterValue{
				Name:       filterName,
				Value:      value,
				Origin:     e.variables[name][language][value][0],
				OriginFull: e.variables[name][language][value][0],
			})
		}
	}
	return ret
}

func updateIntentCount(intentsCount map[string][]Intent, intent Intent) {
	intents := intentsCount[intent.Value.(GrammarIntent).LandingPage]
	intents = append(intents, intent)
	sort.SliceStable(intents, func(i, j int) bool {
		return intents[i].Value.(GrammarIntent).Score > intents[j].Value.(GrammarIntent).Score
	})
	intents = intents[:utils.MinInt(consts.MAX_MATCHES_PER_GRAMMAR_INTENT, len(intents))]
	intentsCount[intent.Value.(GrammarIntent).LandingPage] = intents
}

func (e *ESEngine) searchResultsToIntents(query *Query, language string, result *elastic.SearchResult) ([]Intent, error) {
	// log.Infof("Total Hits: %d, Max Score: %.2f", result.Hits.TotalHits, *result.Hits.MaxScore)
	intentsCount := make(map[string][]Intent)
	for _, hit := range result.Hits.Hits {
		var rule GrammarRule
		if err := json.Unmarshal(*hit.Source, &rule); err != nil {
			return nil, err
		}
		// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", *hit.Score, hit.Index, hit.Type, hit.Id, rule)
		if len(rule.Values) != len(rule.Variables) {
			return nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
		}

		// Check filters match, i.e., existing query filter match at least one supported grammar intent filter.
		if len(query.Filters) > 0 {
			filters, filterExist := consts.GRAMMAR_INTENTS_TO_FILTER_VALUES[rule.Intent]
			if !filterExist {
				return nil, errors.New(fmt.Sprintf("Filters not found for intent: [%s]", rule.Intent))
			}
			common := false
			for filterName, values := range query.Filters {
				sort.Strings(values)
				if grammarValues, ok := filters[filterName]; ok {
					sort.Strings(grammarValues)
					if len(utils.IntersectSortedStringSlices(values, grammarValues)) > 0 {
						common = true
						break
					}
				}
			}
			if !common {
				// No matching filter found, should not trigger intent.
				log.Infof("No common filters for intent [%s]: %+v vs %+v", rule.Intent, query.Filters, filters)
				continue
			}
		}

		vMap := make(map[string][]string)
		for i := range rule.Variables {
			vMap[rule.Variables[i]] = []string{rule.Values[i]}
		}
		if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
			score := *hit.Score * (float64(4) / float64(4+len(vMap))) * YearScorePenalty(vMap)
			// Issue with tf/idf. For query [congress] the score if very low. For [arava] ok.
			// Fix this by moving the grammar index into the common index. So tha similar tf/idf will be used.
			// For now solve by normalizing very small scores.
			// log.Infof("Intent: %+v score: %.2f %.2f %.2f", vMap, *hit.Score, (float64(4) / float64(4+len(vMap))), score)
			updateIntentCount(intentsCount, Intent{
				Type:     consts.GRAMMAR_TYPE_LANDING_PAGE,
				Language: language,
				Value: GrammarIntent{
					LandingPage:  rule.Intent,
					FilterValues: e.VariableMapToFilterValues(vMap, language),
					Score:        score,
					Explanation:  hit.Explanation,
				},
			})
		}
	}
	intents := []Intent(nil)
	for _, intentsByLandingPage := range intentsCount {
		intents = append(intents, intentsByLandingPage...)
	}
	// Normalize score to be from 2000 and below.
	maxScore := 0.0
	for i := range intents {
		if intents[i].Value.(GrammarIntent).Score > maxScore {
			maxScore = intents[i].Value.(GrammarIntent).Score
		}
	}
	normalizedIntents := []Intent(nil)
	for _, intent := range intents {
		grammarIntent := intent.Value.(GrammarIntent)
		grammarIntent.Score = 3000 * (grammarIntent.Score / maxScore)
		intent.Value = grammarIntent
		normalizedIntents = append(normalizedIntents, intent)
	}
	// log.Infof("Intents: %+v", intents)
	return normalizedIntents, nil
}

func (e *ESEngine) ConventionsLandingPageToCollectionHit(year string, location string) (*elastic.SearchHit, error) {
	queryMask := `select c.uid, c.properties from collections c 
	where c.type_id=%d
	%s`
	cityMask := `c.properties ->> 'city' = '%s'`
	countryMask := `c.properties ->> 'country' = '%s'`
	yearMask := `extract(year from (c.properties ->> 'start_date')::date) = %s`

	var country string
	var city string

	if location != "" {
		s := strings.Split(location, "|")
		country = s[0]
		if len(s) > 1 {
			city = s[1]
		}
	}

	whereClauses := make([]string, 0)
	if year != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(yearMask, year))
	}
	if country != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(countryMask, country))
	}
	if city != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(cityMask, city))
	}

	var whereQuery string
	if len(whereClauses) > 0 {
		whereQuery = fmt.Sprintf("and %s", strings.Join(whereClauses, " and "))
	}
	query := fmt.Sprintf(queryMask, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CONGRESS].ID, whereQuery)
	//log.Infof("ConventionsLandingPageToCollectionHit Query: %s", query)
	return e.collectionHitFromSql(query)
}

func (e *ESEngine) HolidaysLandingPageToCollectionHit(year string, holiday string) (*elastic.SearchHit, error) {
	queryMask := `select c.uid, c.properties from collections c
	join tags t on c.properties ->> 'holiday_tag' = t.uid
	%s`
	uidMask := `t.uid = '%s'`
	yearMask := `(extract(year from (c.properties ->> 'start_date')::date) = %s or extract(year from (c.properties ->> 'end_date')::date) = %s)`

	var whereQuery string
	if year != "" && holiday != "" {
		whereQuery = fmt.Sprintf("where %s and %s", fmt.Sprintf(uidMask, holiday), fmt.Sprintf(yearMask, year, year))
	} else if year != "" {
		whereQuery = fmt.Sprintf("where %s", fmt.Sprintf(yearMask, year, year))
	} else if holiday != "" {
		whereQuery = fmt.Sprintf("where %s", fmt.Sprintf(uidMask, holiday))
	}

	query := fmt.Sprintf(queryMask, whereQuery)
	//log.Infof("QUERY: %s", query)
	return e.collectionHitFromSql(query)
}

func (e *ESEngine) collectionHitFromSql(query string) (*elastic.SearchHit, error) {
	var properties json.RawMessage
	var mdbUID string
	var effectiveDate es.EffectiveDate

	err := e.mdb.QueryRow(query).Scan(&mdbUID, &properties)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(properties, &effectiveDate)
	if err != nil {
		return nil, err
	}

	result := es.Result{
		EffectiveDate: effectiveDate.EffectiveDate,
		MDB_UID:       mdbUID,
		ResultType:    consts.ES_RESULT_TYPE_COLLECTIONS,
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	hit := &elastic.SearchHit{
		Source: (*json.RawMessage)(&resultJson),
		Type:   "result",
		Index:  consts.GRAMMAR_LP_SINGLE_COLLECTION,
	}
	return hit, nil
}
