package search_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/search/searchtest"
	"github.com/stretchr/testify/suite"
)

type YearSuite struct {
	searchtest.SearchSuite
}

func TestYear(t *testing.T) {
	suite.Run(t, new(YearSuite))
}

func (suite *YearSuite) TestMatchYearEn() {
	yearVariable := search.MakeYearVariable()
	variables := make(search.VariablesByName)
	variables["$Year"] = &yearVariable
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"congresses,conventions", "nice year", "milenium"}))
	text := suite.Tokens("congresses 2014 is a nice year for congresses", "en")
	pattern := suite.Tokens("congresses $Year is a nice year for congresses", "en")
	match, values, _, err := search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err,
		"$Text", "congresses", "$Year", "2014", "$Text", " is a nice",
		"$Text", " year", "$Text", " for congresses")

	text = suite.Tokens("congresses 1919 is a nice year for congresses", "en")
	pattern = suite.Tokens("congresses $Year is a nice year for congresses", "en")
	match, values, _, err = search.TokensSingleMatch(text, pattern, false, variables)
	suite.Match(match, values, err,
		"$Text", "congresses", "$Year", "1919", "$Text", " is a nice",
		"$Text", " year", "$Text", " for congresses")
}
