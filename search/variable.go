package search

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type VariableValue struct {
	Name       string   // Variable name.
	Value      string   // Variable value.
	Tokenized  []string // Tokenized phrase.
	Origin     string   // Original phrase.
	OriginFull string   // Original phrase with prefix and suffix.
}

// Map from Original Full Phrase => $Var => values
type VariablesByPhrase = map[string]map[string][]string

type Variable interface {
	Name() string
	Match(token *TokenNode, variableToken *TokenNode, matchPrefixes bool, variables map[string]*Variable) (bool, []VariableValue, []*TokenNode, error)
	VariableToPhrases(prefix, suffix string, variables map[string]*Variable) []*PhrasesWithOrigin
}

// Map from language => name => variable.
type VariablesByName = map[string]*Variable
type VariablesByLang = map[string]VariablesByName

// Map from variable => language => value => Tokens
type Translations = map[string]map[string]map[string][][]*TokenNode

func MakeVariables(variablesDir string, esc *elastic.Client, tc *TokensCache) (VariablesByLang, error) {
	translations, err := LoadVariablesTranslations(variablesDir, esc, tc)
	if err != nil {
		return nil, err
	}

	variables := make(VariablesByLang)
	yearVariable := MakeYearVariable()
	for _, lang := range consts.ALL_KNOWN_LANGS {
		variables[lang] = make(map[string]*Variable)

		// Year
		variables[lang][yearVariable.Name()] = &yearVariable

		// Holiday
		//holidayVariable, err := MakeHolidayVariable(lang, translations)
		//if err != nil {
		//	return nil, err
		//}
		//if holidayVariable != nil {
		//	variables[lang][holidayVariable.Name()] = holidayVariable
		//}

		// Convention Location
		conventionLocationVariable := MakeFileVariable(consts.VAR_CONVENTION_LOCATION, lang, translations)
		if conventionLocationVariable != nil {
			variables[lang][conventionLocationVariable.Name()] = &conventionLocationVariable
		}
	}
	return variables, nil
}

func snakeCaseToCamelCase(snakeCase string) string {
	camelCase := ""
	isToUpper := false
	for k, v := range snakeCase {
		if k == 0 {
			camelCase = strings.ToUpper(string(snakeCase[0]))
		} else {
			if isToUpper {
				camelCase += strings.ToUpper(string(v))
				isToUpper = false
			} else {
				if v == '_' {
					isToUpper = true
				} else {
					camelCase += string(v)
				}
			}
		}
	}
	return camelCase
}

func LoadVariablesTranslations(variablesDir string, esc *elastic.Client, tc *TokensCache) (Translations, error) {
	suffix := "variable"
	matches, err := filepath.Glob(filepath.Join(variablesDir, fmt.Sprintf("*.%s", suffix)))
	if err != nil {
		return nil, err
	}

	translations := make(Translations)
	for _, variableFile := range matches {
		basename := filepath.Base(variableFile)
		variable := fmt.Sprintf("$%s", snakeCaseToCamelCase(basename[:len(basename)-len(suffix)-1]))
		variableTranslations, err := LoadVariableTranslations(variableFile, variable, esc, tc)
		if err != nil {
			return nil, err
		}
		translations[variable] = variableTranslations
	}
	return translations, nil
}

func LoadVariableTranslations(variableFile string, variableName string, esc *elastic.Client, tc *TokensCache) (map[string]map[string][][]*TokenNode, error) {
	file, err := os.Open(variableFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading translation file: %s", variableFile)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	lineNum := 1
	translations := make(map[string]map[string][][]*TokenNode)
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
			translations[lang] = make(map[string][][]*TokenNode)
		}
		tokens, err := MakeTokensFromPhrase(translation, lang, esc, tc)
		if err != nil {
			return nil, errors.Wrapf(err, "Error generating tokens from translation: [%s] in %s.", translation, lang)
		}
		translations[lang][value] = append(translations[lang][value], tokens)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "Error reading translation file: %s", variableFile)
	}

	return translations, nil
}
