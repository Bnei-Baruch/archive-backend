package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/consts"

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

var latencyCmd = &cobra.Command{
	Use:   "latency",
	Short: "Get queries latency performance from ElasticSearch",
	Run:   latencyFn,
}

var elasticUrl string
var outputFile string

func init() {
	RootCmd.AddCommand(logCmd)

	logCmd.PersistentFlags().StringVar(&elasticUrl, "elastic", "", "URL of Elastic.")
	logCmd.MarkFlagRequired("elastic")
	viper.BindPFlag("elasticsearch.url", logCmd.PersistentFlags().Lookup("elastic"))

	latencyCmd.PersistentFlags().StringVar(&outputFile, "output_file", "", "CSV path to write.")

	logCmd.AddCommand(queriesCmd)
	logCmd.AddCommand(clicksCmd)
	logCmd.AddCommand(latencyCmd)
}

func logFn(cmd *cobra.Command, args []string) {
	fmt.Println("Use one of the subcommands.")
}

func initLogger() *search.SearchLogger {
	log.Infof("Setting up connection to ElasticSearch: %s", elasticUrl)
	esManager := search.MakeESManager(elasticUrl)

	return search.MakeSearchLogger(esManager)
}

func appendCsvToFile(path string, records [][]string) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0660)
	if err != nil {
		log.Fatalln("cannot open file: ", err)
	}
	defer file.Close()

	writeCsv(file, records)
}

func printCsv(records [][]string) {
	writeCsv(os.Stdout, records)
}

func writeCsv(dist io.Writer, records [][]string) {
	w := csv.NewWriter(dist)
	defer w.Flush()
	for _, record := range records {
		if err := w.Write(record); err != nil {
			log.Fatalln("error writing csv: ", err)
			return
		}
	}
}

func queriesFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	printCsv([][]string{[]string{
		"#", "SearchId", "Created", "Term", "Exact", "Filters",
		"Languages", "From", "Size", "SortBy", "Error", "Suggestion",
		"Deb"}})
	totalQueries := 0
	SLICES := 100
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
				fmt.Sprintf("%t", sl.Query.Deb),
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

func latencyFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	headers := []string{
		"#", "SearchId", "Term", "DoSearch",
	}
	headers = append(headers, consts.LATENCY_LOG_OPERATIONS_FOR_SEARCH...)
	if outputFile != "" {
		appendCsvToFile(outputFile, [][]string{headers})
	} else {
		printCsv([][]string{headers})
	}
	totalQueries := 0
	SLICES := 100
	for i := 0; i < SLICES; i++ {
		s := elastic.NewSliceQuery().Id(i).Max(SLICES)
		queries, err := logger.GetAllQueries(s) //  TBD take fixed amount of queries, not all
		utils.Must(err)
		totalQueries += len(queries)
		sortedQueries := make(search.CreatedSearchLogs, 0, len(queries))
		for _, q := range queries {
			sortedQueries = append(sortedQueries, q)
		}
		sort.Sort(sortedQueries)
		records := [][]string{}
		for i, sl := range sortedQueries {
			utils.Must(err)
			var latencies []string
			for _, op := range consts.LATENCY_LOG_OPERATIONS_FOR_SEARCH {
				hasOp := false
				for _, tl := range sl.ExecutionTimeLog {
					if tl.Operation == op {
						latancy := strconv.FormatInt(tl.Time, 10)
						latencies = append(latencies, latancy)
						hasOp = true
						break
					}
				}
				if !hasOp {
					latencies = append(latencies, "0")
				}
			}
			record := []string{
				fmt.Sprintf("%d", i+1),
				sl.SearchId,
				sl.Query.Term,
			}
			record = append(record, latencies...)
			records = append(records, record)
		}
		if outputFile != "" {
			appendCsvToFile(outputFile, records)
		} else {
			printCsv(records)
		}
	}
	log.Infof("Found %d queries.", totalQueries) //  TBD Change
}
