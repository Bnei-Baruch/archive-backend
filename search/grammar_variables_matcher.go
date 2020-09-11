package search

import (
	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
)

func GrammarFilterVariablesMatch(intent string, variablesByPhrase VariablesByPhrase, cm cache.CacheManager) {
	for phrase, vMap := range variablesByPhrase {
		if !GrammarVariablesMatch(intent, vMap, cm) {
			delete(variablesByPhrase, phrase)
		}
	}
}

func GrammarVariablesMatch(intent string, vMap map[string][]string, cm cache.CacheManager) bool {
	if intent == consts.GRAMMAR_INTENT_FILTER_BY_CONTENT_TYPE {
		hasVarText := false
		for variable, values := range vMap {
			if variable == consts.VAR_TEXT {
				if hasVarText || len(values) != 1 { //  Disable if we have more than one $Text appereance or value
					log.Warning("More than one $Text appereance or value in 'by_content' rule.")
					return false
				}
				hasVarText = true
			}
		}
		return true
	} else if intent == consts.GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS {
		location := ""
		year := ""
		for variable, values := range vMap {
			if variable == consts.VAR_YEAR {
				if len(values) != 1 {
					return false
				}
				year = values[0]
			} else if variable == consts.VAR_CONVENTION_LOCATION {
				if len(values) != 1 {
					return false
				}
				location = values[0]
			}
		}
		// Validate vMap fits convention $ConventionLocation and $Year existing values.
		// log.Infof("location: %s, year: %s => %t", location, year, cm.SearchStats().DoesConventionExist(location, year))
		// Uninitialized, usually for tests. Return false.
		if cm == nil {
			return false
		}
		return cm.SearchStats().DoesConventionExist(location, year)
	} else if intent == consts.GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS {
		year := ""
		holiday := ""
		for variable, values := range vMap {
			if variable == consts.VAR_YEAR {
				if len(values) != 1 {
					return false
				}
				year = values[0]
			} else if variable == consts.VAR_HOLIDAYS {
				if len(values) != 1 {
					return false
				}
				holiday = values[0]
			}
		}
		if cm == nil {
			return false
		}
		return cm.SearchStats().DoesHolidayExist(holiday, year)
	} else {
		return true
	}
}
