package search_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Bnei-Baruch/sqlboiler/boil"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/stretchr/testify/suite"
)

type TokenSuite struct {
	suite.Suite
	utils.TestDBManager
	esc       *elastic.Client
	ctx       context.Context
	indexName string
}

func TestToken(t *testing.T) {
	suite.Run(t, new(TokenSuite))
}

func (suite *TokenSuite) SetupSuite() {
	suite.indexName = "test_token"
	utils.InitConfig("", "../")
	err := suite.InitTestDB()
	if err != nil {
		panic(err)
	}
	suite.ctx = context.Background()

	// Set package db and esc variables.
	common.InitWithDefault(suite.DB)
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
	esc, err := common.ESC.GetClient()
	if err != nil {
		panic(err)
	}
	suite.esc = esc

	bodyString := `{
		"settings" : {
			"number_of_shards": 1,
			"number_of_replicas": 0
		}
	}`
	createRes, err := suite.esc.CreateIndex(suite.indexName).BodyString(bodyString).Do(suite.ctx)
	if err != nil {
		panic(err)
	}
	if !createRes.Acknowledged {
		panic("Creation of index was not Acknowledged.")
	}
	log.Info("Index Created!")
	err = suite.esc.WaitForYellowStatus("10s")
	if err != nil {
		panic(err)
	}
}

func (suite *TokenSuite) TearDownSuite() {
	// Remove test index.
	res, err := suite.esc.DeleteIndex().Index([]string{suite.indexName}).Do(suite.ctx)
	if err != nil {
		panic(err)
	}
	if !res.Acknowledged {
		panic("Creation of index was not Acknowledged.")
	}
	log.Info("Index Deleted!")
	// Close connections.
	common.Shutdown()
	// Drop test database.
	suite.Require().Nil(suite.DestroyTestDB())
}

func (suite *TokenSuite) SetupTest() {
	//r := require.New(suite.T())
	// Remove test index.
	//res, err := suite.esc.DeleteIndex().Index([]string{suite.indexName}).Do(suite.ctx)
	//r.Nil(err)
	//r.True(res.Acknowledged)
	// create test index.
}

func (suite *TokenSuite) TearDownTest() {
	//r := require.New(suite.T())
}

func (suite *TokenSuite) openIndex() {
	openRes, err := suite.esc.OpenIndex(suite.indexName).Do(suite.ctx)
	if err != nil {
		log.Error(errors.Wrapf(err, "OpenIndex: %s", suite.indexName))
		return
	}
	if !openRes.Acknowledged {
		log.Errorf("OpenIndex not Acknowledged: %s", suite.indexName)
		return
	}
}

