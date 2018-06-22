package cmd

import (
	"fmt"
	"sort"
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

var clicksCmd = &cobra.Command{
	Use:   "clicks",
	Short: "Get logged clicks from ElasticSearch",
	Run:   clicksFn,
}

var elasticUrl string

func init() {
	RootCmd.AddCommand(logCmd)

	logCmd.PersistentFlags().StringVar(&elasticUrl, "elastic", "", "URL of Elastic.")
	logCmd.MarkFlagRequired("elastic")
	viper.BindPFlag("elasticsearch.url", logCmd.PersistentFlags().Lookup("elastic"))

	logCmd.AddCommand(queriesCmd)
	logCmd.AddCommand(clicksCmd)
}

func logFn(cmd *cobra.Command, args []string) {
	fmt.Println("Use one of the subcommands.")
}

func initLogger() *search.SearchLogger {
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

	return search.MakeSearchLogger(esc)
}

func queriesFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	queries, err := logger.GetAllQueries()
	utils.Must(err)
	log.Infof("Found %d queries.", len(queries))
	log.Info("#\tSearchId\tCreated\tTerm\tExact\tFilters\tLanguages\tFrom\tSize\tSortBy\tSuggestion\tError")
	sortedQueries := make(search.CreatedSearchLogs, 0, len(queries))
	for _, q := range queries {
		sortedQueries = append(sortedQueries, q)
	}
	sort.Sort(sortedQueries)
	for i, sl := range sortedQueries {
		filters, err := utils.PrintMap(sl.Query.Filters)
		utils.Must(err)
		log.Infof("%5d\t%16s\t%20s\t%40s\t%5s\t%5s\t%10s\t%5d\t%5d\t%10s\t%15s\t%6t",
			i+1,
			sl.SearchId,
			sl.Created.Format("2006-01-02 15:04:05"),
			sl.Query.Term,
			strings.Join(sl.Query.ExactTerms, ","),
			filters,
			strings.Join(sl.Query.LanguageOrder, ","),
			sl.From, sl.Size, sl.SortBy, sl.Suggestion, sl.Error != nil)
	}
}

func clicksFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	clicks, err := logger.GetAllClicks()
	utils.Must(err)
	log.Infof("Found %d clicks.", len(clicks))
	log.Info("#\tSearchId\tCreated\tRank\tMdbUid\tIndex\tType")
	sortedClicks := make(search.CreatedSearchClicks, 0, len(clicks))
	for _, q := range clicks {
		sortedClicks = append(sortedClicks, q)
	}
	sort.Sort(sortedClicks)
	for i, sq := range sortedClicks {
		log.Infof("%5d\t%16s\t%20s\t%3d\t%10s\t%20s\t%17s",
			i+1,
			sq.SearchId,
			sq.Created.Format("2006-01-02 15:04:05"),
			sq.Rank,
			sq.MdbUid,
			sq.Index,
			sq.ResultType)
	}
}
