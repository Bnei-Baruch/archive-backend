package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
		return nil, errors.Wrap(err, "Error looking for grammar suggest.")
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
					var ruleObj GrammarRuleWithPercolatorQuery
					if err := json.Unmarshal(*option.Source, &ruleObj); err != nil {
						return nil, err
					}
					rule := ruleObj.GrammarRule
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

// Return: single hit intents, filtering intents
func (e *ESEngine) SearchGrammarsV2(query *Query, from int, size int, sortBy string, resultTypes []string, preference string) ([]Intent, []Intent, error) {
	singleHitIntents := []Intent{}
	filterIntents := []Intent{}
	if query.Term != "" && len(query.ExactTerms) > 0 {
		// Will never match any grammar for query having simple terms and exact terms.
		// This is not acurate but an edge case. Need to better think of query representation.
		log.Infof("Both term and exact terms are defined, should not trigger grammar: [%s] [%s]", query.Term, strings.Join(query.ExactTerms, " - "))
		return singleHitIntents, filterIntents, nil
	}
	if e.isTermRestricted(query.Term, query.LanguageOrder) {
		log.Infof("Term is restricted, should not trigger grammar: [%s]", query.Term)
		return singleHitIntents, filterIntents, nil
	}
	searchLandingPagesOnly := false
	// queriesNumForLang is the number of multiSearchService requests for each language.
	// The number is 2 if we trigger a percolator search (for free text variables) in addition to a regular grammar search.
	queriesNumForLang := 2
	checkIfTermEqualsSource := true
	for filterKey := range query.Filters {
		if _, ok := consts.AUTO_INTENTS_BY_SOURCE_NAME_SUPPORTED_FILTERS[filterKey]; !ok {
			checkIfTermEqualsSource = false
			break
		}
	}
	if checkIfTermEqualsSource {
		sourceUid, language, isAuthor := e.sourceUidByTerm(query.Term, query.LanguageOrder)
		if sourceUid != nil {
			// Since some source titles contains grammar variable values,
			// we are limiting the grammar search to landing pages only if the term eqauls to a title of a source\author.
			// Some examples for such source titles:
			// 'Book, Author, Story','Connecting to the Source', 'Introduction to articles', 'שיעור ההתגברות', 'ספר הזוהר'
			// If the term is not a name of author, automatically add classification intents and source result.
			log.Infof("The term [%s] is identical to a name of author or source. Search only for Landing Pages.", query.Term)
			searchLandingPagesOnly = true
			queriesNumForLang = 1
			if !isAuthor {
				log.Infof("Adding intents by the source [%s] (%s).", *sourceUid, query.Term)
				parent, position, _, err := e.cache.SearchStats().GetSourceParentAndPosition(*sourceUid, false)
				if err != nil {
					return nil, nil, errors.Wrap(err, "GetSourceParentAndPosition")
				}
				var leafPrefixType *consts.PositionIndexType
				if parent != nil {
					if val, ok := consts.ES_SRC_PARENTS_FOR_CHAPTER_POSITION_INDEX[*parent]; ok {
						leafPrefixType = &val
					}
				}
				path, err := e.sourcePathFromSql(*sourceUid, language, position, leafPrefixType)
				if err != nil {
					return nil, nil, errors.Wrap(err, "sourcePathFromSql")
				}
				intents, err := e.getSingleHitIntentsBySource(*sourceUid, query.Filters, language, path, 3000.0, elastic.SearchExplanation{})
				if err != nil {
					return nil, nil, errors.Wrap(err, "getSingleHitIntentsBySource")
				}
				singleHitIntents = append(singleHitIntents, intents...)
			}
		}
	}
	multiSearchService := e.esc.MultiSearch()
	if searchLandingPagesOnly {
		for _, language := range query.LanguageOrder {
			hitType := "landing-pages"
			multiSearchService.Add(NewSuggestGammarV2Request(query, language, preference, &hitType))
		}
	} else {
		for _, language := range query.LanguageOrder {
			multiSearchService.Add(NewSuggestGammarV2Request(query, language, preference, nil))
			multiSearchService.Add(NewGammarPerculateRequest(query, language, preference))
		}
	}
	beforeGrammarSearch := time.Now()
	mr, err := multiSearchService.Do(context.TODO())
	e.timeTrack(beforeGrammarSearch, consts.LAT_DOSEARCH_GRAMMARS_MULTISEARCHGRAMMARSDO)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error looking for grammar search.")
	}

	if len(mr.Responses) != len(query.LanguageOrder)*queriesNumForLang {
		return nil, nil, errors.New(fmt.Sprintf("Unexpected number of results %d, expected %d",
			len(mr.Responses), len(query.LanguageOrder)*queriesNumForLang))
	}

	start := time.Now()
	filterIntentsByLanguage := map[string][]Intent{}
	for i, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, nil, errors.New(fmt.Sprintf("Failed multi get: %+v", currentResults.Error))
		}
		language := query.LanguageOrder[i/queriesNumForLang]
		if haveHits(currentResults) {
			if languageSingleHitIntents, languageFilterIntents, err := e.searchResultsToIntents(query, language, currentResults); err != nil {
				return nil, nil, err
			} else {
				singleHitIntents = append(singleHitIntents, languageSingleHitIntents...)
				if _, ok := filterIntentsByLanguage[language]; !ok {
					filterIntentsByLanguage[language] = []Intent{}
				}
				filterIntentsByLanguage[language] = append(filterIntentsByLanguage[language], languageFilterIntents...)
			}
		}
	}
	for _, intentsByLang := range filterIntentsByLanguage {
		if len(intentsByLang) > 0 {
			intentsToAdd, err := e.selectFilterIntents(intentsByLang)
			if err != nil {
				return nil, nil, err
			}
			filterIntents = append(filterIntents, intentsToAdd...)
		}
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		fmt.Printf("build grammar intent - %s\n\n", elapsed.String())
	}
	return singleHitIntents, filterIntents, nil
}

