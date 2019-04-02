package es_test

import (
	"fmt"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CollectionsIndexerSuite struct {
	IndexerSuite
}

func TestCollectionsIndexer(t *testing.T) {
	suite.Run(t, new(CollectionsIndexerSuite))
}

func (suite *IndexerSuite) TestCollectionsScopeByContentUnit() {

	fmt.Printf("\n\n\n--- TEST COLLECTIONS SCOPE BY CONTENT UNIT ---\n\n\n")

	// Add test for collection for multiple content units.
	r := require.New(suite.T())
	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
	c1UID := suite.uc(es.Collection{ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")
	c2UID := suite.uc(es.Collection{ContentType: consts.CT_CONGRESS}, cu1UID, "")
	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
	suite.uc(es.Collection{ContentType: consts.CT_SPECIAL_LESSON}, cu2UID, "")

	// dumpDB("TestCollectionsScopeByContentUnit")

	uids, err := es.CollectionsScopeByContentUnit(common.DB, cu1UID)
	r.Nil(err)
	r.ElementsMatch([]string{c2UID, c1UID}, uids)
}

func (suite *IndexerSuite) TestCollectionsScopeByFile() {

	fmt.Printf("\n\n\n--- TEST COLLECTIONS SCOPE BY FILE ---\n\n\n")

	// Add test for collection for multiple content units.
	r := require.New(suite.T())
	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
	c1UID := suite.uc(es.Collection{ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")
	c2UID := suite.uc(es.Collection{ContentType: consts.CT_CONGRESS}, cu1UID, "")
	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
	suite.uc(es.Collection{ContentType: consts.CT_SPECIAL_LESSON}, cu2UID, "")
	f1UID := suite.uf(es.File{Name: "f1"}, cu1UID)
	suite.uf(es.File{Name: "f2"}, cu1UID)
	suite.uf(es.File{Name: "f3"}, cu2UID)
	suite.uf(es.File{Name: "f4"}, cu2UID)

	uids, err := es.CollectionsScopeByFile(common.DB, f1UID)
	r.Nil(err)
	r.ElementsMatch([]string{c2UID, c1UID}, uids)
}
func (suite *IndexerSuite) TestCollectionsIndex() {

	fmt.Printf("\n\n\n--- TEST COLLECTIONS INDEX ---\n\n\n")

	// Add test for collection for multiple content units.
	r := require.New(suite.T())

	esc, err := common.ESC.GetClient()
	r.Nil(err)

	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
	c1UID := suite.uc(es.Collection{Name: "c1", ContentType: consts.CT_VIDEO_PROGRAM}, cu1UID, "")
	c2UID := suite.uc(es.Collection{Name: "c2", ContentType: consts.CT_CONGRESS}, cu1UID, "")
	c3UID := suite.uc(es.Collection{Name: "c3", ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")

	fmt.Printf("\n\n\nReindexing everything.\n\n")
	indexName := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_COLLECTIONS}, common.DB, esc)
	r.Nil(err)
	// Index existing DB data.
	r.Nil(indexer.ReindexAll())
	r.Nil(indexer.RefreshAll())
	fmt.Printf("\n\n\nValidate we have 2 searchable collections with proper content units.\n\n")
	// r.Nil(es.DumpDB(common.DB, "Before validation"))
	// r.Nil(es.DumpIndexes(common.ESC, "Before validation", consts.ES_COLLECTIONS_INDEX))
	suite.validateCollectionsContentUnits(indexName, indexer, map[string][]string{
		c1UID: {cu1UID},
		c2UID: {cu1UID},
	})

	fmt.Println("Update collection content unit and validate.")
	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.uc(es.Collection{MDB_UID: c2UID}, cu2UID, "")
	r.Nil(indexer.CollectionUpdate(c2UID))
	suite.validateCollectionsContentUnits(indexName, indexer, map[string][]string{
		c1UID: {cu1UID},
		c2UID: {cu1UID, cu2UID},
	})

	fmt.Println("Delete collections, reindex and validate we have 0 searchable units.")
	r.Nil(deleteCollections([]string{c1UID, c2UID, c3UID}))
	r.Nil(indexer.ReindexAll())
	suite.validateCollectionsContentUnits(indexName, indexer, map[string][]string{})

	//fmt.Println("Restore docx-folder path to original.")
	//mdb.DocFolder = originalDocxPath

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
