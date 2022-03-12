package search

import (
	"encoding/json"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"gopkg.in/olivere/elastic.v6"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type QuerySuite struct {
	suite.Suite
}

func TestQuery(t *testing.T) {
	suite.Run(t, new(QuerySuite))
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

func (suite *QuerySuite) TestCreateFacetAggregationQuery() {
	q := createFacetAggregationQuery([]string{"111", "222", "333"}, consts.FILTER_TAG)
	src, err := q.Source()
	if err != nil {
		suite.T().Fatal(err)
	}
	data, err := json.Marshal(src)
	if err != nil {
		suite.T().Fatalf("marshaling to JSON failed: %v", err)
	}
	got := string(data)
	expected := `{"filters":{"filters":{"111":{"term":{"filter_values":"tag:111"}},"222":{"term":{"filter_values":"tag:222"}},"333":{"term":{"filter_values":"tag:333"}}}}}`
	suite.Equal(got, expected)
}

func (suite *QuerySuite) TestCreateFacetAggregationQueries() {
	type test struct {
		options  CreateFacetAggregationOptions
		expected string
	}
	tests := []test{
		{
			options: CreateFacetAggregationOptions{
				sourceUIDs: []string{"111", "222"},
			},
			expected: `{"source":{"filters":{"filters":{"111":{"term":{"filter_values":"source:111"}},"222":{"term":{"filter_values":"source:222"}}}}}}`,
		},
		{
			options: CreateFacetAggregationOptions{
				mediaLanguageValues: []string{"111", "222"},
			},
			expected: `{"media_language":{"filters":{"filters":{"111":{"term":{"filter_values":"media_language:111"}},"222":{"term":{"filter_values":"media_language:222"}}}}}}`,
		},
		{
			options: CreateFacetAggregationOptions{
				contentTypeValues: []string{"111", "222"},
			},
			expected: `{"collections_content_types":{"filters":{"filters":{"111":{"term":{"filter_values":"collections_content_type:111"}},"222":{"term":{"filter_values":"collections_content_type:222"}}}}}}`,
		},
		{
			options: CreateFacetAggregationOptions{
				tagUIDs: []string{"111", "222"},
			},
			expected: `{"tag":{"filters":{"filters":{"111":{"term":{"filter_values":"tag:111"}},"222":{"term":{"filter_values":"tag:222"}}}}}}`,
		},
		{
			options: CreateFacetAggregationOptions{
				tagUIDs:             []string{"111", "222"},
				sourceUIDs:          []string{"333", "444"},
				contentTypeValues:   []string{"555", "666"},
				mediaLanguageValues: []string{"777", "888"},
			},
			expected: `{"collections_content_types":{"filters":{"filters":{"555":{"term":{"filter_values":"collections_content_type:555"}},"666":{"term":{"filter_values":"collections_content_type:666"}}}}},"media_language":{"filters":{"filters":{"777":{"term":{"filter_values":"media_language:777"}},"888":{"term":{"filter_values":"media_language:888"}}}}},"source":{"filters":{"filters":{"333":{"term":{"filter_values":"source:333"}},"444":{"term":{"filter_values":"source:444"}}}}},"tag":{"filters":{"filters":{"111":{"term":{"filter_values":"tag:111"}},"222":{"term":{"filter_values":"tag:222"}}}}}}`,
		},
	}

	mapToJson := func(m map[string]elastic.Query) (string, error) {
		resultMap := make(map[string]interface{})
		for key, q := range m {
			src, err := q.Source()
			if err != nil {
				return ``, err
			}
			resultMap[key] = src
		}
		data, err := json.Marshal(resultMap)
		if err != nil {
			return ``, err
		}
		return string(data), nil
	}

	for _, t := range tests {
		queriesMap := createFacetAggregationQueries(t.options)
		got, err := mapToJson(queriesMap)
		if err != nil {
			suite.T().Fatal(err)
		}
		suite.Equal(got, t.expected)
	}
}