// Search according to grammar based filter.
func (e *ESEngine) SearchByFilterIntents(filterIntents []Intent, filters map[string][]string, originalSearchTerm string, from int, size int, sortBy string, resultTypes []string, preference string, deb bool) (map[string][]FilteredSearchResult, error) {
	resultsByLang := map[string][]FilteredSearchResult{}
	for _, intent := range filterIntents {
		if intentValue, ok := intent.Value.(GrammarIntent); ok {
			var contentType string
			var text string
			var programCollection string
			sources := []string{}
			for _, fv := range intentValue.FilterValues {
				if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_CONTENT_TYPE] {
					contentType = fv.Value
				} else if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_TEXT] {
					text = fv.Value
				} else if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_SOURCE] {
					sources = append(sources, fv.Value)
				} else if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_PROGRAM] {
					programCollection = fv.Value
				}
			}
			searchWithoutTerm := text == ""
			if contentType != "" || programCollection != "" || len(sources) > 0 {
				log.Infof("Filtered Search Request: ContentType is '%s', Text is '%s', Program collection is '%s', Sources are '%+v'.", contentType, text, programCollection, sources)
				requests := []*elastic.SearchRequest{}
				textValSearchRequests, err := NewFilteredResultsSearchRequest(text, filters, contentType, programCollection, sources, from, size, sortBy, resultTypes, intent.Language, preference, deb)
				if err != nil {
					return nil, err
				}
				requests = append(requests, textValSearchRequests...)
				if !searchWithoutTerm && contentType != consts.VAR_CT_ARTICLES {
					fullTermSearchRequests, err := NewFilteredResultsSearchRequest(originalSearchTerm, filters, contentType, programCollection, sources, from, size, sortBy, resultTypes, intent.Language, preference, deb)
					if err != nil {
						return nil, err
					}
					requests = append(requests, fullTermSearchRequests...)
				}
				if len(requests) > 0 {
					// All search requests here are for the same language
					var scoreIncrement *float64
					if searchWithoutTerm {
						incr := consts.SCORE_INCREMENT_FOR_SEARCH_WITHOUT_TERM_RESULTS
						scoreIncrement = &incr
					}
					results, hitIdsMap, maxScore, err := e.filterSearch(requests, scoreIncrement) // TBD do it inside goroutene
					if err != nil {
						return nil, err
					}
					resultByLang := FilteredSearchResult{
						Results:                  results,
						Term:                     text,
						PreserveTermForHighlight: programCollection != "",
						HitIdsMap:                hitIdsMap,
						MaxScore:                 maxScore,
					}
					if programCollection != "" {
						resultByLang.ProgramCollection = &programCollection
					}
					resultsByLang[intent.Language] = append(resultsByLang[intent.Language], resultByLang)
				}
			}
		} else {
			return nil, errors.Errorf("FilterSearch error. Intent is not GrammarIntent. Intent: %+v", intent)
		}
	}
	return resultsByLang, nil
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
			var origin string
			if len(e.variables[name][language][value]) > 0 {
				//  we store 'origin' only for variables with a finite values list
				origin = e.variables[name][language][value][0]
			}
			ret = append(ret, FilterValue{
				Name:       filterName,
				Value:      value,
				Origin:     origin,
				OriginFull: origin,
			})
		}
	}
	return ret
}

// For specific landing page, keep only some amount of intents with the highest score (according to MAX_MATCHES_PER_GRAMMAR_INTENT) and filter out all the rest.
// Return the minimum score of the intents slice for the given intent landing page.
func updateIntentCount(intentsCount map[string][]Intent, intent Intent) float64 {
	var minScore float64
	intents := intentsCount[intent.Value.(GrammarIntent).LandingPage]
	if len(intents) > 0 {
		lastElem := intents[len(intents)-1]
		minScore = lastElem.Value.(GrammarIntent).Score
	}
	if intent.Value.(GrammarIntent).SingleHitMdbUid != nil {
		for _, i := range intents {
			if i.Value.(GrammarIntent).SingleHitMdbUid != nil &&
				*i.Value.(GrammarIntent).SingleHitMdbUid == *intent.Value.(GrammarIntent).SingleHitMdbUid {
				// Ignore duplicate collection hits
				return minScore
			}
		}
	}
	intents = append(intents, intent)
	sort.SliceStable(intents, func(i, j int) bool {
		return intents[i].Value.(GrammarIntent).Score > intents[j].Value.(GrammarIntent).Score
	})
	intents = intents[:utils.MinInt(consts.MAX_MATCHES_PER_GRAMMAR_INTENT, len(intents))]
	if len(intents) > 0 {
		lastElem := intents[len(intents)-1]
		minScore = lastElem.Value.(GrammarIntent).Score
	}
	intentsCount[intent.Value.(GrammarIntent).LandingPage] = intents
	return minScore
}

