package cmd

import (
	"fmt"
	"sort"
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
	var keys []int
	for k, _ := range results.UniqueMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		unique := results.UniqueMap[k]
		weighted := results.WeightedMap[k]
		log.Infof("%-15s Unique/Weighted: %7s/%7s", search.SEARCH_QUALITY_NAME[k], float64ToPercent(unique), float64ToPercent(weighted))
	}
}

func Round(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}

func evalFn(cmd *cobra.Command, args []string) {
	log.Infof("Evaluating eval set at %s.", evalSetPath)
	evalSet, err := search.ReadEvalSet(evalSetPath)
	utils.Must(err)
	if baseServerUrl != "" {
		baseResults, expResults, err := runSxS(evalSet, baseServerUrl, serverUrl)
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
		classification := make(map[int][]string)
		for i, baseResult := range baseResults.Results {
			expResult := expResults.Results[i]
			if len(baseResult.SearchQuality) != len(expResult.SearchQuality) {
				log.Errorf("Expected same number of expectations (search quality) for base and exp, got base - %d and exp - %d",
					len(baseResult.SearchQuality), len(expResult.SearchQuality))
				return
			}
			for j, baseSQ := range baseResult.SearchQuality {
				expSQ := expResult.SearchQuality[j]
				cr := search.CompareResults(baseSQ, expSQ)
				queryWeight := Round(float64(evalSet[i].Weight))
				expectationWeight := Round(float64(queryWeight) / float64(len(evalSet[i].Expectations)))
				expectationUniqueWeight := Round(1 / float64(len(evalSet[i].Expectations)))
				expectation := evalSet[i].Expectations[j]
				baseRank := baseResult.Rank[j]
				expRank := expResult.Rank[j]
				str := fmt.Sprintf("\t%.2f/%.2f   %.2f/%.2f - [%s] - Expectation: (type: %s, uid: %s) Rank: (base: %d  exp: %d)",
					expectationUniqueWeight, 1.0, expectationWeight, queryWeight, evalSet[i].Query,
					search.EXPECTATION_HIT_TYPE[expectation.Type], expectation.Uid, baseRank, expRank)
				classification[cr] = append(classification[cr], str)
			}
		}

		var keys []int
		for k, _ := range classification {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			if k != search.CR_SAME {
				log.Infof("%s:", search.COMPARE_RESULTS_NAME[k])
				for _, str := range classification[k] {
					log.Info(str)
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
