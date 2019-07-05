package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

type Token struct {
	Token          string `json:"token"`
	StartOffset    int    `json:"start_offset"`
	EndOffset      int    `json:"end_offset"`
	Type           string `json:"type"`
	Position       int    `json:"position"`
	PositionLength int    `json:"positionLength"`
}

type TokenNode struct {
	Token    Token
	IsEnd    bool
	Children []*TokenNode
}

func MakeTokensFromPhrase(phrase string, lang string, esc *elastic.Client) ([]*TokenNode, error) {
	index := es.IndexAliasName("prod", consts.ES_RESULTS_INDEX, lang)
	return MakeTokensFromPhraseIndex(phrase, lang, esc, index, context.TODO())
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
	tokenNodes := MakeTokenForest(tokens.Tokens)
	// For debug should be deleted.
	//tokensStr := []string{}
	//for i, t := range tokens.Tokens {
	//	tokensStr = append(tokensStr, fmt.Sprintf("[%d]:%+v", i, t))
	//}
	//log.Infof("Tokens for: [%s] Lang: %s, Analyzer: %s:\n%s", phrase, lang, consts.ANALYZERS[lang], strings.Join(tokensStr, "\n"))
	// For debug, should be deleted.
	//printPhrases := TokenNodesToPhrases(tokenNodes)
	//for i := range printPhrases {
	//	log.Infof("Phrase[%d]: %s", i, strings.Join(printPhrases[i], " "))
	//}
	return tokenNodes, nil
}

func TokenNodesToString(root []*TokenNode) string {
	printPhrases := TokenNodesToPhrases(root)
	parts := []string{}
	for i := range printPhrases {
		parts = append(parts, fmt.Sprintf("[%d]: %s", i, strings.Join(printPhrases[i], " ")))
	}
	return strings.Join(parts, "\n")
}

func MakeTokenForest(tokens []Token) []*TokenNode {
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
					}
				}
			}
			tokenEnd = append(tokenEnd[:i], tokenEnd[i+1:]...)
		} else {
			i++
		}
	}
	// Set end tokens.
	for i := range tokenEnd[len(tokenEnd)-1] {
		tokenEnd[len(tokenEnd)-1][i].IsEnd = true
	}
	// Sort
	SortTokenForest(tokenRoot)
	return tokenRoot
}

func SortTokenForest(root []*TokenNode) {
	sorted := make(map[*TokenNode]bool)
	queue := make([]*TokenNode, len(root))
	copy(queue, root)
	for len(queue) > 0 {
		next := queue[0]  // Peek head.
		queue = queue[1:] // Dequeue.
		sort.SliceStable(next.Children, func(i, j int) bool {
			return strings.Compare(next.Children[i].Token.Token, next.Children[j].Token.Token) < 0
		})
		sorted[next] = true
		for i := range next.Children {
			if _, ok := sorted[next.Children[i]]; !ok {
				queue = append(queue, next.Children[i])
			}
		}
	}
}

// Merges |a| and |b|, returns new forest. Assume |a| and |b| are sorted. Keeps things sorted.
func MergeTokenForests(a []*TokenNode, b []*TokenNode) []*TokenNode {
	i := 0
	j := 0
	ret := []*TokenNode{}
	for i < len(a) || j < len(b) {
		if i == len(a) {
			ret = append(ret, b[j:]...)
			return ret
		}
		if j == len(b) {
			ret = append(ret, a[i:]...)
			return ret
		}
		cmp := strings.Compare(a[i].Token.Token, b[j].Token.Token)
		if cmp == 0 { // Merge
			a[i].Children = MergeTokenForests(a[i].Children, b[j].Children)
			a[i].IsEnd = a[i].IsEnd || b[j].IsEnd
			ret = append(ret, a[i])
			i++
			j++
		} else if cmp < 0 {
			ret = append(ret, a[i])
			i++
		} else {
			ret = append(ret, b[j])
			j++
		}
	}
	return ret
}

func TokenNodesToPhrases(root []*TokenNode) [][]string {
	ret := [][]string{}
	for i := range root {
		phrases := TokenNodesToPhrases(root[i].Children)
		if len(phrases) > 0 {
			for j := range phrases {
				t := root[i].Token.Token
				if root[i].IsEnd {
					t = fmt.Sprintf("%s|", root[i].Token.Token)
				}
				phrases[j] = append([]string{t}, phrases[j]...)
				ret = append(ret, phrases[j])
			}
		} else {
			ret = append(ret, []string{root[i].Token.Token})
		}
	}
	return ret
}

func TokensMatch(a []*TokenNode, b []*TokenNode) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	log.Debugf("Cmp a:\n%+v\nwith b:\n%+v\n", TokenNodesToString(a), TokenNodesToString(b))
	i := 0
	j := 0
	for i < len(a) && j < len(b) {
		cmp := strings.Compare(a[i].Token.Token, b[j].Token.Token)
		if cmp == 0 {
			if (a[i].IsEnd && b[j].IsEnd) || TokensMatch(a[i].Children, b[j].Children) {
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