// Each string is a set of words comma separated:
// []string{"a,b,c", "1,2,3"}
func (suite *TokenSuite) updateSynonyms(synonyms []string) error {
	bodyMask := `{
		"index" : {
			"analysis" : {
				"filter" : {
					"english_stop": {
						"stopwords": "_english_", 
						"type": "stop"
					}, 
					"english_stemmer": {
						"type": "stemmer", 
						"language": "english"
					}, 
					"english_possessive_stemmer": {
						"type": "stemmer", 
						"language": "possessive_english"
					},
					"synonym_graph" : {
						"type": "synonym_graph",
						"tokenizer": "keyword",
						"synonyms" : [
							%s
						]
					},
					"he_IL": {
						"locale": "he_IL",
						"type": "hunspell",
						"dedup": "true"
					}
				},
				"char_filter": {
					"quotes": {
						"type": "mapping", 
						"mappings": [
							"\\u0091=>\\u0027", 
							"\\u0092=>\\u0027", 
							"\\u2018=>\\u0027", 
							"\\u2019=>\\u0027", 
							"\\u201B=>\\u0027", 
							"\\u0022=>", 
							"\\u201C=>", 
							"\\u201D=>", 
							"\\u05F4=>"
						]
					}
				}, 
				"analyzer": {
					"hebrew_synonym": {
						"filter": [
							"synonym_graph",
							"he_IL"
						], 
						"char_filter": [
							"quotes"
						], 
						"tokenizer": "standard"
					},
					"english_synonym": {
						"filter": [
							"english_possessive_stemmer", 
							"lowercase", 
							"english_stop", 
							"english_stemmer", 
							"synonym_graph"
						], 
						"tokenizer": "standard"
					}
				}
			}
		}
	}`
	keywords := []string{}
	for _, synonymGroup := range synonyms {
		quoted := fmt.Sprintf("\"%s\"", synonymGroup)
		keywords = append(keywords, quoted)
	}
	synonymsBody := fmt.Sprintf(bodyMask, strings.Join(keywords, ","))

	// Close the index in order to update the synonyms
	closeRes, err := suite.esc.CloseIndex(suite.indexName).Do(suite.ctx)
	if err != nil {
		log.Error(errors.Wrapf(err, "CloseIndex: %s", suite.indexName))
		return err
	}
	if !closeRes.Acknowledged {
		log.Errorf("CloseIndex not Acknowledged: %s", suite.indexName)
		return err
	}

	defer func() {
		suite.openIndex()
		err = suite.esc.WaitForYellowStatus("10s")
		if err != nil {
			panic(err)
		}
	}()

	//log.Infof("Update settings: %+v", synonymsBody)
	settingsRes, err := suite.esc.IndexPutSettings(suite.indexName).BodyString(synonymsBody).Do(suite.ctx)
	if err != nil {
		log.Error(errors.Wrapf(err, "IndexPutSettings: %s", suite.indexName))
		return err
	}
	if !settingsRes.Acknowledged {
		log.Errorf("IndexPutSettings not Acknowledged: %s", suite.indexName)
		return errors.New(fmt.Sprintf("IndexPutSettings not Acknowledged: %s", suite.indexName))
	}

	return nil
}

func (suite *TokenSuite) Tokens(phrase string, lang string) []*search.TokenNode {
	r := require.New(suite.T())
	tokens, err := search.MakeTokensFromPhraseIndex(phrase, lang, suite.esc, suite.indexName, suite.ctx)
	if err != nil {
		log.Infof("Err: %+v", err)
	}
	r.Nil(err)
	log.Infof("%s:\n%s", phrase, search.TokenNodesToString(tokens))
	return tokens
}

func (suite *TokenSuite) TestTokenNodesToPhrases1() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"one,two words"}))
	a := suite.Tokens("this is one thing for me.", "en")
	phrases := search.TokenNodesToPhrases(a)
	r.Equal(2, len(phrases))
	r.Equal("this is one thing for me.", phrases[0].OriginalJoin())
	// Note we don't know how to bring synonym original phrase, just the already reduces.
	// For now leave as is...
	// Should be : "this is two words thing for me."
	r.Equal("this is two word thing for me.", phrases[1].OriginalJoin())

	r.Nil(suite.updateSynonyms([]string{
		"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום",
	}))
	a = suite.Tokens("שיעורי הרב\"ש.", "he")
	for _, phrase := range search.TokenNodesToPhrases(a) {
		r.Equal("שיעורי הרב\"ש.", phrase.OriginalJoin())
	}
}

func (suite *TokenSuite) TestMatchTokensEn() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"one,two words"}))
	a := suite.Tokens("this is one thing for me.", "en")
	b := suite.Tokens("this is two words thing for me.", "en")
	r.True(search.TokensSingleMatch(a, b))
}

func (suite *TokenSuite) TestTokensNodesToPhrasesWithSynonyms() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"חיים קהילתיים,חיי קהילה,קהילה"}))
	a := suite.Tokens("קהילה יהודית מונטריי", "he")
	phrases := search.TokenNodesToPhrases(a)
	r.Equal(3, len(phrases))
	r.Equal("חיי קהילה יהודית מונטריי", phrases[0].OriginalJoin())
	r.Equal("חיים קהילתיים יהודית מונטריי", phrases[1].OriginalJoin())
	r.Equal("קהילה יהודית מונטריי", phrases[2].OriginalJoin())
}

