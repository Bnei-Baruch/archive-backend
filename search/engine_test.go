package search

import (
    "fmt"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
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

type ESLogAdapter struct{ *testing.T }

func (s ESLogAdapter) Printf(format string, v ...interface{}) { s.Logf(format, v...) }

func (suite *EngineSuite) TestESGetSuggestions() {
	engine := ESEngine{esc: suite.esc}
	_, err := engine.GetSuggestions(context.TODO(),
		Query{Term: "pe", LanguageOrder: []string{consts.LANG_ENGLISH}})
	suite.Require().Nil(err)
}

type SRR struct {
    Score float64
    Uid string
}

func SearchResult(hits []SRR) *elastic.SearchResult {
    res := new(elastic.SearchResult)
    res.Hits = new(elastic.SearchHits)
    res.Hits.TotalHits = int64(len(hits))
    for _, srr := range hits {
        if res.Hits.MaxScore == nil || srr.Score > *res.Hits.MaxScore {
            res.Hits.MaxScore = &srr.Score
        }
        sh := new(elastic.SearchHit)
        sh.Score = new(float64)
        *sh.Score = srr.Score
        sh.Uid = srr.Uid
        res.Hits.Hits = append(res.Hits.Hits, sh)
    }
    return res
}

func (suite *EngineSuite) TestJoinResponsesTakeFirstOnEqual() {
	r := require.New(suite.T())
    r1 := SearchResult([]SRR{SRR{2.4, "a"}})
    r2 := SearchResult([]SRR{SRR{2.4, "1"}})
    r3 := joinResponses(r1, r2, "", 0, 1)

    expected := []SRR{SRR{2.4, "a"}}
    for i, h := range r3.Hits.Hits {
        r.Equal(expected[i].Score, *h.Score)
        r.Equal(expected[i].Uid, h.Uid)
    }
}

func (suite *EngineSuite) TestJoinResponsesTakeLargerFirst() {
	r := require.New(suite.T())
    r1 := SearchResult([]SRR{SRR{2.4, "a"}})
    r2 := SearchResult([]SRR{SRR{2.5, "1"}})
    r3 := joinResponses(r1, r2, "", 0, 1)

    expected := []SRR{SRR{2.5, "1"}}
    for i, h := range r3.Hits.Hits {
        r.Equal(expected[i].Score, *h.Score)
        r.Equal(expected[i].Uid, h.Uid)
    }
}

func (suite *EngineSuite) TestJoinResponsesInterleave() {
    fmt.Printf("\n------ TestJoinResponsesInterleave ------\n\n")
	r := require.New(suite.T())
    r1 := SearchResult([]SRR{SRR{2.4, "a"}, SRR{2.0, "b"}, SRR{1.5, "c"}})
    r2 := SearchResult([]SRR{SRR{2.5, "1"}, SRR{2.2, "2"}, SRR{1.6, "3"}})
    r3 := joinResponses(r1, r2, "", 0, 1)

    expected := []SRR{SRR{2.5, "1"}}
    r.Equal(len(expected), len(r3.Hits.Hits))
    for i, h := range r3.Hits.Hits {
        r.Equal(expected[i].Score, *h.Score)
        r.Equal(expected[i].Uid, h.Uid)
    }
}