// Return values: singleHitIntents, filterIntents, error
func (e *ESEngine) searchResultsToIntents(query *Query, language string, result *elastic.SearchResult) ([]Intent, []Intent, error) {
	// log.Infof("Total Hits: %d, Max Score: %.2f", result.Hits.TotalHits, *result.Hits.MaxScore)
	defer e.timeTrack(time.Now(), consts.LAT_DOSEARCH_GRAMMARS_RESULTSTOINTENTS)
	filterIntents := []Intent(nil)
	singleHitIntents := []Intent(nil)
	intentsCount := make(map[string][]Intent)
	minScoreByLandingPage := make(map[string]float64)
	queryTermIsNumber, queryTermHasDigit := utils.HasNumeric(query.Term)
	// In case our query is numeric only, we ignore intents of "program\source position without term" to avoid irrelavnt results.
	// Also we support "program with position without term" intents only if we have a numeric chapter as part of the query.
	addProgramPositionWithoutTerm := !queryTermIsNumber && queryTermHasDigit
	addSourcePositionWithoutTerm := !queryTermIsNumber
	if addSourcePositionWithoutTerm {
		for filterKey := range query.Filters {
			if _, ok := consts.AUTO_INTENTS_BY_SOURCE_NAME_SUPPORTED_FILTERS[filterKey]; !ok {
				addSourcePositionWithoutTerm = false
				break
			}
		}
	}
	for _, hit := range result.Hits.Hits {
		var ruleObj GrammarRuleWithPercolatorQuery
		if err := json.Unmarshal(*hit.Source, &ruleObj); err != nil {
			return nil, nil, err
		}
		rule := ruleObj.GrammarRule
		// log.Infof("Score: %.2f, Index: %s, Type: %s, Id: %s, Source: %+v", *hit.Score, hit.Index, hit.Type, hit.Id, rule)
		if len(rule.Values) != len(rule.Variables) {
			return nil, nil, errors.New(fmt.Sprintf("Expected Variables to be of size %d, but it is %d", len(rule.Values), len(rule.Variables)))
		}

		// Check filters match, i.e., existing query filter match at least one supported grammar intent filter.
		if len(query.Filters) > 0 {
			filters, filterExist := consts.GRAMMAR_INTENTS_TO_FILTER_VALUES[rule.Intent]
			if !filterExist {
				return nil, nil, errors.New(fmt.Sprintf("Filters not found for intent: [%s]", rule.Intent))
			}
			common := hasCommonFilter(query.Filters, filters)
			if !common {
				// No matching filter found, should not trigger intent.
				log.Infof("No common filters for intent [%s]: %+v vs %+v", rule.Intent, query.Filters, filters)
				continue
			}
		}

		vMap := make(map[string][]string)
		for i := range rule.Variables {
			if rule.Variables[i] == consts.VAR_TEXT {
				if hit.Highlight != nil {
					if text, ok := hit.Highlight["search_text"]; ok {
						log.Infof("search_text: %s", text)
						if len(text) == 1 && text[0] != "" {
							textVarValues := retrieveTextVarValues(text[0])
							vMap[rule.Variables[i]] = textVarValues
							log.Infof("$Text values are %+v", textVarValues)
						}
					}
				}
			} else {
				vMap[rule.Variables[i]] = []string{rule.Values[i]}
			}
		}

		if GrammarVariablesMatch(rule.Intent, vMap, e.cache) {
			score := *hit.Score * (float64(4) / float64(4+len(vMap))) * YearScorePenalty(vMap)
			// Issue with tf/idf. For query [congress] the score if very low. For [arava] ok.
			// Fix this by moving the grammar index into the common index. So tha similar tf/idf will be used.
			// For now solve by normalizing very small scores.
			// log.Infof("Intent: %+v score: %.2f %.2f %.2f", vMap, *hit.Score, (float64(4) / float64(4+len(vMap))), score)
			if rule.Intent == consts.GRAMMAR_INTENT_FILTER_BY_CONTENT_TYPE {
				ctBoost := consts.CONTENT_TYPE_INTENTS_BOOST // Since now we handle multiple filter intents, consider to remove this boost
				if queryTermHasDigit {
					// Disable 'by content type' priorty boost if the query contains a number
					ctBoost = 1
				}
				filterIntents = append(filterIntents, Intent{
					Type:     consts.GRAMMAR_TYPE_FILTER,
					Language: language,
					Value: GrammarIntent{
						FilterValues: e.VariableMapToFilterValues(vMap, language),
						Score:        score * ctBoost,
						Explanation:  hit.Explanation,
					}})
			} else if rule.Intent == consts.GRAMMAR_INTENT_FILTER_BY_PROGRAM {
				filterIntents = append(filterIntents, Intent{
					Type:     consts.GRAMMAR_TYPE_FILTER,
					Language: language,
					Value: GrammarIntent{
						FilterValues: e.VariableMapToFilterValues(vMap, language),
						Score:        score,
						Explanation:  hit.Explanation,
					}})
			} else if rule.Intent == consts.GRAMMAR_INTENT_PROGRAM_POSITION_WITHOUT_TERM {
				if !addProgramPositionWithoutTerm {
					continue
				}
				log.Infof("GRAMMAR_INTENT_PROGRAM_POSITION_WITHOUT_TERM %+v", rule)
				filterValues := e.VariableMapToFilterValues(vMap, language)
				var programCollection string
				var position string
				for _, fv := range filterValues {
					if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_PROGRAM] {
						programCollection = fv.Value
					}
					if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_POSITION] {
						position = fv.Value
					}
					if programCollection != "" && position != "" {
						break
					}
				}
				programUid := e.cache.SearchStats().GetProgramByCollectionAndPosition(programCollection, position)
				if programUid == nil {
					return nil, nil, errors.New(fmt.Sprintf("Relevant program content unit is not found by collection '%v' and position '%v'.", programCollection, position))
				}
				singleHit, err := e.contentUnitHitFromSql(*programUid, language)
				if err != nil {
					return nil, nil, err
				}
				var expl elastic.SearchExplanation
				if hit.Explanation != nil {
					expl = *hit.Explanation
				}
				singleProgramIntent := GrammarIntent{
					Score:           score,
					Explanation:     &expl,
					SingleHit:       singleHit,
					SingleHitMdbUid: programUid,
				}
				singleHitIntents = append(singleHitIntents, Intent{"", language, singleProgramIntent})
				addProgramPositionWithoutTerm = false // We add results only one time for this rule type
			} else if rule.Intent == consts.GRAMMAR_INTENT_FILTER_BY_PROGRAM_WITHOUT_TERM {
				filterIntents = append(filterIntents, Intent{
					Type:     consts.GRAMMAR_TYPE_FILTER_WITHOUT_TERM,
					Language: language,
					Value: GrammarIntent{
						FilterValues: e.VariableMapToFilterValues(vMap, language),
						Score:        score,
						Explanation:  hit.Explanation,
					}})
			} else if rule.Intent == consts.GRAMMAR_INTENT_FILTER_BY_SOURCE {
				filterIntents = append(filterIntents, Intent{
					Type:     consts.GRAMMAR_TYPE_FILTER,
					Language: language,
					Value: GrammarIntent{
						FilterValues: e.VariableMapToFilterValues(vMap, language),
						Score:        score,
						Explanation:  hit.Explanation,
					}})
			} else if rule.Intent == consts.GRAMMAR_INTENT_SOURCE_POSITION_WITHOUT_TERM {
				if !addSourcePositionWithoutTerm {
					continue
				}
				log.Infof("GRAMMAR_INTENT_SOURCE_POSITION_WITHOUT_TERM %+v", rule)
				filterValues := e.VariableMapToFilterValues(vMap, language)
				var source string
				var position string
				var divType string
				for _, fv := range filterValues {
					if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_SOURCE] {
						source = fv.Value
					}
					if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_POSITION] {
						position = fv.Value
					}
					if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_DIVISION_TYPE] {
						divType = fv.Value
					}
					if source != "" && position != "" && divType != "" {
						break
					}
				}
				var divTypes []int64
				if divType != "" {
					if val, ok := consts.ES_GRAMMAR_DIVT_TYPE_TO_SOURCE_TYPES[divType]; ok {
						divTypes = val
					}
				}
				relevantSource := e.cache.SearchStats().GetSourceByPositionAndParent(source, position, divTypes)
				if relevantSource == nil {
					return nil, nil, errors.New(fmt.Sprintf("Relevant source is not found by source parent '%v' and position '%v'.", source, position))
				}
				var leafPrefixType *consts.PositionIndexType
				if val, ok := consts.ES_SRC_PARENTS_FOR_CHAPTER_POSITION_INDEX[source]; ok {
					leafPrefixType = &val
				}
				path, err := e.sourcePathFromSql(*relevantSource, language, &position, leafPrefixType)
				if err != nil {
					return nil, nil, err
				}
				var expl elastic.SearchExplanation
				if hit.Explanation != nil {
					expl = *hit.Explanation
				}
				intents, err := e.getSingleHitIntentsBySource(*relevantSource, query.Filters, language, path, *hit.Score, expl)
				if err != nil {
					return nil, nil, err
				}
				singleHitIntents = append(singleHitIntents, intents...)
				addSourcePositionWithoutTerm = false // We add results only one time for this rule type
			} else {
				if intentsByLandingPage, ok := intentsCount[rule.Intent]; ok && len(intentsByLandingPage) >= consts.MAX_MATCHES_PER_GRAMMAR_INTENT {
					if score <= minScoreByLandingPage[rule.Intent] {
						// Initial filtering (before updateIntentCount func.) to avoid the SQL call for converting LP to collection.
						continue
					}
				}
				intentValue := GrammarIntent{
					LandingPage:  rule.Intent,
					FilterValues: e.VariableMapToFilterValues(vMap, language),
					Score:        score,
					Explanation:  hit.Explanation,
				}
				if intentValue.LandingPage == consts.GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS && intentValue.FilterValues != nil {
					var year string
					var location string
					for _, fv := range intentValue.FilterValues {
						if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_CONVENTION_LOCATION] {
							location = fv.Value

						} else if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_YEAR] {
							year = fv.Value
						}
						if year != "" && location != "" {
							break
						}
					}
					if e.cache.SearchStats().DoesConventionSingle(location, year) {
						// Since the LandingPage has only one collection item, convert the LandingPage result to the single collection hit
						log.Infof("Converting LandingPage of %s %s to a single collection.", location, year)
						var err error
						collectionHit, mdbUid, err := e.conventionsLandingPageToCollectionHit(year, location)
						if err != nil {
							log.Warnf("%+v", err)
							return nil, nil, errors.New(fmt.Sprintf("ConventionsLandingPageToCollectionHit Failed: %+v", err))
						}
						intentValue.SingleHit = collectionHit
						intentValue.SingleHitMdbUid = mdbUid
					}
				}
				if intentValue.LandingPage == consts.GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS && intentValue.FilterValues != nil {
					var year string
					var holiday string
					for _, fv := range intentValue.FilterValues {
						if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_HOLIDAYS] {
							holiday = fv.Value

						} else if fv.Name == consts.VARIABLE_TO_FILTER[consts.VAR_YEAR] {
							year = fv.Value
						}
						if year != "" && holiday != "" {
							break
						}
					}
					if e.cache.SearchStats().DoesHolidaySingle(holiday, year) {
						// Since the LandingPage has only one collection item, convert the LandingPage result to the single collection hit
						log.Infof("Converting LandingPage of %s %s to a single collection.", holiday, year)
						var err error
						collectionHit, mdbUid, err := e.holidaysLandingPageToCollectionHit(year, holiday)
						if err != nil {
							log.Warnf("%+v", err)
							return nil, nil, errors.New(fmt.Sprintf("HolidaysLandingPageToCollectionHit Failed: %+v", err))
						}
						intentValue.SingleHit = collectionHit
						intentValue.SingleHitMdbUid = mdbUid
					}
				}
				intent := Intent{
					Type:     consts.GRAMMAR_TYPE_LANDING_PAGE,
					Language: language,
					Value:    intentValue,
				}
				minScoreByLandingPage[intent.Value.(GrammarIntent).LandingPage] = updateIntentCount(intentsCount, intent)
			}
		}
	}
	for _, intentsByLandingPage := range intentsCount {
		singleHitIntents = append(singleHitIntents, intentsByLandingPage...)
	}

	// Normalize score to be from 2000 and below.
	maxScoreForLandingPages := 0.0
	for i := range singleHitIntents {
		if intentValue, ok := singleHitIntents[i].Value.(GrammarIntent); ok {
			if intentValue.Score > maxScoreForLandingPages {
				maxScoreForLandingPages = intentValue.Score
			}
		}
	}
	normalizedSingleHitIntents := []Intent(nil)
	for _, intent := range singleHitIntents {
		if intentValue, ok := intent.Value.(GrammarIntent); ok {
			intentValue.Score = 3000 * (intentValue.Score / maxScoreForLandingPages)
			intent.Value = intentValue
		}
		normalizedSingleHitIntents = append(normalizedSingleHitIntents, intent)
	}
	//log.Infof("Single Hit Intents: %+v", normalizedSingleHitIntents)
	//log.Infof("Filter Intentys: %+v", filterIntents)
	return normalizedSingleHitIntents, filterIntents, nil
}

