package es

import (
    "time"

	log "github.com/Sirupsen/logrus"
)

func IndexCmd(index string) {
	clock := Init()
    indexer := MakeIndexer("prod", []string{index})
    indexer.ReindexAll()
	Shutdown()
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

