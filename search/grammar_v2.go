package search

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type GrammarV2 struct {
	HitType  string
	Language string
	Intent   string
	Filters  map[string][]string
	// Map from variable set (as string) to list of rules.
	Patterns map[string][]string
}

// Map from lang => intent => Grammar
type GrammarsV2 = map[string]map[string]*GrammarV2

func FoldGrammarsV2(first GrammarsV2, second GrammarsV2) {
	for lang, secondByIntent := range second {
		for intent, secondGrammar := range secondByIntent {
			if _, ok := first[lang]; !ok {
				first[lang] = make(map[string]*GrammarV2)
			}
			if firstGrammars, ok := first[lang][intent]; !ok {
				first[lang][intent] = secondGrammar
			} else {
				for variableSet := range secondGrammar.Patterns {
					first[lang][intent].Patterns[variableSet] = append(firstGrammars.Patterns[variableSet], secondGrammar.Patterns[variableSet]...)
				}
			}
		}
	}
}

// Note the sort. It might be the case where order of vars important later on.
// In those cases we will not be able to sort the keys. We will have to keep order
// and distinguish between two different order cases.
func VariablesAsString(vars []string) string {
	sort.Strings(vars)
	return strings.Join(vars, "|")
}

func VariablesFromString(vars string) []string {
	return strings.Split(vars, "|")
}

func ReadGrammarFileV2(grammarFile string) (GrammarsV2, error) {
	re := regexp.MustCompile(`^(.*).grammar$`)
	matches := re.FindStringSubmatch(filepath.Base(grammarFile))
	if len(matches) != 2 {
		return nil, errors.New(fmt.Sprintf("Bad gramamr file: %s, expected: <hit-type>.grammar", grammarFile))
	}
	hitType := matches[1]

	file, err := os.Open(grammarFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading grammar file: %s", grammarFile)
	}
	defer file.Close()

	scanLineRegexp := regexp.MustCompile(`^(.*),(.*) => (.*)$`)
	variablesRegexp := regexp.MustCompile(`\$[a-zA-Z]+`)

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	lineNum := 1
	grammars := make(GrammarsV2)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		matches := scanLineRegexp.FindStringSubmatch(line)
		if len(matches) != 4 {
			return nil, errors.New(fmt.Sprintf("[%s:%d] Error reading pattern: [%s]", grammarFile, lineNum, line))
		}
		lang := matches[1]
		intent := matches[2]
		pattern := matches[3]
		if lang == "" || intent == "" || pattern == "" {
			return nil, errors.New(fmt.Sprintf("[%s:%d] Error reading pattern: [%s]", grammarFile, lineNum, line))
		}
		if _, ok := grammars[lang]; !ok {
			grammars[lang] = make(map[string]*GrammarV2)
		}
		if _, ok := grammars[lang][intent]; !ok {
			filters, filterExist := consts.GRAMMAR_INTENTS_TO_FILTER_VALUES[intent]
			if !filterExist {
				return nil, errors.New(fmt.Sprintf("[%s:%d] Filters not found for intent: [%s]", grammarFile, lineNum, intent))
			}
			grammars[lang][intent] = &GrammarV2{
				HitType:  hitType,
				Language: lang,
				Intent:   intent,
				Filters:  filters,
				Patterns: make(map[string][]string),
			}
		}
		patternVariables := variablesRegexp.FindAllString(pattern, -1)
		log.Infof("Looking for vars in [%s], found %+v", pattern, patternVariables)
		variableSetAsString := ""
		if patternVariables != nil {
			variableSetAsString = VariablesAsString(patternVariables)
		}
		grammars[lang][intent].Patterns[variableSetAsString] = append(grammars[lang][intent].Patterns[variableSetAsString], pattern)

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "Error reading grammar file: %s", grammarFile)
	}

	return grammars, nil
}

func MakeGrammarsV2(grammarsDir string) (GrammarsV2, error) {
	matches, err := filepath.Glob(filepath.Join(grammarsDir, "*.grammar"))
	if err != nil {
		return nil, err
	}

	grammars := make(GrammarsV2)
	for _, grammarFile := range matches {
		grammarsFromFile, err := ReadGrammarFileV2(grammarFile)
		if err != nil {
			return nil, err
		}
		FoldGrammarsV2(grammars, grammarsFromFile)
	}
	return grammars, nil
}
