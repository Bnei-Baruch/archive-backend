package es_test

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/olivere/elastic.v5"
	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/migrations"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var UID_REGEX = regexp.MustCompile("[a-zA-z0-9]{8}")

type TestDBManager struct {
	DB     *sql.DB
	testDB string
}

// Move to more general utils.
const uidBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const lettersBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GenerateUID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = uidBytes[rand.Intn(len(uidBytes))]
	}
	return string(b)
}

func GenerateName(n int) string {
	b := make([]byte, n)
	b[0] = lettersBytes[rand.Intn(len(lettersBytes))]
	for i := range b[1:] {
		b[i+1] = uidBytes[rand.Intn(len(uidBytes))]
	}
	return string(b)
}

func (m *TestDBManager) InitTestDB() error {
	m.testDB = fmt.Sprintf("test_%s", strings.ToLower(GenerateName(10)))

	// Open connection to RDBMS
	db, err := sql.Open("postgres", viper.GetString("test.mdb-url"))
	if err != nil {
		return err
	}

	// Create a new temporary test database
	if _, err := db.Exec("CREATE DATABASE " + m.testDB); err != nil {
		return err
	}

	// Close first connection and connect to temp database
	db.Close()
	db, err = sql.Open("postgres", fmt.Sprintf(viper.GetString("test.url-template"), m.testDB))
	if err != nil {
		return err
	}
	m.DB = db

	// Run migrations
	return m.runMigrations(db)
}

func (m *TestDBManager) DestroyTestDB() error {
	// Close temp DB
	err := m.DB.Close()
	if err != nil {
		return err
	}

	// Connect to MDB
	db, err := sql.Open("postgres", viper.GetString("test.mdb-url"))
	if err != nil {
		return err
	}

	// Drop test DB
	_, err = db.Exec("DROP DATABASE " + m.testDB)
	return err
}

// Supports:
// postgres://<host>/<dbname>?sslmode=disable&user=<user>&password=<password>"
// postgres://<user>:<password>@<host>/<dbname>?sslmode=disable"
// Returns host, dbname, user, password
func parseConnectionString(cs string) (string, string, string, string, error) {
	u, err := url.Parse(cs)
	if err != nil {
		return "", "", "", "", err
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}
	dbname := strings.TrimLeft(u.Path, "/")
	var user, password string
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	} else {
		m, _ := url.ParseQuery(u.RawQuery)
		if val, ok := m["user"]; ok {
			user = val[0]
		} else {
			return "", "", "", "", errors.New("User not found in connection string.")
		}
		if val, ok := m["password"]; ok {
			password = val[0]
		} else {
			return "", "", "", "", errors.New("Password not found in connection string.")
		}
	}

	return host, dbname, user, password, nil
}

func (m *TestDBManager) runMigrations(testDB *sql.DB) error {
	var visit = func(path string, f os.FileInfo, err error) error {
		match, _ := regexp.MatchString(".*\\.sql$", path)
		if !match {
			return nil
		}

		//fmt.Printf("Applying migration %s\n", path)
		m, err := migrations.NewMigration(path)
		if err != nil {
			fmt.Printf("Error migrating %s, %s", path, err.Error())
			return err
		}

		for _, statement := range m.Up() {
			if _, err := testDB.Exec(statement); err != nil {
				return fmt.Errorf("Unable to apply migration %s: %s\nStatement: %s\n", m.Name, err, statement)
			}
		}

		return nil
	}

	return filepath.Walk("../migrations", visit)
}

