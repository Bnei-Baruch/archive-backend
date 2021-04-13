package cmd

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/volatiletech/null.v6"

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

var latencyAggregateCmd = &cobra.Command{
	Use:   "latency_aggregate",
	Short: "Get latency statistics from latency CSV report.",
	Run:   latencyAggregateFn,
}

var queriesAggregateCmd = &cobra.Command{
	Use:   "queries_aggregate",
	Short: "Get 1000 most popular queries and 1000 random queries for each language from ElasticSearch.",
	Run:   queriesAggregateFn,
}

var elasticUrl string
var csvFile string
var lastDays int
var latencyOutputHtml string

var latencyMetaHeaders = []string{
	"#", "SearchId", "Term",
}

func init() {
	RootCmd.AddCommand(logCmd)

	logCmd.PersistentFlags().StringVar(&elasticUrl, "elastic", "", "URL of Elastic.")
	logCmd.MarkFlagRequired("elastic")
	viper.BindPFlag("elasticsearch.url", logCmd.PersistentFlags().Lookup("elastic"))

	latencyCmd.PersistentFlags().StringVar(&csvFile, "output_file", "", "CSV path to write.")
	latencyCmd.PersistentFlags().IntVar(&lastDays, "last_days", 7, "Number of days for the lattest queries (default is 7).")
	latencyAggregateCmd.PersistentFlags().StringVar(&latencyOutputHtml, "output_html", "", "HTML path to write.")
	latencyAggregateCmd.PersistentFlags().StringVar(&csvFile, "csv_file", "", "CSV file that been generated by 'log latency' command.")
	latencyCmd.MarkFlagRequired("output_file")
	latencyAggregateCmd.MarkFlagRequired("output_html")
	latencyAggregateCmd.MarkFlagRequired("csv_file")
	queriesAggregateCmd.PersistentFlags().IntVar(&lastDays, "last_days", 0, "Number of days for the lattest queries (unlimited if 0 or not set.).")

	logCmd.AddCommand(queriesCmd)
	logCmd.AddCommand(clicksCmd)
	logCmd.AddCommand(latencyCmd)
	logCmd.AddCommand(latencyAggregateCmd)
	logCmd.AddCommand(queriesAggregateCmd)
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0660)
	if err != nil {
		panic(fmt.Sprintf("Cannot open file. Error: %s.", err))
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

func printHtmlTr(records []string, isHeaders bool, style string) string {
	var rows string
	var tag string
	if isHeaders {
		tag = "th"
	} else {
		tag = "td"
	}
	for _, record := range records {
		rows = fmt.Sprintf("%s<%s style='%s'>%s</%s>", rows, tag, style, record, tag)
	}
	return fmt.Sprintf("<tr>%s</tr>", rows)
}

func queriesFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	printCsv([][]string{{
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
	printCsv([][]string{{
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
	headers := append(latencyMetaHeaders, consts.LATENCY_LOG_OPERATIONS_FOR_SEARCH...)
	if csvFile != "" {
		appendCsvToFile(csvFile, [][]string{headers})
	} else {
		printCsv([][]string{headers})
	}
	gteStr := null.StringFrom(fmt.Sprintf("now-%dd/d", lastDays))
	totalQueries := 0
	SLICES := 100
	for i := 0; i < SLICES; i++ {
		s := elastic.NewSliceQuery().Id(i).Max(SLICES)
		queries, err := logger.GetLattestQueries(s, gteStr, null.BoolFrom(false))
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
					latencies = append(latencies, "-")
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
		if csvFile != "" {
			appendCsvToFile(csvFile, records)
		} else {
			printCsv(records)
		}
	}
	log.Infof("Found %d queries.", totalQueries)
}

func latencyAggregateFn(cmd *cobra.Command, args []string) {

	const wholeSearchLatencyOperatinIndex = 3 //  index of "DoSearch" column
	const worstQueriesPrintCnt = 5

	var operationsHtmlPart string
	var worstQueriesHtmlPart string
	reader, err := os.Open(csvFile)
	r := csv.NewReader(bufio.NewReader(reader))
	records, err := r.ReadAll()
	utils.Must(err)

	opLatenciesMap := make(map[string][]int, len(consts.LATENCY_LOG_OPERATIONS_FOR_SEARCH))
	for _, op := range consts.LATENCY_LOG_OPERATIONS_FOR_SEARCH {
		opLatenciesMap[op] = make([]int, 0)
	}

	for i := 1; i < len(records); i++ { //  skip first line (headers)

		record := records[i]

		for j := len(latencyMetaHeaders); j < len(record); j++ {
			val := strings.TrimSpace(record[j])
			if val != "-" {
				lat, err := strconv.Atoi(strings.TrimSpace(record[j]))
				utils.Must(err)
				for opIndex, op := range consts.LATENCY_LOG_OPERATIONS_FOR_SEARCH {
					if opIndex == j-len(latencyMetaHeaders) {
						opLatenciesMap[op] = append(opLatenciesMap[op], lat)
						continue
					}
				}
			}
		}
	}
	trs := printHtmlTr([]string{"Stage", "Average", "Worst", "95 percentile", "Active"}, true, "")
	for opName, latencies := range opLatenciesMap {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		sum, max := utils.SumAndMax(latencies)
		sum95Precent := float32(sum) * 0.95
		var percentile95 int
		var calcSum float32
		for i := 0; i < len(latencies); i++ {
			calcSum += float32(latencies[i])
			if calcSum >= sum95Precent {
				percentile95 = latencies[i]
				break
			}
		}
		var avg int
		var activePercent float32
		if sum > 0 {
			avg = sum / len(latencies)
		}
		if len(records) > 1 {
			activePercent = float32(len(latencies)) / float32(len(records)-1) * 100
		}
		activeStr := fmt.Sprintf("%d from %d (%.2f%%)", len(latencies), len(records)-1, activePercent)
		log.Printf("%s Stage\n\nAverage: %d\nWorst: %d\n95 percentile: %d.\nActive: %s\n",
			opName, avg, max, percentile95, activeStr)
		trs = fmt.Sprintf("%s%s", trs, printHtmlTr([]string{opName, strconv.Itoa(avg), strconv.Itoa(max), strconv.Itoa(percentile95), activeStr}, false, ""))
	}
	operationsHtmlPart = fmt.Sprintf("<h3>Latencies</h3><table>%s</table>", trs)

	if len(records) == 0 {
		worstQueriesHtmlPart = ""

	} else {
		sortedRecords := records[1:]
		formatedHeaders := make([]string, 0)
		/// print the worst queries
		sort.Slice(sortedRecords, func(i, j int) bool {
			left, err := strconv.Atoi(strings.TrimSpace(sortedRecords[i][wholeSearchLatencyOperatinIndex]))
			utils.Must(err)
			right, err := strconv.Atoi(strings.TrimSpace(sortedRecords[j][wholeSearchLatencyOperatinIndex]))
			utils.Must(err)
			return left > right
		})
		log.Printf("%d worst queries:\n", worstQueriesPrintCnt)

		for _, r := range records[0] {
			fr := strings.Replace(r, ".", " ", -1)
			formatedHeaders = append(formatedHeaders, fr)
		}
		printCsv([][]string{formatedHeaders}) //  print headers
		worstQueriesTrs := printHtmlTr(formatedHeaders, true, "word-break: break-word; min-width: 170px;")
		for i := 0; i < worstQueriesPrintCnt; i++ {
			printCsv([][]string{sortedRecords[i]})
			worstQueriesTrs = fmt.Sprintf("%s%s", worstQueriesTrs, printHtmlTr(sortedRecords[i], false, ""))
		}
		worstQueriesHtmlPart = fmt.Sprintf("<h3>%d worst queries</h3><table>%s</table>", worstQueriesPrintCnt, worstQueriesTrs)
	}
	finalHtml := fmt.Sprintf("%s%s", operationsHtmlPart, worstQueriesHtmlPart)
	err = ioutil.WriteFile(latencyOutputHtml, []byte(finalHtml), 0644)
	utils.Must(err)
	log.Info("HTML printed.")
}

func queriesAggregateFn(cmd *cobra.Command, args []string) {
	logger := initLogger()
	printCsv([][]string{{"Term", "Count", "SortType", "Language"}})
	SLICES := 100
	RESULTS_FOR_LANGUAGE := 1000
	gteStr := null.NewString("", false)
	languages := []string{consts.LANG_HEBREW, consts.LANG_ENGLISH, consts.LANG_RUSSIAN} // Currently only the 3 main languages are supported due to language recognition issues
	queryCountByLang := map[string]map[string]int{}                                     // Language -> [Search Term -> Count]
	if lastDays > 0 {
		gteStr = null.StringFrom(fmt.Sprintf("now-%dd/d", lastDays))
	}
	for i := 0; i < SLICES; i++ {
		s := elastic.NewSliceQuery().Id(i).Max(SLICES)
		queries, err := logger.GetLattestQueries(s, gteStr, null.NewBool(false, false))
		utils.Must(err)
		for _, sl := range queries {
			term := simpleQuery(sl.Query)
			if term == "" || sl.Error != nil || sl.QueryResult == nil || sl.Query.Deb || sl.From > 0 {
				continue
			}
			lang := sl.QueryResult.(map[string]interface{})["language"].(string)
			if lang == "" {
				sort.Strings(sl.Query.LanguageOrder)
				intersected := utils.IntersectSortedStringSlices(languages, sl.Query.LanguageOrder)
				if len(intersected) > 0 {
					lang = intersected[0]
				}
			}
			if utils.Contains(utils.Is(languages), lang) {
				if _, ok := queryCountByLang[lang]; !ok {
					queryCountByLang[lang] = map[string]int{}
				}
				if _, ok := queryCountByLang[lang][term]; !ok {
					queryCountByLang[lang][term] = 0
				}
				queryCountByLang[lang][term]++
			}
		}
	}
	records := [][]string{}
	for _, lang := range languages {
		tcs := []TermAndCount{}
		for k, v := range queryCountByLang[lang] {
			tcs = append(tcs, TermAndCount{Term: k, Count: v})
		}
		if len(tcs) < RESULTS_FOR_LANGUAGE {
			log.Errorf("Amount of found terms (%d) for language '%s' is smaller than RESULTS_FOR_LANGUAGE const (%d).", len(tcs), lang, RESULTS_FOR_LANGUAGE)
			return
		}
		sort.Slice(tcs, func(i, j int) bool {
			return tcs[i].Count > tcs[j].Count
		})
		for i, tc := range tcs {
			if i < RESULTS_FOR_LANGUAGE {
				records = append(records, []string{tc.Term, strconv.Itoa(tc.Count), "Count", lang})
			}
		}
		// Pick random values using Durstenfeld's algorithm (no need to shuffle the whole slice)
		r := rand.New(rand.NewSource(time.Now().Unix()))
		for i := len(tcs) - 1; i >= len(tcs)-RESULTS_FOR_LANGUAGE; i-- {
			ridx := r.Intn(i + 1)
			tcs[i], tcs[ridx] = tcs[ridx], tcs[i]
			records = append(records, []string{tcs[i].Term, strconv.Itoa(tcs[i].Count), "Random", lang})
		}
	}
	printCsv(records)
	log.Infof("Printed %d rows.", len(records)+1)
}

func simpleQuery(q search.Query) string {
	if q.Term == "" && len(q.ExactTerms) == 1 {
		return q.ExactTerms[0]
	}
	return q.Term
}

type TermAndCount struct {
	Term  string
	Count int
}
