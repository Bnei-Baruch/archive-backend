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
	esc, err := common.ESC.GetClient()
	r.Nil(err)

	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexNameEs := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_SPANISH, "test-date")
	indexNameRu := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_RUSSIAN, "test-date")
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW, "test-date")
	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_TWEETS}, common.DB, esc)
	r.Nil(err)

	r.Nil(indexer.ReindexAll())

	fmt.Printf("\nAdding English tweet and validate.\n\n")
	suite.itt(1, "1", 3, "this is english tweet")
	r.Nil(indexer.TweetUpdate("1"))
	suite.validateContents(indexNameEn, indexer, []string{"this is english tweet"})

	fmt.Printf("\nAdding Spanish tweet and validate.\n\n")
	suite.itt(2, "2", 4, "this is spanish tweet")
	r.Nil(indexer.TweetUpdate("2"))
	suite.validateContents(indexNameEs, indexer, []string{"this is spanish tweet"})

	fmt.Printf("\nAdding Hebrew tweet and validate.\n\n")
	suite.itt(3, "3", 2, "this is hebrew tweet")
	r.Nil(indexer.TweetUpdate("3"))
	suite.validateContents(indexNameHe, indexer, []string{"this is hebrew tweet"})

	fmt.Printf("\nAdding Russian tweet and validate.\n\n")
	suite.itt(4, "4", 1, "this is russian tweet")
	r.Nil(indexer.TweetUpdate("4"))
	suite.validateContents(indexNameRu, indexer, []string{"this is russian tweet"})

	fmt.Printf("\nAdding another Russian tweet and validate.\n\n")
	suite.itt(5, "5", 1, "this is another russian tweet")
	r.Nil(indexer.TweetUpdate("5"))
	suite.validateContents(indexNameRu, indexer, []string{"this is russian tweet", "this is another russian tweet"})

	fmt.Printf("\nRemoving first Russian tweet and validate.\n\n")
	r.Nil(deleteTweets([]string{"4"}))
	r.Nil(indexer.TweetUpdate("4"))
	suite.validateContents(indexNameRu, indexer, []string{"this is another russian tweet"})

	fmt.Println("\nDelete tweets from DB, reindex and validate we have 0 tweets.")
	r.Nil(deleteTweets([]string{"1", "2", "3", "5"}))
	r.Nil(indexer.ReindexAll())
	suite.validateNames(indexNameEn, indexer, []string{})
	suite.validateNames(indexNameEs, indexer, []string{})
	suite.validateNames(indexNameRu, indexer, []string{})
	suite.validateNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
