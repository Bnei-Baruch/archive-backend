package search

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	elastic "gopkg.in/olivere/elastic.v6"
)

type ESManager struct {
	esc *elastic.Client
	url string
}

func MakeESManager(url string) *ESManager {
	esManager := &ESManager{}
	esManager.url = url
	esManager.GetClient()
	return esManager
}

func (esManager *ESManager) GetClient() (*elastic.Client, error) {

	var err error
	if esManager.esc == nil {

		log.Info("Trying to set up new connection to ElasticSearch")
		url := viper.GetString("elasticsearch.url")

		esManager.esc, err = elastic.NewClient(
			elastic.SetURL(url),
			elastic.SetSniff(false),
			elastic.SetHealthcheckInterval(10*time.Second),
			elastic.SetErrorLog(log.StandardLogger()),
			// Should be commented out in prod.
			// elastic.SetInfoLog(log.StandardLogger()),
			// elastic.SetTraceLog(log.StandardLogger()),
		)
	}
	return esManager.esc, err
}

func (esManager *ESManager) Stop() {
	if esManager.esc != nil {
		esManager.esc.Stop()
	}
}
