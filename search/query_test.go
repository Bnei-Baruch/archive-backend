package search

import (
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
	assert.Equal(suite.T(),
		[]string{"article", "of", "rab\"ash", "\" article of rab\"ash \"", "article", "of", "rab\"ash", "\" article of rab\"ash\""},
		tokenize("article of rab\"ash \" article of rab\"ash \" article of rab\"ash \" article of rab\"ash\""))
	assert.Equal(suite.T(), []string{"tag:kuku"}, tokenize(" tag:kuku"))
	// TODO: Also ignore quoted quotes (to support "properly" quoted strings too).
}
