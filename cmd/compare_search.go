package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/search"
)

var compareSearchCmd = &cobra.Command{
	Use:   "compare_search",
	Short: "Compare raw search responses between two backend versions.",
	Long: `Fetches /search results from two backends for every query in one or more
recall-set CSV files and produces a self-contained HTML report showing which
responses are identical and which differ (field-by-field diff).

Both backends must point to the same Elasticsearch index so that any
difference reflects a code regression, not a data difference.

Example:
  ./archive-backend compare_search \
    --base_server=http://localhost:8080/backend \
    --server=http://localhost:8081/backend \
    --eval_sets=data/search/he.recall.csv,data/search/en.recall.csv,data/search/ru.recall.csv \
    --out=/tmp/compare_search.html`,
	Run: compareSearchFn,
}

var (
	csBaseServer   string
	csExpServer    string
	csEvalSets     string
	csOut          string
	csElasticURL   string
	csPageSize     int
	csConcurrency  int
	csTopHits      int
	csIgnoreScores bool
)

func init() {
	compareSearchCmd.PersistentFlags().StringVar(&csBaseServer, "base_server", "", "URL of base (master) backend, e.g. http://localhost:8080/backend")
	compareSearchCmd.MarkFlagRequired("base_server")
	compareSearchCmd.PersistentFlags().StringVar(&csExpServer, "server", "", "URL of experimental (latest) backend, e.g. http://localhost:8081/backend")
	compareSearchCmd.MarkFlagRequired("server")
	compareSearchCmd.PersistentFlags().StringVar(&csEvalSets, "eval_sets", "", "Comma-separated paths to recall CSV files (he.recall.csv,en.recall.csv,ru.recall.csv)")
	compareSearchCmd.MarkFlagRequired("eval_sets")
	compareSearchCmd.PersistentFlags().StringVar(&csOut, "out", "/tmp/compare_search.html", "Path for the HTML output report")
	compareSearchCmd.PersistentFlags().StringVar(&csElasticURL, "elastic_url", "", "Elasticsearch URL for live health checks (e.g. http://localhost:9200); omit to skip health monitoring")
	compareSearchCmd.PersistentFlags().IntVar(&csPageSize, "page_size", 10, "Number of search results to request from each backend")
	compareSearchCmd.PersistentFlags().IntVar(&csConcurrency, "concurrency", 5, "Max queries in flight simultaneously against each server pair")
	compareSearchCmd.PersistentFlags().IntVar(&csTopHits, "top_hits", 6, "Hits below this index are considered page-boundary noise; queries where only tail diffs exist are counted as 'identical top' (0 to disable)")
	compareSearchCmd.PersistentFlags().BoolVar(&csIgnoreScores, "ignore_scores", false, "Strip _score / max_score fields before comparing (useful when scores may drift)")
	RootCmd.AddCommand(compareSearchCmd)
}

func compareSearchFn(cmd *cobra.Command, args []string) {
	paths := strings.Split(csEvalSets, ",")

	var allQueries []search.SearchCompareQuery
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		qs, err := search.ReadSearchCompareQueries(p)
		if err != nil {
			log.Errorf("Failed reading %s: %v", p, err)
			os.Exit(1)
		}
		log.Infof("Loaded %d queries from %s", len(qs), p)
		allQueries = append(allQueries, qs...)
	}

	if len(allQueries) == 0 {
		log.Error("No queries loaded — check eval_sets paths.")
		os.Exit(1)
	}

	log.Infof("Comparing %d queries: base=%s  exp=%s", len(allQueries), csBaseServer, csExpServer)
	log.Infof("page_size=%d  concurrency=%d  ignore_scores=%v", csPageSize, csConcurrency, csIgnoreScores)

	summary := search.RunSearchComparison(allQueries, csBaseServer, csExpServer, csElasticURL, csPageSize, csConcurrency, csTopHits, csIgnoreScores)

	log.Infof("Done. Total=%d  Identical=%d  Different=%d  Errors=%d",
		summary.Total, summary.Identical, summary.Different, summary.Errors)

	html := search.SearchCompareHTMLReport(summary, csBaseServer, csExpServer)

	if err := ioutil.WriteFile(csOut, []byte(html), 0644); err != nil {
		log.Errorf("Failed writing HTML report: %v", err)
		os.Exit(1)
	}
	log.Infof("Report written to: %s", csOut)

	if summary.Different > 0 || summary.Errors > 0 {
		fmt.Printf("\nWARNING: %d different, %d errors — check report at %s\n", summary.Different, summary.Errors, csOut)
		os.Exit(2) // non-zero so CI can detect regressions
	}
	fmt.Printf("\nAll %d queries returned identical responses.\n", summary.Total)
}
