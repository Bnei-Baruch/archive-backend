package es

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

func IndexCmd(index string) {
	clock := mdb.Init()
	indexer := MakeIndexer("prod", []string{index})
	indexer.ReindexAll()
	mdb.Shutdown()
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}
