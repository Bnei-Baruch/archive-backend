package search

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

type Span struct {
	Start int
	End   int
}

func MakeSpan(start, end int) *Span {
	return &Span{Start: start, End: end}
}

type Token struct {
	Token          string `json:"token"`
	StartOffset    int    `json:"start_offset"`
	EndOffset      int    `json:"end_offset"`
	Type           string `json:"type"`
	Position       int    `json:"position"`
	PositionLength int    `json:"positionLength"`
}

type OriginalTokenNode struct {
	OriginalWholePhrase *string
	SkippedPrefix       *Span  // Stopwords before this token that were skipped.
	OriginalPhrase      string // Original string that was tokenized.
	SkippedSuffix       *Span  // Will be set only for IsEnd = true nodes.

	TokenNode *TokenNode
	Parents   []*OriginalTokenNode
	Children  []*OriginalTokenNode
}

type TokenNode struct {
	Token    Token
	IsEnd    bool
	Parents  []*TokenNode
	Children []*TokenNode
	// May be several original phrases per token from several sources that were merged.
	OriginalTokenNodes []*OriginalTokenNode
}

func originalPhraseToString(phrase *string, tn *TokenNode, parentTn *TokenNode) string {
	isMultiWord := parentTn != nil &&
		tn.Token.StartOffset == parentTn.Token.StartOffset &&
		tn.Token.EndOffset == parentTn.Token.EndOffset
	isSynonym := tn.Token.Type == "SYNONYM"
	if isSynonym {
		if isMultiWord {
			return fmt.Sprintf(" %s", tn.Token.Token)
		} else {
			return tn.Token.Token
		}
	} else {
		if isMultiWord {
			return ""
		} else {
			return string([]rune(*phrase)[tn.Token.StartOffset:tn.Token.EndOffset])
		}
	}
}

func (otn *OriginalTokenNode) SkippedSuffixToString() string {
	if otn.SkippedSuffix == nil {
		return ""
	}
	return string([]rune(*otn.OriginalWholePhrase)[otn.SkippedSuffix.Start:otn.SkippedSuffix.End])
}

func (otn *OriginalTokenNode) SkippedPrefixToString() string {
	if otn.SkippedPrefix == nil {
		return ""
	}
	return string([]rune(*otn.OriginalWholePhrase)[otn.SkippedPrefix.Start:otn.SkippedPrefix.End])
}

func (otn *OriginalTokenNode) OriginalFullPhraseToString() string {
	return fmt.Sprintf("%s%s%s",
		otn.SkippedPrefixToString(),
		otn.OriginalPhrase,
		otn.SkippedSuffixToString(),
	)
}

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

func MakeTokensFromPhrase(phrase string, lang string, esc *elastic.Client, tc *TokensCache) ([]*TokenNode, error) {
	if tc.Has(phrase, lang) {
		return tc.Get(phrase, lang), nil
	}
	index := es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, lang)
	tokens, err := MakeTokensFromPhraseIndex(phrase, lang, esc, index, context.TODO())
	if err != nil {
		return nil, err
	}
	tc.Set(phrase, lang, tokens)
	return tokens, nil
}

