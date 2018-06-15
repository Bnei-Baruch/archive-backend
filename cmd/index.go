package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/bindata"
	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Import MDB to ElasticSearch",
	Run:   indexFn,
}

var deleteResultsIndexCmd = &cobra.Command{
	Use:   "delete_results_index",
	Short: "Delete results index.",
	Run:   deleteResultsIndexFn,
}

var indexTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Index tags in ES",
	Run:   indexTagsFn,
}

var indexUnitsCmd = &cobra.Command{
	Use:   "units",
	Short: "Index content units in ES",
	Run:   indexUnitsFn,
}

// var indexCollectionsCmd = &cobra.Command{
// 	Use:   "collections",
// 	Short: "Index content collections in ES",
// 	Run:   indexCollectionsFn,
// }

var indexSourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "Index sources in ES",
	Run:   indexSourcesFn,
}

var restartSearchLogsCmd = &cobra.Command{
	Use:   "restart_search_logs",
	Short: "Restarts search logs.",
	Run:   restartSearchLogsFn,
}

func init() {
	RootCmd.AddCommand(indexCmd)
	indexCmd.AddCommand(indexTagsCmd)
	indexCmd.AddCommand(indexUnitsCmd)
	// indexCmd.AddCommand(indexCollectionsCmd)
	indexCmd.AddCommand(indexSourcesCmd)
	indexCmd.AddCommand(deleteResultsIndexCmd)
	indexCmd.AddCommand(restartSearchLogsCmd)
}

func indexFn(cmd *cobra.Command, args []string) {
	fmt.Println("Use one of the subcommands.")
}

func indexTagsFn(cmd *cobra.Command, args []string) {
	IndexCmd(consts.ES_RESULT_TYPE_TAGS)
}

func indexUnitsFn(cmd *cobra.Command, args []string) {
	IndexCmd(consts.ES_RESULT_TYPE_UNITS)
}

// func indexCollectionsFn(cmd *cobra.Command, args []string) {
// 	IndexCmd(consts.ES_COLLECTIONS_INDEX)
// }

func indexSourcesFn(cmd *cobra.Command, args []string) {
	IndexCmd(consts.ES_RESULT_TYPE_SOURCES)
}

func IndexCmd(index string) {
	clock := common.Init()
	defer common.Shutdown()
	indexer, err := es.MakeIndexer("prod", []string{index}, common.DB, common.ESC)
	if err != nil {
		log.Error(err)
		return
	}
	err = indexer.ReindexAll()
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func deleteResultsIndexFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := es.IndexName("prod", consts.ES_RESULTS_INDEX, lang)
		exists, err := common.ESC.IndexExists(name).Do(context.TODO())
		if err != nil {
			log.Error(err)
			return
		}
		if exists {
			res, err := common.ESC.DeleteIndex(name).Do(context.TODO())
			if err != nil {
				log.Error(errors.Wrap(err, "Delete index"))
				return
			}
			if !res.Acknowledged {
				log.Error(errors.Errorf("Index deletion wasn't acknowledged: %s", name))
				return
			}
		}
	}
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func restartSearchLogsFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	name := "search_logs"
	exists, err := common.ESC.IndexExists(name).Do(context.TODO())
	if err != nil {
		log.Error(err)
		return
	}
	if exists {
		res, err := common.ESC.DeleteIndex(name).Do(context.TODO())
		if err != nil {
			log.Error(errors.Wrap(err, "Delete index"))
			return
		}
		if !res.Acknowledged {
			log.Error(errors.Errorf("Index deletion wasn't acknowledged: %s", name))
			return
		}
	}

	definition := fmt.Sprintf("data/es/mappings/%s.json", name)
	// Read mappings and create index
	mappings, err := bindata.Asset(definition)
	if err != nil {
		log.Error(errors.Wrapf(err, "Failed loading mapping %s", definition))
		return
	}
	var bodyJson map[string]interface{}
	if err = json.Unmarshal(mappings, &bodyJson); err != nil {
		log.Error(errors.Wrap(err, "json.Unmarshal"))
		return
	}
	// Create index.
	res, err := common.ESC.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
	if err != nil {
		log.Error(errors.Wrap(err, "Create index"))
		return
	}
	if !res.Acknowledged {
		log.Error(errors.Errorf("Index creation wasn't acknowledged: %s", name))
		return
	}
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}
