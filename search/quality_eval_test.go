package search

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	null "gopkg.in/volatiletech/null.v6"
)

type QualityEvalSuite struct {
	suite.Suite
}

type TestDBManager struct {
	DB     *sql.DB
	testDB string
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

func (suite *QualityEvalSuite) SetupSuite() {
	// TBD replace DB
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
//func TestEngine(t *testing.T) {
//	suite.Run(t, new(QualityEvalSuite))
//}

func (suite *QualityEvalSuite) TestGetLatestUidByFilters() {
	fmt.Printf("\n------ TestGetLatestUidByFilters ------\n\n")
	r := require.New(suite.T())

	parentTag := mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}
	suite.updateTagParent("L2jMWyce", parentTag, false)
	parentSource := mdbmodels.Source{Pattern: null.String{"bs-akdama-pi-hacham", true}, ID: 4, TypeID: 1, UID: sourceUID2}
	suite.updateSourceParent(sourceUID1, parentSource, false)

	suite.ucu(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, true, true, "{\"film_date\":\"2017-01-01\"}")
	suite.ucut(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, parentTag, false)
	suite.acus(es.ContentUnit{MDB_UID: cu1UID}, consts.LANG_ENGLISH, parentSource, mdbmodels.Author{ID: 1}, false)

	suite.ucu(es.ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, true, true, "{\"film_date\":\"2018-01-01\"}")
	suite.ucut(es.ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, parentTag, false)
	suite.acus(es.ContentUnit{MDB_UID: cu2UID}, consts.LANG_ENGLISH, parentSource, mdbmodels.Author{ID: 1}, false)

}

func (suite *QualityEvalSuite) updateSourceParent(sourceUID string, parentSource mdbmodels.Source, insertParent bool) error {
	var mdbSource mdbmodels.Source
	src, err := mdbmodels.Sources(common.DB, qm.Where("uid = ?", sourceUID)).One()
	if err != nil {
		return err
	}
	mdbSource = *src

	if parentSource.UID != "" {
		err = mdbSource.SetParent(common.DB, insertParent, &parentSource)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("parentSource.UID is empty")
	}
}

func (suite *QualityEvalSuite) updateTagParent(tagUID string, parent mdbmodels.Tag, insertParent bool) error {
	var mdbTag mdbmodels.Tag

	tag, err := mdbmodels.Tags(common.DB, qm.Where("uid = ?", tagUID)).One()
	if err != nil {
		return err
	}
	mdbTag = *tag

	if parent.UID != "" {
		err = mdbTag.SetParent(common.DB, insertParent, &parent)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("parent.UID is empty")
	}
}

func updateContentUnit(cu mdbmodels.ContentUnit, lang string, properties string) (string, error) {
	if cu.UID == "" {
		cu.UID = GenerateUID(8)
		if err := mdbContentUnit.Insert(common.DB); err != nil {
			return "", err
		}
	}

	cu.Properties = properties
	if err := cu.Update(common.DB); err != nil {
		return "", err
	}

	return cu.UID, nil
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
