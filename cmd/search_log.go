package cmd

import (
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "log commands",
	Run:   logFn,
}

var queriesCmd = &cobra.Command{
	Use:   "queries",
	Short: "Get logged queries from ElasticSearch",
	Run:   queriesFn,
}

var elasticUrl string

func init() {
	RootCmd.AddCommand(logCmd)

	queriesCmd.PersistentFlags().StringVar(&elasticUrl, "elastic", "", "URL of Elastic.")
	queriesCmd.MarkFlagRequired("elastic")
	viper.BindPFlag("elasticsearch.url", queriesCmd.PersistentFlags().Lookup("elastic"))
	logCmd.AddCommand(queriesCmd)
}

func logFn(cmd *cobra.Command, args []string) {
	fmt.Println("Use one of the subcommands.")
}

func queriesFn(cmd *cobra.Command, args []string) {
	log.Infof("Setting up connection to ElasticSearch: %s", elasticUrl)
	esc, err := elastic.NewClient(
		elastic.SetURL(elasticUrl),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetErrorLog(log.StandardLogger()),
		// Should be commented out in prod.
		// elastic.SetInfoLog(log.StandardLogger()),
		// elastic.SetTraceLog(log.StandardLogger()),
	)
	utils.Must(err)

	logger := search.MakeSearchLogger(esc)
	queries, err := logger.GetAllQueries()
	utils.Must(err)
	log.Infof("Found %d queries.", len(queries))
	log.Info("#\tCreated\tTerm\tExact\tFilters\tLanguages\tFrom\tSize\tSortBy\tError")
	for i, sl := range queries {
		filters, err := utils.PrintMap(sl.Query.Filters)
		utils.Must(err)
		log.Infof("%5d\t%s\t%40s\t%5s\t%5s\t%10s\t%5d\t%5d\t%10s\t%6t",
			i,
			sl.Created.Format("2006-01-02 15:04:05"),
			sl.Query.Term,
			strings.Join(sl.Query.ExactTerms, ","),
			filters,
			strings.Join(sl.Query.LanguageOrder, ","),
			sl.From, sl.Size, sl.SortBy, sl.Error != nil)
	}
}
