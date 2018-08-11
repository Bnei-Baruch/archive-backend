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

type BlogIndexerSuite struct {
	IndexerSuite
}

func TestBlogIndexer(t *testing.T) {
	suite.Run(t, new(BlogIndexerSuite))
}

func (suite *BlogIndexerSuite) TestBlogIndex() {
	fmt.Printf("\n\n\n--- TEST BLOG INDEX ---\n\n\n")

	r := require.New(suite.T())

	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexNameEs := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_SPANISH, "test-date")
	indexNameRu := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_RUSSIAN, "test-date")
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW, "test-date")
	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_BLOG_POSTS}, common.DB, common.ESC)
	r.Nil(err)

	r.Nil(indexer.ReindexAll())
	//r.Nil(indexer.RefreshAll())

	fmt.Printf("\nAdding English post and validate.\n\n")
	suite.ibp(1, 2, "this is english post", false)
	r.Nil(indexer.BlogPostUpdate(1))

	// TBD consider make a single "validate names" function for all result types
	suite.validateSourceNames(indexNameEn, indexer, []string{"this is english post"})

	fmt.Printf("\nAdding Spanish post and validate.\n\n")
	suite.ibp(2, 3, "this is spanish post", false)
	r.Nil(indexer.BlogPostUpdate(2))
	suite.validateSourceNames(indexNameEn, indexer, []string{"this is spanish post"})

	fmt.Printf("\nAdding Hebrew post and validate.\n\n")
	suite.ibp(3, 4, "this is hebrew post", false)
	r.Nil(indexer.BlogPostUpdate(3))
	suite.validateSourceNames(indexNameEn, indexer, []string{"this is hebrew post"})

	fmt.Printf("\nAdding Russian post and validate.\n\n")
	suite.ibp(4, 1, "this is russian post", false)
	r.Nil(indexer.BlogPostUpdate(4))
	suite.validateSourceNames(indexNameEn, indexer, []string{"this is russian post"})

	fmt.Println("\nValidate adding filtered post - should not index.")
	suite.ibp(4, 2, "today morning lesson", true)
	suite.validateSourceNames(indexNameEn, indexer, []string{"this is english post"})

	fmt.Println("\nDelete posts from DB, reindex and validate we have 0 posts.")
	r.Nil(deletePosts([]int64{1, 2, 3, 4}))
	r.Nil(indexer.ReindexAll())
	suite.validateSourceNames(indexNameEn, indexer, []string{})
	suite.validateSourceNames(indexNameEs, indexer, []string{})
	suite.validateSourceNames(indexNameRu, indexer, []string{})
	suite.validateSourceNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
