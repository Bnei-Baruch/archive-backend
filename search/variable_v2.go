package search

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

// Translations language => value => phrases
type TranslationsV2 = map[string]map[string][]string

// Map from variable => language => value => phrases
type VariablesV2 = map[string]TranslationsV2

const (
	START_YEAR = 1996
)

func MakeYearVariablesV2() map[string][]string {
	ret := make(map[string][]string)
	year := START_YEAR
	nowYear := time.Now().Year()
	for year <= nowYear {
		yearStr := fmt.Sprintf("%d", year)
		ret[yearStr] = []string{yearStr}
		year++
	}
	return ret
}

func YearScorePenalty(vMap map[string][]string) float64 {
	if yearStrs, ok := vMap[consts.VAR_YEAR]; ok {
		maxRet := 0.0
		for _, yearStr := range yearStrs {
			nowYear := time.Now().Year()
			if year, err := strconv.Atoi(yearStr); err != nil || year >= nowYear {
				return 1.0
			} else {
				ret := 0.3*(1-float64(nowYear-year)/float64(nowYear-START_YEAR)) + 0.7
				if ret > maxRet {
					maxRet = ret
				}
			}
		}
		return maxRet
	}
	return 1.0
}

func MakeHolidayVariables() map[string][]string {
	ret := make(map[string][]string)

	return ret
}

func MakeVariablesV2(variablesDir string) (VariablesV2, error) {
	// Loads all variables.
	variables, err := LoadVariablesTranslationsV2(variablesDir)
	if err != nil {
		return nil, err
	}

	years := MakeYearVariablesV2()
	variables[consts.VAR_YEAR] = make(TranslationsV2)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		// Year
		variables[consts.VAR_YEAR][lang] = years

		// Holiday
		//holidayVariable, err := MakeHolidayVariable(lang, translations)
		holidayVariable, err := MakeHolidayVariable(lang)
		if err != nil {
			return nil, err
		}
		if holidayVariable != nil {
			//variables[lang][holidayVariable.Name()] = holidayVariable
			//variables[consts.VAT_HOLIDAYS][lang] = holidayVariable
		}
	}
	return variables, nil
}

func MakeHolidayVariable(lang string) (TranslationsV2, error) {
	return TranslationsV2{}, errors.New("not implemented")
}

func LoadVariablesTranslationsV2(variablesDir string) (VariablesV2, error) {

	// Load variables from files

	suffix := "variable"
	matches, err := filepath.Glob(filepath.Join(variablesDir, fmt.Sprintf("*.%s", suffix)))
	if err != nil {
		return nil, err
	}

	log.Infof("Globed %d variable translation files.", len(matches))
	variables := make(VariablesV2)
	for _, variableFile := range matches {
		basename := filepath.Base(variableFile)
		variable := fmt.Sprintf("$%s", snakeCaseToCamelCase(basename[:len(basename)-len(suffix)-1]))
		variableTranslations, err := LoadVariableTranslationsFromFile(variableFile, variable)
		if err != nil {
			return nil, err
		}
		variables[variable] = variableTranslations
	}

	// Load holiday variables from DB

	return variables, nil
}

func LoadVariableTranslationsFromFile(variableFile string, variableName string) (TranslationsV2, error) {
	file, err := os.Open(variableFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading translation file: %s", variableFile)
	}
	defer file.Close()
	log.Infof("Reading %s variable transations file.", variableFile)

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	lineNum := 1
	translations := make(TranslationsV2) // Map from language to value to phrases.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re := regexp.MustCompile(`^(.*),(.*) => (.*)$`)
		matches := re.FindStringSubmatch(line)
		if len(matches) != 4 {
			return nil, errors.New(fmt.Sprintf("[%s:%d] Error reading pattern: [%s]", variableFile, lineNum, line))
		}
		lang := matches[1]
		value := matches[2]
		translation := matches[3]
		if lang == "" || value == "" || translation == "" {
			return nil, errors.New(fmt.Sprintf("[%s:%d] Error reading pattern: [%s]", variableFile, lineNum, line))
		}
		if _, ok := translations[lang]; !ok {
			translations[lang] = make(map[string][]string) // Map from value to phrases.
		}
		translations[lang][value] = append(translations[lang][value], translation)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "Error reading translation file: %s", variableFile)
	}

	return translations, nil
}