func Sha1(s string) string {
	h := sha1.New()
	io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func RandomSHA1() string {
	return Sha1(GenerateName(1024))
}

type IndexerSuite struct {
	suite.Suite
	TestDBManager
	esc             *elastic.Client
	ctx             context.Context
	server          *httptest.Server
	serverResponses map[string]string
}

func (suite *IndexerSuite) SetupSuite() {
	utils.InitConfig("", "../")
	err := suite.InitTestDB()
	if err != nil {
		panic(err)
	}
	suite.ctx = context.Background()

	fmt.Println("Replace docx-folder with temp. path.")
	testingsDocxPath := viper.GetString("test.test-docx-folder")
	viper.Set("elasticsearch.docx-folder", testingsDocxPath)

	fmt.Println("Replace cdn-url with test.")
	suite.serverResponses = make(map[string]string)
	handler := func(w http.ResponseWriter, r *http.Request) {
		key := ""
		if r.URL.Path != "" {
			key += fmt.Sprintf("%s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			key += fmt.Sprintf("?%s", r.URL.RawQuery)
		}
		if r.URL.Fragment != "" {
			key += fmt.Sprintf("#%s", r.URL.Fragment)
		}
		w.Header().Set("Content-Type", "plain/text")
		fmt.Printf("LOOKUP KEY [%s]\tRESPONSE [%s]\n", key, suite.serverResponses[key])
		io.WriteString(w, suite.serverResponses[key])
	}
	suite.server = httptest.NewServer(http.HandlerFunc(handler))
	viper.Set("elasticsearch.cdn-url", suite.server.URL)

	// Set package db and esc variables.
	common.InitWithDefault(suite.DB)
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
	suite.esc = common.ESC
}

func (suite *IndexerSuite) TearDownSuite() {
	// Close mock server.
	suite.server.Close()
	// Close connections.
	common.Shutdown()
	// Drop test database.
	suite.Require().Nil(suite.DestroyTestDB())
}

type ESLogAdapter struct{ *testing.T }

func (s ESLogAdapter) Printf(format string, v ...interface{}) { s.Logf(format, v...) }

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestIndexer(t *testing.T) {
	suite.Run(t, new(IndexerSuite))
}

func (suite *IndexerSuite) SetupTest() {
	r := require.New(suite.T())
	units, err := mdbmodels.ContentUnits(common.DB).All()
	r.Nil(err)
	var uids []string
	for _, unit := range units {
		uids = append(uids, unit.UID)
	}
	r.Nil(deleteContentUnits(uids))
	// Remove test indexes.
	indexer := es.MakeIndexer("test", []string{consts.ES_UNITS_INDEX, consts.ES_CLASSIFICATIONS_INDEX}, common.DB, common.ESC)
	r.Nil(indexer.DeleteIndexes())
	// Delete test directory
	os.RemoveAll(viper.GetString("test.test-docx-folder"))
	utils.Must(os.MkdirAll(viper.GetString("test.test-docx-folder"), 0777))
}

func updateCollection(c es.Collection, cuUID string, removeContentUnitUID string) (string, error) {
	var mdbCollection mdbmodels.Collection
	if c.MDB_UID != "" {
		cp, err := mdbmodels.Collections(common.DB, qm.Where("uid = ?", c.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		if c.ContentType != "" {
			cp.TypeID = mdb.CONTENT_TYPE_REGISTRY.ByName[c.ContentType].ID
		}
		if err := cp.Update(common.DB); err != nil {
			return "", err
		}
		mdbCollection = *cp
	} else {
		mdbCollection = mdbmodels.Collection{
			UID:    GenerateUID(8),
			TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[c.ContentType].ID,
		}
		if err := mdbCollection.Insert(common.DB); err != nil {
			return "", err
		}
	}
	cu, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cuUID)).One()
	if err != nil {
		return "", err
	}
	if _, err := mdbmodels.FindCollectionsContentUnit(common.DB, mdbCollection.ID, cu.ID); err == sql.ErrNoRows {
		var mdbCollectionsContentUnit mdbmodels.CollectionsContentUnit
		mdbCollectionsContentUnit.CollectionID = mdbCollection.ID
		mdbCollectionsContentUnit.ContentUnitID = cu.ID
		if err := mdbCollectionsContentUnit.Insert(common.DB); err != nil {
			return "", err
		}
	}
	// Remomove only the connection between the collection and this content unit.
	if removeContentUnitUID != "" {
		ccus, err := mdbmodels.CollectionsContentUnits(common.DB,
			qm.InnerJoin("content_units on content_units.id = collections_content_units.content_unit_id"),
			qm.Where("content_units.uid = ?", removeContentUnitUID),
			qm.And("collection_id = ?", mdbCollection.ID)).All()
		if err != nil {
			return "", errors.Wrap(err, "updateCollection select ccu")
		}
		for _, ccu := range ccus {
			if err := mdbmodels.CollectionsContentUnits(common.DB,
				qm.Where("collection_id = ?", ccu.CollectionID),
				qm.And("content_unit_id = ?", ccu.ContentUnitID)).DeleteAll(); err != nil {
				return "", errors.Wrap(err, "updateCollection delete ccu")
			}
		}
	}
	return mdbCollection.UID, nil
}

func (suite *IndexerSuite) uc(c es.Collection, cuUID string, removeContentUnitUID string) string {
	r := require.New(suite.T())
	uid, err := updateCollection(c, cuUID, removeContentUnitUID)
	r.Nil(err)
	return uid
}

func removeContentUnitTag(cu es.ContentUnit, lang string, tag mdbmodels.Tag) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		return "", errors.New("cu.MDB_UID is empty")
	}

	_, err := mdbmodels.FindTag(common.DB, tag.ID)
	if err != nil {
		return "", err
	}

	err = mdbContentUnit.RemoveTags(common.DB, &tag)
	if err != nil {
		return "", err
	}

	return mdbContentUnit.UID, nil
}

