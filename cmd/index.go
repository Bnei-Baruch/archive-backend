package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	elastic "gopkg.in/olivere/elastic.v6"

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

var updateSynonymsCmd = &cobra.Command{
	Use:   "update_synonyms",
	Short: "Update synonym keywords list.",
	Run:   updateSynonymsFn,
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
	RootCmd.AddCommand(updateSynonymsCmd)
	switchAliasCmd.PersistentFlags().StringVar(&indexDate, "index_date", "", "Index date to switch to.")
	switchAliasCmd.MarkFlagRequired("index_date")
}

func indexFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	t := time.Now()
	date := strings.ToLower(t.Format(time.RFC3339))

	err, prevDate := es.ProdAliasedIndexDate(common.ESC)
	if err != nil {
		log.Error(err)
		return
	}

	if date == prevDate {
		log.Info(fmt.Sprintf("New index date is the same as previous index date %s. Wait a minute and rerun.", prevDate))
		return
	}

	indexer, err := es.MakeProdIndexer(date, common.DB, common.ESC)
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
	err = es.SwitchProdAliasToCurrentIndex(date, common.ESC)
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

	err := es.SwitchProdAliasToCurrentIndex(strings.ToLower(indexDate), common.ESC)
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

	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := es.IndexName("prod", consts.ES_RESULTS_INDEX, lang, strings.ToLower(indexDate))
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

func updateSynonymsFn(cmd *cobra.Command, args []string) {
	clock := common.Init()
	defer common.Shutdown()

	synonymsConfigPath := viper.GetString("elasticsearch.synonyms-config-path")
	synonymsIndex := viper.GetString("elasticsearch.synonyms-index")

	keywords := make([]string, 0)
	file, err := os.Open(synonymsConfigPath)
	if err != nil {
		log.Error(errors.Wrap(err, "Unable to open synonyms file."))
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		//  Blank lines and lines starting with pound are comments (like Solr format).
		if line != "" && !strings.HasPrefix(line, "#") {
			fline := fmt.Sprintf("\"%s\"", line)
			keywords = append(keywords, fline)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error(errors.Wrap(err, "Synonym config file scan error."))
		return
	}

	//log.Printf("Keywords: %v", keywords)

	bodyMask := `{
			"index" : {
				"analysis" : {
					"filter" : {
						"synonym" : {
							"type" : "synonym",
							"synonyms" : [
								%s
							]
						}
					}
				}
			}
		}`

	body := fmt.Sprintf(bodyMask, strings.Join(keywords, ","))

	//log.Printf("Update synonyms request body: %v", body)

	// Close the index in order to update the synonyms
	closeRes, err := common.ESC.CloseIndex(synonymsIndex).Do(context.TODO())
	if err != nil {
		log.Error(errors.Wrap(err, "CloseIndex"))
		return
	}
	if !closeRes.Acknowledged {
		log.Errorf("CloseIndex not Acknowledged")
		return
	}

	_, err = common.ESC.PerformRequest(context.TODO(), elastic.PerformRequestOptions{
		Method: "PUT",
		Path:   fmt.Sprintf("/%s/_settings", synonymsIndex),
		Body:   body,
	})
	if err != nil {
		log.Error(errors.Wrap(err, "Updating synonyms to elastic index error."))
		log.Info("Reopening the index")
		openRes, err := common.ESC.OpenIndex(synonymsIndex).Do(context.TODO())
		if err != nil {
			log.Error(errors.Wrap(err, "OpenIndex"))
		}
		if !openRes.Acknowledged {
			log.Errorf("OpenIndex not Acknowledged")
		}
		return
	}

	//  Now reopen the index
	openRes, err := common.ESC.OpenIndex(synonymsIndex).Do(context.TODO())
	if err != nil {
		log.Error(errors.Wrap(err, "OpenIndex"))
		return
	}
	if !openRes.Acknowledged {
		log.Errorf("OpenIndex not Acknowledged")
		return
	}

	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}