// This test will fail due to synonyms being before Hunspell
// Commenting out until this is fixed in elastic.
//func (suite *TokenSuite) TestMatchTokensHe1() {
//	r := require.New(suite.T())
//	r.Nil(suite.updateSynonyms([]string{"זוהר לעם,זוהר,ספר הזוהר,הזוהר"}))
//	a := suite.Tokens("מבוא לספר הזוהר", "he")
//	b := suite.Tokens("מבוא לזוהר", "he")
//	r.True(search.TokensSingleMatch(a, b))
//}

func (suite *TokenSuite) TestMatchTokensHe2() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"ניסיון,ניב טוב", "ספר הזוהר,זוהר,לספר הזוהר,לספר זוהר,לזוהר"}))
	a := suite.Tokens("ניסיון מבוא לספר הזוהר", "he")
	b := suite.Tokens("ניב טוב מבוא לזוהר", "he")
	r.True(search.TokensSingleMatch(a, b))
}

func (suite *TokenSuite) TestTokenCache() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{}))
	tc := search.MakeTokensCache(2)
	// Cache empty.
	r.False(tc.Has("phrase one", "en"))
	r.Nil(tc.Get("phrase one", "en"))
	// Set one and two phrases.
	a := suite.Tokens("phrase one.", "en")
	tc.Set("phrase one", "en", a)
	b := suite.Tokens("phrase two.", "en")
	tc.Set("phrase two", "en", b)
	// Check both exist.
	r.True(tc.Has("phrase one", "en"))
	r.True(search.TokensSingleMatch(tc.Get("phrase one", "en"), a))
	r.True(tc.Has("phrase two", "en"))
	r.True(search.TokensSingleMatch(tc.Get("phrase two", "en"), b))
	// Add third phrase.
	c := suite.Tokens("phrase three", "en")
	tc.Set("phrase three", "en", c)
	// Check first is out, second and third are in.
	r.False(tc.Has("phrase one", "en"))
	r.Nil(tc.Get("phrase one", "en"))
	r.True(tc.Has("phrase three", "en"))
	r.True(search.TokensSingleMatch(tc.Get("phrase three", "en"), c))
	r.True(tc.Has("phrase two", "en"))
	r.True(search.TokensSingleMatch(tc.Get("phrase two", "en"), b))
	// Now two is fresher then three. Add fourth phrase.
	d := suite.Tokens("phrase four", "en")
	tc.Set("phrase four", "en", d)
	// Make sure third is out and second and fourth are in.
	r.False(tc.Has("phrase three", "en"))
	r.Nil(tc.Get("phrase three", "en"))
	r.True(tc.Has("phrase two", "en"))
	r.True(search.TokensSingleMatch(tc.Get("phrase two", "en"), b))
	r.True(tc.Has("phrase four", "en"))
	r.True(search.TokensSingleMatch(tc.Get("phrase four", "en"), d))
}

//func (suite *TokenSuite) TestTokensMerge() {
//	r := require.New(suite.T())
//	a := suite.Tokens("me is the best", "en")
//	b := suite.Tokens("me is the worst", "en")
//	c := search.MergeTokenGraphs(a, b)
//
//	d := suite.Tokens("best", "en")
//	s, err := search.TokensSingleSearch(d, c)
//	r.Nil(err)
//	r.Equal("me is the best", s)
//
//	e := suite.Tokens("worst", "en")
//	s, err = search.TokensSingleSearch(e, c)
//	r.Nil(err)
//	r.Equal("me is the worst", s)
//
//	r.Nil(suite.updateSynonyms([]string{"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום"}))
//
//	a = suite.Tokens("שיעורי הרב\"ש", "he")
//	b = suite.Tokens("שיעורי רב\"ש", "he")
//	c = suite.Tokens("שיעורים עם הרב\"ש", "he")
//
//	d = search.MergeTokenGraphs(c, a)
//	e = search.MergeTokenGraphs(d, b)
//
//	f := suite.Tokens("הרבש", "he")
//	s, err = search.TokensSingleSearch(f, e)
//	r.Nil(err)
//	r.Equal("שיעורי הרב\"ש", s)
//
//	r.Nil(suite.updateSynonyms([]string{
//		"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום",
//		"בעל הסולם,בעהס,רב יהודה אשלג,רב יהודה הלוי אשלג,רב יהודה ליב אשלג,רב יהודה ליב הלוי אשלג,רב אשלג,יהודה אשלג",
//	}))
//	toMerge := []string{
//		"בעל הסולם",
//		"רב\"ש",
//	}
//	a = suite.Tokens(toMerge[0], "he")
//	for _, phrase := range toMerge[1:] {
//		a = search.MergeTokenGraphs(a, suite.Tokens(phrase, "he"))
//	}
//	log.Infof("a:\n%s", search.TokenNodesToString(a))
//
//	b = suite.Tokens("רבש", "he")
//	s, err = search.TokensSingleSearch(b, a)
//	r.Nil(err)
//	// Note that we don't know how to match the correct synonym,
//	// we just match something according to alphabetical order.
//	// We should try to match according to the requested original form.
//	r.Equal("רב ברוך שלום הלוי אשלג", s)
//
//	b = suite.Tokens("רב ברוך", "he")
//	s, err = search.TokensSingleSearch(b, a)
//	r.Nil(err)
//	r.Equal("רב ברוך שלום הלוי אשלג", s)
//}

