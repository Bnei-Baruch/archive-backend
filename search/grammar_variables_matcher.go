package search

import (
	"strconv"

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
	switch intent {
	case consts.GRAMMAR_INTENT_FILTER_BY_PROGRAM:
		return filterByProgramMatch(vMap)
	case consts.GRAMMAR_INTENT_PROGRAM_POSITION_WITHOUT_TERM:
		return programPositionWithoutTermMatch(vMap, cm)
	case consts.GRAMMAR_INTENT_FILTER_BY_PROGRAM_WITHOUT_TERM:
		return filterByProgramWithoutTermMatch(vMap)
	case consts.GRAMMAR_INTENT_FILTER_BY_SOURCE:
		return filterBySourceMatch(vMap)
	case consts.GRAMMAR_INTENT_SOURCE_POSITION_WITHOUT_TERM:
		return sourcePositionWithoutTermMatch(vMap, cm)
	case consts.GRAMMAR_INTENT_FILTER_BY_CONTENT_TYPE:
		return filterByContentTypeMatch(vMap)
	case consts.GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS:
		return landingPageConventionsMatch(vMap, cm)
	case consts.GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS:
		return landingPageHolidaysMatch(vMap, cm)
	default:
		return true
	}
}

func filterByProgramMatch(vMap map[string][]string) bool {
	hasVarText := false
	hasVarProgram := false
	hasVarContentType := false
	for variable, values := range vMap {
		if variable == consts.VAR_TEXT {
			if hasVarText || len(values) != 1 { //  Disable if we have more than one $Text appereance or value
				log.Warningf("Number of $Text appearances or values in 'by_program' rule is not 1. Values: %+v", values)
				return false
			}
			if _, err := strconv.Atoi(values[0]); err == nil {
				log.Warningf("$Text (%v) is numeric in 'by_program' rule. Should not trigger.", values[0])
				return false
			}
			hasVarText = true
		}
		if variable == consts.VAR_PROGRAM {
			if hasVarProgram || len(values) != 1 { //  Disable if we have more than one $Program appereance or value
				log.Warningf("Number of $Program appearances or values in 'by_program' rule is not 1. Values: %+v", values)
				return false
			}
			hasVarProgram = true
		}
		if variable == consts.VAR_CONTENT_TYPE {
			if hasVarContentType || len(values) != 1 { //  Disable if we have more than one $ContentType appereance or value
				log.Warningf("Number of $ContentType appearances or values in 'by_program' rule is not 1. Values: %+v", values)
				return false
			}
			if values[0] != consts.VAR_CT_PROGRAMS {
				log.Warningf("$ContentType value in 'by_program' rule should be 'programs'. We have: %v.", values[0])
				return false
			}
			hasVarContentType = true
		}
	}
	if !(hasVarProgram && hasVarText) {
		log.Warningf("Filter intent by program must have one appearance of $Text and one appearance of $Program")
		return false
	}
	return true
}

func programPositionWithoutTermMatch(vMap map[string][]string, cm cache.CacheManager) bool {
	hasContentType := false
	varPosition := ""
	varProgramCollection := ""
	varDivType := ""
	for variable, values := range vMap {
		if variable == consts.VAR_CONTENT_TYPE {
			if hasContentType || len(values) != 1 { //  Disable if we have more than one $ContentType appereance or value
				log.Warningf("Number of $ContentType appearances or values in 'by_position' rule is not 1. Values: %+v", values)
				return false
			}
			if values[0] != consts.VAR_CT_PROGRAMS {
				return false
			}
			hasContentType = true
		}
		if variable == consts.VAR_POSITION {
			if varPosition != "" || len(values) != 1 { //  Disable if we have more than one $Position appereance or value
				log.Warningf("Number of $Position appearances or values in 'by_position' rule is not 1. Values: %+v", values)
				return false
			}
			varPosition = values[0]
		}
		if variable == consts.VAR_PROGRAM {
			if varProgramCollection != "" || len(values) != 1 { //  Disable if we have more than one $Program appereance or value
				log.Warningf("Number of $Program appearances or values in 'by_position' rule is not 1. Values: %+v", values)
				return false
			}
			varProgramCollection = values[0]
		}
		if variable == consts.VAR_DIVISION_TYPE {
			if varDivType != "" || len(values) != 1 { //  Disable if we have more than one $DivisionType appereance or value
				log.Warningf("Number of $DivisionType appearances or values in 'by_position' rule is not 1. Values: %+v", values)
				return false
			}
			varDivType = values[0]
		}
	}
	if varPosition == "" {
		log.Warningf("Intent of program by position must have one appearance of $Position")
		return false
	}
	if varDivType != "" {
		if val, ok := consts.ES_GRAMMAR_PROGRAM_SUPPORTED_DIV_TYPES[varDivType]; !ok || !val {
			return false
		}
	}
	if _, err := strconv.Atoi(varPosition); err != nil {
		// Letter as position is not supported for programs, only for sources.
		return false
	}
	if varProgramCollection == "" {
		varProgramCollection = consts.PROGRAM_COLLECTION_NEW_LIFE
	}
	c := cm.SearchStats().GetProgramByCollectionAndPosition(varProgramCollection, varPosition)
	return c != nil
}

func filterByProgramWithoutTermMatch(vMap map[string][]string) bool {
	hasVarProgram := false
	hasVarContentType := false
	hasVarPosition := false
	for variable, values := range vMap {
		if variable == consts.VAR_PROGRAM {
			if hasVarProgram || len(values) != 1 { //  Disable if we have more than one $Program appereance or value
				log.Warningf("Number of $Program appearances or values in 'by_program_without_term' rule is not 1. Values: %+v", values)
				return false
			}
			hasVarProgram = true
		}
		if variable == consts.VAR_POSITION {
			if hasVarPosition || len(values) != 1 { //  Disable if we have more than one $Position appereance or value
				log.Warningf("Number of $Position appearances or values in 'by_program_without_term' rule is not 1. Values: %+v", values)
				return false
			}
			hasVarPosition = true
		}
		if variable == consts.VAR_CONTENT_TYPE {
			if hasVarContentType || len(values) != 1 { //  Disable if we have more than one $ContentType appereance or value
				log.Warningf("Number of $ContentType appearances or values in 'by_program_without_term' rule is not 1. Values: %+v", values)
				return false
			}
			if values[0] != consts.VAR_CT_PROGRAMS {
				log.Warningf("$ContentType value in 'by_program_without_term' rule should be 'programs'. We have: %v.", values[0])
				return false
			}
			hasVarContentType = true
		}
	}
	if !hasVarProgram {
		log.Warningf("Filter intent 'by program without term' must have one appearance of $Program")
		return false
	}
	return true
}

func filterBySourceMatch(vMap map[string][]string) bool {
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
}

func sourcePositionWithoutTermMatch(vMap map[string][]string, cm cache.CacheManager) bool {
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
}

func filterByContentTypeMatch(vMap map[string][]string) bool {
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
}

func landingPageConventionsMatch(vMap map[string][]string, cm cache.CacheManager) bool {
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
}

func landingPageHolidaysMatch(vMap map[string][]string, cm cache.CacheManager) bool {
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
}
