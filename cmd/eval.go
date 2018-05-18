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

func runSxS(evalSet []search.EvalQuery, baseUrl string, expUrl string) (
	search.EvalResults, map[int][]search.Loss, search.EvalResults, map[int][]search.Loss, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var baseResults search.EvalResults
	var baseLosses map[int][]search.Loss
	var baseErr error
	go func() {
		defer wg.Done()
		baseResults, baseLosses, baseErr = search.Eval(evalSet, baseUrl)
	}()

	var expResults search.EvalResults
	var expLosses map[int][]search.Loss
	var expErr error
	go func() {
		defer wg.Done()
		expResults, expLosses, expErr = search.Eval(evalSet, expUrl)
	}()
	wg.Wait()
	if baseErr != nil {
		return search.EvalResults{}, nil, search.EvalResults{}, nil, baseErr
	}
	if expErr != nil {
		return search.EvalResults{}, nil, search.EvalResults{}, nil, expErr
	}
	return baseResults, baseLosses, expResults, expLosses, nil
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

func printLosses(results search.EvalResults, losses map[int][]search.Loss) {
	log.Infof("Found %d loss types (Unknown).", len(losses))
	var lKeys []int
	for k, _ := range losses {
		lKeys = append(lKeys, k)
	}
	sort.Ints(lKeys)
	for _, eType := range lKeys {
		lList := losses[eType]
		totalUnique := float64(0)
		totalWeighted := float64(0)
		for _, l := range lList {
			totalUnique += l.Unique
			totalWeighted += l.Weighted
		}
		log.Infof("%s - %7s/%7s ", search.EXPECTATION_HIT_TYPE[eType],
			float64ToPercent(totalUnique/float64(results.TotalUnique)),
			float64ToPercent(totalWeighted/float64(results.TotalWeighted)))
		for _, l := range lList {
			log.Infof("\t%7s/%7s - [%s] %s %+v", float64ToPercent(l.Unique/float64(results.TotalUnique)),
				float64ToPercent(l.Weighted/float64(results.TotalWeighted)), l.Query.Query, l.Query.Bucket, l.Expectation)
		}
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
		baseResults, baseLosses, expResults, expLosses, err := runSxS(evalSet, baseServerUrl, serverUrl)
		utils.Must(err)
		log.Infof("Base:")
		printResults(baseResults)
		log.Infof("Exp:")
		printResults(expResults)
		log.Infof("Base:")
		printLosses(baseResults, baseLosses)
		log.Infof("Exp:")
		printLosses(expResults, expLosses)
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
		results, losses, err := search.Eval(evalSet, serverUrl)
		utils.Must(err)
		printResults(results)
		printLosses(results, losses)

	}
	utils.Must(err)
	log.Infof("Done evaluating queries.")
}
