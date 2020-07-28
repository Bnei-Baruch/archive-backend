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
	source1UID, source1ID := suite.us(Source{Name: "test-name-1"}, consts.LANG_ENGLISH)
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

	fmt.Printf("\n\n\nAdd source parent like Shamati and validate.\n\n")
	parentChapterPosition := Source{Name: "Shamati"}
	parentChapterPositionUID, parentChapterPositionID := suite.us(parentChapterPosition, consts.LANG_ENGLISH)
	_, _ = suite.us(parentChapterPosition, consts.LANG_HEBREW)
	consts.ES_SRC_PARENTS_FOR_CHAPTER_POSITION_INDEX[parentChapterPositionUID] = true
	suite.usfc(parentChapterPositionUID, consts.LANG_ENGLISH)
	suite.asa(Source{MDB_UID: parentChapterPositionUID}, consts.LANG_ENGLISH, mdbmodels.Author{Name: "Test Name 2", ID: 7, Code: "t5"}, true, true)
	r.Nil(indexer.SourceUpdate(parentChapterPositionUID))
	suite.validateNames(indexNameEn, indexer, []string{"test-name-1", "test-name-2", "Shamati"})
	suite.validateFullNames(indexNameEn, indexer, []string{"Test Name > test-name-1", "Test Name 2 > test-name-2", "Test Name 2 > Shamati"})

	fmt.Printf("Add sources where the position (1) should be indexed as part of the full title (like Shamati chapters).")
	chapterPositionUID, _ := suite.us(Source{Name: "test-name-3",
		ParentID: null.Int64From(parentChapterPositionID),
		Position: null.IntFrom(1)}, consts.LANG_ENGLISH)
	suite.us(Source{Name: "שם-בדיקה-3", MDB_UID: chapterPositionUID,
		ParentID: null.Int64From(parentChapterPositionID),
		Position: null.IntFrom(1)}, consts.LANG_HEBREW)
	suite.usfc(chapterPositionUID, consts.LANG_ENGLISH)
	suite.usfc(chapterPositionUID, consts.LANG_HEBREW)
	r.Nil(indexer.SourceUpdate(chapterPositionUID))

	fmt.Printf("Reindexing everything.")
	r.Nil(indexer.ReindexAll(esc))
	r.Nil(indexer.RefreshAll())

	r.Nil(es.DumpIndexes(esc, "Before validation", consts.ES_RESULT_TYPE_SOURCES))
	fmt.Printf("Validate we have source with 2 languages and the position is indexed in the full title.")
	suite.validateNames(indexNameEn, indexer, []string{"test-name-1", "test-name-2", "Shamati", "test-name-3"})
	suite.validateNames(indexNameHe, indexer, []string{"שם-בדיקה-2", "שם-בדיקה-1", "שם-בדיקה-3"})
	suite.validateFullNames(indexNameEn, indexer,
		[]string{"Test Name > test-name-1", "Test Name 2 > test-name-2",
			"Test Name 2 > Shamati", "Test Name 2 > Shamati > 1. test-name-3"})
	suite.validateFullNames(indexNameHe, indexer,
		[]string{"שם לבדיקה > שם-בדיקה-1", "שם נוסף לבדיקה > שם-בדיקה-2", "א. שם-בדיקה-3"})

	fmt.Println("Set position to -1 and validate. Should index without position in full title.")
	suite.us(Source{MDB_UID: chapterPositionUID, Position: null.NewInt(-1, true)}, consts.LANG_ENGLISH)
	suite.us(Source{MDB_UID: chapterPositionUID, Position: null.NewInt(-1, true)}, consts.LANG_HEBREW)
	r.Nil(indexer.ReindexAll(esc))
	suite.validateFullNames(indexNameEn, indexer,
		[]string{"Test Name > test-name-1", "Test Name 2 > test-name-2",
			"Test Name 2 > Shamati", "Test Name 2 > Shamati > test-name-3"})
	suite.validateFullNames(indexNameHe, indexer,
		[]string{"שם לבדיקה > שם-בדיקה-1", "שם נוסף לבדיקה > שם-בדיקה-2", "שם-בדיקה-3"})

	fmt.Println("Set position to 244 and validate.")
	suite.us(Source{MDB_UID: chapterPositionUID, Position: null.IntFrom(244)}, consts.LANG_ENGLISH)
	suite.us(Source{MDB_UID: chapterPositionUID, Position: null.IntFrom(244)}, consts.LANG_HEBREW)
	r.Nil(indexer.ReindexAll(esc))
	suite.validateFullNames(indexNameEn, indexer,
		[]string{"Test Name > test-name-1", "Test Name 2 > test-name-2", "Test Name 2 > Shamati", "Test Name 2 > Shamati > 244. test-name-3"})
	suite.validateFullNames(indexNameHe, indexer,
		[]string{"שם לבדיקה > שם-בדיקה-1", "שם נוסף לבדיקה > שם-בדיקה-2", "רמד. שם-בדיקה-3"})

	fmt.Println("Change the parent for 'Shamati chapter' and validate. Should index without position in full title.")
	suite.us(Source{MDB_UID: chapterPositionUID, ParentID: null.Int64From(source1ID)}, consts.LANG_ENGLISH)
	r.Nil(indexer.ReindexAll(esc))
	suite.validateFullNames(indexNameEn, indexer,
		[]string{"Test Name > test-name-1", "Test Name 2 > test-name-2", "Test Name 2 > Shamati", "Test Name > test-name-1 > test-name-3"})
	suite.validateFullNames(indexNameHe, indexer,
		[]string{"שם לבדיקה > שם-בדיקה-1", "שם נוסף לבדיקה > שם-בדיקה-2", "שם לבדיקה > שם-בדיקה-1 > שם-בדיקה-3"})

	fmt.Println("Delete sources from DB, reindex and validate we have 0 sources.")
	suite.rsa(Source{MDB_UID: source1UID}, mdbmodels.Author{ID: 3})
	suite.rsa(Source{MDB_UID: source1UID}, mdbmodels.Author{ID: 4})
	suite.rsa(Source{MDB_UID: source2UID}, mdbmodels.Author{ID: 5})
	suite.rsa(Source{MDB_UID: source2UID}, mdbmodels.Author{ID: 6})
	suite.rsa(Source{MDB_UID: parentChapterPositionUID}, mdbmodels.Author{ID: 7})

	UIDs := []string{source1UID, source2UID, parentChapterPositionUID, chapterPositionUID}
	r.Nil(deleteSources(UIDs))
	r.Nil(indexer.ReindexAll(esc))

	suite.validateFullNames(indexNameEn, indexer, []string{})
	suite.validateFullNames(indexNameHe, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