func (e *ESEngine) conventionsLandingPageToCollectionHit(year string, location string) (*elastic.SearchHit, *string, error) {
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

func (e *ESEngine) holidaysLandingPageToCollectionHit(year string, holiday string) (*elastic.SearchHit, *string, error) {
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

func (e *ESEngine) collectionHitFromSql(query string) (*elastic.SearchHit, *string, error) {
	var properties json.RawMessage
	var mdbUID string
	var effectiveDate es.EffectiveDate

	err := e.mdb.QueryRow(query).Scan(&mdbUID, &properties)
	if err != nil {
		return nil, nil, err
	}

	err = json.Unmarshal(properties, &effectiveDate)
	if err != nil {
		return nil, nil, err
	}

	result := es.Result{
		EffectiveDate: effectiveDate.EffectiveDate,
		MDB_UID:       mdbUID,
		ResultType:    consts.ES_RESULT_TYPE_COLLECTIONS,
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		return nil, nil, err
	}

	hit := &elastic.SearchHit{
		Source: (*json.RawMessage)(&resultJson),
		Type:   "result",
		Index:  consts.GRAMMAR_LP_SINGLE_COLLECTION,
	}
	return hit, &mdbUID, nil
}

func (e *ESEngine) contentUnitHitFromSql(uid string, language string) (*elastic.SearchHit, error) {
	var title string
	var properties json.RawMessage
	var effectiveDate es.EffectiveDate

	queryMask := `select cu.properties, cun.name
		from content_units cu join content_unit_i18n cun on cu.id = cun.content_unit_id
		where cu.uid = '%s' and cun.language = '%s'`
	query := fmt.Sprintf(queryMask, uid, language)
	err := e.mdb.QueryRow(query).Scan(&properties, &title)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(properties, &effectiveDate)
	if err != nil {
		return nil, err
	}

	result := es.Result{
		EffectiveDate: effectiveDate.EffectiveDate,
		MDB_UID:       uid,
		ResultType:    consts.ES_RESULT_TYPE_UNITS,
		Title:         title,
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	hit := &elastic.SearchHit{
		Source: (*json.RawMessage)(&resultJson),
		Type:   "result",
		Index:  consts.GRAMMAR_GENERATED_CU_HIT,
	}
	return hit, nil
}

func (e *ESEngine) sourcePathFromSql(sourceUid string, language string, position *string, leafPrefixType *consts.PositionIndexType) (string, error) {
	queryMask := `with recursive sourcesPath as (
		select s.id, s.uid, s.parent_id, sn.name from source_i18n sn
		  join  sources s on sn.source_id = s.id
		 and s.uid='%s'
			  where sn.language='%s'
	  
		union all
	  
		select  s.id, s.uid, s.parent_id, sn.name  from source_i18n sn
		  join  sources s on sn.source_id = s.id
		join sourcesPath on sourcesPath.parent_id = s.id
			  where sn.language='%s'
	  )
	  
	  select name from sourcesPath sp
	  union all
	  select aun.name as id from author_i18n aun
	  join authors_sources aus on aus.author_id = aun.author_id
	  join sourcesPath sp on sp.id = aus.source_id
	  where aun.language='%s'`
	query := fmt.Sprintf(queryMask, sourceUid, language, language, language)
	rows, err := e.mdb.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			return "", err
		}
		names = append([]string{name}, names...)
	}
	err = rows.Err()
	if err != nil {
		return "", err
	}
	if len(names) == 0 && language != consts.LANG_HEBREW {
		// Try with default language
		return e.sourcePathFromSql(sourceUid, consts.LANG_HEBREW, position, leafPrefixType)
	}
	if leafPrefixType != nil && position != nil && len(names) > 0 {
		if language == consts.LANG_HEBREW && *leafPrefixType == consts.LETTER_IF_HEBREW {
			posInt, err := strconv.Atoi(*position)
			if err != nil {
				return "", err
			}
			hebLetter := utils.NumberInHebrew(posInt) //  Convert to Hebrew letter
			position = &hebLetter
		}
		names[len(names)-1] = fmt.Sprintf("%s. %s", *position, names[len(names)-1])
	}
	ret := strings.Join(names, " > ")
	return ret, nil
}

func (e *ESEngine) getSingleHitIntentsBySource(source string, filters map[string][]string, language string, title string, score float64, explanation elastic.SearchExplanation) ([]Intent, error) {
	var getLessonCI bool
	var getProgramCI bool
	var getSourceGI bool
	if len(filters) == 0 {
		getLessonCI = true
		getProgramCI = true
		getSourceGI = true
	} else {
		if values, ok := filters[consts.FILTERS[consts.FILTER_UNITS_CONTENT_TYPES]]; ok {
			for _, value := range values {
				if value == consts.CT_LESSON_PART {
					getLessonCI = true
				}
				if value == consts.CT_VIDEO_PROGRAM_CHAPTER {
					getProgramCI = true
				}
			}
		}
		if _, ok := filters[consts.FILTERS[consts.FILTER_SECTION_SOURCES]]; ok {
			getSourceGI = true
		}
	}
	log.Infof("getSingleHitIntentsBySource filters: %+v. getLessonCI = %v. getProgramCI = %v. getSourceGI = %v.", filters, getLessonCI, getProgramCI, getSourceGI)
	ciScore := score * 0.98 // lower classification intents score to display the source above them
	intents := []Intent{}
	if getLessonCI {
		lessonsIntent := ClassificationIntent{
			ResultType:  consts.ES_RESULT_TYPE_SOURCES,
			MDB_UID:     source,
			ContentType: consts.CT_LESSON_PART,
			Exist:       e.cache.SearchStats().IsSourceWithEnoughUnits(source, consts.INTENTS_MIN_UNITS, consts.CT_LESSON_PART),
			Score:       &ciScore,
			Explanation: explanation,
			Title:       title, // Actually this value is generated in client for classification intent results.
		}
		intents = append(intents, Intent{consts.INTENT_TYPE_SOURCE, language, lessonsIntent})
	}
	if getProgramCI {
		programsIntent := ClassificationIntent{
			ResultType:  consts.ES_RESULT_TYPE_SOURCES,
			MDB_UID:     source,
			ContentType: consts.CT_VIDEO_PROGRAM_CHAPTER,
			Exist:       e.cache.SearchStats().IsSourceWithEnoughUnits(source, consts.INTENTS_MIN_UNITS, consts.CT_VIDEO_PROGRAM_CHAPTER),
			Score:       &ciScore,
			Explanation: explanation,
			Title:       title, // Actually this value is generated in client for classification intent results.
		}
		intents = append(intents, Intent{consts.INTENT_TYPE_SOURCE, language, programsIntent})
	}
	if getSourceGI {
		srcResult := es.Result{
			MDB_UID:    source,
			ResultType: consts.ES_RESULT_TYPE_SOURCES,
			FullTitle:  title,
		}
		srcResultJson, err := json.Marshal(srcResult)
		if err != nil {
			return []Intent{}, err
		}
		singleSourceIntent := GrammarIntent{
			Score:       score,
			Explanation: &explanation,
			SingleHit: &elastic.SearchHit{
				Source: (*json.RawMessage)(&srcResultJson),
				Type:   "result",
				Index:  consts.GRAMMAR_GENERATED_SOURCE_HIT,
			},
		}
		intents = append(intents, Intent{"", language, singleSourceIntent})
	}
	return intents, nil
}

// This function retrieves the 'free text' values from a grammar result that was searched by perculator query with highlight.
// The 'highlighted' part of the input string contains the values that are NOT 'free text'. This parts starts and ends with PERCULATE_HIGHLIGHT_SEPERATOR rune ('$').
// The return value of the function is a slice of all term parts thar are outside of the 'highlight'.
// For example, the 'free text' values for the term 'aaa $bbb$ ccc $ddd' are 'aaa' and 'ccc'.
// We have a test for this function in engine_test.go
func retrieveTextVarValues(str string) []string {
	runes := []rune(str)
	var filtered []rune
	var textVarValues []string
	var inHighlight bool
	for i, r := range runes {
		if r == PERCULATE_HIGHLIGHT_SEPERATOR || i == len(runes)-1 {
			inHighlight = !inHighlight
			if inHighlight && len(filtered) > 0 {
				if r != PERCULATE_HIGHLIGHT_SEPERATOR {
					filtered = append(filtered, r)
				}
				trimmed := strings.Trim(string(filtered), " ")
				if trimmed != "" {
					textVarValues = append(textVarValues, trimmed)
				}
			}
			filtered = make([]rune, 0)
		} else if !inHighlight {
			filtered = append(filtered, r)
		}
	}
	return textVarValues
}

// Results search according to grammar based filter.
// Return: Results, Unique list of hit id's as a map, Max score
func (e *ESEngine) filterSearch(requests []*elastic.SearchRequest, scoreIncrement *float64) ([]*elastic.SearchResult, map[string]bool, *float64, error) {
	results := []*elastic.SearchResult{}
	hitIdsMap := map[string]bool{}
	var maxScore *float64

	multiSearchFilteredService := e.esc.MultiSearch()
	multiSearchFilteredService.Add(requests...)
	beforeFilterSearch := time.Now()
	mr, err := multiSearchFilteredService.Do(context.TODO())
	e.timeTrack(beforeFilterSearch, consts.LAT_DOSEARCH_GRAMMARS_MULTISEARCHGRAMMARSDO) // TBC differentiate calls to filterSearch under single request

	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Error looking for grammar based filter search.")
	}

	for _, currentResults := range mr.Responses {
		if currentResults.Error != nil {
			log.Warnf("%+v", currentResults.Error)
			return nil, nil, nil, errors.New(fmt.Sprintf("Failed multi get in grammar based filter search: %+v", currentResults.Error))
		}
		if haveHits(currentResults) {
			var currentMaxScore *float64
			for _, hit := range currentResults.Hits.Hits {
				hitIdsMap[hit.Id] = true
				if hit.Score != nil {
					if scoreIncrement != nil {
						*hit.Score += *scoreIncrement
					}
					if currentMaxScore == nil || *hit.Score > *currentMaxScore {
						currentMaxScore = hit.Score
					}
				}
			}
			currentResults.Hits.MaxScore = currentMaxScore
			if currentResults.Hits.MaxScore != nil &&
				(maxScore == nil || *currentResults.Hits.MaxScore > *maxScore) {
				maxScore = currentResults.Hits.MaxScore
			}
			results = append(results, currentResults)
		}
	}
	return results, hitIdsMap, maxScore, nil
}

