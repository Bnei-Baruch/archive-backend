package api

import (
	"database/sql"

	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/sqlboiler/boil"
	"gopkg.in/gin-gonic/gin.v1"

	models2 "github.com/Bnei-Baruch/archive-backend/mydb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type MyRestSuite struct {
	suite.Suite
	utils.TestDBManager
	tx   *sql.Tx
	ctx  *gin.Context
	kcId string
}

func (suite *MyRestSuite) SetupSuite() {
	suite.Require().Nil(suite.InitTestMyDB())
	suite.kcId = "test_keycloak_user_id"
	suite.ctx = &gin.Context{}
	suite.ctx.Set("KC_ID", suite.kcId)
}

func (suite *MyRestSuite) TearDownSuite() {
	suite.Require().Nil(suite.DestroyTestMyDB())
}

func (suite *MyRestSuite) SetupTest() {
	var err error
	suite.tx, err = suite.DB.Begin()
	suite.Require().Nil(err)
}

func (suite *MyRestSuite) TearDownTest() {
	err := suite.tx.Rollback()
	suite.Require().Nil(err)
}

func (suite *MyRestSuite) TestLikesList() {

	req := ListRequest{StartIndex: 1, StopIndex: 5}

	resp, err := handleGetLikes(suite.tx, suite.kcId, req)
	suite.Require().Nil(err)
	suite.EqualValues(0, resp.Total, "empty total")
	suite.Empty(resp.Likes, "empty data")

	likes := suite.createDummyLike(10)

	resp, err = handleGetLikes(suite.tx, suite.kcId, req)
	suite.Require().Nil(err)
	suite.EqualValues(10, resp.Total, "total")
	for i, x := range resp.Likes {
		suite.assertEqualDummyLikes(likes[i], x, i)
	}

	likes[1].AccountID = "new_account_id"
	suite.NotNil(likes[1].Insert(suite.tx, boil.Infer()))
	resp, err = handleGetLikes(suite.tx, suite.kcId, req)
	suite.Require().Nil(err)
	suite.EqualValues(9, resp.Total, "total")

	req.StartIndex = 6
	req.StopIndex = 10
	resp, err = handleGetLikes(suite.tx, suite.kcId, req)
	suite.Require().Nil(err)
	suite.EqualValues(10, resp.Total, "total")
	for i, x := range resp.Likes {
		suite.assertEqualDummyLikes(likes[i+5], x, i+5)
	}
	req.StartIndex = 0
	req.StopIndex = 10

	like := &models2.Like{
		ID:             11,
		AccountID:      suite.kcId,
		ContentUnitUID: utils.GenerateUID(8),
	}
	likes = append(likes, like)
	nLikes, err := handleAddLike(suite.tx, []string{like.ContentUnitUID}, suite.kcId)
	suite.Nil(err)
	suite.Len(nLikes, 1)
	suite.assertEqualDummyLikes(like, nLikes[0], 11)

	dLikes, err := handleRemoveLikes(suite.tx, []int64{like.ID}, suite.kcId)
	suite.Nil(err)
	suite.Len(dLikes, 1)
	suite.assertEqualDummyLikes(like, dLikes[0], 11)

	resp, err = handleGetLikes(suite.tx, suite.kcId, req)
	suite.Require().Nil(err)
	suite.EqualValues(9, resp.Total, "total")

}

func (suite *MyRestSuite) createDummyLike(n int64) []*models2.Like {
	likes := make([]*models2.Like, n)
	for _, l := range likes {
		l.ContentUnitUID = utils.GenerateUID(8)
		l.AccountID = suite.kcId
		utils.Must(l.Insert(suite.tx, boil.Infer()))
	}
	return likes
}

func (suite *MyRestSuite) assertEqualDummyLikes(l *models2.Like, x *models2.Like, idx int) {
	suite.Equal(l.ID, x.ID, "like.ID [%d]", idx)
	suite.Equal(l.AccountID, x.AccountID, "like.AccountID [%d]", idx)
	suite.Equal(l.ContentUnitUID, x.ContentUnitUID, "like.ContentUnitUID [%d]", idx)
}