func (suite *TokenSuite) TestTokensSingleSearch() {
	r := require.New(suite.T())
	a := suite.Tokens("me is th rest", "en")
	b := suite.Tokens("some thing interesting", "en")
	s, err := search.TokensSingleSearch(a, b)
	r.Nil(err)
	r.Equal("some thing interesting", s)

	a = suite.Tokens("רות נ", "he")
	b = suite.Tokens("סדרות לימוד נבחרות", "he")
	s, err = search.TokensSingleSearch(a, b)
	r.Nil(err)
	r.Equal("סדרות לימוד נבחרות", s)

	a = suite.Tokens("רות נב", "he")
	b = suite.Tokens("סדרות לימוד נבחרות", "he")
	s, err = search.TokensSingleSearch(a, b)
	r.Nil(err)
	r.Equal(s, "סדרות לימוד נבחרות")
	a = suite.Tokens("רות נ", "he")
	b = suite.Tokens("סדרות לימוד נבחרות", "he")
	s, err = search.TokensSingleSearch(a, b)
	r.Nil(err)
	r.Equal("סדרות לימוד נבחרות", s)
}

func (suite *TokenSuite) TestMatchTokensWithSynonymsHe() {
	log.Info("TestMatchTokensWithSynonymsHe")
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"רב לייטמן,רב דר מיכאל לייטמן,רב מיכאל לייטמן"}))
	a := suite.Tokens("רב לייטמן", "he")
	b := suite.Tokens("רב דר מיכאל לייטמן", "he")
	r.True(search.TokensSingleMatch(a, b))
}

func (suite *TokenSuite) TestSortWorks() {
	log.Info("TestSortWorks")
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"aaaa,bbbb,cccc,dddd"}))
	a := suite.Tokens("dddd cccc aaaa bbbb", "he")
	phrases := search.TokenNodesToPhrases(a)
	r.Equal(256, len(phrases))
	r.Equal("aaaa aaaa aaaa aaaa", phrases[0].Join(" "))
	r.Equal("aaaa aaaa aaaa bbbb", phrases[1].Join(" "))
	r.Equal("cccc aaaa aaaa aaaa", phrases[128].Join(" "))
	r.Equal("dddd dddd dddd dddd", phrases[255].Join(" "))
}

//func (suite *TokenSuite) TestHaklatot() {
//	log.Info("TestHaklatot")
//	r := require.New(suite.T())
//	r.Nil(suite.updateSynonyms([]string{}))
//
//	toMerge := []string{
//		"הקלטות רב\"ש",
//		"הקלטות רבש",
//		"שיעורי הרב\"ש",
//		"שיעורי רב\"ש",
//		"שיעורים עם הרב\"ש",
//	}
//	a := suite.Tokens(toMerge[0], "he")
//	for _, phrase := range toMerge[1:] {
//		a = search.MergeTokenGraphs(a, suite.Tokens(phrase, "he"))
//	}
//	log.Infof("a:\n%s", search.TokenNodesToString(a))
//	r.True(false)
//}
