package mdb

import (
	"database/sql"
	"os"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/boil"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

var (
	DB           *sql.DB
	ESC          *elastic.Client
	SofficeBin   string
	DocFolder    string
	ParseDocsBin string
	CDNUrl       string
	PythonPath   string
	Os           string
)

func Init() time.Time {
	return InitWithDefault(nil)
}

func InitWithDefault(defaultDb *sql.DB) time.Time {
	var err error
	clock := time.Now()

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	//log.SetLevel(log.WarnLevel)

	if defaultDb != nil {
		DB = defaultDb
	} else {
		log.Info("Setting up connection to MDB")
		DB, err = sql.Open("postgres", viper.GetString("mdb.url"))
		utils.Must(err)
		utils.Must(DB.Ping())
	}
	boil.SetDB(DB)
	boil.DebugMode = viper.GetString("server.boiler-mode") == "debug"
	log.Info("Initializing type registries")
	utils.Must(InitTypeRegistries(DB))

	// MOVE THIS CODE UNDER es PACKAGE.
	log.Info("Setting up connection to ElasticSearch")
	url := viper.GetString("elasticsearch.url")
	ESC, err = elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetErrorLog(log.StandardLogger()),
		// Should be commented out in prod.
		// elastic.SetInfoLog(log.StandardLogger()),
		// elastic.SetTraceLog(log.StandardLogger()),
	)
	utils.Must(err)

	esversion, err := ESC.ElasticsearchVersion(url)
	utils.Must(err)
	log.Infof("Elasticsearch version %s", esversion)

	viper.SetDefault("elasticsearch.python-path", "C:\\Python27\\python.exe") //for Windows OS only
	viper.SetDefault("mdb.os", "linux")
	PythonPath = viper.GetString("elasticsearch.python-path")
	Os = viper.GetString("mdb.os")

	SofficeBin = viper.GetString("elasticsearch.soffice-bin")
	if SofficeBin == "" {
		panic("Soffice binary should be set in config.")
	}
	if _, err := os.Stat(SofficeBin); os.IsNotExist(err) {
		panic("Soffice binary not found.")
	}

	ParseDocsBin = viper.GetString("elasticsearch.parse-docs-bin")
	if ParseDocsBin == "" {
		panic("parse_docs.py binary should be set in config.")
	}
	if _, err := os.Stat(ParseDocsBin); os.IsNotExist(err) {
		panic("parse_docs.py not found.")
	}

	DocFolder = path.Join(viper.GetString("elasticsearch.docx-folder"))
	utils.Must(os.MkdirAll(DocFolder, 0777))

	CDNUrl = viper.GetString("elasticsearch.cdn-url")
	if CDNUrl == "" {
		panic("cdn url should be set in config.")
	}

	return clock
}

func Shutdown() {
	utils.Must(DB.Close())
	ESC.Stop()
}
