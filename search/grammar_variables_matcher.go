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
	if intent == consts.GRAMMAR_INTENT_CLASSIFICATION_BY_CONTENT_TYPE_AND_SOURCE {
		var contentType string
		var source string
		for variable, values := range vMap {
			if variable == consts.VAR_CONTENT_TYPE {
				if contentType != "" || len(values) != 1 { //  Disable if we have more than one $Text appereance or value
					log.Warningf("Number of $Text appearances or values in 'by_content_type_and_source' rule is not 1. Values: %+v", values)
					return false
				}
				contentType = values[0]
			}
			if variable == consts.VAR_SOURCE {
				if source != "" || len(values) != 1 { //  Disable if we have more than one $Source appereance or value
					// TBD consider support for multiple $Source values
					log.Warningf("Number of $Source appearances or values in 'by_content_type_and_source' rule is not 1. Values: %+v", values)
					return false
				}
				source = values[0]
			}
		}
		if contentType == "" || source == "" {
			log.Warningf("Classification intent by content type and source must have one appearance of $Source and one appearance of $ContentType")
			return false
		}
		if opt, ok := consts.INTENT_OPTIONS_BY_GRAMMAR_CT_VARIABLES[contentType]; ok {
			for _, cut := range opt.ContentTypes {
				if !cm.SearchStats().IsSourceWithEnoughUnits(source, consts.INTENTS_MIN_UNITS, cut) {
					return false
				}
			}
			return true
		}
		return false
	} else if intent == consts.GRAMMAR_INTENT_FILTER_BY_SOURCE {
		hasVarText := false
		hasVarSource := false
		for variable, values := range vMap {
			if variable == consts.VAR_TEXT {
				if hasVarText || len(values) != 1 { //  Disable if we have more than one $Text appereance or value
					log.Warningf("Number of $Text appearances or values in 'by_source' rule is not 1. Values: %+v", values)
					return false
				}
				hasVarText = true
			}
			if variable == consts.VAR_SOURCE {
				if hasVarSource || len(values) != 1 { //  Disable if we have more than one $Source appereance or value
					// TBD consider support for multiple $Source values
					log.Warningf("Number of $Source appearances or values in 'by_source' rule is not 1. Values: %+v", values)
					return false
				}
				hasVarSource = true
			}
		}
		if !(hasVarText && hasVarSource) {
			log.Warningf("Filter intent by content type must have one appearance of $Text and one appearance of $Source")
			return false
		}
		return true
	} else if intent == consts.GRAMMAR_INTENT_FILTER_BY_CONTENT_TYPE {
		hasVarText := false
		hasVarContentType := false
		for variable, values := range vMap {
			if variable == consts.VAR_TEXT {
				if hasVarText || len(values) != 1 { //  Disable if we have more than one $Text appereance or value
					log.Warningf("Number of $Text appearances or values in 'by_content' rule is not 1. Values: %+v", values)
					return false
				}
				hasVarText = true
			}
			if variable == consts.VAR_CONTENT_TYPE {
				if hasVarContentType || len(values) != 1 { //  Disable if we have more than one $ContentType appereance or value
					// TBD consider support for multiple $ContentType values
					log.Warningf("Number of $ContentType appearances or values in 'by_content' rule is not 1. Values: %+v", values)
					return false
				}
				hasVarContentType = true
			}
		}
		if !(hasVarText && hasVarContentType) {
			log.Warningf("Filter intent by content type must have one appearance of $Text and one appearance of $ContentType")
			return false
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
