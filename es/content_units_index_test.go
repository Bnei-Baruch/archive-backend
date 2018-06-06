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
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

type UnitsIndexerSuite struct {
    IndexerSuite
}

func TestIndexer(t *testing.T) {
	suite.Run(t, new(UnitsIndexerSuite))
}

func (suite *UnitsIndexerSuite) TestContentUnitsIndex() {
	fmt.Printf("\n\n\n--- TEST CONTENT UNITS INDEX ---\n\n\n")

	r := require.New(suite.T())
	fmt.Printf("\n\n\nAdding content units.\n\n")
	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
	suite.ucu(es.ContentUnit{MDB_UID: cu1UID, Name: "משהוא"}, consts.LANG_HEBREW, true, true)
	suite.ucu(es.ContentUnit{MDB_UID: cu1UID, Name: "чтото"}, consts.LANG_RUSSIAN, true, true)
	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
	cuNotPublishedUID := suite.ucu(es.ContentUnit{Name: "not published"}, consts.LANG_ENGLISH, false, true)
	cuNotSecureUID := suite.ucu(es.ContentUnit{Name: "not secured"}, consts.LANG_ENGLISH, true, false)
	UIDs := []string{cu1UID, cu2UID, cuNotPublishedUID, cuNotSecureUID}

	fmt.Printf("\n\n\nReindexing everything.\n\n")
	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH)
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW)
	indexNameRu := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_RUSSIAN)
	indexer := es.MakeIndexer("test", []string{consts.ES_RESULTS_INDEX}, common.DB, common.ESC)

	// Index existing DB data.
	r.Nil(indexer.ReindexAll())
	r.Nil(indexer.RefreshAll())

	fmt.Println("Validate we have 2 searchable content units.")
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "something else"})

	fmt.Println("Add a file to content unit and validate.")
	transcriptContent := "1234"
	suite.serverResponses["/dEvgPVpr"] = transcriptContent
	file := mdbmodels.File{ID: 1, Name: "heb_o_rav_2017-05-25_lesson_achana_n1_p0.doc", UID: "dEvgPVpr", Language: null.String{"he", true}, Secure: 0, Published: true}
	f1UID := suite.ucuf(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_HEBREW, file, true)
	r.Nil(indexer.FileUpdate(f1UID))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "something else"})
	suite.validateContentUnitFiles(indexNameHe, indexer, []string{"he"}, null.Int{len(transcriptContent), true})
	fmt.Println("Remove a file from content unit and validate.")
	suite.ucuf(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_HEBREW, file, false)
	r.Nil(indexer.FileUpdate(f1UID))
	r.Nil(es.DumpDB(common.DB, "DumpDB"))
	r.Nil(es.DumpIndexes(common.ESC, "DumpIndexes", consts.ES_RESULTS_INDEX))
	suite.validateContentUnitFiles(indexNameHe, indexer, []string{}, null.Int{-1, false})

	fmt.Println("Add a tag to content unit and validate.")
	suite.ucut(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"ibur", true}, ID: 1, UID: "L2jMWyce"}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitTags(indexNameEn, indexer, []string{"L2jMWyce"})
	fmt.Println("Add second tag to content unit and validate.")
	suite.ucut(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitTags(indexNameEn, indexer, []string{"L2jMWyce", "L3jMWyce"})
	fmt.Println("Remove one tag from content unit and validate.")
	suite.ucut(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"ibur", true}, ID: 1, UID: "L2jMWyce"}, false)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitTags(indexNameEn, indexer, []string{"L3jMWyce"})
	fmt.Println("Remove the second tag.")
	suite.ucut(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}, false)

	fmt.Println("Add a source to content unit and validate.")
	sourceUID1 := "ALlyoveA"
	sourceUID2 := "1vCj4qN9"
	sourceUIDs := []string{sourceUID1, sourceUID2}
	suite.acus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-zohar", true}, ID: 3, TypeID: 1, UID: sourceUID1}, mdbmodels.Author{ID: 1}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitSources(indexNameEn, indexer, []string{sourceUID1})
	fmt.Println("Add second source to content unit and validate.")
	suite.acus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-pi-hacham", true}, ID: 4, TypeID: 1, UID: sourceUID2}, mdbmodels.Author{ID: 1}, false)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitSources(indexNameEn, indexer, sourceUIDs)
	fmt.Println("Remove one source from content unit and validate.")
	suite.rcus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-zohar", true}, ID: 3, TypeID: 1, UID: sourceUID1})
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitSources(indexNameEn, indexer, []string{sourceUID2})
	fmt.Println("Remove the second source.")
	suite.rcus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-pi-hacham", true}, ID: 4, TypeID: 1, UID: sourceUID2})

	fmt.Println("Make content unit not published and validate.")
	//r.Nil(es.DumpDB(common.DB, "TestContentUnitsIndex, BeforeDB"))
	//r.Nil(es.DumpIndexes(common.ESC, "TestContentUnitsIndex, BeforeIndexes", consts.ES_RESULTS_INDEX))
	suite.ucu(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, false, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	//r.Nil(es.DumpDB(common.DB, "TestContentUnitsIndex, AfterDB"))
	//r.Nil(es.DumpIndexes(common.ESC, "TestContentUnitsIndex, AfterIndexes", consts.ES_RESULTS_INDEX))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something else"})
	suite.validateContentUnitNames(indexNameHe, indexer, []string{})
	suite.validateContentUnitNames(indexNameRu, indexer, []string{})

	fmt.Println("Make content unit not secured and validate.")
	suite.ucu(es.ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, true, false)
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{})
	suite.validateContentUnitNames(indexNameHe, indexer, []string{})
	suite.validateContentUnitNames(indexNameRu, indexer, []string{})

	fmt.Println("Secure and publish content units again and check we have 2 searchable content units.")
	suite.ucu(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.ucu(es.ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "something else"})
	suite.validateContentUnitNames(indexNameHe, indexer, []string{"משהוא"})
	suite.validateContentUnitNames(indexNameRu, indexer, []string{"чтото"})

	fmt.Println("Validate adding content unit incrementally.")
	var cu3UID string
	cu3UID = suite.ucu(es.ContentUnit{Name: "third something"}, consts.LANG_ENGLISH, true, true)
	UIDs = append(UIDs, cu3UID)
	r.Nil(indexer.ContentUnitUpdate(cu3UID))
	suite.validateContentUnitNames(indexNameEn, indexer,
		[]string{"something", "something else", "third something"})

	fmt.Println("Update content unit and validate.")
	suite.ucu(es.ContentUnit{MDB_UID: cu3UID, Name: "updated third something"}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu3UID))
	suite.validateContentUnitNames(indexNameEn, indexer,
		[]string{"something", "something else", "updated third something"})

	fmt.Println("Delete content unit and validate nothing changes as the database did not change!")
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "something else", "updated third something"})

	fmt.Println("Now actually delete the content unit also from database.")
	r.Nil(deleteContentUnits([]string{cu2UID}))
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "updated third something"})

	fmt.Println("Delete units, reindex and validate we have 0 searchable units.")
	r.Nil(deleteContentUnits(UIDs))
	r.Nil(deleteSources(sourceUIDs))
	r.Nil(indexer.ReindexAll())
	suite.validateContentUnitNames(indexNameEn, indexer, []string{})

	//fmt.Println("Restore docx-folder path to original.")
	//mdb.DocFolder = originalDocxPath

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}

