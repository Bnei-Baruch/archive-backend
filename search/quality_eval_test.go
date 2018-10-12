package search_test

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"testing"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/volatiletech/null.v6"
)

type QualityEvalSuite struct {
	suite.Suite
	utils.TestDBManager
	ctx context.Context
}

func (suite *QualityEvalSuite) SetupSuite() {
	utils.InitConfig("", "../")
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
func TestEval(t *testing.T) {
	suite.Run(t, new(QualityEvalSuite))
}

func (suite *QualityEvalSuite) TestParseExpectation() {
	fmt.Printf("\n------ TestParseExpectation ------\n\n")
	r := require.New(suite.T())

	var latestUIDByDate string
	var latestWomenLessonUID string
	var latestMealUID string
	var latestFriendsGatheringsUID string
	var latestLectureUID string
	var latestVirtualLessonUID string
	var latestUIDByPosition string
	filmDateMask := "{\"film_date\":\"2018-01-%s\"}"

	childTag := mdbmodels.Tag{Pattern: null.String{"ibur", true}, ID: 1, UID: "L2jMWyce"}
	childSource := mdbmodels.Source{Pattern: null.String{"bs-akdama-zohar", true}, ID: 1, TypeID: 1, UID: "ALlyoveA"}
	parentTag := mdbmodels.Tag{Pattern: null.String{"arvut", true}, ID: 2, UID: "L3jMWyce"}
	parentSource := mdbmodels.Source{Pattern: null.String{"bs-akdmot", true}, ID: 2, TypeID: 1, UID: "1vCj4qN9"}

	firstCollectionProperties := null.JSON{JSON: []byte(fmt.Sprintf(filmDateMask, "1")), Valid: true}
	lastCollectionProperties := null.JSON{JSON: []byte(fmt.Sprintf(filmDateMask, "2")), Valid: true}

	cUID, err := suite.updateCollection(mdbmodels.Collection{Properties: firstCollectionProperties, TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CONGRESS].ID}, "", 0)
	r.Nil(err)
	latsCollectionUID, err := suite.updateCollection(mdbmodels.Collection{Properties: lastCollectionProperties, TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CONGRESS].ID}, "", 0)
	r.Nil(err)

	r.Nil(suite.updateTagParent(childTag, parentTag, true, true))
	r.Nil(suite.updateSourceParent(childSource, parentSource, true, true))

	for i := 1; i < 13; i++ {
		properties := null.JSON{JSON: []byte(fmt.Sprintf(filmDateMask, strconv.Itoa(i))), Valid: true}
		lessonPartCUUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(i), TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID},
			consts.LANG_ENGLISH, childTag)
		womenLessonCUUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(100 + i), TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_WOMEN_LESSON].ID},
			consts.LANG_ENGLISH, childTag)
		mealCUUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(200 + i), TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_MEAL].ID},
			consts.LANG_ENGLISH, childTag)
		friendsGatheringsCUUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(300 + i), TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_FRIENDS_GATHERING].ID},
			consts.LANG_ENGLISH, childTag)
		lectureCUUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(400 + i), TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LECTURE].ID},
			consts.LANG_ENGLISH, childTag)
		virtualLessonCUUID, err := suite.addContentUnitTag(mdbmodels.ContentUnit{Properties: properties, Secure: 0, Published: true, ID: int64(500 + i), TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIRTUAL_LESSON].ID},
			consts.LANG_ENGLISH, childTag)
		r.Nil(err)
		_, err = suite.addContentUnitSource(mdbmodels.ContentUnit{UID: lessonPartCUUID}, consts.LANG_ENGLISH, childSource)
		r.Nil(err)
		_, err = suite.updateCollection(mdbmodels.Collection{UID: cUID}, lessonPartCUUID, 13-i)
		r.Nil(err)
		if i == 1 {
			latestUIDByPosition = lessonPartCUUID
		}
		latestUIDByDate = lessonPartCUUID
		latestWomenLessonUID = womenLessonCUUID
		latestMealUID = mealCUUID
		latestFriendsGatheringsUID = friendsGatheringsCUUID
		latestLectureUID = lectureCUUID
		latestVirtualLessonUID = virtualLessonCUUID
	}

	fmt.Printf("Test content_units \n")
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/programs/cu/%s", latestUIDByDate), // Using arbitrary UID
		search.Expectation{search.ET_CONTENT_UNITS, latestUIDByDate, nil, ""}, r)

	fmt.Printf("Test program collections \n")
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/programs/c/%s", cUID),
		search.Expectation{search.ET_COLLECTIONS, cUID, nil, ""}, r)

	fmt.Printf("Test event collections \n")
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/events/c/%s?language=he", cUID),
		search.Expectation{search.ET_COLLECTIONS, cUID, nil, ""}, r)

	fmt.Printf("Test lesson collections \n")
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/lessons/series/c/%s", cUID),
		search.Expectation{search.ET_COLLECTIONS, cUID, nil, ""}, r)

	fmt.Printf("Test lessons \n")
	src := fmt.Sprintf("bs_%s_%s", parentSource.UID, childSource.UID)
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/lessons?source=%s", src),
		search.Expectation{search.ET_LESSONS, "", []search.Filter{search.Filter{Name: search.FILTER_NAME_SOURCE, Value: src}}, ""}, r)

	fmt.Printf("Test programs \n")
	tag := fmt.Sprintf("%s_%s", parentTag.UID, childTag.UID)
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/programs?topic=%s", tag),
		search.Expectation{search.ET_PROGRAMS, "", []search.Filter{search.Filter{Name: search.FILTER_NAME_TOPIC, Value: tag}}, ""}, r)

	fmt.Printf("Test source page \n")
	suite.validateExpectation(fmt.Sprintf("https://kabbalahmedia.info/he/sources/%s", parentSource.UID),
		search.Expectation{search.ET_SOURCES, parentSource.UID, nil, ""}, r)

	fmt.Printf("Test sources main page \n")
	suite.validateExpectation("https://kabbalahmedia.info/he/sources",
		search.Expectation{search.ET_SOURCES, "", nil, ""}, r)

	fmt.Printf("Test events page \n")
	suite.validateExpectation("https://kabbalahmedia.info/he/events",
		search.Expectation{search.ET_LANDING_PAGE, "events", nil, ""}, r)

	fmt.Printf("Test events page by geo location \n")
	suite.validateExpectation("https://kabbalahmedia.info/he/events?location=Russia%7CMoscow",
		search.Expectation{search.ET_LANDING_PAGE, "events", []search.Filter{search.Filter{Name: "location", Value: "Russia%7CMoscow"}}, ""}, r)

	fmt.Printf("Test events page by event type \n")
	suite.validateExpectation("https://kabbalahmedia.info/he/events/conventions",
		search.Expectation{search.ET_LANDING_PAGE, "conventions", nil, ""}, r)

	fmt.Printf("Test lessons page \n")
	suite.validateExpectation("https://kabbalahmedia.info/he/lessons",
		search.Expectation{search.ET_LANDING_PAGE, "lessons", nil, ""}, r)

	fmt.Printf("Test lessons page by type \n")
	suite.validateExpectation("https://kabbalahmedia.info/he/lessons/women",
		search.Expectation{search.ET_LANDING_PAGE, "women", nil, ""}, r)

	fmt.Printf("Test [latest] by source \n")
	suite.validateExpectation(fmt.Sprintf("[latest]https://kabbalahmedia.info/he/lessons?source=%s", parentSource.UID),
		search.Expectation{search.ET_CONTENT_UNITS, latestUIDByDate, nil, ""}, r)

	fmt.Printf("Test [latest] by topic \n")
	suite.validateExpectation(fmt.Sprintf("[latest]https://kabbalahmedia.info/he/programs?topic=%s", parentTag.UID),
		search.Expectation{search.ET_CONTENT_UNITS, latestUIDByDate, nil, ""}, r)

	fmt.Printf("Test [latest] by source and topic \n")
	suite.validateExpectation(fmt.Sprintf("[latest]https://kabbalahmedia.info/he/lessons?source=%s&topic=%s", parentSource.UID, parentTag.UID),
		search.Expectation{search.ET_CONTENT_UNITS, latestUIDByDate, nil, ""}, r)

	fmt.Printf("Test [latest] by collection \n")
	suite.validateExpectation(fmt.Sprintf("[latest]https://kabbalahmedia.info/he/programs/c/%s", cUID),
		search.Expectation{search.ET_CONTENT_UNITS, latestUIDByPosition, nil, ""}, r)

	fmt.Printf("Test [latest] by women lesson \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/lessons/women",
		search.Expectation{search.ET_CONTENT_UNITS, latestWomenLessonUID, nil, ""}, r)

	fmt.Printf("Test [latest] lesson preperation \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/lessons",
		search.Expectation{search.ET_CONTENT_UNITS, latestUIDByDate, nil, ""}, r)

	fmt.Printf("Test [latest] meal event \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/events/meals",
		search.Expectation{search.ET_CONTENT_UNITS, latestMealUID, nil, ""}, r)

	fmt.Printf("Test [latest] friends-gatherings \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/events/friends-gatherings",
		search.Expectation{search.ET_CONTENT_UNITS, latestFriendsGatheringsUID, nil, ""}, r)

	fmt.Printf("Test [latest] lecture \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/lessons/lectures",
		search.Expectation{search.ET_CONTENT_UNITS, latestLectureUID, nil, ""}, r)

	fmt.Printf("Test [latest] virtual lesson \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/lessons/virtual",
		search.Expectation{search.ET_CONTENT_UNITS, latestVirtualLessonUID, nil, ""}, r)

	fmt.Printf("Test [latest] congress event \n")
	suite.validateExpectation("[latest]https://kabbalahmedia.info/he/events",
		search.Expectation{search.ET_COLLECTIONS, latsCollectionUID, nil, ""}, r)
}

