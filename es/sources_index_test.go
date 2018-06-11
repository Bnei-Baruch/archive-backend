package es_test

import (
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

type SourcesIndexerSuite struct {
	IndexerSuite
}

func (suite *SourcesIndexerSuite) TestSourcesIndex() {
	fmt.Printf("\n\n\n--- TEST SOURCES INDEX ---\n\n\n")

	r := require.New(suite.T())

	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH)
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW)
	indexer, err := es.MakeIndexer("test", []string{consts.ES_RESULT_TYPE_SOURCES}, common.DB, common.ESC)
	r.Nil(err)

	fmt.Printf("\n\n\nAdding source.\n\n")
	source1UID := suite.us(es.Source{Name: "test-name-1"}, consts.LANG_ENGLISH)
	suite.us(es.Source{MDB_UID: source1UID, Name: "שם-בדיקה-1"}, consts.LANG_HEBREW)
	suite.asa(es.Source{MDB_UID: source1UID}, consts.LANG_ENGLISH, mdbmodels.Author{Name: "Test Name", ID: 3, Code: "t1"}, true, true)
	suite.asa(es.Source{MDB_UID: source1UID}, consts.LANG_HEBREW, mdbmodels.Author{Name: "שם לבדיקה", ID: 4, Code: "t2"}, true, true)
	fmt.Printf("\n\n\nAdding content files for each language.\n\n")
	suite.usfc(source1UID, consts.LANG_ENGLISH)
	suite.usfc(source1UID, consts.LANG_HEBREW)

	fmt.Printf("\n\n\nReindexing everything.\n\n")

	// Index existing DB data.
	r.Nil(indexer.ReindexAll())
	r.Nil(indexer.RefreshAll())

	fmt.Printf("\n\n\nValidate we have source with 2 languages.\n\n")
	suite.validateSourceNames(indexNameEn, indexer, []string{"test-name-1"})
	suite.validateSourceNames(indexNameHe, indexer, []string{"שם-בדיקה-1"})

	fmt.Println("Validate source files.")
	suite.validateSourceFile(indexNameEn, indexer, map[string]string{
		"test-name-1": "TEST CONTENT",
	})
	suite.validateSourceFile(indexNameHe, indexer, map[string]string{
		"שם-בדיקה-1": "TEST CONTENT",
	})

	fmt.Println("Validate source full path.")
	suite.validateSourcesFullPath(indexNameEn, indexer, []string{source1UID, "t1", "t2"})

	fmt.Println("Validate adding source without file and author - should not index.")
	source2UID := suite.us(es.Source{Name: "test-name-2"}, consts.LANG_ENGLISH)
	suite.us(es.Source{MDB_UID: source2UID, Name: "שם-בדיקה-2"}, consts.LANG_HEBREW)
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateSourceNames(indexNameEn, indexer, []string{"test-name-1"})

	fmt.Println("Validate adding source with file but without author - should not index.")
	suite.usfc(source2UID, consts.LANG_ENGLISH)
	suite.usfc(source2UID, consts.LANG_HEBREW)
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateSourceNames(indexNameEn, indexer, []string{"test-name-1"})

	fmt.Println("Validate adding source with file and author and validate.")
	suite.asa(es.Source{MDB_UID: source2UID}, consts.LANG_ENGLISH, mdbmodels.Author{Name: "Test Name 2", ID: 5, Code: "t3"}, true, true)
	suite.asa(es.Source{MDB_UID: source2UID}, consts.LANG_HEBREW, mdbmodels.Author{Name: "שם נוסף לבדיקה", ID: 6, Code: "t4"}, true, true)
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateSourceNames(indexNameEn, indexer, []string{"test-name-1", "test-name-2"})
	suite.validateSourceAuthors(indexNameEn, indexer, []string{"Test Name", "Test Name 2"})
	suite.validateSourceAuthors(indexNameHe, indexer, []string{"שם נוסף לבדיקה", "שם לבדיקה"})

	suite.validateSourceFile(indexNameEn, indexer, map[string]string{
		"test-name-1": "TEST CONTENT",
		"test-name-2": "TEST CONTENT",
	})
	suite.validateSourcesFullPath(indexNameEn, indexer, []string{source1UID, source2UID, "t1", "t2", "t3", "t4"})

	fmt.Println("Remove 1 author and validate.")
	suite.rsa(es.Source{MDB_UID: source2UID}, mdbmodels.Author{ID: 5})
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateSourceAuthors(indexNameEn, indexer, []string{"Test Name"})

	fmt.Println("Delete sources from DB, reindex and validate we have 0 sources.")
	suite.rsa(es.Source{MDB_UID: source1UID}, mdbmodels.Author{ID: 3})
	suite.rsa(es.Source{MDB_UID: source1UID}, mdbmodels.Author{ID: 4})
	suite.rsa(es.Source{MDB_UID: source2UID}, mdbmodels.Author{ID: 6})
	UIDs := []string{source1UID, source2UID}
	r.Nil(deleteSources(UIDs))
	r.Nil(indexer.ReindexAll())
	suite.validateSourceNames(indexNameEn, indexer, []string{})
	suite.validateSourceNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