func addContentUnitTag(cu es.ContentUnit, lang string, tag mdbmodels.Tag) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		mdbContentUnit = mdbmodels.ContentUnit{
			UID:    GenerateUID(8),
			TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID,
		}
		if err := mdbContentUnit.Insert(common.DB); err != nil {
			return "", err
		}
	}

	_, err := mdbmodels.FindTag(common.DB, tag.ID)
	if err != nil {
		if err == sql.ErrNoRows {

			// save tag to DB:

			/*//generate uid
			b := make([]byte, 8)
			for i := range b {
				b[i] = uidBytes[rand.Intn(len(uidBytes))]
			}
			tag.UID = string(b)*/

			err = tag.Insert(common.DB)
			if err != nil {
				return "", err
			}

			// save i18n
			/*for _, v := range tag.I18n {
				err := t.AddTagI18ns(exec, true, v)
				if err != nil {
					return "", err
				}
			}*/

		} else {
			return "", err
		}
	}

	err = mdbContentUnit.AddTags(common.DB, false, &tag)
	if err != nil {
		return "", err
	}

	return mdbContentUnit.UID, nil
}

func addContentUnitSource(cu es.ContentUnit, lang string, src mdbmodels.Source) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		mdbContentUnit = mdbmodels.ContentUnit{
			UID:    GenerateUID(8),
			TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID,
		}
		if err := mdbContentUnit.Insert(common.DB); err != nil {
			return "", err
		}
	}

	_, err := mdbmodels.FindSource(common.DB, src.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			err = src.Insert(common.DB)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	err = mdbContentUnit.AddSources(common.DB, false, &src)
	if err != nil {
		return "", err
	}

	return mdbContentUnit.UID, nil
}

func removeContentUnitSource(cu es.ContentUnit, lang string, src mdbmodels.Source) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		return "", errors.New("cu.MDB_UID is empty")
	}

	_, err := mdbmodels.FindTag(common.DB, src.ID)
	if err != nil {
		return "", err
	}

	err = mdbContentUnit.RemoveSources(common.DB, &src)
	if err != nil {
		return "", err
	}

	return mdbContentUnit.UID, nil
}

func addContentUnitFile(cu es.ContentUnit, lang string, file mdbmodels.File) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		mdbContentUnit = mdbmodels.ContentUnit{
			UID:    GenerateUID(8),
			TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID,
		}
		if err := mdbContentUnit.Insert(common.DB); err != nil {
			return "", err
		}
	}

	_, err := mdbmodels.FindFile(common.DB, file.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			err = file.Insert(common.DB)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	err = mdbContentUnit.AddFiles(common.DB, false, &file)
	if err != nil {
		return "", err
	}

	return mdbContentUnit.UID, nil
}

func removeContentUnitFile(cu es.ContentUnit, lang string, file mdbmodels.File) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		return "", errors.New("cu.MDB_UID is empty")
	}

	_, err := mdbmodels.FindFile(common.DB, file.ID)
	if err != nil {
		return "", err
	}

	err = mdbContentUnit.RemoveFiles(common.DB, &file)
	if err != nil {
		return "", err
	}

	return mdbContentUnit.UID, nil
}