func MakeTokensFromPhraseIndex(phrase string, lang string, esc *elastic.Client, index string, ctx context.Context) ([]*TokenNode, error) {
	// TODO: Skip Variables, don't analyze them.
	encodedIndex := url.QueryEscape(index)
	res, err := esc.PerformRequest(ctx, elastic.PerformRequestOptions{
		Method: "GET",
		Path:   fmt.Sprintf("/%s/_analyze", encodedIndex),
		Body: struct {
			Text     string `json:"text"`
			Analyzer string `json:"analyzer"`
		}{
			Text:     phrase,
			Analyzer: consts.ANALYZERS[lang],
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Error analyzing [%s] in %s with analyzer %s, index [%s]",
			phrase, lang, consts.ANALYZERS[lang], index)
	}
	tokens := struct {
		Tokens []Token `json:"tokens"`
	}{Tokens: []Token{}}
	if err = json.Unmarshal(res.Body, &tokens); err != nil {
		return nil, errors.Wrapf(err, "Error unmarshling analyze body while analyzing [%s] in %s with analyzer %s, index [%s]",
			phrase, lang, consts.ANALYZERS[lang], index)
	}
	tokenNodes := MakeTokenForest(tokens.Tokens, phrase)
	return tokenNodes, nil
}

func TokenNodesToString(root []*TokenNode) string {
	printPhrases := TokenNodesToPhrases(root)
	parts := []string{}
	for i := range printPhrases {
		parts = append(parts, fmt.Sprintf("[%d]: %s", i, printPhrases[i].ToString()))
	}
	return strings.Join(parts, "\n")
}

func MakeTokenForest(tokens []Token, phrase string) []*TokenNode {
	tokenRoot := []*TokenNode{}
	tokenEnd := [][]*TokenNode{}
	minPos := -1
	for i := range tokens {
		// Update PositionLength, 0 basically means 1.
		if tokens[i].PositionLength == 0 {
			tokens[i].PositionLength = 1
		}
		if tokens[i].Position < minPos || minPos == -1 {
			minPos = tokens[i].Position
		}
	}
	for _, t := range tokens {
		node := TokenNode{Token: t}
		if t.Position == minPos {
			tokenRoot = append(tokenRoot, &node)
		}
		for len(tokenEnd) <= t.Position+t.PositionLength {
			tokenEnd = append(tokenEnd, []*TokenNode{})
		}
		tokenEnd[t.Position+t.PositionLength] = append(tokenEnd[t.Position+t.PositionLength], &node)
		if t.Position > 0 {
			for i := range tokenEnd[t.Position] {
				tokenEnd[t.Position][i].Children = append(tokenEnd[t.Position][i].Children, &node)
				node.Parents = append(node.Parents, tokenEnd[t.Position][i])
			}
		}
	}
	// Skip gaps if exist, as positions might jump.
	i := 0
	for i < len(tokenEnd) {
		if len(tokenEnd[i]) == 0 {
			if i > 0 && i < len(tokenEnd)-1 {
				// connect
				for a := range tokenEnd[i-1] {
					for b := range tokenEnd[i+1] {
						tokenEnd[i-1][a].Children = append(tokenEnd[i-1][a].Children, tokenEnd[i+1][b])
						tokenEnd[i+1][b].Parents = append(tokenEnd[i+1][b].Parents, tokenEnd[i-1][a])
					}
				}
			}
			tokenEnd = append(tokenEnd[:i], tokenEnd[i+1:]...)
		} else {
			i++
		}
	}
	// Set end tokens.
	if len(tokenEnd) > 0 {
		for i := range tokenEnd[len(tokenEnd)-1] {
			tokenEnd[len(tokenEnd)-1][i].IsEnd = true
		}
	}
	// Fill skipped phrases
	fillOriginalTokens(tokenRoot, nil, &phrase, 0, make(map[*TokenNode]bool))
	// Sort
	tokenRoot = sortTokenGraph(tokenRoot, nil, make(map[*TokenNode]bool), 0)
	originalTokenRoot := []*OriginalTokenNode{}
	for i := range tokenRoot {
		originalTokenRoot = append(originalTokenRoot, tokenRoot[i].OriginalTokenNodes...)
	}
	sortOriginalTokenGraph(originalTokenRoot, nil, make(map[*OriginalTokenNode]bool), 0)
	// Uncommend for debug.
	// depth := 0
	// iterator := tokenRoot
	// for iterator != nil && len(iterator) > 0 {
	// 	log.Infof("\n\n======= DEPTH %d ======\n", depth)
	// 	for i := range iterator {
	// 		log.Infof("%sTN: %p %p %+v", strings.Repeat("    ", depth), &iterator, &iterator[i], iterator[i])
	// 		//log.Infof("OTN: %+v", iterator[i].OriginalTokenNodes)
	// 		for j := range iterator[i].OriginalTokenNodes {
	// 			log.Infof("  %sOTN[%d]: %p %+v\n", strings.Repeat("  ", depth), j, iterator[i].OriginalTokenNodes[j], iterator[i].OriginalTokenNodes[j])
	// 		}
	// 	}
	// 	iterator = iterator[0].Children
	// 	depth++
	// }
	return tokenRoot
}

// Merges second token to first if they have the same Token.
// Returns true if were merged. False otherwise.
func mergeSortTokens(first *TokenNode, second *TokenNode) {
	for i := range second.Parents {
		for j := range second.Parents[i].Children {
			if second.Parents[i].Children[j] == second {
				second.Parents[i].Children = append(second.Parents[i].Children[:j], second.Parents[i].Children[j+1:]...)
				break
			}
		}
	}
	for i := range second.Children {
		for j := range second.Children[i].Parents {
			if second.Children[i].Parents[j] == second {
				second.Children[i].Parents = append(second.Children[i].Parents[:j], second.Children[i].Parents[j+1:]...)
				break
			}
		}
	}
	parentsMap := make(map[*TokenNode]bool)
	for i := range first.Parents {
		parentsMap[first.Parents[i]] = true
	}
	childrenMap := make(map[*TokenNode]bool)
	for i := range first.Children {
		parentsMap[first.Children[i]] = true
	}
	for i := range second.Parents {
		if _, ok := parentsMap[second.Parents[i]]; !ok {
			first.Parents = append(first.Parents, second.Parents[i])
		}
	}
	for i := range second.Children {
		if _, ok := childrenMap[second.Children[i]]; !ok {
			first.Children = append(first.Children, second.Children[i])
		}
	}
	for i := range second.OriginalTokenNodes {
		first.OriginalTokenNodes = append(first.OriginalTokenNodes, second.OriginalTokenNodes[i])
	}
}

func sortTokenGraph(children []*TokenNode, parent *TokenNode, visited map[*TokenNode]bool, depth int) []*TokenNode {
	for i := range children {
		tn := children[i]
		if _, ok := visited[tn]; !ok {
			tn.Children = sortTokenGraph(tn.Children, tn, visited, depth+1)
			visited[tn] = true
		}
	}
	// Sort and merch children after all their children are sorted and merged.
	sort.SliceStable(children, func(i, j int) bool {
		return strings.Compare(children[i].Token.Token, children[j].Token.Token) < 0
	})
	return children
}

func sortOriginalTokenGraph(children []*OriginalTokenNode, parent *OriginalTokenNode, visited map[*OriginalTokenNode]bool, depth int) []*OriginalTokenNode {
	for i := range children {
		otn := children[i]
		if _, ok := visited[otn]; !ok {
			otn.Children = sortOriginalTokenGraph(otn.Children, otn, visited, depth+1)
			visited[otn] = true
		}
	}
	// Sort and merch children after all their children are sorted and merged.
	sort.SliceStable(children, func(i, j int) bool {
		return strings.Compare(children[i].OriginalFullPhraseToString(), children[j].OriginalFullPhraseToString()) < 0
	})
	return children
}

func makeOriginalTokenNodes(tokenNode *TokenNode, parent *TokenNode, phrase *string, doneIndex int) {
	prefixSpan := (*Span)(nil)
	if doneIndex < tokenNode.Token.StartOffset {
		prefix := string([]rune(*phrase)[doneIndex:tokenNode.Token.StartOffset])
		if prefix != "" {
			prefixSpan = MakeSpan(doneIndex, tokenNode.Token.StartOffset)
		}
	}
	suffixSpan := (*Span)(nil)
	if tokenNode.IsEnd && tokenNode.Token.EndOffset < len([]rune(*phrase)) {
		suffix := string([]rune(*phrase)[tokenNode.Token.EndOffset:])
		if suffix != "" {
			suffixSpan = MakeSpan(tokenNode.Token.EndOffset, len([]rune(*phrase)))
		}
	}

	parentOriginalTokenNodes := []*OriginalTokenNode{nil}
	if parent != nil {
		parentOriginalTokenNodes = parent.OriginalTokenNodes
	}
	for i := range parentOriginalTokenNodes {
		originalTokenNode := (*OriginalTokenNode)(nil)
		if len(tokenNode.OriginalTokenNodes) == 1 {
			originalTokenNode = tokenNode.OriginalTokenNodes[0]
		} else if len(tokenNode.OriginalTokenNodes) > 1 {
			panic("Should never be more than 1")
		} else {
			originalTokenNode = &OriginalTokenNode{
				OriginalWholePhrase: phrase,
				SkippedPrefix:       prefixSpan,
				OriginalPhrase:      originalPhraseToString(phrase, tokenNode, parent),
				SkippedSuffix:       suffixSpan,
				TokenNode:           tokenNode,
			}
			tokenNode.OriginalTokenNodes = append(tokenNode.OriginalTokenNodes, originalTokenNode)
		}
		if parentOriginalTokenNodes[i] != nil {
			originalTokenNode.Parents = append(originalTokenNode.Parents, parentOriginalTokenNodes[i])
			parentOriginalTokenNodes[i].Children = append(parentOriginalTokenNodes[i].Children, originalTokenNode)
		}
	}
}

func fillOriginalTokens(children []*TokenNode, parent *TokenNode, phrase *string, doneIndex int, visited map[*TokenNode]bool) {
	for i := range children {
		makeOriginalTokenNodes(children[i], parent, phrase, doneIndex)
		if _, ok := visited[children[i]]; !ok {
			visited[children[i]] = true
			fillOriginalTokens(children[i].Children, children[i], phrase, children[i].Token.EndOffset, visited)
		}
	}
}

type PhrasesWithOrigin struct {
	OriginalPhrases []string
	Phrases         []string
}

func (p *PhrasesWithOrigin) Join(s string) string {
	return strings.Join(p.Phrases, s)
}

func (p *PhrasesWithOrigin) OriginalJoin() string {
	return strings.Join(p.OriginalPhrases, "")
}

func (p *PhrasesWithOrigin) ToString() string {
	return fmt.Sprintf("[P:%s|O:%s]", p.Join(" "), p.OriginalJoin())
}

func TokenNodesToPhrases(root []*TokenNode) []PhrasesWithOrigin {
	otns := []*OriginalTokenNode{}
	for i := range root {
		otns = append(otns, root[i].OriginalTokenNodes...)
	}
	return OriginalTokenNodesToPhrases(otns)
}

func OriginalTokenNodesToPhrases(otns []*OriginalTokenNode) []PhrasesWithOrigin {
	ret := []PhrasesWithOrigin{}
	for i := range otns {
		p := PhrasesWithOrigin{
			[]string{otns[i].OriginalFullPhraseToString()},
			[]string{otns[i].TokenNode.Token.Token},
		}
		phrases := OriginalTokenNodesToPhrases(otns[i].Children)
		for j := range phrases {
			phrases[j].OriginalPhrases = append(p.OriginalPhrases, phrases[j].OriginalPhrases...)
			phrases[j].Phrases = append(p.Phrases, phrases[j].Phrases...)
			ret = append(ret, phrases[j])
		}
		if otns[i].TokenNode.IsEnd || len(phrases) == 0 {
			ret = append(ret, p)
		}
	}
	return ret
}

func TokensMatch(a []*TokenNode, b [][]*TokenNode) bool {
	for i := range b {
		if TokensSingleMatch(a, b[i]) {
			return true
		}
	}
	return false
}

func TokensSingleMatch(a []*TokenNode, b []*TokenNode) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	i := 0
	j := 0
	for i < len(a) && j < len(b) {
		cmp := strings.Compare(a[i].Token.Token, b[j].Token.Token)
		if cmp == 0 {
			if (a[i].IsEnd && b[j].IsEnd) || TokensSingleMatch(a[i].Children, b[j].Children) {
				return true
			}
			i++
			j++
		} else if cmp < 0 {
			i++
		} else {
			j++
		}
	}
	return false
}

// Searches tokens |a| inside tokens |b|, returns the matching part.
// Can be optimized? Current complexity is O(|a|^2 * |b|^2), where |a| is nubmer of tokens in the whole graph of |a|.
func TokensSearch(a []*TokenNode, b [][]*TokenNode) (string, error) {
	for i := range b {
		match, err := TokensSingleSearch(a, b[i])
		if err != nil || match != "" {
			return match, err
		}
	}
	return "", nil
}

func TokensSingleSearch(a []*TokenNode, b []*TokenNode) (string, error) {
	phrasesARegExp := []*regexp.Regexp{}
	aPhrases := TokenNodesToPhrases(a)
	for _, partsA := range aPhrases {
		re, err := regexp.Compile(partsA.Join(".*"))
		if err != nil {
			return "", err
		}
		phrasesARegExp = append(phrasesARegExp, re)
	}
	for i := range phrasesARegExp {
		for _, partsB := range TokenNodesToPhrases(b) {
			candidate := partsB.Join(" ")
			searchMatch := phrasesARegExp[i].Find([]byte(candidate))
			if searchMatch != nil {
				return partsB.OriginalJoin(), nil
			}
		}
	}
	return "", nil
}
