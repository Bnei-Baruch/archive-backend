package common

import (
	"database/sql"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/boil"
	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var (
	DB     *sql.DB
	ESC    *search.ESManager
	LOGGER *search.SearchLogger
	CACHE  cache.CacheManager
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
	utils.Must(mdb.InitTypeRegistries(DB))

	log.Info("Setting up connection to ElasticSearch")
	url := viper.GetString("elasticsearch.url")
	ESC = search.MakeESManager(url)

	LOGGER = search.MakeSearchLogger(ESC)

	//esversion, err := ESC.ElasticsearchVersion(url)
	//utils.Must(err)
	//log.Infof("Elasticsearch version %s", esversion)

	es.InitVars()

	viper.SetDefault("cache.refresh-search-stats", 5*time.Minute)
	refreshIntervals := map[string]time.Duration{
		"SearchStats": viper.GetDuration("cache.refresh-search-stats"),
	}
	CACHE = cache.NewCacheManagerImpl(DB, refreshIntervals)

	return clock
}

func Shutdown() {
	utils.Must(DB.Close())
	ESC.Stop()
	CACHE.Close()
}
