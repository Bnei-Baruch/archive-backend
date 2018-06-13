package es_test

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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

func TestIndexer(t *testing.T) {
	suite.Run(t, new(UnitsIndexerSuite))
	suite.Run(t, new(SourcesIndexerSuite))
}

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

	fmt.Println("Replace sources folder with temp. path.")
	testingsSourcesFolder := viper.GetString("test.test-sources-folder")
	viper.Set("elasticsearch.sources-folder", testingsSourcesFolder)

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

// MOVE THIS TO EACH RESULT TYPE TEST!
// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
// func TestIndexer(t *testing.T) {
// 	suite.Run(t, new(IndexerSuite))
// }

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
	indexer, err := es.MakeIndexer("test", []string{consts.ES_RESULT_TYPE_UNITS}, common.DB, common.ESC)
	r.Nil(err)
	r.Nil(indexer.DeleteIndexes())
	// Delete test directory
	os.RemoveAll(viper.GetString("test.test-docx-folder"))
	utils.Must(os.MkdirAll(viper.GetString("test.test-docx-folder"), 0777))
	utils.Must(os.MkdirAll(viper.GetString("test.test-sources-folder"), 0777))
}

func updateCollection(c es.Collection, cuUID string, removeContentUnitUID string) (string, error) {
	var mdbCollection mdbmodels.Collection
	if c.MDB_UID != "" {
		cp, err := mdbmodels.Collections(common.DB, qm.Where("uid = ?", c.MDB_UID)).One()
		if err != nil {
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
	if c.ContentType != "" {
		mdbCollection.TypeID = mdb.CONTENT_TYPE_REGISTRY.ByName[c.ContentType].ID
	}
	mdbCollection.Secure = int16(0)
	mdbCollection.Published = true
	if err := mdbCollection.Update(common.DB); err != nil {
		return "", err
	}
	// I18N
	var mdbCollectionI18n mdbmodels.CollectionI18n
	lang := consts.LANG_ENGLISH
	ci18np, err := mdbmodels.FindCollectionI18n(common.DB, mdbCollection.ID, lang)
	if err == sql.ErrNoRows {
		mdbCollectionI18n = mdbmodels.CollectionI18n{
			CollectionID: mdbCollection.ID,
			Language:     lang,
		}
		if err := mdbCollectionI18n.Insert(common.DB); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		mdbCollectionI18n = *ci18np
	}
	if c.Name != "" {
		mdbCollectionI18n.Name = null.NewString(c.Name, c.Name != "")
	}
	if c.Description != "" {
		mdbCollectionI18n.Description = null.NewString(c.Description, c.Description != "")
	}
	if err := mdbCollectionI18n.Update(common.DB); err != nil {
		return "", err
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

func addContentUnitSource(cu es.ContentUnit, lang string, src mdbmodels.Source, author mdbmodels.Author, insertAuthor bool) (string, error) {
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
			err = src.AddAuthors(common.DB, insertAuthor, &author)
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

	_, err := mdbmodels.FindSource(common.DB, src.ID)
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

	return file.UID, nil
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

	return file.UID, nil
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
	collectionsI18ns, err := mdbmodels.CollectionI18ns(common.DB,
		qm.InnerJoin("collections on collections.id = collection_i18n.collection_id"),
		qm.WhereIn("collections.uid = ?", UID)).All()
	if err != nil {
		return errors.Wrap(err, "deleteCollections, select cu i18n.")
	}
	idsI := make([]interface{}, len(collectionsI18ns))
	for i, v := range collectionsI18ns {
		idsI[i] = v.CollectionID
	}
	if len(collectionsI18ns) > 0 {
		if err := mdbmodels.CollectionI18ns(common.DB, qm.WhereIn("collection_id in ?", idsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteCollections, delete cu i18n.")
		}
	}
	ccu, err := mdbmodels.CollectionsContentUnits(common.DB,
		qm.InnerJoin("collections on collections.id = collections_content_units.collection_id"),
		qm.Where("collections.uid = ?", UID)).All()
	if err != nil {
		return err
	}
	ccu.DeleteAll(common.DB)
	return mdbmodels.Collections(common.DB, qm.Where("uid = ?", UID)).DeleteAll()
}

func deleteCollections(UIDs []string) error {
	for _, uid := range UIDs {
		err := deleteCollection(uid)
		if err != nil {
			return err
		}
	}
	return nil
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
		if err := mdbmodels.CollectionI18ns(common.DB,
			qm.WhereIn("collection_id IN ?", collectionIdsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteContentUnitw, delete c i18n.")
		}
		if err := mdbmodels.Collections(common.DB,
			qm.WhereIn("id IN ?", collectionIdsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteContentUnits, delete collections.")
		}
	}
	return mdbmodels.ContentUnits(common.DB, qm.WhereIn("uid in ?", UIDsI...)).DeleteAll()
}

func deleteSources(UIDs []string) error {
	if len(UIDs) == 0 {
		return nil
	}
	UIDsI := make([]interface{}, len(UIDs))
	for i, v := range UIDs {
		UIDsI[i] = v
	}
	sourcesI18ns, err := mdbmodels.SourceI18ns(common.DB,
		qm.InnerJoin("sources on sources.id = source_i18n.source_id"),
		qm.WhereIn("sources.uid in ?", UIDsI...)).All()
	if err != nil {
		return errors.Wrap(err, "deleteSources, select source i18n.")
	}
	idsI := make([]interface{}, len(sourcesI18ns))
	for i, v := range sourcesI18ns {
		idsI[i] = v.SourceID
	}
	if len(sourcesI18ns) > 0 {
		if err := mdbmodels.SourceI18ns(common.DB, qm.WhereIn("source_id in ?", idsI...)).DeleteAll(); err != nil {
			return errors.Wrap(err, "deleteSources, delete source i18n.")
		}
	}
	// Delete authors_sources and authors
	sources, err := mdbmodels.Sources(common.DB, qm.WhereIn("sources.uid in ?", UIDsI...)).All()
	if err != nil {
		return err
	}
	sourcesIdsI := make([]interface{}, 0)
	for _, s := range sources {
		sourcesIdsI = append(sourcesIdsI, s.ID)
	}
	authors, err := mdbmodels.Authors(common.DB,
		qm.InnerJoin("authors_sources on authors_sources.author_id = authors.id"),
		qm.WhereIn("authors_sources.source_id in ?", sourcesIdsI...)).All()
	if err != nil {
		return errors.Wrap(err, "Select authors")
	}
	if len(authors) > 0 {
		for _, s := range sources {
			err = s.RemoveAuthors(common.DB, authors...)
			if err != nil {
				return errors.Wrap(err, "RemoveAuthors")
			}
		}
		err = authors.DeleteAll(common.DB)
		if err != nil {
			return errors.Wrap(err, "deleteSources, delete authors.")
		}
	}
	return mdbmodels.Sources(common.DB, qm.WhereIn("uid in ?", UIDsI...)).DeleteAll()
}

func updateSource(source es.Source, lang string) (string, error) {
	var mdbSource mdbmodels.Source
	if source.MDB_UID != "" {
		s, err := mdbmodels.Sources(common.DB, qm.Where("uid = ?", source.MDB_UID)).One()
		if err != nil {
			return "", err
		}
		mdbSource = *s
	} else {
		mdbSource = mdbmodels.Source{
			UID:    GenerateUID(8),
			TypeID: 2,
			Name:   source.Name,
		}
		if source.Description != "" {
			mdbSource.Description = null.NewString(source.Description, source.Description != "")
		}
		if err := mdbSource.Insert(common.DB); err != nil {
			return "", err
		}
	}
	var mdbSourceI18n mdbmodels.SourceI18n
	source18np, err := mdbmodels.FindSourceI18n(common.DB, mdbSource.ID, lang)
	if err == sql.ErrNoRows {
		mdbSourceI18n = mdbmodels.SourceI18n{
			SourceID: mdbSource.ID,
			Language: lang,
		}
		if err := mdbSourceI18n.Insert(common.DB); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		mdbSourceI18n = *source18np
	}
	if source.Name != "" {
		mdbSourceI18n.Name = null.NewString(source.Name, source.Name != "")
	}
	if source.Description != "" {
		mdbSourceI18n.Description = null.NewString(source.Description, source.Description != "")
	}
	if err := mdbSourceI18n.Update(common.DB); err != nil {
		return "", err
	}

	//add folder for source files
	folder, err := es.SourcesFolder()
	if err != nil {
		return "", err
	}
	uidPath := path.Join(folder, mdbSource.UID)
	if _, err := os.Stat(uidPath); os.IsNotExist(err) {
		err = os.MkdirAll(uidPath, os.FileMode(0775))
		if err != nil {
			return "", err
		}
	}

	return mdbSource.UID, nil
}

func updateSourceFileContent(uid string, lang string) error {
	folder, err := es.SourcesFolder()
	if err != nil {
		return err
	}
	uidPath := path.Join(folder, uid)
	jsonPath := path.Join(uidPath, "index.json")
	contentFileName := fmt.Sprintf("sample-content-%s.docx", lang)
	contentPath := path.Join(uidPath, contentFileName)
	m := make(map[string]map[string]string)

	if _, err := os.Stat(jsonPath); err == nil {
		jsonCnt, err := ioutil.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("Unable to read from file %s. Error: %+v", jsonPath, err)
		}
		err = json.Unmarshal(jsonCnt, &m)
		if err != nil {
			return err
		}
	}

	m[lang] = make(map[string]string)
	m[lang]["docx"] = contentFileName

	newJsonCnt, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("Cannot serialize to Json. Error: %+v", err)
	}

	err = ioutil.WriteFile(jsonPath, newJsonCnt, 0666)
	if err != nil {
		return fmt.Errorf("Unable to write into file %s. Error: %+v", jsonPath, err)
	}

	fileToCopy := viper.GetString("test.test-source-content-docx")
	data, err := ioutil.ReadFile(fileToCopy)
	if err != nil {
		return fmt.Errorf("Unable to read file %s. Error: %+v", fileToCopy, err)
	}
	err = ioutil.WriteFile(contentPath, data, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write into file %s. Error: %+v", contentPath, err)
	}

	return nil
}

func addAuthorToSource(source es.Source, lang string, mdbAuthor mdbmodels.Author, insertAuthor bool, insertI18n bool) error {
	var mdbSource mdbmodels.Source
	if source.MDB_UID != "" {
		src, err := mdbmodels.Sources(common.DB, qm.Where("uid = ?", source.MDB_UID)).One()
		if err != nil {
			return err
		}
		mdbSource = *src
	} else {
		mdbSource = mdbmodels.Source{
			UID:    GenerateUID(8),
			TypeID: 2,
		}
		if err := mdbSource.Insert(common.DB); err != nil {
			return err
		}
	}

	err := mdbSource.AddAuthors(common.DB, insertAuthor, &mdbAuthor)
	if err != nil {
		return err
	}

	if insertI18n {
		var mdbAuthorI18n mdbmodels.AuthorI18n
		author18n, err := mdbmodels.FindAuthorI18n(common.DB, mdbAuthor.ID, lang)
		if err == sql.ErrNoRows {
			mdbAuthorI18n = mdbmodels.AuthorI18n{
				AuthorID: mdbAuthor.ID,
				Language: lang,
			}
			if err := mdbAuthorI18n.Insert(common.DB); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			mdbAuthorI18n = *author18n
		}
		if mdbAuthor.Name != "" {
			mdbAuthorI18n.Name = null.NewString(mdbAuthor.Name, mdbAuthor.Name != "")
		}
		if mdbAuthor.FullName.Valid && mdbAuthor.FullName.String != "" {
			mdbAuthorI18n.FullName = null.NewString(mdbAuthor.FullName.String, true)
		}
		if err := mdbAuthorI18n.Update(common.DB); err != nil {
			return err
		}
	}

	return nil
}

func removeAuthorFromSource(source es.Source, mdbAuthor mdbmodels.Author) error {
	var mdbSource mdbmodels.Source
	if source.MDB_UID != "" {
		src, err := mdbmodels.Sources(common.DB, qm.Where("uid = ?", source.MDB_UID)).One()
		if err != nil {
			return err
		}
		mdbSource = *src
	} else {
		return errors.New("source.MDB_UID is empty")
	}

	_, err := mdbmodels.FindAuthor(common.DB, mdbAuthor.ID)
	if err != nil {
		return err
	}

	err = mdbSource.RemoveAuthors(common.DB, &mdbAuthor)
	if err != nil {
		return err
	}

	return nil
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

func (suite *IndexerSuite) acus(cu es.ContentUnit, lang string, src mdbmodels.Source, author mdbmodels.Author, insertAuthor bool) string {
	r := require.New(suite.T())

	var err error
	var uid string

	uid, err = addContentUnitSource(cu, lang, src, author, insertAuthor)
	r.Nil(err)
	return uid
}

func (suite *IndexerSuite) rcus(cu es.ContentUnit, lang string, src mdbmodels.Source) string {
	r := require.New(suite.T())

	var err error
	var uid string

	uid, err = removeContentUnitSource(cu, lang, src)

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

//update source
func (suite *IndexerSuite) us(source es.Source, lang string) string {
	r := require.New(suite.T())
	uid, err := updateSource(source, lang)
	r.Nil(err)
	return uid
}

//update source file content
func (suite *IndexerSuite) usfc(uid string, lang string) {
	r := require.New(suite.T())
	err := updateSourceFileContent(uid, lang)
	r.Nil(err)
}

//add source author
func (suite *IndexerSuite) asa(source es.Source, lang string, mdbAuthor mdbmodels.Author, insertAuthor bool, insertI18n bool) {
	r := require.New(suite.T())
	err := addAuthorToSource(source, lang, mdbAuthor, insertAuthor, insertI18n)
	r.Nil(err)
}

//remove source author
func (suite *IndexerSuite) rsa(source es.Source, mdbAuthor mdbmodels.Author) {
	r := require.New(suite.T())
	err := removeAuthorFromSource(source, mdbAuthor)
	r.Nil(err)
}

func (suite *IndexerSuite) validateCollectionsContentUnits(indexName string, indexer *es.Indexer, expectedCUs map[string][]string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	cus := make(map[string][]string)
	for _, hit := range res.Hits.Hits {
		var c es.Collection
		json.Unmarshal(*hit.Source, &c)
		uids, err := es.KeyValuesToValues("content_unit", c.TypedUIDs)
		r.Nil(err)
		if val, ok := cus[c.MDB_UID]; ok {
			r.Nil(errors.New(fmt.Sprintf(
				"Two identical UID: %s\tFirst : %+v\tSecond: %+v",
				c.MDB_UID, c, val)))
		}
		cus[c.MDB_UID] = uids
	}
	suite.validateMaps(expectedCUs, cus)
}

func (suite *IndexerSuite) validateContentUnitNames(indexName string, indexer *es.Indexer, expectedNames []string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	log.Infof("indexName: %+v", indexName)
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	if err != nil {
		log.Infof("Error: %+v", err.Error())
		r.Nil(err)
	}
	names := make([]string, len(res.Hits.Hits))
	for i, hit := range res.Hits.Hits {
		var cu es.Result
		json.Unmarshal(*hit.Source, &cu)
		names[i] = cu.Title
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
		var cu es.Result
		json.Unmarshal(*hit.Source, &cu)
		hitTags, err := es.KeyValuesToValues("tag", cu.FilterValues)
		r.Nil(err)
		tags = append(tags, hitTags...)
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
		var cu es.Result
		json.Unmarshal(*hit.Source, &cu)
		hitSources, err := es.KeyValuesToValues("source", cu.FilterValues)
		r.Nil(err)
		sources = append(sources, hitSources...)
	}
	r.Equal(len(expectedSources), len(sources))
	r.ElementsMatch(expectedSources, sources)
}

func (suite *IndexerSuite) validateContentUnitFiles(indexName string, indexer *es.Indexer, expectedTranscriptLength null.Int) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)

	// if len(expectedLangs) > 0 {
	// 	langs := make([]string, 0)
	// 	for _, hit := range res.Hits.Hits {
	// 		var cu es.Result
	// 		json.Unmarshal(*hit.Source, &cu)
	// 		for _, t := range cu.Translations {
	// 			langs = append(langs, t)
	// 		}
	// 	}
	//
	// 	r.Equal(len(expectedLangs), len(langs))
	// 	r.ElementsMatch(expectedLangs, langs)
	// }

	// Get transcript,
	transcriptLengths := make([]int, 0)
	for _, hit := range res.Hits.Hits {
		var cu es.Result
		json.Unmarshal(*hit.Source, &cu)
		if cu.Content != "" {
			transcriptLengths = append(transcriptLengths, len(cu.Content))
		}
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

// Searches content units index and validates their types with expected types.
func (suite *IndexerSuite) validateContentUnitTypes(indexName string, indexer *es.Indexer, expectedTypes map[string][]string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	cus := make(map[string]es.Result)
	for _, hit := range res.Hits.Hits {
		var cu es.Result
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
		collectionsContentTypes, err := es.KeyValuesToValues("collections_content_type", cu.FilterValues)
		r.Nil(err)
		types[k] = collectionsContentTypes
	}
	suite.validateMaps(expectedTypes, types)

}

func (suite *IndexerSuite) validateSourceNames(indexName string, indexer *es.Indexer, expectedNames []string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	names := make([]string, len(res.Hits.Hits))
	matched := make([]string, 0)

	for i, hit := range res.Hits.Hits {
		var src es.Result
		json.Unmarshal(*hit.Source, &src)
		names[i] = src.Title
	}

	for _, name := range names {
		for _, expected := range expectedNames {
			if strings.Contains(name, expected) {
				exist := false
				for _, m := range matched {
					if m == expected {
						exist = true
						break
					}
				}
				if !exist {
					matched = append(matched, expected)
				}
			}
		}
	}

	r.Equal(len(expectedNames), len(matched))
	r.ElementsMatch(expectedNames, matched)
}

func (suite *IndexerSuite) validateSourcesFullPath(indexName string, indexer *es.Indexer, expectedSources []string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	sources := make([]string, 0)
	for _, hit := range res.Hits.Hits {
		var src es.Result
		json.Unmarshal(*hit.Source, &src)
		for _, s := range src.FilterValues {
			sources = append(sources, s)
		}
	}
	formatedSources, err := es.KeyValuesToValues("source", sources)
	r.Nil(err)
	r.Equal(len(expectedSources), len(formatedSources))
	r.ElementsMatch(expectedSources, formatedSources)
}

func (suite *IndexerSuite) validateSourceFile(indexName string, indexer *es.Indexer, expectedContentsByNames map[string]string) {
	r := require.New(suite.T())
	err := indexer.RefreshAll()
	r.Nil(err)
	var res *elastic.SearchResult
	res, err = common.ESC.Search().Index(indexName).Do(suite.ctx)
	r.Nil(err)
	contentsByNames := make(map[string]string)
	for _, hit := range res.Hits.Hits {
		var src es.Result
		json.Unmarshal(*hit.Source, &src)
		contentsByNames[src.MDB_UID] = src.Content
	}

	r.True(reflect.DeepEqual(expectedContentsByNames, contentsByNames))
}

// func (suite *IndexerSuite) TestCollectionsScopeByContentUnit() {
// 	// Add test for collection for multiple content units.
// 	r := require.New(suite.T())
// 	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
// 	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
// 	c1UID := suite.uc(es.Collection{ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")
// 	c2UID := suite.uc(es.Collection{ContentType: consts.CT_CONGRESS}, cu1UID, "")
// 	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
// 	suite.uc(es.Collection{ContentType: consts.CT_SPECIAL_LESSON}, cu2UID, "")
//
// 	// dumpDB("TestCollectionsScopeByContentUnit")
//
// 	uids, err := es.CollectionsScopeByContentUnit(common.DB, cu1UID)
// 	r.Nil(err)
// 	r.ElementsMatch([]string{c2UID, c1UID}, uids)
// }
//
// func (suite *IndexerSuite) TestCollectionsScopeByFile() {
// 	// Add test for collection for multiple content units.
// 	r := require.New(suite.T())
// 	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
// 	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
// 	c1UID := suite.uc(es.Collection{ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")
// 	c2UID := suite.uc(es.Collection{ContentType: consts.CT_CONGRESS}, cu1UID, "")
// 	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
// 	suite.uc(es.Collection{ContentType: consts.CT_SPECIAL_LESSON}, cu2UID, "")
// 	f1UID := suite.uf(es.File{Name: "f1"}, cu1UID)
// 	suite.uf(es.File{Name: "f2"}, cu1UID)
// 	suite.uf(es.File{Name: "f3"}, cu2UID)
// 	suite.uf(es.File{Name: "f4"}, cu2UID)
//
// 	uids, err := es.CollectionsScopeByFile(common.DB, f1UID)
// 	r.Nil(err)
// 	r.ElementsMatch([]string{c2UID, c1UID}, uids)
// }
// func (suite *IndexerSuite) TestCollectionsIndex() {
// 	// Add test for collection for multiple content units.
// 	r := require.New(suite.T())
// 	fmt.Printf("\n\n\nAdding content units and collections.\n\n")
// 	cu1UID := suite.ucu(es.ContentUnit{Name: "something"}, consts.LANG_ENGLISH, true, true)
// 	c1UID := suite.uc(es.Collection{Name: "c1", ContentType: consts.CT_VIDEO_PROGRAM}, cu1UID, "")
// 	c2UID := suite.uc(es.Collection{Name: "c2", ContentType: consts.CT_CONGRESS}, cu1UID, "")
// 	c3UID := suite.uc(es.Collection{Name: "c3", ContentType: consts.CT_DAILY_LESSON}, cu1UID, "")
//
// 	fmt.Printf("\n\n\nReindexing everything.\n\n")
// 	indexName := es.IndexName("test", consts.ES_COLLECTIONS_INDEX, consts.LANG_ENGLISH)
// 	indexer := es.MakeIndexer("test", []string{consts.ES_COLLECTIONS_INDEX}, common.DB, common.ESC)
//
// 	// Index existing DB data.
// 	r.Nil(indexer.ReindexAll())
// 	r.Nil(indexer.RefreshAll())
// 	fmt.Printf("\n\n\nValidate we have 2 searchable collections with proper content units.\n\n")
// 	r.Nil(es.DumpDB(common.DB, "Before validation"))
// 	r.Nil(es.DumpIndexes(common.ESC, "Before validation", consts.ES_COLLECTIONS_INDEX))
// 	suite.validateCollectionsContentUnits(indexName, indexer, map[string][]string{
// 		c1UID: {cu1UID},
// 		c2UID: {cu1UID},
// 	})
//
// 	fmt.Println("Update collection content unit and validate.")
// 	cu2UID := suite.ucu(es.ContentUnit{Name: "something else"}, consts.LANG_ENGLISH, true, true)
// 	r.Nil(indexer.ContentUnitUpdate(cu2UID))
// 	suite.uc(es.Collection{MDB_UID: c2UID}, cu2UID, "")
// 	r.Nil(indexer.CollectionUpdate(c2UID))
// 	suite.validateCollectionsContentUnits(indexName, indexer, map[string][]string{
// 		c1UID: {cu1UID},
// 		c2UID: {cu1UID, cu2UID},
// 	})
//
// 	fmt.Println("Delete collections, reindex and validate we have 0 searchable units.")
// 	r.Nil(deleteCollections([]string{c1UID, c2UID, c3UID}))
// 	r.Nil(indexer.ReindexAll())
// 	suite.validateCollectionsContentUnits(indexName, indexer, map[string][]string{})
//
// 	//fmt.Println("Restore docx-folder path to original.")
// 	//mdb.DocFolder = originalDocxPath
//
// 	// Remove test indexes.
// 	r.Nil(indexer.DeleteIndexes())
// }
