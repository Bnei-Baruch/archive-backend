package search

import (
	// log "github.com/Sirupsen/logrus"

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
	if intent == consts.GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS {
		location := ""
		year := ""
		for variable, values := range vMap {
			if variable == "$Year" {
				if len(values) != 1 {
					return false
				}
				year = values[0]
			} else if variable == "$ConventionLocation" {
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
	} else {
		return true
	}
}
