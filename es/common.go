package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/boil"
	"gopkg.in/olivere/elastic.v5"

	"fmt"
	"github.com/Bnei-Baruch/archive-backend/bindata"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var (
	db  *sql.DB
	esc *elastic.Client
)

func Init() time.Time {
	var err error
	clock := time.Now()

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	//log.SetLevel(log.WarnLevel)

	log.Info("Setting up connection to MDB")
	db, err = sql.Open("postgres", viper.GetString("mdb.url"))
	utils.Must(err)
	utils.Must(db.Ping())
	boil.SetDB(db)
	//boil.DebugMode = true

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

func recreateIndex(name string, definition string) error {
	log.Infof("Recreating index %s from %s", name, definition)

	// delete index if it's already exists
	exists, err := esc.IndexExists(name).Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Index exists ?")
	}
	if exists {
		log.Debugf("Index %s already exist, deleting...", name)
		res, err := esc.DeleteIndex(name).Do(context.TODO())
		if err != nil {
			return errors.Wrap(err, "Delete index")
		}
		if !res.Acknowledged {
			return errors.Errorf("Index deletion wasn't acknowledged: %s", name)
		}
	}

	// read mappings and create index
	mappings, err := bindata.Asset(definition)
	if err != nil {
		return errors.Wrap(err, "Load binary data")
	}
	var bodyJson map[string]interface{}
	if err = json.Unmarshal(mappings, &bodyJson); err != nil {
		return errors.Wrap(err, "json.Unmarshal")
	}
	res, err := esc.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Create index")
	}
	if !res.Acknowledged {
		return errors.Errorf("Index creation wasn't acknowledged: %s", name)
	}

	// update index's settings for bulk indexing.
	// Uncomment before commit.
	 res2, err := esc.IndexPutSettings(name).BodyJson(map[string]interface{}{
	 	"refresh_interval":   "-1",
	 	"number_of_replicas": 0,
	 }).Do(context.TODO())
	 if err != nil {
	 	return errors.Wrap(err, "Change index settings")
	 }
	 if !res2.Acknowledged {
	 	return errors.Errorf("Update index settings wasn't acknowledged: %s", name)
	 }

	return nil
}

func finishIndexing(name string) error {
	// change back index's settings after bulk indexing
	res, err := esc.IndexPutSettings(name).BodyJson(map[string]interface{}{
		"refresh_interval":   "1s",
		"number_of_replicas": 1,
	}).Do(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Change index settings")
	}
	if !res.Acknowledged {
		return errors.Errorf("Update index settings wasn't acknowledged: %s", name)
	}

	return nil
}

func IndexName(prefix string, language string) string {
	return fmt.Sprintf("%s_%s", prefix, language)
}