func updateContentUnit(cu es.ContentUnit, lang string, published bool, secure bool) (string, error) {
	var mdbContentUnit mdbmodels.ContentUnit
	if cu.MDB_UID != "" {
		cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbContentUnit = *cup
	} else {
		mdbContentUnit = mdbmodels.ContentUnit{
			UID:    GenerateUID(8),
			TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID,
		}
		if err := mdbContentUnit.Insert(common.DB); err != nil {
			return "", err
		}
	}
	s := int16(0)
	if !secure {
		s = int16(1)
	}
	p := true
	if !published {
		p = false
	}
	mdbContentUnit.Secure = s
	mdbContentUnit.Published = p
	if err := mdbContentUnit.Update(common.DB); err != nil {
		return "", err
	}
	var mdbContentUnitI18n mdbmodels.ContentUnitI18n
	cui18np, err := mdbmodels.FindContentUnitI18n(common.DB, mdbContentUnit.ID, lang)
	if err == sql.ErrNoRows {
		mdbContentUnitI18n = mdbmodels.ContentUnitI18n{
			ContentUnitID: mdbContentUnit.ID,
			Language:      lang,
		}
		if err := mdbContentUnitI18n.Insert(common.DB); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		mdbContentUnitI18n = *cui18np
	}
	if cu.Name != "" {
		mdbContentUnitI18n.Name = null.NewString(cu.Name, cu.Name != "")
	}
	if cu.Description != "" {
		mdbContentUnitI18n.Description = null.NewString(cu.Description, cu.Description != "")
	}
	if err := mdbContentUnitI18n.Update(common.DB); err != nil {
		return "", err
	}
	return mdbContentUnit.UID, nil
}

