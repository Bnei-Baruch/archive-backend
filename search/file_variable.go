package search

import (
	"fmt"
	"strings"
)

type FileVariable struct {
	name   string
	values map[string][][]*TokenNode
}

func MakeFileVariable(name string, lang string, translations Translations) Variable {
	values := (map[string][][]*TokenNode)(nil)
	if variableLangs, variableOk := translations[name]; variableOk {
		if variableValues, langOk := variableLangs[lang]; langOk {
			values = variableValues
		}
	}
	if values == nil {
		return nil
	}
	return FileVariable{
		name:   name,
		values: values,
	}
}

func (fv FileVariable) Name() string {
	return fv.name
}

func (fv FileVariable) Match(token *TokenNode, variableToken *TokenNode, matchPrefixes bool, variables map[string]*Variable) (bool, []VariableValue, []*TokenNode, error) {
	fmt.Printf("Match in FileVariable:\n")
	PrintToken(token, variables)
	PrintToken(variableToken, variables)
	for value := range fv.values {
		if match, values, tokensContinue, err := ManyTokensMatch(fv.values[value], [][]*TokenNode{[]*TokenNode{token}}, true, variables); err != nil {
			return false, nil, nil, err
		} else if match {
			fmt.Printf("Prefix, matched.\n")
			tokenizedParts := []string{}
			originParts := []string{}
			originFullParts := []string{}
			for i := range values {
				tokenizedParts = append(tokenizedParts, values[i].Tokenized...)
				originParts = append(originParts, values[i].Origin)
				originFullParts = append(originFullParts, values[i].OriginFull)
			}
			values = []VariableValue{VariableValue{
				Name:       fv.Name(),
				Value:      value,
				Tokenized:  tokenizedParts,
				Origin:     strings.Join(originParts, " "),
				OriginFull: strings.Join(originFullParts, " "),
			}}
			if suffixMatch, suffixValues, suffixContinue, suffixErr := TokensSingleMatch(tokensContinue, variableToken.Children, matchPrefixes, variables); suffixErr != nil {
				return false, nil, nil, suffixErr
			} else if suffixMatch {
				fmt.Printf("suffixMatch, matched.\n")
				values = append(values, suffixValues...)
				return true, values, suffixContinue, nil
			}
		}
	}
	return false, nil, nil, nil
}

func (fv FileVariable) VariableToPhrases(prefix, suffix string, variables map[string]*Variable) []PhrasesWithOrigin {
	ret := []PhrasesWithOrigin(nil)
	for value, manyTokens := range fv.values {
		for _, tokens := range manyTokens {
			manyPhrasesWithOrigin := TokenNodesToPhrases(tokens, variables)
			for i := range manyPhrasesWithOrigin {
				manyPhrasesWithOrigin[i].Reduce(fv.Name(), value, prefix, suffix)
			}
			ret = append(ret, manyPhrasesWithOrigin...)
		}
	}
	return ret
}
