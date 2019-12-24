package search

import (
	//"fmt"
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
	//	debugSpan += 4
	//	defer func() {
	//		debugSpan -= 4
	//	}()
	//	fmt.Printf("\n%sFileVariableMatch: %t\n", strings.Repeat("    ", debugSpan), matchPrefixes)
	//	PrintTokens([]*TokenNode{token}, "token", variables)
	//	PrintTokens([]*TokenNode{variableToken}, "variableToken", variables)
	for value := range fv.values {
		//fmt.Printf("%sTry match token with value: %+v\n", strings.Repeat("    ", debugSpan), value)
		for i := range fv.values[value] {
			if match, values, tokensContinue, err := TokensSingleMatch(fv.values[value][i], []*TokenNode{token}, true, variables); err != nil {
				return false, nil, nil, err
			} else if match {
				tokenizedParts := []string{}
				originParts := []string{}
				originFullParts := []string{}
				for j := range values {
					tokenizedParts = append(tokenizedParts, values[j].Tokenized...)
					originParts = append(originParts, values[j].Origin)
					originFullParts = append(originFullParts, values[j].OriginFull)
				}
				values = []VariableValue{VariableValue{
					Name:       fv.Name(),
					Value:      value,
					Tokenized:  tokenizedParts,
					Origin:     strings.Join(originParts, " "),
					OriginFull: strings.Join(originFullParts, ""),
				}}
				if suffixMatch, suffixValues, suffixContinue, suffixErr := TokensSingleMatch(tokensContinue, variableToken.Children, matchPrefixes, variables); suffixErr != nil {
					//fmt.Printf("%sSuffix error.\n", strings.Repeat("    ", debugSpan))
					return false, nil, nil, suffixErr
				} else if suffixMatch {
					//fmt.Printf("%sSuffix matched.\n", strings.Repeat("    ", debugSpan))
					values = append(values, suffixValues...)
					return true, values, suffixContinue, nil
				}
				//fmt.Printf("%sSuffix did NOT matched.\n", strings.Repeat("    ", debugSpan))
			}
			//fmt.Printf("%sPrerfix %d did NOT match.\n", strings.Repeat("    ", debugSpan), i)
		}
		//fmt.Printf("%sAny prefix did NOT match.\n", strings.Repeat("    ", debugSpan))
	}
	return false, nil, nil, nil
}

func (fv FileVariable) VariableToPhrases(prefix, suffix string, variables map[string]*Variable) []*PhrasesWithOrigin {
	//fmt.Printf("VariableToPhrases: prefix %s suffix %s", prefix, suffix)
	ret := []*PhrasesWithOrigin(nil)
	for value, manyTokens := range fv.values {
		for i := range manyTokens {
			manyPhrasesWithOrigin := CopyPhrasesWithOrigin(TokenNodesToPhrases(manyTokens[i], variables, true /*=reduceVariables*/))
			for j := range manyPhrasesWithOrigin {
				manyPhrasesWithOrigin[j].Reduce(fv.Name(), value, prefix, suffix)
			}
			ret = append(ret, manyPhrasesWithOrigin...)
		}
	}
	return ret
}
