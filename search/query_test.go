package search

import (
	"encoding/json"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type QuerySuite struct {
	suite.Suite
}

func TestQuery(t *testing.T) {
	suite.Run(t, new(QuerySuite))
}

func TestResultsQueryLikutimExclusion(t *testing.T) {
	for _, tc := range []struct {
		name           string
		includeLikutim bool
		explicitFilter bool
	}{
		{name: "regular"},
		{name: "regular with explicit filter", explicitFilter: true},
		{name: "reasoning", includeLikutim: true},
		{name: "reasoning with explicit filter", includeLikutim: true, explicitFilter: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := Query{Term: "test", IncludeLikutim: tc.includeLikutim}
			if tc.explicitFilter {
				query.Filters = map[string][]string{consts.FILTER_CONTENT_TYPE: {consts.CT_LIKUTIM}}
			}
			result, err := createResultsQuery([]string{consts.ES_RESULT_TYPE_UNITS}, query, nil, nil, false)
			require.NoError(t, err)
			source, err := result.Source()
			require.NoError(t, err)
			payload, err := json.Marshal(source)
			require.NoError(t, err)
			exclusion := `{"term":{"filter_values":"content_type:LIKUTIM"}}`
			if tc.includeLikutim {
				assert.NotContains(t, string(payload), exclusion)
			} else {
				assert.Contains(t, string(payload), exclusion)
			}
		})
	}
}

func (suite *QuerySuite) TestTokenize() {
	assert.Nil(suite.T(), tokenize(""))
	assert.Equal(suite.T(), []string{"a"}, tokenize("a"))
	assert.Equal(suite.T(), []string{"\""}, tokenize("\""))
	assert.Equal(suite.T(), []string{"\"\""}, tokenize("\"\""))
	assert.Equal(suite.T(), []string{"\"\"\""}, tokenize("\"\"\""))
	assert.Equal(suite.T(), []string{"שלום", "\"isk\"", "test"}, tokenize("שלום \"isk\" test"))
	assert.Equal(suite.T(), []string{"שלום", "\"is\"k\"", "test"}, tokenize("שלום \"is\"k\" test"))
	assert.Equal(suite.T(), []string{"שלום", "\"i\"s\"k\"", "test"}, tokenize("שלום \"i\"s\"k\" test"))
	assert.Equal(suite.T(), []string{"שלום", "\"i\"", "s\"k\"", "test"}, tokenize("שלום \"i\" s\"k\" test"))
	assert.Equal(suite.T(), []string{"שלום", "\"i\"s \"k\"", "test"}, tokenize("שלום \"i\"s \"k\" test"))
	assert.Equal(suite.T(), []string{"\"שלום", "שלום"}, tokenize("\"שלום שלום"))
	assert.Equal(suite.T(), []string{"\"ab"}, tokenize("\"ab "))
	assert.Equal(suite.T(), []string{"aaa", "\"ab"}, tokenize("aaa \"ab "))
	assert.Equal(suite.T(), []string{"aaa", "\"ab \""}, tokenize("aaa \"ab \""))
	assert.Equal(suite.T(), []string{"aaa", "\"ab \"", "another"}, tokenize("aaa \"ab \" another"))
	assert.Equal(suite.T(), []string{"aaa", "\"ab \"", "another\""}, tokenize("aaa \"ab \" another\""))
	assert.Equal(suite.T(), []string{"aaa", "\"ab \"", "another\"one"}, tokenize("aaa \"ab \" another\"one"))
	assert.Equal(suite.T(), []string{"aaa", "\"ab another\""}, tokenize("aaa \"ab another\""))
	assert.Equal(suite.T(), []string{"aaa", "\"ab another\"one else\""}, tokenize("aaa \"ab another\"one else\""))
	assert.Equal(suite.T(),
		[]string{"article", "of", "rab\"ash", "\" article of rab\"ash \"", "article", "of", "rab\"ash", "\" article of rab\"ash\""},
		tokenize("article of rab\"ash \" article of rab\"ash \" article of rab\"ash \" article of rab\"ash\""))
	assert.Equal(suite.T(), []string{"tag:kuku"}, tokenize(" tag:kuku"))
	// TODO: Also ignore quoted quotes (to support "properly" quoted strings too).
}