// Return: source uid, language, is author?
func (e *ESEngine) sourceUidByTerm(term string, languages []string) (*string, string, bool) {
	termWithoutQuotes := strings.Replace(term, "\"", "", -1)
	termLC := strings.ToLower(term)
	for _, language := range languages {
		varsByLang := e.variables[consts.VAR_SOURCE][language]
		for k, v := range varsByLang {
			for _, srcName := range v {
				var identical bool
				if language == consts.LANG_HEBREW {
					srcNameWithoutQuotes := strings.Replace(srcName, "\"", "", -1)
					identical = termWithoutQuotes == srcNameWithoutQuotes
				} else {
					identical = termLC == strings.ToLower(srcName)
				}
				if identical {
					return &k, language, len(k) < 4
				}
			}
		}
	}
	return nil, "", false
}

func (e *ESEngine) isTermRestricted(term string, languages []string) bool {
	// Here we are checking if the given term exists in the list of terms that are restricted from being processed in the grammar engine.
	// It might be better to tokenize this list of terms and make the check using Elastic. However this could make the check less accurate and restrict terms that should not be restricted.
	// The option of tokenization was not tested and we should consider testing it.
	termLC := strings.ToLower(term)
	for _, language := range languages {
		varsByLang := e.variables[consts.VAR_RESTRICTED][language]
		for _, v := range varsByLang {
			for _, resTerm := range v {
				if termLC == strings.ToLower(resTerm) {
					return true
				}
			}
		}
	}
	return false
}

