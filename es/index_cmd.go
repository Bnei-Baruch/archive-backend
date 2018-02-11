package es

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

func IndexCmd(index string) {
	clock := mdb.Init()
	indexer := MakeIndexer("prod", []string{index})
	err := indexer.ReindexAll()
	if err != nil {
		log.Error(err)
	}
	mdb.Shutdown()
	if err == nil {
		log.Info("Success")
		log.Infof("Total run time: %s", time.Now().Sub(clock).String())
	}
}
