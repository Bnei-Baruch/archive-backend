package cmd

import (
	"fmt"
    "sync"

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
var serverUrl string
var baseServerUrl string

func init() {
	evalCmd.PersistentFlags().StringVar(&evalSetPath, "eval_set", "", "Path to tsv eval set.")
	evalCmd.MarkFlagRequired("eval_set")
	evalCmd.PersistentFlags().StringVar(&serverUrl, "server", "", "URL of experimental archive backend to evaluate.")
	evalCmd.MarkFlagRequired("server")
	evalCmd.PersistentFlags().StringVar(&baseServerUrl, "base_server", "", "URL of base archive backend to evaluate.")
	evalCmd.MarkFlagRequired("base_server")
	RootCmd.AddCommand(evalCmd)
}

func roundD(val float64) int {
	if val < 0 {
		return int(val - 1.0)
	}
	return int(val)
}

func float64ToPercent(val float64) string {
	return fmt.Sprintf("%.2f%%", float64(roundD(val*10000))/float64(100))
}

func runSxS(evalSet []search.EvalQuery, baseUrl string, expUrl string) (search.EvalResults, search.EvalResults, error) {
    var wg sync.WaitGroup
    wg.Add(2)

    var baseResults search.EvalResults
    var baseErr error
    go func() {
        defer wg.Done()
        baseResults, baseErr = search.Eval(evalSet, baseUrl)
    }()

    var expResults search.EvalResults
    var expErr error
    go func() {
        defer wg.Done()
        expResults, expErr = search.Eval(evalSet, expUrl)
    }()
    wg.Wait()
    if baseErr != nil {
        return search.EvalResults{}, search.EvalResults{}, baseErr
    }
    if expErr != nil {
        return search.EvalResults{}, search.EvalResults{}, expErr
    }
    return baseResults, expResults, nil
}

func printResults(results search.EvalResults) {
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

func evalFn(cmd *cobra.Command, args []string) {
	log.Infof("Evaluating eval set at %s.", evalSetPath)
	evalSet, err := search.ReadEvalSet(evalSetPath)
	utils.Must(err)
    if baseServerUrl != "" {
        baseResults, expResults, err := runSxS(evalSet, serverUrl, baseServerUrl)
        utils.Must(err)
        log.Infof("Base:")
        printResults(baseResults)
        log.Infof("Exp:")
        printResults(expResults)
        if len(baseResults.Results) != len(expResults.Results) {
            log.Errorf("Expected same number of results for exp and base, got base - %d and exp - %d.",
                       len(baseResults.Results), len(expResults.Results))
            return
        }
        classification := make(map[uint32][]string)
        for i, baseResult := range baseResults.Results {
            expResult := expResults.Results[i]
            cr := search.CompareResults(baseResult, expResult)
            classification[cr] = append(classification[cr], evalSet[i].Query)
        }

        for k, v := range classification {
            if k != search.CR_SAME {
                log.Infof("%s:", search.COMPARE_RESULTS_NAME[k])
                for _, q := range v {
                    log.Infof("\t[%s]", q)
                }
            }
        }

    } else {
        results, err := search.Eval(evalSet, serverUrl)
        utils.Must(err)
        printResults(results)
    }
	utils.Must(err)
	log.Infof("Done evaluating queries.")
}
