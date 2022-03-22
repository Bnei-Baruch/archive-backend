package tests

import (
	"context"
	"fmt"
	"github.com/Bnei-Baruch/archive-backend/api"
	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
	"gopkg.in/volatiletech/null.v6"
	"testing"
	"time"
)

type TagDashboardSuite struct {
	suite.Suite
	utils.TestDBManager
	Cache cache.CacheManager
	ctx   context.Context
}

func (s *TagDashboardSuite) SetupSuite() {
	utils.InitConfig("", "../")
	err := s.InitTestDB()
	if err != nil {
		panic(err)
	}
	s.ctx = context.Background()
	utils.Must(mdb.InitTypeRegistries(s.DB))

	refreshIntervals := map[string]time.Duration{"SearchStats": 5 * time.Minute}
	s.Cache = cache.NewCacheManagerImpl(s.DB, refreshIntervals)
	common.InitWithDefault(s.DB, &s.Cache)
	// Set package db and esc variables.
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
}

func (s *TagDashboardSuite) TearDownSuite() {
	// Close connections.
	common.Shutdown()
	// Drop test database.
	s.Require().Nil(s.DestroyTestDB())
}

func TestTagDashboard(t *testing.T) {
	suite.Run(t, new(TagDashboardSuite))
}

func (s *TagDashboardSuite) TestMediaPagination() {
	boil.DebugMode = true
	tag := &mdbmodels.Tag{
		UID:     utils.GenerateUID(8),
		Pattern: null.StringFrom("test"),
	}
	s.NoError(tag.Insert(s.DB))
	s.Cache.Refresh()

	cusLessons := s.mkCUs(9, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID, tag)
	cusPrograms := s.mkCUs(28, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID, tag)
	labels := s.mkLabels(20, "media", tag)

	pageSize := 18
	total := len(cusLessons) + len(cusPrograms) + len(labels)

	for i := 1; pageSize*(i-1) <= total; i++ {
		if i != 3 {
			continue
		}
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			TagsFilter:  api.TagsFilter{Tags: []string{tag.UID}},
		}

		resp, err := api.HandleTagDashboard(common.CACHE, s.DB, r)
		s.Nil(err)
		count := pageSize
		if i*pageSize > total {
			count = total % pageSize
		}
		s.EqualValues(count, len(resp.Items))
		s.EqualValues(total, resp.MediaTotal)

		s.Nil(err)
		byType := s.respCusByCT(resp)
		expLesson := 0
		expProg := 0
		expLabels := 0
		if i == 1 {
			expLesson = 6
			expProg = 6
			expLabels = 6
		} else if i == 2 {
			expLesson = 3
			expProg = 8
			expLabels = 7
		} else if i == 3 {
			expLesson = 0
			expProg = 11
			expLabels = 7
		} else if i == 3 {
			expLesson = 0
			expProg = 3
			expLabels = 0
		}
		s.EqualValues(expLesson, len(byType[mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID]), fmt.Sprintf("number of LESSONS, Page n %d", i))
		s.EqualValues(expProg, len(byType[mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID]), fmt.Sprintf("number of PROGRAMS, Page n %d", i))
		s.EqualValues(expLabels, len(byType[-2]), fmt.Sprintf("number of labels, Page n %d", i))
	}
}

func (s *TagDashboardSuite) TestTextPagination() {
	boil.DebugMode = true
	tag := &mdbmodels.Tag{
		UID:     utils.GenerateUID(8),
		Pattern: null.StringFrom("test"),
	}
	s.NoError(tag.Insert(s.DB))

	cusArticle := s.mkCUs(100, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_ARTICLE].ID, tag)

	labels := s.mkLabels(100, "text", tag)
	pageSize := 20
	total := len(cusArticle) + len(labels)

	for i := 0; i <= total/pageSize; i++ {
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			TagsFilter:  api.TagsFilter{Tags: []string{tag.UID}},
		}

		resp, err := api.HandleTagDashboard(common.CACHE, s.DB, r)
		s.Nil(err)
		count := pageSize
		if i+1 > total/pageSize {
			count = total % pageSize
		}
		s.EqualValues(count, len(resp.Items))
		s.EqualValues(total, resp.TextTotal)
	}
}

//helpers
func (s *TagDashboardSuite) respCusByCT(resp *api.TagsDashboardResponse) map[int64][]string {
	cuByType := make(map[int64][]string, 0)
	// for labels
	cuByType[-2] = make([]string, 0)
	uids := make([]string, len(resp.Items))
	for i, x := range resp.Items {
		if x.LabelID != "" {
			cuByType[-2] = append(cuByType[-2], x.LabelID)
		}
		uids[i] = x.ContentUnitID
	}

	all, err := mdbmodels.ContentUnits(s.DB, qm.WhereIn("uid IN ?", utils.ConvertArgsString(uids)...)).All()
	s.NoError(err)
	for _, unit := range all {
		if _, ok := cuByType[unit.TypeID]; !ok {
			cuByType[unit.TypeID] = make([]string, 0)
		}
		cuByType[unit.TypeID] = append(cuByType[unit.TypeID], unit.UID)
	}
	return cuByType
}

func (s *TagDashboardSuite) mkCUs(n int, typeId int64, tag *mdbmodels.Tag) []*mdbmodels.ContentUnit {
	cus := make([]*mdbmodels.ContentUnit, n)
	for i, _ := range cus {
		cu := &mdbmodels.ContentUnit{
			UID:       utils.GenerateUID(8),
			Secure:    0,
			TypeID:    typeId,
			Published: true,
		}
		s.NoError(cu.Insert(s.DB))
		s.NoError(cu.AddTags(s.DB, false, tag))
		i18n := &mdbmodels.ContentUnitI18n{Language: "he", Name: null.StringFrom(fmt.Sprintf("cu %d", cu.ID))}
		s.NoError(cu.AddContentUnitI18ns(s.DB, true, i18n))
		cus[i] = cu
	}
	return cus
}

func (s *TagDashboardSuite) mkLabels(n int, mtype string, tag *mdbmodels.Tag) []*mdbmodels.Label {
	labels := make([]*mdbmodels.Label, n)
	for i, _ := range labels {
		cu := &mdbmodels.ContentUnit{
			UID:    utils.GenerateUID(8),
			Secure: 0,
			TypeID: mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SOURCE].ID,
		}
		s.NoError(cu.Insert(s.DB))

		i18n := &mdbmodels.ContentUnitI18n{Language: "he", Name: null.StringFrom(fmt.Sprintf("label cu %d", cu.ID))}
		s.NoError(cu.AddContentUnitI18ns(s.DB, true, i18n))

		l := &mdbmodels.Label{
			UID:           utils.GenerateUID(8),
			ContentUnitID: cu.ID,
			MediaType:     mtype,
			ApproveState:  consts.APR_APPROVED,
		}

		s.NoError(l.Insert(s.DB))

		lI18n := &mdbmodels.LabelI18n{Language: "he", Name: null.StringFrom("test")}
		s.NoError(l.AddLabelI18ns(s.DB, true, lI18n))
		s.NoError(l.AddTags(s.DB, false, tag))

		labels[i] = l
	}
	return labels
}
