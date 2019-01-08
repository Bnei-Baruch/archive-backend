package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/olivere/elastic.v6"

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

func printCsv(records [][]string) {
	w := csv.NewWriter(os.Stdout)
	w.WriteAll(records)
	if err := w.Error(); err != nil {
		log.Fatalln("error writing csv:", err)
	}
}

func queriesFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	printCsv([][]string{[]string{
		"#", "SearchId", "Created", "Term", "Exact", "Filters",
		"Languages", "From", "Size", "SortBy", "Error", "Suggestion"}})
	totalQueries := 0
	SLICES := 10
	for i := 0; i < SLICES; i++ {
		s := elastic.NewSliceQuery().Id(i).Max(SLICES)
		queries, err := logger.GetAllQueries(s)
		utils.Must(err)
		totalQueries += len(queries)
		sortedQueries := make(search.CreatedSearchLogs, 0, len(queries))
		for _, q := range queries {
			sortedQueries = append(sortedQueries, q)
		}
		sort.Sort(sortedQueries)
		records := [][]string{}
		for i, sl := range sortedQueries {
			filters, err := utils.PrintMap(sl.Query.Filters)
			utils.Must(err)
			records = append(records, []string{
				fmt.Sprintf("%d", i+1),
				sl.SearchId,
				sl.Created.Format("2006-01-02 15:04:05"),
				sl.Query.Term,
				strings.Join(sl.Query.ExactTerms, ","),
				filters,
				strings.Join(sl.Query.LanguageOrder, ","),
				fmt.Sprintf("%d", sl.From),
				fmt.Sprintf("%d", sl.Size),
				sl.SortBy,
				fmt.Sprintf("%t", sl.Error != nil),
				sl.Suggestion,
			})
		}
		printCsv(records)
	}
	log.Infof("Found %d queries.", totalQueries)
}

func clicksFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	printCsv([][]string{[]string{
		"#", "SearchId", "Created", "Rank", "MdbUid", "Index", "ResultType"}})
	clicks, err := logger.GetAllClicks()
	utils.Must(err)
	log.Infof("Found %d clicks.", len(clicks))
	log.Info("#\tSearchId\tCreated\tRank\tMdbUid\tIndex\tType")
	sortedClicks := make(search.CreatedSearchClicks, 0, len(clicks))
	for _, q := range clicks {
		sortedClicks = append(sortedClicks, q)
	}
	sort.Sort(sortedClicks)
	records := [][]string{}
	for i, sq := range sortedClicks {

		records = append(records, []string{
			fmt.Sprintf("%d", i+1),
			sq.SearchId,
			sq.Created.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%d", sq.Rank),
			sq.MdbUid,
			sq.Index,
			sq.ResultType,
		})
	}
	printCsv(records)
}
