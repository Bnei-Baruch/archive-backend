package searchtest

import (
	"context"
	"fmt"
	"strings"

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

type SearchSuite struct {
	suite.Suite
	utils.TestDBManager
	Esc       *elastic.Client
	Ctx       context.Context
	IndexName string
}

func (suite *SearchSuite) SetupSuite() {
	suite.IndexName = "test_token"
	utils.InitConfig("", "../")
	err := suite.InitTestDB()
	if err != nil {
		panic(err)
	}
	suite.Ctx = context.Background()

	// Set package db and esc variables.
	common.InitWithDefault(suite.DB)
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
}

func (suite *SearchSuite) TearDownSuite() {
	// Close connections.
	common.Shutdown()
	// Drop test database.
	suite.Require().Nil(suite.DestroyTestDB())
}

func (suite *SearchSuite) SetupTest() {
	log.Info("===SetupTest===")
	esc, err := common.ESC.GetClient()
	if err != nil {
		panic(err)
	}
	suite.Esc = esc

	bodyString := `{
		"settings" : {
			"number_of_shards": 1,
			"number_of_replicas": 0
		}
	}`
	createRes, err := suite.Esc.CreateIndex(suite.IndexName).BodyString(bodyString).Do(suite.Ctx)
	if err != nil {
		panic(err)
	}
	if !createRes.Acknowledged {
		panic("Creation of index was not Acknowledged.")
	}
	log.Info("Index Created!")
	err = suite.Esc.WaitForYellowStatus("10s")
	if err != nil {
		panic(err)
	}
}

func (suite *SearchSuite) TearDownTest() {
	log.Info("===TearDownTest===")
	// Remove test index.
	res, err := suite.Esc.DeleteIndex().Index([]string{suite.IndexName}).Do(suite.Ctx)
	if err != nil {
		panic(err)
	}
	if !res.Acknowledged {
		panic("Creation of index was not Acknowledged.")
	}
	log.Info("Index Deleted!")
}

func (suite *SearchSuite) openIndex() {
	openRes, err := suite.Esc.OpenIndex(suite.IndexName).Do(suite.Ctx)
	if err != nil {
		log.Error(errors.Wrapf(err, "OpenIndex: %s", suite.IndexName))
		return
	}
	if !openRes.Acknowledged {
		log.Errorf("OpenIndex not Acknowledged: %s", suite.IndexName)
		return
	}
}

// Each string is a set of words comma separated:
// []string{"a,b,c", "1,2,3"}
func (suite *SearchSuite) UpdateSynonyms(synonyms []string) error {
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
	closeRes, err := suite.Esc.CloseIndex(suite.IndexName).Do(suite.Ctx)
	if err != nil {
		log.Error(errors.Wrapf(err, "CloseIndex: %s", suite.IndexName))
		return err
	}
	if !closeRes.Acknowledged {
		log.Errorf("CloseIndex not Acknowledged: %s", suite.IndexName)
		return err
	}

	defer func() {
		suite.openIndex()
		err = suite.Esc.WaitForYellowStatus("10s")
		if err != nil {
			panic(err)
		}
	}()

	//log.Infof("Update settings: %+v", synonymsBody)
	settingsRes, err := suite.Esc.IndexPutSettings(suite.IndexName).BodyString(synonymsBody).Do(suite.Ctx)
	if err != nil {
		log.Error(errors.Wrapf(err, "IndexPutSettings: %s", suite.IndexName))
		return err
	}
	if !settingsRes.Acknowledged {
		log.Errorf("IndexPutSettings not Acknowledged: %s", suite.IndexName)
		return errors.New(fmt.Sprintf("IndexPutSettings not Acknowledged: %s", suite.IndexName))
	}

	return nil
}

func (suite *SearchSuite) Match(match bool, values []search.VariableValue, err error, variables ...string) {
	r := require.New(suite.T())
	r.Nil(err)
	r.True(match)
	strValues := []string{}
	for i := range values {
		strValues = append(strValues, values[i].Name)
		strValues = append(strValues, values[i].Value)
	}
	r.Equal(strings.Join(variables, "|"), strings.Join(strValues, "|"))
	if len(variables)%2 == 1 {
		panic("Expecting eval number of variable strings for matching.")
	}
	r.Equal(len(variables)/2, len(values))
}

func (suite *SearchSuite) Tokens(phrase string, lang string) []*search.TokenNode {
	return suite.TokensWithVariables(phrase, lang, make(search.VariablesByName))
}

func (suite *SearchSuite) TokensWithVariables(phrase string, lang string, variables map[string]*search.Variable) []*search.TokenNode {
	r := require.New(suite.T())
	tokens, err := search.MakeTokensFromPhraseIndex(phrase, lang, suite.Esc, suite.IndexName, suite.Ctx)
	if err != nil {
		log.Infof("Err: %+v", err)
	}
	r.Nil(err)
	log.Infof("%s:\n%s", phrase, search.TokenNodesToString(tokens, variables))
	return tokens
}

func (suite *SearchSuite) MakeTranslations(variable string, language string, values map[string][]string) search.Translations {
	r := require.New(suite.T())
	t := make(search.Translations)
	t[variable] = make(map[string]map[string][][]*search.TokenNode)
	t[variable][language] = make(map[string][][]*search.TokenNode)
	for value, translations := range values {
		for _, translation := range translations {
			tokens, err := search.MakeTokensFromPhraseIndex(translation, language, suite.Esc, suite.IndexName, suite.Ctx)
			fmt.Printf("Translation: [%s]\n", translation)
			search.PrintTokens(tokens, "Tokens ", nil)

			r.Nil(err)
			t[variable][language][value] = append(t[variable][language][value], tokens)
		}
	}
	return t
}
