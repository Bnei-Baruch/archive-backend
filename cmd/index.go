package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

var prepareDocsCmd = &cobra.Command{
	Use:   "prepare_docs",
	Short: "Prepares all docs via Unzip service.",
	Run:   prepareDocsFn,
}

var deleteIndexCmd = &cobra.Command{
	Use:   "delete_index",
	Short: "Delete index.",
	Run:   deleteIndexFn,
}

var restartSearchLogsCmd = &cobra.Command{
	Use:   "restart_search_logs",
	Short: "Restarts search logs.",
	Run:   restartSearchLogsFn,
}

var switchAliasCmd = &cobra.Command{
	Use:   "switch_alias",
	Short: "Switch Elastic to use different index.",
	Run:   switchAliasFn,
}

var simulateUpdateCmd = &cobra.Command{
	Use:   "simulate_update",
	Short: "Simulate index update.",
	Run:   simulateUpdateFn,
}

var indexDate string

func init() {
	RootCmd.AddCommand(indexCmd)
	RootCmd.AddCommand(prepareDocsCmd)
	deleteIndexCmd.PersistentFlags().StringVar(&indexDate, "index_date", "", "Index date to be deleted.")
	deleteIndexCmd.MarkFlagRequired("index_date")
	RootCmd.AddCommand(deleteIndexCmd)
	RootCmd.AddCommand(restartSearchLogsCmd)
	RootCmd.AddCommand(switchAliasCmd)
	switchAliasCmd.PersistentFlags().StringVar(&indexDate, "index_date", "", "Index date to switch to.")
	switchAliasCmd.MarkFlagRequired("index_date")
	RootCmd.AddCommand(simulateUpdateCmd)
}

func indexFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	t := time.Now()
	date := strings.ToLower(t.Format(time.RFC3339))

	esc, err := common.ESC.GetClient()
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to connect to ElasticSearch."))
		return
	}

	err, prevDate := es.ProdAliasedIndexDate(esc)
	if err != nil {
		log.Error(err)
		return
	}

	if date == prevDate {
		log.Info(fmt.Sprintf("New index date is the same as previous index date %s. Wait a minute and rerun.", prevDate))
		return
	}

	indexer, err := es.MakeProdIndexer(date, common.DB, esc)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Preparing all documents with Unzip.")
	err = es.ConvertDocx(common.DB)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Done preparing documents.")
	err = indexer.ReindexAll()
	if err != nil {
		log.Error(err)
		return
	}
	err = es.SwitchProdAliasToCurrentIndex(date, esc)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func prepareDocsFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	log.Info("Preparing all documents with Unzip.")
	err := es.ConvertDocx(common.DB)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Done preparing documents.")
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func switchAliasFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	esc, err := common.ESC.GetClient()
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to connect to ElasticSearch."))
		return
	}

	err = es.SwitchProdAliasToCurrentIndex(strings.ToLower(indexDate), esc)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func deleteIndexFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	esc, err := common.ESC.GetClient()
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to connect to ElasticSearch."))
		return
	}

	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := es.IndexName("prod", consts.ES_RESULTS_INDEX, lang, strings.ToLower(indexDate))
		exists, err := esc.IndexExists(name).Do(context.TODO())
		if err != nil {
			log.Error(err)
			return
		}
		if exists {
			res, err := esc.DeleteIndex(name).Do(context.TODO())
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

	esc, err := common.ESC.GetClient()
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to connect to ElasticSearch."))
		return
	}

	name := "search_logs"
	exists, err := esc.IndexExists(name).Do(context.TODO())
	if err != nil {
		log.Error(err)
		return
	}
	if exists {
		res, err := esc.DeleteIndex(name).Do(context.TODO())
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
	res, err := esc.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
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

func simulateUpdateFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	client, err := common.ESC.GetClient()
	if err != nil {
		log.Error(err)
		return
	}

	err, date := es.ProdAliasedIndexDate(client)
	if err != nil {
		log.Error(err)
		return
	}

	indexer, err := es.MakeProdIndexer(date, common.DB, client)
	if err != nil {
		log.Error(err)
		return
	}

	err = indexer.Update(es.Scope{CollectionUID: "zf4lLwyI"})
	//err = indexer.Update(es.Scope{ContentUnitUID: "S5cSiwqb"})
	//err = indexer.Update(es.Scope{FileUID: "QSMWk1lj"})
	//err = indexer.Update(es.Scope{ContentUnitUID: "eA0g9XRf"})
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}
