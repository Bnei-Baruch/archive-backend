package es

import (
	"database/sql"
	"time"

	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/boil"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var (
	db  *sql.DB
	esc *elastic.Client
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
        db = defaultDb
    } else {
        log.Info("Setting up connection to MDB")
        db, err = sql.Open("postgres", viper.GetString("mdb.url"))
        utils.Must(err)
        utils.Must(db.Ping())
        boil.SetDB(db)
        //boil.DebugMode = true
    }

	log.Info("Setting up connection to ElasticSearch")
	url := viper.GetString("elasticsearch.url")
	esc, err = elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetErrorLog(log.StandardLogger()),
		//elastic.SetInfoLog(log.StandardLogger()),
	)
	utils.Must(err)

	esversion, err := esc.ElasticsearchVersion(url)
	utils.Must(err)
	log.Infof("Elasticsearch version %s", esversion)

	log.Info("Initializing static data from MDB")
	utils.Must(mdb.InitTypeRegistries(db))

	return clock
}

func Shutdown() {
	utils.Must(db.Close())
	esc.Stop()
}

