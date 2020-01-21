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
	text := suite.Tokens("next congress at only two five six", "en")
	pattern := suite.Tokens("next congress at $ConventionLocation five six", "en")
	match, values, _, err := search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err, "$Text", "next", "$Text", " congress", "$ConventionLocation", "two", "$Text", " five", "$Text", " six")
}

func (suite *FileVariableSuite) TestMatchFileVariableSynonym1() {
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

func (suite *FileVariableSuite) TestMatchFileVariableSynonym2() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"kenes,kenes kabbalah"}))
	translations := suite.MakeTranslations("$ConventionLocation", "en", map[string][]string{
		"kenes kabbalah - value": []string{"kenes kabbalah", "kenes kabbalah olami"},
	})
	conventionLocationVariable := search.MakeFileVariable("$ConventionLocation", "en", translations)
	variables := make(search.VariablesByName)
	variables["$ConventionLocation"] = &conventionLocationVariable
	text := suite.Tokens("congress at kenes kabbalah olami", "en")
	pattern := suite.Tokens("congress at $ConventionLocation", "en")
	match, values, _, err := search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err, "$Text", "congress", "$ConventionLocation", "kenes kabbalah - value")
}

func (suite *FileVariableSuite) TestMatchFileVariableSynonym3() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"כנס קבלה,כנסים,כנס"}))
	translations := suite.MakeTranslations("$ConventionLocation", "he", map[string][]string{
		"תל אביב": []string{"תל אביב", "גני התערוכה", "קבלה לעם", "קבלה לעם העולמי"},
	})
	conventionLocationVariable := search.MakeFileVariable("$ConventionLocation", "he", translations)
	variables := make(search.VariablesByName)
	variables["$ConventionLocation"] = &conventionLocationVariable
	text := suite.Tokens("כנס קבלה לעם העולמי", "he")
	pattern := suite.Tokens("כנס $ConventionLocation", "he")
	match, values, _, err := search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err, "$Text", "כנס", "$ConventionLocation", "תל אביב")
}
