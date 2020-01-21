package search

import (
	"container/list"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var debugSpan = 0

type TokensCache struct {
	entries map[string]map[string]*list.Element
	order   *list.List
	mux     *sync.Mutex
	limit   int
}

func MakeTokensCache(size int) *TokensCache {
	return &TokensCache{
		entries: make(map[string]map[string]*list.Element),
		order:   list.New(),
		mux:     &sync.Mutex{},
		limit:   size,
	}
}

type TokensCacheElement struct {
	Phrase string
	Lang   string
	Tokens []*TokenNode
}

func (tc *TokensCache) Has(phrase string, lang string) bool {
	tc.mux.Lock()
	defer tc.mux.Unlock()
	if _, ok := tc.entries[phrase]; ok {
		_, okLang := tc.entries[phrase][lang]
		return okLang
	}
	return false
}

func (tc *TokensCache) Get(phrase string, lang string) []*TokenNode {
	tc.mux.Lock()
	defer tc.mux.Unlock()
	if _, ok := tc.entries[phrase]; ok {
		if element, okLang := tc.entries[phrase][lang]; okLang {
			tc.order.MoveToFront(element)
			return element.Value.(*TokensCacheElement).Tokens
		}
	}
	return nil
}

func (tc *TokensCache) Set(phrase string, lang string, tokens []*TokenNode) {
	tc.mux.Lock()
	defer tc.mux.Unlock()
	exist := false
	if _, exist = tc.entries[phrase]; exist {
		if element, exist := tc.entries[phrase][lang]; exist {
			// If keys exist, just make the emement fresh
			tc.order.MoveToFront(element)
			element.Value = &TokensCacheElement{phrase, lang, tokens}
		}
	}
	if !exist {
		if len(tc.entries) >= tc.limit {
			// Throw last used element.
			element := tc.order.Remove(tc.order.Back()).(*TokensCacheElement)
			phraseEntries := tc.entries[element.Phrase]
			delete(phraseEntries, element.Lang)
			if len(phraseEntries) == 0 {
				delete(tc.entries, element.Phrase)
			}
		}
		// Add new element.
		if _, ok := tc.entries[phrase]; !ok {
			tc.entries[phrase] = make(map[string]*list.Element)
		}
		tc.entries[phrase][lang] = tc.order.PushFront(&TokensCacheElement{phrase, lang, tokens})
	}
}

func TokenNodesToString(root []*TokenNode, variables map[string]*Variable) string {
	printPhrases := TokenNodesToPhrases(root, variables, true /*=reduceVariables*/)
	parts := []string{}
	for i := range printPhrases {
		parts = append(parts, fmt.Sprintf("[%d]: %s", i, printPhrases[i].ToString()))
	}
	return strings.Join(parts, "\n")
}

func VariablesMapToString(variablesMap map[string][]string) string {
	parts := []string(nil)
	for k, v := range variablesMap {
		parts = append(parts, fmt.Sprintf("%s=[%s]", k, strings.Join(v, ",")))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func MakeVariablePhraseWithOrigin(variable, prefix, suffix string) *PhrasesWithOrigin {
	return &PhrasesWithOrigin{[]VariableValue{MakeVariableValue(variable, prefix, variable, suffix, variable, variable)}, nil, "", "", ""}
}

func MakePhrasesWithOrigin(prefix, phrase, suffix, token string) *PhrasesWithOrigin {
	return &PhrasesWithOrigin{[]VariableValue{MakeTextVariableValue(prefix, phrase, suffix, token)}, nil, "", "", ""}
}

func MakeTextVariableValue(prefix, phrase, suffix, token string) VariableValue {
	return MakeVariableValue("$Text", prefix, phrase, suffix, token, fmt.Sprintf("%s%s%s", prefix, phrase, suffix))
}

func MakeVariableValue(variable, prefix, phrase, suffix, token, value string) VariableValue {
	ret := VariableValue{
		Name:       variable,                                      // Variable name.
		Value:      value,                                         // Variable value.
		Tokenized:  []string{token},                               // Tokenized phrase.
		Origin:     phrase,                                        // Original phrase.
		OriginFull: fmt.Sprintf("%s%s%s", prefix, phrase, suffix), // Original phrase with prefix and suffix.
	}
	//if strings.HasPrefix(ret.OriginFull, " ") {
	//	fmt.Printf("!!![%s|%s|%s]\n", prefix, phrase, suffix)
	//}
	return ret
}

type PhrasesWithOrigin struct {
	VariableValues []VariableValue

	// Caching.
	variablesMap       map[string][]string
	variablesMapString string
	joinSpace          string
	originalJoin       string
}

func (p *PhrasesWithOrigin) Invalidate() {
	p.variablesMap = nil
	p.variablesMapString = ""
	p.joinSpace = ""
	p.originalJoin = ""
}

func CopyPhrasesWithOrigin(p []*PhrasesWithOrigin) []*PhrasesWithOrigin {
	ret := []*PhrasesWithOrigin(nil)
	for i := range p {
		pCopy := *p[i]
		pCopy.Invalidate()
		ret = append(ret, &pCopy)
	}
	return ret
}

var countVM = 0
var countVMFirst = 0

func (pwo *PhrasesWithOrigin) VariablesMap() (map[string][]string, string) {
	countVM++
	if len(pwo.variablesMap) == 0 {
		countVMFirst++
		pwo.variablesMap = make(map[string][]string)
		for i := range pwo.VariableValues {
			if pwo.VariableValues[i].Name != "$Text" {
				pwo.variablesMap[pwo.VariableValues[i].Name] = append(pwo.variablesMap[pwo.VariableValues[i].Name], pwo.VariableValues[i].Value)
			}
		}
		pwo.variablesMapString = VariablesMapToString(pwo.variablesMap)
	}
	return pwo.variablesMap, pwo.variablesMapString
}

func (p *PhrasesWithOrigin) Reduce(variableName, variableValue, prefix, suffix string) {
	tokenized := []string(nil)
	for i := range p.VariableValues {
		tokenized = append(tokenized, p.VariableValues[i].Tokenized...)
	}

	//	if strings.HasPrefix(fmt.Sprintf("%s%s%s", prefix, p.OriginalJoin(), suffix), " ") {
	//		fmt.Printf("[%s] [%s] ||| ", variableName, variableValue)
	//		for k := range p.VariableValues {
	//			fmt.Printf("[%s]", p.VariableValues[k].OriginFull)
	//		}
	//		fmt.Printf("===[%s|%s|%s]\n", prefix, p.OriginalJoin(), suffix)
	//	}

	p.VariableValues = []VariableValue{VariableValue{
		Name:       variableName,
		Value:      variableValue,
		Tokenized:  tokenized,
		Origin:     p.OriginalJoin(),
		OriginFull: fmt.Sprintf("%s%s%s", prefix, p.OriginalJoin(), suffix),
	}}
	p.Invalidate()
}

func (p *PhrasesWithOrigin) Join(s string) string {
	if s == " " && p.joinSpace != "" {
		return p.joinSpace
	}
	parts := []string{}
	for i := range p.VariableValues {
		parts = append(parts, strings.Join(p.VariableValues[i].Tokenized, s))
	}
	ret := strings.Join(parts, s)
	if s == " " && p.joinSpace == "" {
		p.joinSpace = ret
	}
	return ret
}

func (p *PhrasesWithOrigin) OriginalJoin() string {
	if p.originalJoin == "" {
		parts := []string{}
		for i := range p.VariableValues {
			parts = append(parts, p.VariableValues[i].OriginFull)
		}
		p.originalJoin = strings.Join(parts, "")
	}
	return p.originalJoin
}

func (p *PhrasesWithOrigin) VariableValuesJoin() string {
	parts := []string{}
	for i := range p.VariableValues {
		parts = append(parts, fmt.Sprintf("%s:%s", p.VariableValues[i].Name, p.VariableValues[i].Value))
	}
	return strings.Join(parts, ",")
}

func (p *PhrasesWithOrigin) ToString() string {
	return fmt.Sprintf("[P:%s|O:%s|V:%s]", p.Join(" "), p.OriginalJoin(), p.VariableValuesJoin())
}

// Not good! need to be exact here. Should probably prepare phrases ahead.
//var phrasesCache = make(map[string][]PhrasesWithOrigin)
//
//func TokenNodesKey(root []*TokenNode) string {
//	parts := []string(nil)
//	for i := range root {
//		parts = append(parts, fmt.Sprintf("%p", root[i]))
//	}
//	return strings.Join(parts, "")
//}

func TokenNodesToPhrases(root []*TokenNode, variables map[string]*Variable, reduceVariables bool) []*PhrasesWithOrigin {
	ret := []*PhrasesWithOrigin(nil)
	for i := range root {
		ret = append(ret, tokenNodeToPhrases(root[i], variables, reduceVariables)...)
	}
	return ret
}

func tokenNodeToPhrases(root *TokenNode, variables map[string]*Variable, reduceVariables bool) []*PhrasesWithOrigin {
	if len(root.Phrases) == 0 {
		root.Phrases = OriginalTokenNodesToPhrases(root.OriginalTokenNodes, variables, reduceVariables)
	}
	return root.Phrases
}

func OriginalTokenNodesToPhrases(otns []*OriginalTokenNode, variables map[string]*Variable, reduceVariables bool) []*PhrasesWithOrigin {
	ret := []*PhrasesWithOrigin(nil)
	for i := range otns {
		currentPhrases := []*PhrasesWithOrigin(nil)
		if variable, ok := variables[otns[i].OriginalVariableToString()]; ok {
			if reduceVariables {
				currentPhrases = (*variable).VariableToPhrases(otns[i].OriginalVariablePrefixToString(), otns[i].SkippedSuffixToString(), variables)
			} else {
				currentPhrases = append(currentPhrases, MakeVariablePhraseWithOrigin(otns[i].OriginalVariableToString(), otns[i].SkippedPrefixToString(), otns[i].SkippedSuffixToString()))
			}
		} else {
			currentPhrases = append(currentPhrases, MakePhrasesWithOrigin(otns[i].SkippedPrefixToString(), otns[i].OriginalPhrase, otns[i].SkippedSuffixToString(), otns[i].TokenNode.Token.Token))
		}
		phrases := OriginalTokenNodesToPhrases(otns[i].Children, variables, reduceVariables)
		for j := range phrases {
			for k := range currentPhrases {
				phrasesCopy := *phrases[j]
				phrasesCopy.VariableValues = append(currentPhrases[k].VariableValues, phrasesCopy.VariableValues...)
				ret = append(ret, &phrasesCopy)
			}
		}
		if otns[i].TokenNode.IsEnd || len(phrases) == 0 {
			ret = append(ret, currentPhrases...)
		}
	}
	return ret
}

//func ManyTokensMatch(tokens [][]*TokenNode, patterns [][]*TokenNode, matchPrefixes bool, variables map[string]*Variable) (bool, []VariableValue, []*TokenNode, error) {
//	for i := range tokens {
//		if match, values, tokensContinue, err := TokensMatch(tokens[i], patterns, matchPrefixes, variables); err != nil {
//			return false, nil, nil, err
//		} else if match {
//			return true, values, tokensContinue, nil
//		}
//	}
//	return false, nil, nil, nil
//}

func TokensMatch(tokens []*TokenNode, patterns [][]*TokenNode, matchPrefixes bool, variables map[string]*Variable) (bool, []VariableValue, []*TokenNode, error) {
	for i := range patterns {
		if match, values, tokensContinue, err := TokensSingleMatch(tokens, patterns[i], matchPrefixes, variables); err != nil {
			return false, nil, nil, err
		} else if match {
			return true, values, tokensContinue, nil
		}
	}
	return false, nil, nil, nil
}

func SplitPatterns(patterns []*TokenNode, variables map[string]*Variable) ([]*TokenNode, []*TokenNode) {
	tokens := []*TokenNode{}
	variableTokens := []*TokenNode{}
	for i := range patterns {
		if _, ok := variables[patterns[i].OriginalTokenNodes[0].OriginalVariableToString()]; ok {
			variableTokens = append(variableTokens, patterns[i])
		} else {
			tokens = append(tokens, patterns[i])
		}
	}
	return tokens, variableTokens
}

func PrintOriginalPhrase(t []*TokenNode, variables map[string]*Variable) {
	parts := []string{}
	for i := range t {
		parts = append(parts, t[i].OriginalTokenNodes[0].OriginalVariableToString())
		if len(t[i].OriginalTokenNodes) > 1 {
			panic(fmt.Sprintf("Error unexpected variable token with many original phrases: [%s]",
				TokenNodesToString([]*TokenNode{t[i]}, variables)))
		}
	}
	fmt.Printf("[%s]\n", strings.Join(parts, ","))
}

func PrintToken(token *TokenNode, variables map[string]*Variable) {
	phrases := TokenNodesToPhrases([]*TokenNode{token}, variables, true /*=reduceVariables*/)
	for i := range phrases {
		fmt.Printf("%s%s|%s\n", strings.Repeat("    ", debugSpan), phrases[i].OriginalJoin(), phrases[i].Join("_"))
	}
}

func PrintTokens(tokens []*TokenNode, prefix string, variables map[string]*Variable) {
	fmt.Printf("%s%s - %d:\n", strings.Repeat("    ", debugSpan), prefix, len(tokens))
	for i := range tokens {
		PrintToken(tokens[i], variables)
	}
}

func PrintManyTokens(tokens [][]*TokenNode, prefix string, variables map[string]*Variable) {
	fmt.Printf("Many Tokens - %d Tokens:\n", len(tokens))
	for i := range tokens {
		PrintTokens(tokens[i], prefix, variables)
	}
}

func TokensSingleMatch(tokens []*TokenNode, patterns []*TokenNode, matchPrefixes bool, variables map[string]*Variable) (bool, []VariableValue, []*TokenNode, error) {
	debugSpan += 4
	defer func() {
		debugSpan -= 4
	}()
	// Uncomment for debug:
	// fmt.Printf("\n%sTokensSingleMatch: %t\n", strings.Repeat("    ", debugSpan), matchPrefixes)
	// PrintTokens(tokens, "tokens", variables)
	if len(tokens) == 0 && (len(patterns) == 0 || matchPrefixes) {
		// PrintTokens(patterns, "patterns", variables)
		if len(patterns) > 0 {
			return true, nil, patterns, nil
		}
		return true, nil, nil, nil
	}
	patterns, variableTokens := SplitPatterns(patterns, variables)
	// PrintTokens(patterns, "patterns", variables)
	// PrintTokens(variableTokens, "variableTokens", variables)
	// First try match variables.
	for i := range tokens {
		token := tokens[i].OriginalTokenNodes[0].OriginalVariableToString()
		if _, ok := variables[token]; ok {
			return false, nil, nil, errors.New(fmt.Sprintf("Variable found in tokens: %s.", token))
		}
		for j := range variableTokens {
			originalToken := variableTokens[j].OriginalTokenNodes[0].OriginalVariableToString()
			if variable, ok := variables[originalToken]; !ok {
				return false, nil, nil, errors.New(fmt.Sprintf("Variable not found: %s.", originalToken))
			} else if match, values, tokensContinue, err := (*variable).Match(tokens[i], variableTokens[j], matchPrefixes, variables); err != nil {
				return false, nil, nil, errors.New(fmt.Sprintf("Error matching %s variable with token %s.", originalToken, token))
			} else if match {
				if len(variableTokens[j].OriginalTokenNodes) > 1 {
					// TODO: Need to validate it is actually true
					// Otherwise need to rewrite.
					return false, nil, nil, errors.New(fmt.Sprintf("Not expecting more than one original phrase for %s",
						*(variableTokens[j].OriginalTokenNodes[len(variableTokens[j].OriginalTokenNodes)-1].OriginalWholePhrase)))
				}
				return match, values, tokensContinue, nil
			}
		}
	}

	// Try matching non-variable tokens.
	i := 0
	j := 0
	for i < len(tokens) && j < len(patterns) {
		cmp := strings.Compare(tokens[i].Token.Token, patterns[j].Token.Token)
		if cmp == 0 {
			if tokens[i].IsEnd && patterns[j].IsEnd {
				if len(tokens[i].OriginalTokenNodes) == 0 {
					return false, nil, nil, errors.New(fmt.Sprintf("Expected at least one original token for [%s].", tokens[i].Token.Token))
				}
				return true, []VariableValue{MakeTextVariableValue(
					tokens[i].OriginalTokenNodes[0].SkippedPrefixToString(),
					tokens[i].OriginalTokenNodes[0].OriginalPhrase,
					tokens[i].OriginalTokenNodes[0].SkippedSuffixToString(),
					tokens[i].Token.Token)}, nil, nil
			} else if match, values, tokensContinue, err := TokensSingleMatch(tokens[i].Children, patterns[j].Children, matchPrefixes, variables); err != nil {
				return false, nil, nil, err
			} else if match {
				if len(tokens[i].OriginalTokenNodes) == 0 {
					return false, nil, nil, errors.New(fmt.Sprintf("Expected at least one original token for [%s].", tokens[i].Token.Token))
				}
				values = append([]VariableValue{MakeTextVariableValue(
					tokens[i].OriginalTokenNodes[0].SkippedPrefixToString(),
					tokens[i].OriginalTokenNodes[0].OriginalPhrase,
					tokens[i].OriginalTokenNodes[0].SkippedSuffixToString(),
					tokens[i].Token.Token)}, values...)
				return true, values, tokensContinue, nil
			}
			i++
			j++
		} else if cmp < 0 {
			i++
		} else {
			j++
		}
	}
	return false, nil, nil, nil
}

// Searches tokens |a| inside tokens |b|, returns the matching part.
// Can be optimized? Current complexity is O(|a|^2 * |b|^2), where |a| is nubmer of tokens in the whole graph of |a|.
func TokensSearch(a []*TokenNode, b [][]*TokenNode, variables map[string]*Variable) (VariablesByPhrase, error) {
	variablesExist := make(map[string]bool)
	ret := make(VariablesByPhrase)
	for i := range b {
		start := time.Now()
		//count := tokenSingleSearchCalls
		matches, err := TokensSingleSearch(a, b[i], variables)
		elapsed := time.Since(start)
		if elapsed > 10*time.Millisecond {
			fmt.Printf("TokenSingleSearch - %s\n\n", elapsed.String())
		}
		if err != nil {
			return VariablesByPhrase(nil), err
		}
		start = time.Now()
		if len(matches) > 0 {
			for phrase, vMap := range matches {
				asString := VariablesMapToString(vMap)
				if _, ok := variablesExist[asString]; !ok {
					variablesExist[asString] = true
					ret[phrase] = vMap
				}
			}
		}
		elapsed = time.Since(start)
		if elapsed > 10*time.Millisecond {
			fmt.Printf("Matches - %s\n", elapsed.String())
		}
	}
	return ret, nil
}

//var tokenSingleSearchCalls = 0

type PhrasesByVariables struct {
	P []*PhrasesWithOrigin
	M map[string][]string
}

func TokensSingleSearch(a []*TokenNode, b []*TokenNode, variables map[string]*Variable) (VariablesByPhrase, error) {
	durations := make(map[string]time.Duration)
	counts := make(map[string]int)
	//tokenSingleSearchCalls++
	start := time.Now()
	aPhrases := TokenNodesToPhrases(a, variables, true /*=reduceVariables*/)
	durations["TokenNodesToPhrases A"] += time.Since(start)
	counts["TokenNodesToPhrases A"]++
	startCompile := time.Now()
	partsARegExp := []string(nil)
	for i := range aPhrases {
		partsARegExp = append(partsARegExp, aPhrases[i].Join(".*"))
	}
	phrasesARegExp, err := regexp.Compile(strings.Join(partsARegExp, "|"))
	if err != nil {
		return nil, err
	}
	durations["Compile"] += time.Since(startCompile)
	counts["Compile"]++

	variablesExist := make(map[string]bool)
	ret := make(VariablesByPhrase)

	startLoop := time.Now()
	bPhrases := TokenNodesToPhrases(b, variables, true /*=reduceVariables*/)
	durations["TokenNodesToPhrases B"] += time.Since(startLoop)
	counts["TokenNodesToPhrases B"]++

	startLoop = time.Now()
	phrasesByVariables := make(map[string]*PhrasesByVariables)
	for i := range bPhrases {
		vMap, asString := bPhrases[i].VariablesMap()
		if _, ok := phrasesByVariables[asString]; !ok {
			phrasesByVariables[asString] = &PhrasesByVariables{P: []*PhrasesWithOrigin(nil), M: vMap}
		}
		phrasesByVariables[asString].P = append(phrasesByVariables[asString].P, bPhrases[i])
	}
	durations["Prep"] += time.Since(startLoop)
	counts["Prep"]++

	allVariables := make(map[string]int)
	for asString := range phrasesByVariables {
		//for i := range bPhrases {
		//startInsideLoop := time.Now()
		//vMap, asString := bPhrases[i].VariablesMap()
		allVariables[asString]++
		//durations["partsB.VariablesMap"] += time.Since(startInsideLoop)
		//counts["partsB.VariablesMap"]++
		//if _, ok := variablesExist[asString]; !ok {
		for i := range phrasesByVariables[asString].P {
			startInsideLoop := time.Now()
			candidate := phrasesByVariables[asString].P[i].Join(" ")
			durations["Join"] += time.Since(startInsideLoop)
			counts["Join"]++
			//for j := range phrasesARegExp {
			startInsideLoop = time.Now()
			//fmt.Printf("%s\n", candidate)
			searchMatch := phrasesARegExp.Find([]byte(candidate))
			durations["variablesExist1"] += time.Since(startInsideLoop)
			counts["variablesExist1"]++
			if searchMatch != nil {
				startInsideLoop = time.Now()
				variablesExist[asString] = true
				ret[phrasesByVariables[asString].P[i].OriginalJoin()] = phrasesByVariables[asString].M
				durations["variablesExist2"] += time.Since(startInsideLoop)
				counts["variablesExist2"]++
				break
			}
			//}
			//elapsedInsideLoop := time.Since(startInsideLoop)
			//fmt.Printf("RegExp Find - %s\n", elapsedInsideLoop.String())
		}
	}
	elapsed := time.Since(start)
	durations["Total"] += elapsed
	counts["Total"]++
	if elapsed > 10*time.Millisecond {
		fmt.Printf("Search within TokensSingleSearch - %s\n", elapsed.String())
		for k, v := range durations {
			fmt.Printf("%s - (%d) - %s - avg: %.4fms\n", k, counts[k], v.String(), float64(int64(v)/int64(counts[k]))/1000000)
		}
		fmt.Printf("bPhrases: %d\n", len(bPhrases))
		fmt.Printf("All Variables: %d\n", len(allVariables))
		fmt.Printf("Variables exist: %d\n", len(variablesExist))
		//for k := range variablesExist {
		//	fmt.Printf("%s\n", k)
		//}
		fmt.Printf("\n\n")
	}

	return ret, nil
}

func TokensSingleSearchTest(a []*TokenNode, b []*TokenNode, variables map[string]*Variable) (VariablesByPhrase, error) {
	aPhrases := TokenNodesToPhrases(a, variables, true /*=reduceVariables*/)
	partsARegExp := []string(nil)
	for i := range aPhrases {
		partsARegExp = append(partsARegExp, aPhrases[i].Join(".*"))
	}
	phrasesARegExp, err := regexp.Compile(strings.Join(partsARegExp, "|"))
	if err != nil {
		return nil, err
	}

	variablesExist := make(map[string]bool)
	ret := make(VariablesByPhrase)

	bPhrases := TokenNodesToPhrases(b, variables, false /*=reduceVariables*/)

	phrasesByVariables := make(map[string]*PhrasesByVariables)
	for i := range bPhrases {
		vMap, asString := bPhrases[i].VariablesMap()
		if _, ok := phrasesByVariables[asString]; !ok {
			phrasesByVariables[asString] = &PhrasesByVariables{P: []*PhrasesWithOrigin(nil), M: vMap}
		}
		phrasesByVariables[asString].P = append(phrasesByVariables[asString].P, bPhrases[i])
	}

	for asString := range phrasesByVariables {
		for i := range phrasesByVariables[asString].P {
			candidate := phrasesByVariables[asString].P[i].Join(" ")
			searchMatch := phrasesARegExp.Find([]byte(candidate))
			if searchMatch != nil {
				variablesExist[asString] = true
				ret[phrasesByVariables[asString].P[i].OriginalJoin()] = phrasesByVariables[asString].M
				break
			}
		}
	}

	return ret, nil
}
