package es_test

import (
	"fmt"
	"testing"

	"gopkg.in/volatiletech/null.v6"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
)

type SourcesIndexerSuite struct {
	IndexerSuite
}

func TestSourcesIndexer(t *testing.T) {
	suite.Run(t, new(SourcesIndexerSuite))
}

func (suite *SourcesIndexerSuite) TestSourcesIndex() {
	fmt.Printf("\n\n\n--- TEST SOURCES INDEX ---\n\n\n")

	r := require.New(suite.T())

	esc, err := common.ESC.GetClient()
	r.Nil(err)

	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW, "test-date")
	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_SOURCES}, common.DB, esc)
	r.Nil(err)

	fmt.Printf("\n\n\nAdding source.\n\n")
	source1UID, _ := suite.us(Source{Name: "test-name-1"}, consts.LANG_ENGLISH)
	suite.us(Source{MDB_UID: source1UID, Name: "שם-בדיקה-1"}, consts.LANG_HEBREW)
	suite.asa(Source{MDB_UID: source1UID}, consts.LANG_ENGLISH, mdbmodels.Author{Name: "Test Name", ID: 3, Code: "t1"}, true, true)
	suite.asa(Source{MDB_UID: source1UID}, consts.LANG_HEBREW, mdbmodels.Author{Name: "שם לבדיקה", ID: 4, Code: "t2"}, true, true)
	fmt.Printf("\n\n\nAdding content files for each language.\n\n")
	suite.usfc(source1UID, consts.LANG_ENGLISH)
	suite.usfc(source1UID, consts.LANG_HEBREW)

	fmt.Printf("\n\n\nReindexing everything.\n\n")

	// Index existing DB data.
	r.Nil(indexer.ReindexAll(esc))
	r.Nil(indexer.RefreshAll())

	r.Nil(es.DumpDB(common.DB, "Before validation"))
	r.Nil(es.DumpIndexes(esc, "Before validation", consts.ES_RESULT_TYPE_SOURCES))

	fmt.Printf("\n\n\nValidate we have source with 2 languages.\n\n")
	suite.validateNames(indexNameEn, indexer, []string{"test-name-1"})
	suite.validateNames(indexNameHe, indexer, []string{"שם-בדיקה-1"})
	suite.validateFullNames(indexNameEn, indexer, []string{"Test Name > test-name-1"})
	suite.validateFullNames(indexNameHe, indexer, []string{"שם לבדיקה > שם-בדיקה-1"})

	fmt.Println("Validate source files.")
	suite.validateSourceFile(indexNameEn, indexer, map[string]string{
		source1UID: "TEST CONTENT",
	})
	suite.validateSourceFile(indexNameHe, indexer, map[string]string{
		source1UID: "TEST CONTENT",
	})

	fmt.Println("Validate source full path.")
	suite.validateSourcesFullPath(indexNameEn, indexer, [][]string{[]string{source1UID, "t1", "t2"}})

	fmt.Println("Validate adding source without file and author - should not index.")
	source2UID, _ := suite.us(Source{Name: "test-name-2"}, consts.LANG_ENGLISH)
	suite.us(Source{MDB_UID: source2UID, Name: "שם-בדיקה-2"}, consts.LANG_HEBREW)
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateNames(indexNameEn, indexer, []string{"test-name-1"})
	suite.validateFullNames(indexNameEn, indexer, []string{"Test Name > test-name-1"})

	fmt.Println("Validate adding source with file but without author - should not index.")
	suite.usfc(source2UID, consts.LANG_ENGLISH)
	suite.usfc(source2UID, consts.LANG_HEBREW)
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateNames(indexNameEn, indexer, []string{"test-name-1"})
	suite.validateFullNames(indexNameEn, indexer, []string{"Test Name > test-name-1"})

	fmt.Println("Validate adding source with file and author and and validate.")
	suite.asa(Source{MDB_UID: source2UID}, consts.LANG_ENGLISH, mdbmodels.Author{Name: "Test Name 2", ID: 5, Code: "t3"}, true, true)
	suite.asa(Source{MDB_UID: source2UID}, consts.LANG_HEBREW, mdbmodels.Author{Name: "שם נוסף לבדיקה", ID: 6, Code: "t4"}, true, true)
	r.Nil(indexer.SourceUpdate(source2UID))
	suite.validateNames(indexNameEn, indexer, []string{"test-name-1", "test-name-2"})
	suite.validateFullNames(indexNameEn, indexer, []string{"Test Name > test-name-1", "Test Name 2 > test-name-2"})

	suite.validateSourceFile(indexNameEn, indexer, map[string]string{
		source1UID: "TEST CONTENT",
		source2UID: "TEST CONTENT",
	})
	suite.validateSourcesFullPath(indexNameEn, indexer, [][]string{[]string{source1UID, "t1", "t2"}, []string{source2UID, "t3", "t4"}})

	//TBD add test for indexing with position (chapter)sourceShamatiUID := suite.us(Source{Name: "test-name-3", ParentID: null.NewInt64(int64(consts.SRC_SHAMATI), true),

	fmt.Printf("\n\n\nAdd source Shamati.\n\n")
	sourceShamati := Source{Name: "Shamati"}

	sourceShamatiUID, sourceShamatiID := suite.us(sourceShamati, consts.LANG_ENGLISH)

	consts.ES_SRC_PARENTS_FOR_CHAPTER_POSITION_INDEX[sourceShamatiUID] = true
	//r.Nil(indexer.SourceUpdate(sourceShamatiUID))

	fmt.Println("Validate adding source with file but without author - should not index.")
	suite.usfc(sourceShamatiUID, consts.LANG_ENGLISH)
	//r.Nil(indexer.SourceUpdate(sourceShamatiUID))

	fmt.Println("Validate adding source with file and author and validate.")
	suite.asa(Source{MDB_UID: sourceShamatiUID}, consts.LANG_ENGLISH, mdbmodels.Author{Name: "Test Name 2", ID: 7, Code: "t5"}, true, true)
	r.Nil(indexer.SourceUpdate(sourceShamatiUID))
	///
	sourceShamatiDetailUID, _ := suite.us(Source{Name: "test-name-3",
		ParentID: null.NewInt64(sourceShamatiID, true),
		Position: null.NewInt(1, true)}, consts.LANG_ENGLISH)
	suite.us(Source{Name: "שם-בדיקה-3", MDB_UID: sourceShamatiDetailUID,
		ParentID: null.NewInt64(sourceShamatiID, true),
		Position: null.NewInt(1, true)}, consts.LANG_HEBREW)
	suite.usfc(sourceShamatiDetailUID, consts.LANG_ENGLISH)
	suite.usfc(sourceShamatiDetailUID, consts.LANG_HEBREW)
	//r.Nil(indexer.SourceUpdate(sourceShamatiDetailUID))
	///

	r.Nil(indexer.SourceUpdate(sourceShamatiDetailUID))

	fmt.Printf("\n\n\nReindexing everything for Shamati.\n\n")
	// Index existing DB data.
	r.Nil(indexer.ReindexAll(esc))
	r.Nil(indexer.RefreshAll())
	fmt.Printf("\n\n\nValidate we have source with 2 languages for Shamati.\n\n")

	suite.validateFullNames(indexNameEn, indexer, []string{"Test Name > test-name-1", "Test Name 2 > test-name-2", "Test Name 2 > Shamati", "Test Name 2 > Shamati > 1. test-name-3"})
	//TBD add test for indexing with description

	fmt.Println("Delete sources from DB, reindex and validate we have 0 sources.")
	suite.rsa(Source{MDB_UID: source1UID}, mdbmodels.Author{ID: 3})
	suite.rsa(Source{MDB_UID: source1UID}, mdbmodels.Author{ID: 4})
	suite.rsa(Source{MDB_UID: source2UID}, mdbmodels.Author{ID: 5})
	suite.rsa(Source{MDB_UID: source2UID}, mdbmodels.Author{ID: 6})
	//
	suite.rsa(Source{MDB_UID: sourceShamatiUID}, mdbmodels.Author{ID: 7})

	UIDs := []string{source1UID, source2UID, sourceShamatiUID, sourceShamatiDetailUID}
	r.Nil(deleteSources(UIDs))
	r.Nil(indexer.ReindexAll(esc))
	///

	suite.validateFullNames(indexNameEn, indexer, []string{})
	suite.validateFullNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())

}
