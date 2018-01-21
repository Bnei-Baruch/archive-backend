package es

import (
	"context"
	"crypto/sha1"
	"database/sql"
    "encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/olivere/elastic.v5"
	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
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
	esc *elastic.Client
	ctx context.Context
}

func (suite *IndexerSuite) SetupSuite() {
	utils.InitConfig("", "../")
	err := suite.InitTestDB()
	if err != nil {
		panic(err)
	}
	suite.ctx = context.Background()

	// Set package db and esc variables.
	mdb.InitWithDefault(suite.DB)
	// Show all SQLs
	boil.DebugMode = false  // true
	suite.esc = mdb.ESC
}

func (suite *IndexerSuite) TearDownSuite() {
	// Close connections.
	mdb.Shutdown()
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

func addContentUnit(cu ContentUnit) (string, error) {
	mdbContentUnit := mdbmodels.ContentUnit{
		UID:       GenerateUID(8),
		Secure:    0,
		Published: true,
		TypeID:    mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID,
		// Properties: film_date, ...
	}
	if err := mdbContentUnit.Insert(mdb.DB); err != nil {
		return "", err
	}
	mdbContentUnitI18n := mdbmodels.ContentUnitI18n{
		ContentUnitID: mdbContentUnit.ID,
		Language:      "en",
		Name:          null.NewString(cu.Name, cu.Name != ""),
		Description:   null.NewString(cu.Description, cu.Description != ""),
	}
	if err := mdbContentUnitI18n.Insert(mdb.DB); err != nil {
		return "", err
	}
	return mdbContentUnit.UID, nil
}

func updateContentUnit(cu ContentUnit) (string, error) {
    mdbContentUnit, err := mdbmodels.ContentUnits(mdb.DB, qm.Where("uid = ?", cu.MDB_UID)).One()
    if err != nil {
        return "", err
    }
    // Missing update fields. For now update only i18ns.
	// if err := mdbContentUnit.Update(mdb.DB); err != nil {
	// 	return "", err
	// }
	mdbContentUnitI18n, err := mdbmodels.FindContentUnitI18n(mdb.DB, mdbContentUnit.ID, "en")
    if err != nil {
        return "", err
    }
    mdbContentUnitI18n.Name = null.NewString(cu.Name, cu.Name != "")
    mdbContentUnitI18n.Description = null.NewString(cu.Description, cu.Description != "")
	if err := mdbContentUnitI18n.Update(mdb.DB); err != nil {
		return "", err
	}
	return mdbContentUnit.UID, nil

}

func deleteContentUnits(UIDs []string) error {
	UIDsI := make([]interface{}, len(UIDs))
	for i, v := range UIDs {
		UIDsI[i] = v
	}
	ids, err := mdbmodels.ContentUnitI18ns(mdb.DB, qm.Select("content_unit_id"),
		qm.InnerJoin("content_units on content_units.id = content_unit_i18n.content_unit_id"),
		qm.WhereIn("content_units.uid in ?", UIDsI...)).All()
	if err != nil {
		return err
	}
	idsI := make([]interface{}, len(ids))
	for i, v := range ids {
		idsI[i] = v.ContentUnitID
	}
	if err := mdbmodels.ContentUnitI18ns(mdb.DB,
		qm.WhereIn("content_unit_id in ?", idsI...)).DeleteAll(); err != nil {
		return err
	}
	return mdbmodels.ContentUnits(mdb.DB, qm.WhereIn("uid in ?", UIDsI...)).DeleteAll()
}

func (suite *IndexerSuite) validateContentUnitNames(indexName string, indexer *Indexer, expectedNames []string) {
    t := suite.T()
    err := indexer.RefreshAll()
	assert.Nil(t, err)
    var res *elastic.SearchResult
	res, err = mdb.ESC.Search().Index(indexName).Do(suite.ctx)
	assert.Nil(t, err)
    names := make([]string, len(res.Hits.Hits))
    for i, hit := range res.Hits.Hits {
        var cu ContentUnit
        json.Unmarshal(*hit.Source, &cu)
        names[i] = cu.Name
    }
	assert.Equal(t, int64(len(expectedNames)), res.Hits.TotalHits)
    assert.ElementsMatch(t, expectedNames, names)
}

func (suite *IndexerSuite) TestContentUnitsIndex() {
    t := suite.T()
	fmt.Println("Adding two content units.")
	UIDs := make([]string, 0)
	cu1UID, err := addContentUnit(ContentUnit{Name: "something"})
	UIDs = append(UIDs, cu1UID)
	assert.Nil(t, err)
	var cu2UID string
	cu2UID, err = addContentUnit(ContentUnit{Name: "something else"})
	assert.Nil(t, err)
	UIDs = append(UIDs, cu2UID)

	fmt.Println("Reindexing everything.")
	indexName := IndexName("test", consts.ES_UNITS_INDEX, "en")
	indexer := MakeIndexer("test", []string{consts.ES_UNITS_INDEX})
	// Index existing DB data.
	err = indexer.ReindexAll()
	assert.Nil(t, err)
	err = indexer.RefreshAll()
	assert.Nil(t, err)

	fmt.Println("Validate we have 2 searchable content units.")
    suite.validateContentUnitNames(
        indexName, indexer,
        []string{"something", "something else"})

	fmt.Println("Validate adding content unit incrementally.")
	var cu3UID string
	cu3UID, err = addContentUnit(ContentUnit{Name: "third something"})
	assert.Nil(t, err)
	UIDs = append(UIDs, cu3UID)
	err = indexer.ContentUnitAdd(cu3UID)
	assert.Nil(t, err)
    suite.validateContentUnitNames(
        indexName, indexer,
        []string{"something", "something else", "third something"})

    fmt.Println("Update content unit and validate.")
    _, err = updateContentUnit(ContentUnit{MDB_UID: cu3UID, Name: "updated third something"})
	assert.Nil(t, err)
    i18ns, err := mdbmodels.ContentUnitI18ns(mdb.DB).All()
	assert.Nil(t, err)
    for i, i18n := range i18ns {
        fmt.Printf("Updated values[%d]: %+v\n", i+1, i18n)
    }
	err = indexer.ContentUnitUpdate(cu3UID)
	assert.Nil(t, err)
    suite.validateContentUnitNames(
        indexName, indexer,
        []string{"something", "something else", "updated third something"})

    fmt.Println("Delete content unit and validate.")
	err = indexer.ContentUnitDelete(cu2UID)
	assert.Nil(t, err)
    suite.validateContentUnitNames(
        indexName, indexer,
        []string{"something", "updated third something"})

	fmt.Println("Delete units, reindex and validate we have 0 searchable units.")
	err = deleteContentUnits(UIDs)
	assert.Nil(t, err)
	err = indexer.ReindexAll()
	assert.Nil(t, err)
    suite.validateContentUnitNames(
        indexName, indexer,
        []string{})

	// Remove test indexes.
	assert.Nil(t, indexer.DeleteIndexes())
}
