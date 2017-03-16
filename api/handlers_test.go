package api

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

type HandlersSuite struct {
	suite.Suite
	esc *elastic.Client
}

func (suite *HandlersSuite) SetupSuite() {
	utils.InitConfig("", "../")

	la := ESLogAdapter{T: suite.T()}
	var err error
	suite.esc, err = elastic.NewClient(
		elastic.SetURL(viper.GetString("elasticsearch.url")),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetErrorLog(la),
		elastic.SetInfoLog(la),
	)
	suite.Require().Nil(err)
}

func (suite *HandlersSuite) TearDownSuite() {
	suite.esc.Stop()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHandlers(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}

func (suite *HandlersSuite) TestHandleSearch() {
	res, err := handleSearch(suite.esc, "mdb_collections", "sulam", 0)
	suite.Require().Nil(err)
	suite.NotNil(res.Hits)
	suite.NotEmpty(res.Hits.TotalHits)
}

type ESLogAdapter struct{ *testing.T }

func (s ESLogAdapter) Printf(format string, v ...interface{}) { s.Logf(format, v...) }