func (e *ESEngine) selectFilterIntents(intents []Intent) ([]Intent, error) {
	var selected []Intent
	// Intent with max score of filter intents with search term.
	var maxWithTerm *Intent
	// Intent with max score of filter intents without search term.
	var maxWithoutTerm *Intent
	grammarIntents, nonGrammarIntents := utils.Filter(utils.Is(intents), func(v interface{}) bool {
		_, ok := v.(Intent).Value.(GrammarIntent)
		return ok
	})
	if len(nonGrammarIntents) > 0 {
		return nil, errors.New("Non grammar intents sent to selectFilterIntent.")
	}
	// Print optional intents for debug info
	log.Info("Optional Intents:")
	for i, intentIs := range grammarIntents {
		intent := intentIs.(Intent)
		log.Infof("#%d\nType: '%s',\nScore:%v,\nFilterValues: [%+v].", i+1, intent.Type, intent.Value.(GrammarIntent).Score, intent.Value.(GrammarIntent).FilterValues)
	}
	intentsWithoutTerm, intentsWithTerm := utils.Filter(grammarIntents, func(v interface{}) bool {
		return v.(Intent).Type == consts.GRAMMAR_TYPE_FILTER_WITHOUT_TERM
	})
	mapByProgram := utils.GroupBy(intentsWithoutTerm, func(v interface{}) interface{} {
		uid := getFilterValue(v.(Intent).Value.(GrammarIntent).FilterValues, consts.VARIABLE_TO_FILTER[consts.VAR_PROGRAM])
		if uid == nil {
			return "no_program"
		}
		return *uid
	})
	if len(mapByProgram) > 1 {
		log.Infof("More than one program collection found in intents without term. Ignoring intents without term.")
	} else {
		maxWithoutTermIs := utils.MaxByValue(intentsWithoutTerm, func(v interface{}) float64 {
			return v.(Intent).Value.(GrammarIntent).Score
		})
		if maxWithoutTermIs != nil {
			maxWithoutTermT := interface{}(maxWithoutTermIs).(Intent)
			maxWithoutTerm = &maxWithoutTermT
		}
	}
	maxWithTermIs := utils.MaxByValue(intentsWithTerm, func(v interface{}) float64 {
		return v.(Intent).Value.(GrammarIntent).Score
	})
	if maxWithTermIs != nil {
		maxWithTermT := interface{}(maxWithTermIs).(Intent)
		maxWithTerm = &maxWithTermT
	}
	if maxWithTerm != nil || maxWithoutTerm != nil {
		if maxWithTerm == nil {
			selected = []Intent{*maxWithoutTerm}
		} else if maxWithoutTerm == nil {
			selected = interfaceSliceToIntentSlice(intentsWithTerm)
		} else {
			intentWithTermCT := getFilterValue(maxWithTerm.Value.(GrammarIntent).FilterValues, consts.VARIABLE_TO_FILTER[consts.VAR_CONTENT_TYPE])
			intentWithoutTermCT := getFilterValue(maxWithoutTerm.Value.(GrammarIntent).FilterValues, consts.VARIABLE_TO_FILTER[consts.VAR_CONTENT_TYPE])
			if intentWithTermCT != nil && intentWithoutTermCT != nil {
				if *intentWithTermCT == *intentWithoutTermCT {
					// If for both intent types (with term and without term) we have the same content type value,
					// we select the intent without term.
					// E.g. query "programs new life" has "by content type" intent and "by program without term" intent, we select "by program without term".
					selected = []Intent{*maxWithoutTerm}
				} else {
					selected = interfaceSliceToIntentSlice(intentsWithTerm)
				}
			} else {
				selected = interfaceSliceToIntentSlice(intentsWithTerm)
			}
		}
		log.Info("SELECTED Intents:")
		for i, intent := range selected {
			log.Infof("#%d\nType: '%s',\nScore:%v,\nFilterValues: [%+v].", i+1, intent.Type, intent.Value.(GrammarIntent).Score, intent.Value.(GrammarIntent).FilterValues)
		}
	}
	return selected, nil
}

func getFilterValue(filterValues []FilterValue, filterName string) *string {
	filter := utils.First(utils.Is(filterValues), func(v interface{}) bool {
		return v.(FilterValue).Name == filterName
	})
	if filter == nil {
		return nil
	}
	val := filter.(FilterValue).Value
	return &val
}

func interfaceSliceToIntentSlice(slice []interface{}) []Intent {
	ret := []Intent{}
	for _, e := range slice {
		ret = append(ret, e.(Intent))
	}
	return ret
}
