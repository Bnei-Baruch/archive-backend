package cmd

import (
    "fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate recall set.",
	Run:   evalFn,
}

var evalSetPath string
var serverUrl   string

func init() {
    evalCmd.PersistentFlags().StringVar(&evalSetPath, "eval_set", "", "Path to tsv eval set.")
    evalCmd.MarkFlagRequired("eval_set")
    evalCmd.PersistentFlags().StringVar(&serverUrl, "server", "", "URL of archive backend to evaluate.")
    evalCmd.MarkFlagRequired("server")
	RootCmd.AddCommand(evalCmd)
}

func roundD(val float64) int {
    if val < 0 { return int(val-1.0) }
    return int(val)
}

func float64ToPercent(val float64) string {
    return fmt.Sprintf("%.2f%%", float64(roundD(val*10000))/float64(100))
}

func evalFn(cmd *cobra.Command, args []string) {
	log.Infof("Evaluating eval set at %s.", evalSetPath)
    evalSet, err := search.ReadEvalSet(evalSetPath)
    utils.Must(err)
    results, err := search.Eval(evalSet, serverUrl)
    utils.Must(err)
    log.Infof("Done evaluating queries.")
    log.Infof("Unique queries: %d", results.TotalUnique)
    log.Infof("Weighted queries: %d", results.TotalWeighted)
    log.Infof("Errors: %d", results.TotalErrors)
    log.Infof("Recall Unique: %s", float64ToPercent(results.RecallUnique))
    log.Infof("Recall Weighted: %s", float64ToPercent(results.RecallWeighted))
    log.Infof("Regular Unique: %s", float64ToPercent(results.RegularUnique))
    log.Infof("Regular Weighted: %s", float64ToPercent(results.RegularWeighted))
    log.Infof("Unknown Unique: %s", float64ToPercent(results.UnknownUnique))
    log.Infof("Unknown Weighted: %s", float64ToPercent(results.UnknownWeighted))
    log.Infof("ServerError Unique: %s", float64ToPercent(results.ServerErrorUnique))
    log.Infof("ServerError Weighted: %s", float64ToPercent(results.ServerErrorWeighted))
}

