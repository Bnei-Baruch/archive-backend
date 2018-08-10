package es_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

type TagsIndexerSuite struct {
	IndexerSuite
}

func TestTagsIndexer(t *testing.T) {
	suite.Run(t, new(TagsIndexerSuite))
}

func (suite *TagsIndexerSuite) TestTagsIndex() {
	fmt.Printf("\n\n\n--- TEST TAGS INDEX ---\n\n\n")

	r := require.New(suite.T())

	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW, "test-date")
	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_TAGS}, common.DB, common.ESC)
	r.Nil(err)

	fmt.Printf("\n\n\nAdding tag.\n\n")
	tUid1 := suite.ut(1, null.Int64{Valid: false}, "root", consts.LANG_ENGLISH)
	suite.ut(1, null.Int64{Valid: false}, "שרש", consts.LANG_HEBREW)
	tUid2 := suite.ut(2, null.Int64{Valid: true, Int64: 1}, "branch", consts.LANG_ENGLISH)
	suite.ut(2, null.Int64{Valid: true, Int64: 1}, "ענף", consts.LANG_HEBREW)

	fmt.Printf("\n\n\nReindexing everything.\n\n")

	// Index existing DB data.
	r.Nil(indexer.ReindexAll())
	r.Nil(indexer.RefreshAll())

	fmt.Printf("\n\n\nValidate we have tag with 2 languages.\n\n")
	suite.validateTagNames(indexNameEn, indexer, []string{"root - branch"})
	suite.validateTagNames(indexNameHe, indexer, []string{"שרש - ענף"})

	fmt.Println("Validate tag full path.")
	suite.validateTagsFullPath(indexNameEn, indexer, [][]string{[]string{tUid1, tUid2}})

	fmt.Println("Delete tags from DB, reindex and validate we have 0 tags.")
	suite.rt(2)
	suite.rt(1)

	indexer.TagUpdate(tUid1)
	indexer.TagUpdate(tUid2)
	r.Nil(indexer.RefreshAll())

	r.Nil(es.DumpDB(common.DB, "TAGS Before validation"))
	r.Nil(es.DumpIndexes(common.ESC, "TAGS Before validation", consts.ES_RESULT_TYPE_SOURCES))

	suite.validateTagNames(indexNameEn, indexer, []string{})
	suite.validateTagNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
