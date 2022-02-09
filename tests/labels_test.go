package tests

import (
	"context"
	"fmt"
	"github.com/Bnei-Baruch/archive-backend/api"
	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
	"gopkg.in/volatiletech/null.v6"
	"math/rand"
	"testing"
)

type LabelSuite struct {
	suite.Suite
	utils.TestDBManager
	ctx context.Context
}

func (s *LabelSuite) SetupSuite() {
	utils.InitConfig("", "../")
	err := s.InitTestDB()
	if err != nil {
		panic(err)
	}
	s.ctx = context.Background()

	// Set package db and esc variables.
	common.InitWithDefault(s.DB)
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
}

func (s *LabelSuite) TearDownSuite() {
	// Close connections.
	common.Shutdown()
	// Drop test database.
	s.Require().Nil(s.DestroyTestDB())
}

func TestLabels(t *testing.T) {
	suite.Run(t, new(LabelSuite))
}

func (s *LabelSuite) TestTextPagination() {
	boil.DebugMode = true
	tag := &mdbmodels.Tag{
		UID:     utils.GenerateUID(8),
		Pattern: null.StringFrom("test"),
	}
	s.NoError(tag.Insert(s.DB))

	cus := s.mkCUs(100, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_ARTICLE].ID, tag)
	//_ = s.mkCUs(10, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_DAILY_LESSON].ID, tag)

	labels := s.mkLabels(100, "text", tag)
	pageSize := 20
	total := len(cus) + len(labels)

	for i := 0; i <= total/pageSize; i++ {
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			UID:         tag.UID,
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

func (s *LabelSuite) mkCUs(n int, typeId int64, tag *mdbmodels.Tag) []*mdbmodels.ContentUnit {
	cus := make([]*mdbmodels.ContentUnit, rand.Intn(n))
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

func (s *LabelSuite) mkLabels(n int, mtype string, tag *mdbmodels.Tag) []*mdbmodels.Label {
	labels := make([]*mdbmodels.Label, rand.Intn(n))
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
			MediaType:     mtype,
			Secure:        0,
			ContentUnitID: cu.ID,
		}

		s.NoError(l.Insert(s.DB))

		lI18n := &mdbmodels.LabelI18n{Language: "he", Name: null.StringFrom("test")}
		s.NoError(l.AddLabelI18ns(s.DB, true, lI18n))
		s.NoError(l.AddTags(s.DB, false, tag))

		labels[i] = l
	}
	return labels
}
