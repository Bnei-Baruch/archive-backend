package search

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v6"

	"context"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
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
	engine := ESEngine{esc: suite.esc, ExecutionTimeLog: NewTimeLogMap()}
	_, err := engine.GetSuggestions(context.TODO(),
		Query{Term: "pe", LanguageOrder: []string{consts.LANG_ENGLISH}}, "pref")
	suite.Require().Nil(err)
}

type SRR struct {
	Score         float64
	Uid           string
	EffectiveDate utils.Date
}

func parse(date string) utils.Date {
	val, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return utils.Date{Time: val}
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
		ed := es.EffectiveDate{&srr.EffectiveDate}
		msg, err := json.Marshal(ed)
		sh.Source = (*json.RawMessage)(&msg)
		if err != nil {
			panic(err)
		}
		res.Hits.Hits = append(res.Hits.Hits, sh)
	}
	return res
}

func (suite *EngineSuite) TestJoinResponsesNoResults() {
	fmt.Printf("\n------ TestJoinResponsesNoResults ------\n\n")
	r := require.New(suite.T())
	results := make([]*elastic.SearchResult, 0)
	ret, err := joinResponses(consts.SORT_BY_RELEVANCE, 0, 1, results...)
	r.Nil(err)
	r.Nil(ret)
}

func (suite *EngineSuite) TestJoinResponsesTakeFirstOnEqual() {
	fmt.Printf("\n------ TestJoinResponsesTakeFirstOnEqual ------\n\n")
	r := require.New(suite.T())
	r1 := SearchResult([]SRR{SRR{2.4, "a", parse("1111-11-11")}})
	r2 := SearchResult([]SRR{SRR{2.4, "1", parse("1111-11-11")}})
	r3, err := joinResponses(consts.SORT_BY_RELEVANCE, 0, 1, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{2.4, "a", parse("1111-11-11")}}
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}

func (suite *EngineSuite) TestJoinResponsesTakeLargerFirst() {
	fmt.Printf("\n------ TestJoinResponsesTakeLargerFirst ------\n\n")
	r := require.New(suite.T())
	r1 := SearchResult([]SRR{SRR{2.4, "a", parse("1111-11-11")}})
	r2 := SearchResult([]SRR{SRR{2.5, "1", parse("1111-11-11")}})
	r3, err := joinResponses(consts.SORT_BY_RELEVANCE, 0, 1, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{2.5, "1", parse("1111-11-11")}}
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}

func (suite *EngineSuite) TestJoinResponsesInterleave() {
	fmt.Printf("\n------ TestJoinResponsesInterleave ------\n\n")
	r := require.New(suite.T())
	d := parse("1111-11-11")
	r1 := SearchResult([]SRR{SRR{2.4, "a", d}, SRR{2.0, "b", d}, SRR{1.5, "c", d}, SRR{1.2, "d", d}, SRR{0.4, "e", d}})
	r2 := SearchResult([]SRR{SRR{2.5, "1", d}, SRR{2.2, "2", d}, SRR{1.6, "3", d}, SRR{1.0, "4", d}, SRR{0.7, "5", d}})
	r3, err := joinResponses(consts.SORT_BY_RELEVANCE, 0, 4, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{2.5, "1", parse("1111-11-11")}, SRR{2.4, "a", parse("1111-11-11")}, SRR{2.2, "2", parse("1111-11-11")}, SRR{2.0, "b", parse("1111-11-11")}}
	r.Equal(len(expected), len(r3.Hits.Hits))
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}

func (suite *EngineSuite) TestJoinResponsesInterleaveSecondPage() {
	fmt.Printf("\n------ TestJoinResponsesInterleaveSecondPage ------\n\n")
	r := require.New(suite.T())
	d := parse("1111-11-11")
	r1 := SearchResult([]SRR{SRR{2.4, "a", d}, SRR{2.0, "b", d}, SRR{1.5, "c", d}, SRR{1.2, "d", d}, SRR{0.4, "e", d}})
	r2 := SearchResult([]SRR{SRR{2.5, "1", d}, SRR{2.2, "2", d}, SRR{1.6, "3", d}, SRR{1.0, "4", d}, SRR{0.7, "5", d}})
	r3, err := joinResponses(consts.SORT_BY_RELEVANCE, 4, 4, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{1.6, "3", d}, SRR{1.5, "c", d}, SRR{1.2, "d", d}, SRR{1.0, "4", d}}
	r.Equal(len(expected), len(r3.Hits.Hits))
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}

func (suite *EngineSuite) TestJoinResponsesInterleaveSecondPageOneSide() {
	fmt.Printf("\n------ TestJoinResponsesInterleaveSecondPageOneSide ------\n\n")
	r := require.New(suite.T())
	d := parse("1111-11-11")
	r1 := SearchResult([]SRR{SRR{2.4, "a", d}, SRR{2.0, "b", d}, SRR{1.5, "c", d}, SRR{1.2, "d", d}, SRR{0.4, "e", d}})
	r2 := SearchResult([]SRR{})
	r3, err := joinResponses(consts.SORT_BY_RELEVANCE, 4, 4, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{0.4, "e", d}}
	r.Equal(len(expected), len(r3.Hits.Hits))
	fmt.Printf("\n------ TestJoinResponsesInterleaveSecondPageOneSide ------\n\n")
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}

func (suite *EngineSuite) TestJoinResponsesNewerToOlder() {
	fmt.Printf("\n------ TestJoinResponsesNewerToOlder ------\n\n")
	r := require.New(suite.T())
	r1 := SearchResult([]SRR{SRR{2.5, "a", parse("2018-01-06")}, SRR{2.0, "b", parse("2015-05-22")}, SRR{1.5, "c", parse("2015-05-21")}})
	r2 := SearchResult([]SRR{SRR{2.4, "1", parse("2018-01-16")}, SRR{2.2, "2", parse("2015-05-20")}, SRR{1.6, "3", parse("2014-05-05")}})
	r3, err := joinResponses(consts.SORT_BY_NEWER_TO_OLDER, 0, 4, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{2.4, "1", parse("2018-01-16")}, SRR{2.5, "a", parse("2018-01-06")},
		SRR{2.0, "b", parse("2015-05-22")}, SRR{1.5, "c", parse("2015-05-21")}}
	r.Equal(len(expected), len(r3.Hits.Hits))
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}

func (suite *EngineSuite) TestJoinResponsesTimeOlderToNewer() {
	fmt.Printf("\n------ TestJoinResponsesTimeOlderToNewer ------\n\n")
	r := require.New(suite.T())
	r1 := SearchResult([]SRR{SRR{1.5, "c", parse("2015-05-21")}, SRR{2.0, "b", parse("2015-05-22")}, SRR{2.5, "a", parse("2018-01-06")}})
	r2 := SearchResult([]SRR{SRR{1.6, "3", parse("2014-05-05")}, SRR{2.2, "2", parse("2015-05-21")}, SRR{2.4, "1", parse("2018-01-16")}})
	r3, err := joinResponses(consts.SORT_BY_OLDER_TO_NEWER, 0, 4, r1, r2)
	r.Nil(err)

	expected := []SRR{SRR{1.6, "3", parse("2014-05-05")}, SRR{2.2, "2", parse("2015-05-21")}, SRR{1.5, "c", parse("2015-05-21")}, SRR{2.0, "b", parse("2015-05-22")}}
	r.Equal(len(expected), len(r3.Hits.Hits))
	for i, h := range r3.Hits.Hits {
		r.Equal(expected[i].Score, *h.Score)
		r.Equal(expected[i].Uid, h.Uid)
	}
}
