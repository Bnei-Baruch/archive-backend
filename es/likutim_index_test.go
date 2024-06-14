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

type LikutimIndexerSuite struct {
	IndexerSuite
}

func TestLikutimIndexer(t *testing.T) {
	suite.Run(t, new(LikutimIndexerSuite))
}

func (suite *LikutimIndexerSuite) TestLikutimIndex() {
	fmt.Printf("\n\n\n--- TEST Likutim INDEX ---\n\n\n")

	r := require.New(suite.T())

	esc, err := common.ESC.GetClient()
	r.Nil(err)

	indexer, err := es.MakeIndexer("test", "test-date", []string{consts.ES_RESULT_TYPE_LIKUTIM}, common.DB, esc)
	r.Nil(err)
	r.Nil(indexer.ReindexAll(esc))

	fmt.Printf("\n\n\nAdding content units.\n\n")
	cu1UID := suite.ucu(ContentUnit{Name: "something", ContentType: consts.CT_LIKUTIM}, consts.LANG_ENGLISH, true, true)

	suite.ucu(ContentUnit{MDB_UID: cu1UID, Name: "משהוא"}, consts.LANG_HEBREW, true, true)
	suite.ucu(ContentUnit{MDB_UID: cu1UID, Name: "чтото"}, consts.LANG_RUSSIAN, true, true)

	cu2UID := suite.ucu(ContentUnit{Name: "something else", ContentType: consts.CT_LIKUTIM}, consts.LANG_ENGLISH, true, true)
	cuNotLikutimUID := suite.ucu(ContentUnit{Name: "not likutim", ContentType: consts.CT_LESSON_PART}, consts.LANG_ENGLISH, true, true)
	cuNotPublishedUID := suite.ucu(ContentUnit{Name: "not published"}, consts.LANG_ENGLISH, false, true)
	cuNotSecureUID := suite.ucu(ContentUnit{Name: "not secured"}, consts.LANG_ENGLISH, true, false)
	UIDs := []string{cu1UID, cu2UID, cuNotLikutimUID, cuNotPublishedUID, cuNotSecureUID}

	fmt.Printf("\n\n\nReindexing everything.\n\n")
	indexNameEn := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	indexNameHe := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_HEBREW, "test-date")
	indexNameRu := es.IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_RUSSIAN, "test-date")

	// Index existing DB data.
	r.Nil(indexer.ReindexAll(esc))
	r.Nil(indexer.RefreshAll())

	fileHe1 := mdbmodels.File{ID: 1, Name: "heb_o_rav_bs-shamati-038-irat-ashem_2016-09-16_lesson.doc", UID: "8awHBfjU", Language: null.String{consts.LANG_HEBREW, true}, Secure: 0, Published: true}
	fileRu1 := mdbmodels.File{ID: 6, Name: "heb_o_rb_ahana_siba-leastara_tes-09_avodat-ashem_002.doc", UID: "JlJ0WMHJ", Language: null.String{consts.LANG_RUSSIAN, true}, Secure: 0, Published: true}
	fileEn1 := mdbmodels.File{ID: 2, Name: "eng_t_rav_2017-06-09_kitei-makor_mi-ichud-le-hafatza_n1_p1.docx", UID: "frnHYhIw", Language: null.String{consts.LANG_ENGLISH, true}, Secure: 0, Published: true}
	fileEn2 := mdbmodels.File{ID: 3, Name: "eng_o_rav_2020-12-24_art_on-jewish-unity-no-6.docx", UID: "QrtIVYJA", Language: null.String{consts.LANG_ENGLISH, true}, Secure: 0, Published: true}
	fileEnNotSecure := mdbmodels.File{ID: 5, Name: "heb_o_rb_ahana_eih-lilmod_tes-15_avodat-ashem_.doc", UID: "lRBwElZ9", Language: null.String{consts.LANG_ENGLISH, true}, Secure: 0, Published: true}
	fileEnNotPublished := mdbmodels.File{ID: 4, Name: "heb_o_rav_achana_2014-05-28_lesson.doc", UID: "bBBIDCiy", Language: null.String{consts.LANG_ENGLISH, true}, Secure: 0, Published: true}

	_ = suite.ucuf(ContentUnit{MDB_UID: cu1UID}, consts.LANG_HEBREW, fileHe1, true)
	fContHe1 := "File content on HE, 1"
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileHe1.UID)] = fContHe1

	_ = suite.ucuf(ContentUnit{MDB_UID: cu1UID}, consts.LANG_RUSSIAN, fileRu1, true)
	fContRu1 := "File content on RU, 12"
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileRu1.UID)] = fContRu1

	_ = suite.ucuf(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, fileEn1, true)
	fContEn1 := "File content on EN, 123"
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileEn1.UID)] = fContEn1

	_ = suite.ucuf(ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, fileEn2, true)
	fContEn2 := "File content on EN, 1234"
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileEn2.UID)] = fContEn2

	_ = suite.ucuf(ContentUnit{MDB_UID: cuNotSecureUID}, consts.LANG_ENGLISH, fileEnNotSecure, true)
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileEnNotSecure.UID)] = "File not secure"

	_ = suite.ucuf(ContentUnit{MDB_UID: cuNotPublishedUID}, consts.LANG_ENGLISH, fileEnNotPublished, true)
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileEnNotPublished.UID)] = "File not published"

	// Index existing DB data.
	r.Nil(indexer.ReindexAll(esc))
	r.Nil(indexer.RefreshAll())

	fmt.Println("Validate we have 2 searchable content units.")
	suite.validateNames(indexNameEn, indexer, []string{"something", "something else"})
	suite.validateContentUnitFiles(indexNameHe, indexer, null.Int{len(fContHe1), true})
	suite.validateContentUnitFiles(indexNameEn, indexer, null.Int{len(fContEn1), true})
	suite.validateContentUnitFiles(indexNameEn, indexer, null.Int{len(fContEn2), true})
	suite.validateContentUnitFiles(indexNameRu, indexer, null.Int{len(fContRu1), true})

	fmt.Println("Add a tag to content unit and validate.")
	suite.ucut(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"ibur", true}, ID: 1, UID: "L2jMWyce"}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitTags(indexNameEn, indexer, []string{"L2jMWyce"})
	fmt.Println("Add second tag to content unit and validate.")
	suite.ucut(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitTags(indexNameEn, indexer, []string{"L2jMWyce", "L3jMWyce"})
	fmt.Println("Remove one tag from content unit and validate.")
	suite.ucut(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"ibur", true}, ID: 1, UID: "L2jMWyce"}, false)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitTags(indexNameEn, indexer, []string{"L3jMWyce"})
	fmt.Println("Remove the second tag.")
	suite.ucut(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}, false)

	fmt.Println("Make content unit not published and validate.")
	//r.Nil(es.DumpDB(common.DB, "TestContentUnitsIndex, BeforeDB"))
	//r.Nil(es.DumpIndexes(common.ESC, "TestContentUnitsIndex, BeforeIndexes", consts.ES_RESULT_TYPE_UNITS))
	suite.ucu(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, false, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	//r.Nil(es.DumpDB(common.DB, "TestContentUnitsIndex, AfterDB"))
	//r.Nil(es.DumpIndexes(common.ESC, "TestContentUnitsIndex, AfterIndexes", consts.ES_RESULT_TYPE_UNITS))
	suite.validateNames(indexNameEn, indexer, []string{"something else"})
	suite.validateNames(indexNameHe, indexer, []string{})
	suite.validateNames(indexNameRu, indexer, []string{})

	fmt.Println("Make content unit not secured and validate.")
	suite.ucu(ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, true, false)
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateNames(indexNameEn, indexer, []string{})
	suite.validateNames(indexNameHe, indexer, []string{})
	suite.validateNames(indexNameRu, indexer, []string{})

	fmt.Println("Secure and publish content units again and check we have 2 searchable content units.")
	suite.ucu(ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.ucu(ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateNames(indexNameEn, indexer, []string{"something", "something else"})
	suite.validateNames(indexNameHe, indexer, []string{"משהוא"})
	suite.validateNames(indexNameRu, indexer, []string{"чтото"})

	fmt.Println("Validate adding content unit incrementally.")
	var cu3UID string
	cu3UID = suite.ucu(ContentUnit{Name: "third something", ContentType: consts.CT_LIKUTIM}, consts.LANG_ENGLISH, true, true)
	UIDs = append(UIDs, cu3UID)
	fileEn3 := mdbmodels.File{ID: 7, Name: "heb_o_rb_ahana_zadik-o-rasha_tes-09_avodat-ashem_013.doc", UID: "ajY7njZt", Language: null.String{consts.LANG_ENGLISH, true}, Secure: 0, Published: true}
	_ = suite.ucuf(ContentUnit{MDB_UID: cu3UID}, consts.LANG_ENGLISH, fileEn3, true)
	suite.serverResponses[fmt.Sprintf("/doc2text/%s", fileEn3.UID)] = "File content on EN, 12345"

	r.Nil(indexer.ContentUnitUpdate(cu3UID))
	suite.validateNames(indexNameEn, indexer,
		[]string{"something", "something else", "third something"})

	fmt.Println("Update content unit and validate.")
	suite.ucu(ContentUnit{MDB_UID: cu3UID, Name: "updated third something"}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu3UID))
	suite.validateNames(indexNameEn, indexer,
		[]string{"something", "something else", "updated third something"})

	fmt.Println("Delete content unit and validate nothing changes as the database did not change!")
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateNames(indexNameEn, indexer, []string{"something", "something else", "updated third something"})

	fmt.Println("Now actually delete the content unit also from database.")
	r.Nil(deleteContentUnits([]string{cu2UID}))
	r.Nil(indexer.ContentUnitUpdate(cu2UID))
	suite.validateNames(indexNameEn, indexer, []string{"something", "updated third something"})

	fmt.Println("Delete units, reindex and validate we have 0 searchable units.")
	r.Nil(deleteContentUnits(UIDs))
	r.Nil(indexer.ReindexAll(esc))
	suite.validateNames(indexNameEn, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}
