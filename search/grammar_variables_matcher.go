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
	if intent == consts.GRAMMAR_INTENT_FILTER_BY_SOURCE {
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
	} else if intent == consts.GRAMMAR_INTENT_SOURCE_POSITION_WITHOUT_TERM {
		varPosition := ""
		varSource := ""
		varDivType := ""
		for variable, values := range vMap {
			if variable == consts.VAR_POSITION {
				if varPosition != "" || len(values) != 1 { //  Disable if we have more than one $Position appereance or value
					log.Warningf("Number of $Position appearances or values in 'by_position' rule is not 1. Values: %+v", values)
					return false
				}
				varPosition = values[0]
			}
			if variable == consts.VAR_SOURCE {
				if varSource != "" || len(values) != 1 { //  Disable if we have more than one $Source appereance or value
					// TBD consider support for multiple $Source values
					log.Warningf("Number of $Source appearances or values in 'by_position' rule is not 1. Values: %+v", values)
					return false
				}
				varSource = values[0]
			}
			if variable == consts.VAR_DIVISION_TYPE {
				if varDivType != "" || len(values) != 1 { //  Disable if we have more than one $DivisionType appereance or value
					log.Warningf("Number of $DivisionType appearances or values in 'by_position' rule is not 1. Values: %+v", values)
					return false
				}
				varDivType = values[0]
			}
		}
		if varPosition == "" || varSource == "" {
			log.Warningf("Intent of source by position must have one appearance of $Position and one appearance of $Source")
			return false
		}
		var divTypes []int64
		if varDivType != "" {
			if val, ok := consts.ES_GRAMMAR_DIVT_TYPE_TO_SOURCE_TYPES[varDivType]; ok {
				divTypes = val
			}
		}
		// If divTypes is not assigned, GetSourceByPositionAndParent will check all types
		src := cm.SearchStats().GetSourceByPositionAndParent(varSource, varPosition, divTypes)
		return src != nil
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
