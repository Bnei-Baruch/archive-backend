package search

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type Grammar struct {
	HitType  string
	Language string
	Intent   string
	Patterns []string
	Filters  map[string][]string
}

type Grammars = map[string]map[string]*Grammar

func FoldGrammars(first Grammars, second Grammars) {
	for lang, secondByIntent := range second {
		for intent, secondGrammar := range secondByIntent {
			if _, ok := first[lang]; !ok {
				first[lang] = make(map[string]*Grammar)
			}
			if firstGrammars, ok := first[lang][intent]; !ok {
				first[lang][intent] = secondGrammar
			} else {
				first[lang][intent].Patterns = append(firstGrammars.Patterns, secondGrammar.Patterns...)
			}
		}
	}
}

func ReadGrammarFile(grammarFile string) (Grammars, error) {
	re := regexp.MustCompile(`^(.*).grammar$`)
	matches := re.FindStringSubmatch(path.Base(grammarFile))
	if len(matches) != 2 {
		return nil, errors.New(fmt.Sprintf("Bad gramamr file: %s, expected: <hit-type>.grammar", grammarFile))
	}
	hitType := matches[1]

	file, err := os.Open(grammarFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading grammar file: %s", grammarFile)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	lineNum := 1
	grammars := make(Grammars)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re := regexp.MustCompile(`^(.*),(.*) => (.*)$`)
		matches := re.FindStringSubmatch(line)
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
			grammars[lang] = make(map[string]*Grammar)
		}
		if _, ok := grammars[lang][intent]; !ok {
			filters, filterExist := consts.GRAMMAR_INTENTS_TO_FILTER_VALUES[intent]
			if !filterExist {
				return nil, errors.New(fmt.Sprintf("[%s:%d] Filters not found for intent: [%s]", grammarFile, lineNum, intent))
			}
			grammars[lang][intent] = &Grammar{HitType: hitType, Language: lang, Intent: intent, Patterns: []string{}, Filters: filters}
		}
		grammars[lang][intent].Patterns = append(grammars[lang][intent].Patterns, pattern)

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "Error reading grammar file: %s", grammarFile)
	}

	return grammars, nil
}

func MakeGrammars(grammarsDir string) (Grammars, error) {
	matches, err := filepath.Glob(filepath.Join(grammarsDir, "*.grammar"))
	if err != nil {
		return nil, err
	}

	grammars := make(Grammars)
	for _, grammarFile := range matches {
		grammarsFromFile, err := ReadGrammarFile(grammarFile)
		if err != nil {
			return nil, err
		}
		FoldGrammars(grammars, grammarsFromFile)
	}

	log.Infof("Grammars: %+v", grammars)

	return grammars, nil
}
