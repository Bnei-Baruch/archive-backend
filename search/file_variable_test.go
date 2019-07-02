package search_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/search/searchtest"
	"github.com/stretchr/testify/suite"
)

type FileVariableSuite struct {
	searchtest.SearchSuite
}

func TestFileVariable(t *testing.T) {
	suite.Run(t, new(FileVariableSuite))
}

func (suite *FileVariableSuite) TestMatchFileVariable() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"only two,one twoone"}))
	translations := suite.MakeTranslations("$ConventionLocation", "en", map[string][]string{
		"one": []string{"one", "only one"},
		"two": []string{"only two", "too you", "two of you"},
	})
	conventionLocationVariable := search.MakeFileVariable("$ConventionLocation", "en", translations)
	variables := make(search.VariablesByName)
	variables["$ConventionLocation"] = &conventionLocationVariable
	text := suite.Tokens("next congress at only two", "en")
	pattern := suite.Tokens("next congress at $ConventionLocation", "en")
	match, values, _, err := search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err, "$Text", "next", "$Text", " congress", "$ConventionLocation", "two")
}

func (suite *FileVariableSuite) TestMatchFileVariableSynonym() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"only two,one twoone"}))
	translations := suite.MakeTranslations("$ConventionLocation", "en", map[string][]string{
		"one": []string{"one", "only one"},
		"two": []string{"only two", "too you", "two of you"},
	})
	conventionLocationVariable := search.MakeFileVariable("$ConventionLocation", "en", translations)
	variables := make(search.VariablesByName)
	variables["$ConventionLocation"] = &conventionLocationVariable
	text := suite.Tokens("next congress at one twoone something", "en")
	pattern := suite.Tokens("next congress at $ConventionLocation something", "en")
	match, values, _, err := search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err, "$Text", "next", "$Text", " congress", "$ConventionLocation", "two", "$Text", " something")
}
