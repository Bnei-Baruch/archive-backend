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
							"he_IL",
							"synonym_graph"
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
	return tokens
}

func (suite *TokenSuite) TestMatchTokensEn() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"one,two words"}))
	a := suite.Tokens("this is one thing for me.", "en")
	log.Infof("a:\n%+v", search.TokenNodesToString(a))
	b := suite.Tokens("this is two words thing for me.", "en")
	log.Infof("b:\n%+v", search.TokenNodesToString(b))
	r.True(search.TokensMatch(a, b))
}

func (suite *TokenSuite) TestMatchTokensHe1() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"זוהר לעם,זוהר,ספר הזוהר,הזוהר"}))
	a := suite.Tokens("מבוא לספר הזוהר", "he")
	b := suite.Tokens("מבוא לזוהר", "he")
	r.True(search.TokensMatch(a, b))
}

func (suite *TokenSuite) TestMatchTokensHe2() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"ניסיון,ניב טוב", "ספר הזוהר,זוהר"}))
	a := suite.Tokens("ניסיון מבוא לספר הזוהר", "he")
	b := suite.Tokens("ניב טוב מבוא לזוהר", "he")
	r.True(search.TokensMatch(a, b))
}

func (suite *TokenSuite) TestMatchTokensHe2WithoutHForZohar() {
	r := require.New(suite.T())
	r.Nil(suite.updateSynonyms([]string{"ניסיון,ניב טוב", "ספר הזוהר,זוהר"}))
	a := suite.Tokens("ניסיון מבוא לספר הזוהר", "he")
	b := suite.Tokens("ניב טוב מבוא לזוהר", "he")
	r.True(search.TokensMatch(a, b))
}
