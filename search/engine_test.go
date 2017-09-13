package search

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v5"

	"context"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type EngineSuite struct {
	suite.Suite
	esc *elastic.Client
}

func (suite *EngineSuite) SetupSuite() {
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

func (suite *EngineSuite) TearDownSuite() {
	suite.esc.Stop()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

func (suite *EngineSuite) TestESGetSuggestions() {
	engine := ESEngine{esc: suite.esc}
	_, err := engine.GetSuggestions(context.TODO(),
		Query{Term: "pe", LanguageOrder: []string{consts.LANG_ENGLISH}})
	suite.Require().Nil(err)
}

type ESLogAdapter struct{ *testing.T }

func (s ESLogAdapter) Printf(format string, v ...interface{}) { s.Logf(format, v...) }
