package search

import (
	"fmt"
	"strconv"
	"time"
)

type YearVariable struct {
	name string
}

func MakeYearVariable() Variable {
	return YearVariable{name: "$Year"}
}

func (yv YearVariable) Name() string {
	return yv.name
}

func (yv YearVariable) Match(token *TokenNode, variableToken *TokenNode, matchPrefixes bool, variables map[string]*Variable) (bool, []VariableValue, []*TokenNode, error) {
	if year, err := strconv.ParseInt(token.Token.Token, 10, 64); err != nil {
		return false, nil, nil, nil
	} else if year > 1900 && year < 2100 {
		// TODO: We should later add support for:
		// 1. Multiple values! Which is possible, i.e., ambiguations.
		// 2. Short yaer, i.e., [17]
		// 3. Hebrew letters years.
		// 4. Before and after era.
		if match, values, tokensContinue, err := TokensSingleMatch(token.Children, variableToken.Children, matchPrefixes, variables); err != nil {
			return false, nil, nil, err
		} else {
			values = append([]VariableValue{MakeVariableValue(
				yv.name, // variable
				token.OriginalTokenNodes[0].SkippedPrefixToString(), // prefix
				token.OriginalTokenNodes[0].OriginalPhrase,          // phrase
				token.OriginalTokenNodes[0].SkippedSuffixToString(), // suffix
				token.Token.Token,       // token
				fmt.Sprintf("%d", year), // value
			)}, values...)
			return match, values, tokensContinue, nil
		}
	}
	return false, nil, nil, nil
}

func (yv YearVariable) VariableToPhrases(prefix, suffix string, variables map[string]*Variable) []PhrasesWithOrigin {
	ret := []PhrasesWithOrigin(nil)
	year := 1996
	nowYear := time.Now().Year()
	for year < nowYear {
		yearString := fmt.Sprintf("%d", year)
		ret = append(ret, PhrasesWithOrigin{[]VariableValue{
			MakeVariableValue(
				yv.name,    // variable
				prefix,     // prefix
				yearString, // phrase
				suffix,     // suffix
				yearString, // token
				yearString, // value
			)}})
		year++
	}
	return ret
}
