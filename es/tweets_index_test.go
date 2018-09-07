package es_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

type TweeterIndexerSuite struct {
	IndexerSuite
}

func TestTweeterIndexer(t *testing.T) {
	suite.Run(t, new(TweeterIndexerSuite))
}

func (suite *TweeterIndexerSuite) TestTwitterIndex() {
	fmt.Printf("\n\n\n--- TEST TWITTER INDEX ---\n\n\n")

	r := require.New(suite.T())

	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexNameEs := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_SPANISH, "test-date")
	indexNameRu := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_RUSSIAN, "test-date")
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW, "test-date")
	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_TWEETS}, common.DB, common.ESC)
	r.Nil(err)

	r.Nil(indexer.ReindexAll())
	//r.Nil(indexer.RefreshAll())

	fmt.Printf("\nAdding English tweet and validate.\n\n")
	suite.it(1, "1", 3, "this is english tweet")
	r.Nil(indexer.TweetUpdate("1"))
	suite.validateNames(indexNameEn, indexer, []string{"this is english tweet"})

	fmt.Printf("\nAdding Spanish tweet and validate.\n\n")
	suite.it(2, "2", 4, "this is spanish tweet")
	r.Nil(indexer.TweetUpdate("2"))
	suite.validateNames(indexNameEs, indexer, []string{"this is spanish tweet"})

	fmt.Printf("\nAdding Hebrew tweet and validate.\n\n")
	suite.it(3, "3", 2, "this is hebrew tweet")
	r.Nil(indexer.TweetUpdate("3"))
	suite.validateNames(indexNameHe, indexer, []string{"this is hebrew tweet"})

	fmt.Printf("\nAdding Russian tweet and validate.\n\n")
	suite.it(4, "4", 1, "this is russian tweet")
	r.Nil(indexer.TweetUpdate("4"))
	suite.validateNames(indexNameRu, indexer, []string{"this is russian tweet"})

	fmt.Println("\nDelete tweets from DB, reindex and validate we have 0 tweets.")
	r.Nil(deleteTweets([]string{"1", "2", "3", "4"}))
	r.Nil(indexer.ReindexAll())
	suite.validateNames(indexNameEn, indexer, []string{})
	suite.validateNames(indexNameEs, indexer, []string{})
	suite.validateNames(indexNameRu, indexer, []string{})
	suite.validateNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