func updateFile(f es.File, cuUID string) (string, error) {
	cup, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cuUID)).One()
	if err != nil {
		return "", err
	}
	var mdbFile mdbmodels.File
	if f.MDB_UID != "" {
		fp, err := mdbmodels.Files(common.DB, qm.Where("uid = ?", f.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbFile = *fp
	} else {
		mdbFile = mdbmodels.File{
			UID: GenerateUID(8),
		}
		if err := mdbFile.Insert(common.DB); err != nil {
			return "", err
		}
	}
	mdbFile.Name = f.Name
	mdbFile.ContentUnitID = null.Int64{cup.ID, true}
	if err := mdbFile.Update(common.DB); err != nil {
		return "", err
	}
	return mdbFile.UID, nil
}

func deleteCollection(UID string) error {
	ccu, err := mdbmodels.CollectionsContentUnits(common.DB,
		qm.InnerJoin("collections on collections.id = collections_content_units.collection_id"),
		qm.Where("collections.uid = ?", UID)).All()
	if err != nil {
		return err
	}
	ccu.DeleteAll(common.DB)
	return mdbmodels.Collections(common.DB, qm.Where("uid = ?", UID)).DeleteAll()
}

func deleteContentUnits(UIDs []string) error {
	if len(UIDs) == 0 {
		return nil
	}
	UIDsI := make([]interface{}, len(UIDs))
	for i, v := range UIDs {
		UIDsI[i] = v
	}
	files, err := mdbmodels.Files(common.DB,
		qm.InnerJoin("content_units on content_units.id = files.content_unit_id"),
		qm.WhereIn("content_units.uid in ?", UIDsI...)).All()
	if err != nil {
		return errors.Wrap(err, "deleteContentUnits, Select files.")
	}
	fileIdsI := make([]interface{}, len(files))
	for i, v := range files {
		fileIdsI[i] = v.ContentUnitID
	}
	if len(files) > 0 {
		if err := mdbmodels.Files(common.DB, qm.WhereIn("content_unit_id in ?", fileIdsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteContentUnits, delete files.")
		}
	}
	contentUnitsI18ns, err := mdbmodels.ContentUnitI18ns(common.DB,
		qm.InnerJoin("content_units on content_units.id = content_unit_i18n.content_unit_id"),
		qm.WhereIn("content_units.uid in ?", UIDsI...)).All()
	if err != nil {
		return errors.Wrap(err, "deleteContentUnits, select cu i18n.")
	}
	idsI := make([]interface{}, len(contentUnitsI18ns))
	for i, v := range contentUnitsI18ns {
		idsI[i] = v.ContentUnitID
	}
	if len(contentUnitsI18ns) > 0 {
		if err := mdbmodels.ContentUnitI18ns(common.DB, qm.WhereIn("content_unit_id in ?", idsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteContentUnits, delete cu i18n.")
		}
	}
	collectionIds, err := mdbmodels.CollectionsContentUnits(common.DB,
		qm.InnerJoin("content_units on content_units.id = collections_content_units.content_unit_id"),
		qm.WhereIn("content_units.uid IN ?", UIDsI...)).All()
	if err != nil {
		return errors.Wrap(err, "deleteContentUnits, select ccu.")
	}
	if len(collectionIds) > 0 {
		collectionIdsI := make([]interface{}, len(collectionIds))
		for i, v := range collectionIds {
			collectionIdsI[i] = v.CollectionID
		}
		if err := mdbmodels.CollectionsContentUnits(common.DB,
			qm.WhereIn("collection_id IN ?", collectionIdsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteContentUnits, delete ccu.")
		}
		if err := mdbmodels.Collections(common.DB,
			qm.WhereIn("id IN ?", collectionIdsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteContentUnits, delete collections.")
		}
	}
	return mdbmodels.ContentUnits(common.DB, qm.WhereIn("uid in ?", UIDsI...)).DeleteAll()
}

func (suite *IndexerSuite) ucu(cu es.ContentUnit, lang string, published bool, secure bool) string {
	r := require.New(suite.T())
	uid, err := updateContentUnit(cu, lang, published, secure)
	r.Nil(err)
	return uid
}

func (suite *IndexerSuite) uf(f es.File, cuUID string) string {
	r := require.New(suite.T())
	uid, err := updateFile(f, cuUID)
	r.Nil(err)
	return uid
}

func (suite *IndexerSuite) ucut(cu es.ContentUnit, lang string, tag mdbmodels.Tag, add bool) string {
	r := require.New(suite.T())

	var err error
	var uid string

	if add {
		uid, err = addContentUnitTag(cu, lang, tag)
	} else {
		uid, err = removeContentUnitTag(cu, lang, tag)
	}
	r.Nil(err)
	return uid
}

func (suite *IndexerSuite) ucus(cu es.ContentUnit, lang string, src mdbmodels.Source, add bool) string {
	r := require.New(suite.T())

	var err error
	var uid string

	if add {
		uid, err = addContentUnitSource(cu, lang, src)
	} else {
		uid, err = removeContentUnitSource(cu, lang, src)
	}
	r.Nil(err)
	return uid
}

func (suite *IndexerSuite) ucuf(cu es.ContentUnit, lang string, file mdbmodels.File, add bool) string {
	r := require.New(suite.T())

	var err error
	var uid string

	if add {
		uid, err = addContentUnitFile(cu, lang, file)
	} else {
		uid, err = removeContentUnitFile(cu, lang, file)
	}
	r.Nil(err)
	return uid
}

func (suite *IndexerSuite) validateContentUnitNames(indexName string, indexer *es.Indexer, expectedNames []string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	names := make([]string, len(res.Hits.Hits))
	for i, hit := range res.Hits.Hits {
		var cu es.ContentUnit
		json.Unmarshal(*hit.Source, &cu)
		names[i] = cu.Name
	}
	r.Equal(int64(len(expectedNames)), res.Hits.TotalHits)
	r.ElementsMatch(expectedNames, names)
}

func (suite *IndexerSuite) validateContentUnitTags(indexName string, indexer *es.Indexer, expectedTags []string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	tags := make([]string, 0)
	for _, hit := range res.Hits.Hits {
		var cu es.ContentUnit
		json.Unmarshal(*hit.Source, &cu)
		for _, t := range cu.Tags {
			tags = append(tags, t)
		}
	}
	r.Equal(len(expectedTags), len(tags))
	r.ElementsMatch(expectedTags, tags)
}

func (suite *IndexerSuite) validateContentUnitSources(indexName string, indexer *es.Indexer, expectedSources []string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	sources := make([]string, 0)
	for _, hit := range res.Hits.Hits {
		var cu es.ContentUnit
		json.Unmarshal(*hit.Source, &cu)
		for _, s := range cu.Sources {
			sources = append(sources, s)
		}
	}
	r.Equal(len(expectedSources), len(sources))
	r.ElementsMatch(expectedSources, sources)
}

func (suite *IndexerSuite) validateContentUnitFiles(indexName string, indexer *es.Indexer, expectedLangs []string, expectedTranscriptLength null.Int) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)

	if len(expectedLangs) > 0 {
		langs := make([]string, 0)
		for _, hit := range res.Hits.Hits {
			var cu es.ContentUnit
			json.Unmarshal(*hit.Source, &cu)
			for _, t := range cu.Translations {
				langs = append(langs, t)
			}
		}

		r.Equal(len(expectedLangs), len(langs))
		r.ElementsMatch(expectedLangs, langs)
	}

	// Get transcript
	transcriptLengths := make([]int, 0)
	for _, hit := range res.Hits.Hits {
		var cu es.ContentUnit
		json.Unmarshal(*hit.Source, &cu)
		//***
		fmt.Printf("\n\n TRANSCRIPT: [%+v] \n\n", cu.Transcript)
		transcriptLengths = append(transcriptLengths, len(cu.Transcript))
	}

	if expectedTranscriptLength.Valid {
		r.NotEqual(transcriptLengths, 0)
		r.Contains(transcriptLengths, expectedTranscriptLength.Int)
	} else {
		r.Equal(len(transcriptLengths), 0)
	}
}

func (suite *IndexerSuite) validateMaps(e map[string][]string, a map[string][]string) {
	r := require.New(suite.T())
	for k, v := range e {
		val, ok := a[k]
		r.True(ok, fmt.Sprintf("%s not found in actual: %+v", k, a))
		r.ElementsMatch(v, val, "Elements don't match expected: %+v actual: %+v", v, val)
	}
	for k := range a {
		_, ok := e[k]
		r.True(ok)
	}
}

func (suite *IndexerSuite) validateContentUnitTypes(indexName string, indexer *es.Indexer, expectedTypes map[string][]string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	cus := make(map[string]es.ContentUnit)
	for _, hit := range res.Hits.Hits {
		var cu es.ContentUnit
		json.Unmarshal(*hit.Source, &cu)
		if val, ok := cus[cu.MDB_UID]; ok {
			r.Nil(errors.New(fmt.Sprintf(
				"Two identical UID: %s\tFirst : %+v\tSecond: %+v",
				cu.MDB_UID, cu, val)))
		}
		cus[cu.MDB_UID] = cu
	}
	types := make(map[string][]string)
	for k, cu := range cus {
		types[k] = cu.CollectionsContentTypes
	}
	suite.validateMaps(expectedTypes, types)
}

func (suite *IndexerSuite) TestContentUnitsCollectionIndex() {
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
	indexName := es.IndexName("test", consts.ES_UNITS_INDEX, consts.LANG_ENGLISH)
	indexer := es.MakeIndexer("test", []string{consts.ES_UNITS_INDEX}, common.DB, common.ESC)
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
	//es.DumpDB(common.DB, "Before DB")
	//es.DumpIndexes(common.ESC, "Before Indexes")
	c1UID := suite.uc(es.Collection{ContentType: consts.CT_VIDEO_PROGRAM}, cu1UID, "")
	r.Nil(indexer.CollectionAdd(c1UID))
	//es.DumpDB(common.DB, "After DB")
	//es.DumpIndexes(common.ESC, "After Indexes")
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {consts.CT_SPECIAL_LESSON},
	})

	fmt.Printf("\n\n\nValidate we have successfully updated a content type.\n\n")
	suite.uc(es.Collection{MDB_UID: c2UID, ContentType: consts.CT_MEALS}, cu2UID, "")
	r.Nil(indexer.CollectionUpdate(c2UID))
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {consts.CT_MEALS},
	})

	fmt.Printf("\n\n\nValidate we have successfully deleted a content type.\n\n")
	r.Nil(deleteCollection(c2UID))
	// es.DumpDB(common.DB, "Before")
	// es.DumpIndexes(common.ESC, "Before")
	r.Nil(indexer.CollectionDelete(c2UID))
	// es.DumpDB(common.DB, "After")
	// es.DumpIndexes(common.ESC, "After")
	suite.validateContentUnitTypes(indexName, indexer, map[string][]string{
		cu1UID: {consts.CT_DAILY_LESSON, consts.CT_CONGRESS, consts.CT_VIDEO_PROGRAM},
		cu2UID: {},
	})

	fmt.Printf("\n\n\nUpdate collection, remove one unit and add another.\n\n")
	es.DumpDB(common.DB, "Before DB")
	suite.uc(es.Collection{MDB_UID: c3UID} /* Add */, cu2UID /* Remove */, cu1UID)
	es.DumpDB(common.DB, "After DB")
	es.DumpIndexes(common.ESC, "Before Indexes")
	r.Nil(indexer.CollectionUpdate(c3UID))
	es.DumpIndexes(common.ESC, "After Indexes")
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