func (suite *UnitsIndexerSuite) TestContentUnitsCollectionIndex() {
	fmt.Printf("\n\n\n--- TEST CONTENT UNITS COLLECTION INDEX ---\n\n\n")
	// Show all SQLs
	// boil.DebugMode = true
	// defer func() { boil.DebugMode = false }()

	// Add test for collection for multiple content units.
	r := require.New(suite.T())
	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
	c3UID := suite.uc(es.Collection{ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")
	suite.uc(es.Collection{ContentType: consts.CT_CONGRESS}, cu1UID, "")
	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
	c2UID := suite.uc(es.Collection{ContentType: consts.CT_SPECIAL_LESSON}, cu2UID, "")
	UIDs := []string{cu1UID, cu2UID}

	fmt.Printf("\n\n\nReindexing everything.\n\n")
	indexName := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH)
	indexer := es.MakeIndexer("test", []string{consts.ES_RESULTS_INDEX}, common.DB, common.ESC)
	// Index existing DB data.
	r.Nil(indexer.ReindexAll())
	r.Nil(indexer.RefreshAll())

	fmt.Printf("\n\n\nValidate we have 2 searchable content units with proper content types.\n\n")
	suite.validateContentUnitNames(indexName, indexer, []string{"something", "something else"})
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS},
		cu2UID: {consts.CT_SPECIAL_LESSON},
	})

	fmt.Printf("\n\n\nValidate we have successfully added a content type.\n\n")
	//r.Nil(es.DumpDB(common.DB, "Before DB"))
	//r.Nil(es.DumpIndexes(common.ESC, "Before Indexes", consts.ES_RESULTS_INDEX))
	c1UID := suite.uc(es.Collection{ContentType: consts.CT_VIDEO_PROGRAM}, cu1UID, "")
	r.Nil(indexer.CollectionUpdate(c1UID))
	//r.Nil(es.DumpDB(common.DB, "After DB"))
	//r.Nil(es.DumpIndexes(common.ESC, "After Indexes", consts.ES_RESULTS_INDEX))
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {consts.CT_SPECIAL_LESSON},
	})

	fmt.Printf("\n\n\nValidate we have successfully updated a content type.\n\n")
	// r.Nil(es.DumpDB(common.DB, "Before DB"))
	suite.uc(es.Collection{MDB_UID: c2UID, ContentType: consts.CT_MEALS}, cu2UID, "")
	// r.Nil(es.DumpDB(common.DB, "After DB"))
	// r.Nil(es.DumpIndexes(common.ESC, "Before Indexes", consts.ES_RESULTS_INDEX))
	r.Nil(indexer.CollectionUpdate(c2UID))
	// r.Nil(es.DumpIndexes(common.ESC, "After Indexes", consts.ES_RESULTS_INDEX))
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {consts.CT_MEALS},
	})

	fmt.Printf("\n\n\nValidate we have successfully deleted a content type.\n\n")
	r.Nil(deleteCollection(c2UID))
	// r.Nil(es.DumpDB(common.DB, "Before"))
	// r.Nil(es.DumpIndexes(common.ESC, "Before", consts.ES_RESULTS_INDEX))
	r.Nil(indexer.CollectionUpdate(c2UID))
	// r.Nil(es.DumpDB(common.DB, "After"))
	// r.Nil(es.DumpIndexes(common.ESC, "After", consts.ES_RESULTS_INDEX))
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {},
	})

	fmt.Printf("\n\n\nUpdate collection, remove one unit and add another.\n\n")
	// r.Nil(es.DumpDB(common.DB, "Before DB"))
	suite.uc(es.Collection{MDB_UID: c3UID} /* Add */, cu2UID /* Remove */, cu1UID)
	// r.Nil(es.DumpDB(common.DB, "After DB"))
	// r.Nil(es.DumpIndexes(common.ESC, "Before Indexes", consts.ES_RESULTS_INDEX))
	r.Nil(indexer.CollectionUpdate(c3UID))
	// r.Nil(es.DumpIndexes(common.ESC, "After Indexes", consts.ES_RESULTS_INDEX))
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {consts.CT_DAILY_LESSON},
	})

	fmt.Printf("\n\n\nDelete units, reindex and validate we have 0 searchable units.\n\n")
	r.Nil(deleteContentUnits(UIDs))
	r.Nil(indexer.ReindexAll())
	suite.validateContentUnitNames(indexName, indexer, []string{})
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}