func (suite *QualityEvalSuite) validateExpectation(url string, exp search.Expectation, r *require.Assertions) {

	// Expectation Source is not tested.

	resultExp := search.ParseExpectation(url, common.DB)
	r.Equal(resultExp.Uid, exp.Uid)
	r.Equal(resultExp.Type, exp.Type)
	if (exp.Filters != nil && resultExp.Filters == nil) || (exp.Filters == nil && resultExp.Filters != nil) {
		r.Fail("Comparing nil value filters with non-nil value filters.")
	}
	if exp.Filters != nil {
		r.Equal(int64(len(resultExp.Filters)), int64(len(exp.Filters)))
		r.ElementsMatch(resultExp.Filters, exp.Filters)
	}
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
		cu.UID = utils.GenerateUID(8)
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
		cu.UID = utils.GenerateUID(8)
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

func (suite *QualityEvalSuite) updateCollection(c mdbmodels.Collection, cuUID string, position int) (string, error) {
	if c.UID != "" {
		cFromDB, err := mdbmodels.Collections(common.DB, qm.Where("uid = ?", c.UID)).One()
		if err != nil {
			return "", err
		}
		c = *cFromDB
	} else {
		c.UID = utils.GenerateUID(8)
		c.TypeID = c.TypeID
		c.Secure = int16(0)
		c.Published = true
		if err := c.Insert(common.DB); err != nil {
			return "", err
		}
	}

	if cuUID != "" {
		cu, err := mdbmodels.ContentUnits(common.DB, qm.Where("uid = ?", cuUID)).One()
		if err != nil {
			return "", err
		}
		if _, err := mdbmodels.FindCollectionsContentUnit(common.DB, c.ID, cu.ID); err == sql.ErrNoRows {
			var mdbCollectionsContentUnit mdbmodels.CollectionsContentUnit
			mdbCollectionsContentUnit.CollectionID = c.ID
			mdbCollectionsContentUnit.ContentUnitID = cu.ID
			mdbCollectionsContentUnit.Position = position
			if err := mdbCollectionsContentUnit.Insert(common.DB); err != nil {
				return "", err
			}
		}
	}
	return c.UID, nil
}
