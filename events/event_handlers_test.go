package events

import (
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/suite"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

type HandlersSuite struct {
	suite.Suite
}

func (suite *HandlersSuite) SetupSuite() {
	utils.InitConfig("", "../")
}

func TestHandlers(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}

func (suite *HandlersSuite) TestApiGet() {
	log.SetLevel(log.DebugLevel)
	apiType := "unzip"
	uid := "ZLuOz4ih"
	AssetsAPI(apiType, uid)
}