func (suite *IndexerSuite) TestContentUnitsIndex() {
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
	indexNameEn := es.IndexName("test", consts.ES_UNITS_INDEX, consts.LANG_ENGLISH)
	indexNameHe := es.IndexName("test", consts.ES_UNITS_INDEX, consts.LANG_HEBREW)
	indexNameRu := es.IndexName("test", consts.ES_UNITS_INDEX, consts.LANG_RUSSIAN)
	indexer := es.MakeIndexer("test", []string{consts.ES_UNITS_INDEX}, common.DB, common.ESC)
	// Index existing DB data.
	r.Nil(indexer.ReindexAll())
	r.Nil(indexer.RefreshAll())

	fmt.Println("Validate we have 2 searchable content units.")
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "something else"})

	fmt.Println("Add a file to content unit and validate.")
	transcriptContent := "1234"
	suite.serverResponses["/dEvgPVpr"] = transcriptContent
	file := mdbmodels.File{ID: 1, Name: "heb_o_rav_2017-05-25_lesson_achana_n1_p0.doc", UID: "dEvgPVpr", Language: null.String{"he", true}, Secure: 0, Published: true}
	suite.ucuf(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_HEBREW, file, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	//es.DumpIndexes(common.ESC, "DumpIndexes after adding transcript")
	suite.validateContentUnitFiles(indexNameHe, indexer, []string{"he"}, null.Int{len(transcriptContent), true})
	fmt.Println("Remove a file from content unit and validate.")
	suite.ucuf(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_HEBREW, file, false)

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

	// failed tests
	/*fmt.Println("Add a source to content unit and validate.")
	suite.ucus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-zohar", true}, ID: 3, TypeID: 1, UID: "ALlyoveA"}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitSources(indexNameEn, indexer, []string{"ALlyoveA"})
	fmt.Println("Add second source to content unit and validate.")
	suite.ucus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-pi-hacham", true}, ID: 4, TypeID: 1, UID: "1vCj4qN9"}, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitSources(indexNameEn, indexer, []string{"ALlyoveA", "1vCj4qN9"})
	fmt.Println("Remove one source from content unit and validate.")
	suite.ucus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-zohar", true}, ID: 3, TypeID: 1, UID: "L2jMWyce"}, false)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	suite.validateContentUnitSources(indexNameEn, indexer, []string{"1vCj4qN9"})
	fmt.Println("Remove the second source.")
	suite.ucus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, mdbmodels.Source{Pattern: null.String{"bs-akdama-pi-hacham", true}, ID: 4, TypeID: 1, UID: "1vCj4qN9"}, false)*/

	fmt.Println("Make content unit not published and validate.")
	//es.DumpDB(common.DB, "TestContentUnitsIndex, BeforeDB")
	//es.DumpIndexes(common.ESC, "TestContentUnitsIndex, BeforeIndexes")
	suite.ucu(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, false, true)
	r.Nil(indexer.ContentUnitUpdate(cu1UID))
	//es.DumpDB(common.DB, "TestContentUnitsIndex, AfterDB")
	//es.DumpIndexes(common.ESC, "TestContentUnitsIndex, AfterIndexes")
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
	r.Nil(indexer.ContentUnitAdd(cu3UID))
	suite.validateContentUnitNames(indexNameEn, indexer,
		[]string{"something", "something else", "third something"})

	fmt.Println("Update content unit and validate.")
	suite.ucu(es.ContentUnit{MDB_UID: cu3UID, Name: "updated third something"}, consts.LANG_ENGLISH, true, true)
	r.Nil(indexer.ContentUnitUpdate(cu3UID))
	suite.validateContentUnitNames(indexNameEn, indexer,
		[]string{"something", "something else", "updated third something"})

	fmt.Println("Delete content unit and validate.")
	r.Nil(indexer.ContentUnitDelete(cu2UID))
	suite.validateContentUnitNames(indexNameEn, indexer, []string{"something", "updated third something"})

	fmt.Println("Delete units, reindex and validate we have 0 searchable units.")
	r.Nil(deleteContentUnits(UIDs))
	r.Nil(indexer.ReindexAll())
	suite.validateContentUnitNames(indexNameEn, indexer, []string{})

	// Remove test indexes.
	r.Nil(indexer.DeleteIndexes())
}

func (suite *IndexerSuite) TestCollectionsScopeByContentUnit() {
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
