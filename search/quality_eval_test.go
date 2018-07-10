package search_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/migrations"
	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	null "gopkg.in/volatiletech/null.v6"
)

type QualityEvalSuite struct {
	suite.Suite
	TestDBManager
	ctx context.Context
}

type TestDBManager struct {
	DB     *sql.DB
	testDB string
}

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

func (suite *QualityEvalSuite) SetupSuite() {
	err := suite.InitTestDB()
	if err != nil {
		panic(err)
	}
	suite.ctx = context.Background()

	// Set package db.
	common.InitWithDefault(suite.DB)
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
}

func (suite *QualityEvalSuite) TearDownSuite() {
	// Close connections.
	common.Shutdown()
	// Drop test database.
	suite.Require().Nil(suite.DestroyTestDB())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestEngine(t *testing.T) {
	suite.Run(t, new(QualityEvalSuite))
}

func (suite *QualityEvalSuite) TestGetLatestUidByFilters() {
	fmt.Printf("\n------ TestGetLatestUidByFilters ------\n\n")
	r := require.New(suite.T())

	var latestUID string
	filmDateMask := "{\"film_date\":\"2018-01-%s\"}"

	childTag := mdbmodels.Tag{Pattern: null.String{"ibur", true}, ID: 1, UID: "L2jMWyce"}
	childSource := mdbmodels.Source{Pattern: null.String{"bs-akdama-zohar", true}, ID: 1, TypeID: 1, UID: "ALlyoveA"}
	parentTag := mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}
	parentSource := mdbmodels.Source{Pattern: null.String{"bs-akdmot", true}, ID: 2, TypeID: 1, UID: "1vCj4qN9"}

	r.Nil(suite.updateTagParent(childTag, parentTag, true, true))
	r.Nil(suite.updateSourceParent(childSource, parentSource, true, true))

	for i := 1; i < 13; i++ {
		properties := null.JSON{JSON: []byte(fmt.Sprintf(filmDateMask, strconv.Itoa(i))), Valid: true}
		cuUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(i)}, consts.LANG_ENGLISH, childTag)
		r.Nil(err)
		_, err = suite.addContentUnitSource(mdbmodels.ContentUnit{UID: cuUID}, consts.LANG_ENGLISH, childSource)
		r.Nil(err)
		latestUID = cuUID
	}

	sourceFilter := search.Filter{Name: search.FILTER_NAME_SOURCE, Value: parentSource.UID}
	tagsFilter := search.Filter{Name: search.FILTER_NAME_TOPIC, Value: parentTag.UID}

	fmt.Printf("Test by source \n")
	resultUID, err := search.GetLatestUidByFilters([]search.Filter{sourceFilter}, common.DB)
	r.Nil(err)
	r.Equal(latestUID, resultUID)

	fmt.Printf("Test by topic \n")
	resultUID, err = search.GetLatestUidByFilters([]search.Filter{tagsFilter}, common.DB)
	r.Nil(err)
	r.Equal(latestUID, resultUID)

	fmt.Printf("Test by source and topic \n")
	resultUID, err = search.GetLatestUidByFilters([]search.Filter{sourceFilter, tagsFilter}, common.DB)
	r.Nil(err)
	r.Equal(latestUID, resultUID)
}

func (suite *QualityEvalSuite) updateSourceParent(child mdbmodels.Source, parentSource mdbmodels.Source, insertChild bool, insertParent bool) error {
	childFromDB, err := mdbmodels.Sources(common.DB, qm.Where("uid = ?", child.UID)).One()
	if err != nil {
		if err == sql.ErrNoRows && insertChild {
			err = child.Insert(common.DB)
			if err != nil {
				return err
			}

		} else {
			return err
		}
	} else {
		child = *childFromDB
	}

	if parentSource.UID != "" {
		err = child.SetParent(common.DB, insertParent, &parentSource)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("parentSource.UID is empty")
	}
}

func (suite *QualityEvalSuite) updateTagParent(child mdbmodels.Tag, parent mdbmodels.Tag, insertChild bool, insertParent bool) error {
	childFromDB, err := mdbmodels.Tags(common.DB, qm.Where("uid = ?", child.UID)).One()
	if err != nil {
		if err == sql.ErrNoRows && insertChild {

			err = child.Insert(common.DB)
			if err != nil {
				return err
			}

		} else {
			return err
		}
	} else {
		child = *childFromDB
	}

	if parent.UID != "" {
		err = child.SetParent(common.DB, insertParent, &parent)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("parent.UID is empty")
	}
}

func (suite *QualityEvalSuite) addContentUnitTag(cu mdbmodels.ContentUnit, lang string, tag mdbmodels.Tag) (string, error) {
	if cu.UID != "" {
		cuFromDB, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.UID)).One()
		if err != nil {
			return "", err
		}
		cu = *cuFromDB
	} else {
		cu.UID = GenerateUID(8)
		cu.TypeID = mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID
		if err := cu.Insert(common.DB); err != nil {
			return "", err
		}
	}

	_, err := mdbmodels.FindTag(common.DB, tag.ID)
	if err != nil {
		return "", err
	}

	err = cu.AddTags(common.DB, false, &tag)
	if err != nil {
		return "", err
	}

	return cu.UID, nil
}

func (suite *QualityEvalSuite) addContentUnitSource(cu mdbmodels.ContentUnit, lang string, src mdbmodels.Source) (string, error) {
	if cu.UID != "" {
		cuFromDB, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cu.UID)).One()
		if err != nil {
			return "", err
		}
		cu = *cuFromDB
	} else {
		cu.UID = GenerateUID(8)
		cu.TypeID = mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID
		if err := cu.Insert(common.DB); err != nil {
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

	err = cu.AddSources(common.DB, false, &src)
	if err != nil {
		return "", err
	}

	return cu.UID, nil
}
