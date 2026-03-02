package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/compare"
	"github.com/Bnei-Baruch/archive-backend/consts"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
)

var compareCmd = &cobra.Command{
	Use:   "compare-indices",
	Short: "Compare ES6 and ES9 indices for consistency",
	Long: `Compare documents between ES6 and ES9 indices to verify data integrity.
Samples random documents and performs field-by-field comparison.

Examples:
  # Compare 10 random content units in Hebrew
  ./archive-backend compare-indices --type content_units --language he --sample-size 10

  # Compare specific documents
  ./archive-backend compare-indices --type content_units --ids "abc123,def456,ghi789"

  # Full detailed diff output
  ./archive-backend compare-indices --type content_units --language en --format diff

  # Raw JSON documents from both ES6 and ES9
  ./archive-backend compare-indices --type content_units --language he --format raw --sample-size 2

  # JSON output for programmatic use
  ./archive-backend compare-indices --type content_units --language ru --format json > comparison.json
`,
	Run: compareIndicesHandler,
}

var (
	compareType       string
	compareLanguage   string
	compareSampleSize int
	compareIDs        string
	compareFormat     string
	compareOutput     string
)

func init() {
	compareCmd.Flags().StringVar(&compareType, "type", "content_units", "Result type to compare (content_units, tweets, sources, tags)")
	compareCmd.Flags().StringVar(&compareLanguage, "language", "he", "Language to compare (he, en, ru, es, de, etc.)")
	compareCmd.Flags().IntVar(&compareSampleSize, "sample-size", 10, "Number of random documents to sample")
	compareCmd.Flags().StringVar(&compareIDs, "ids", "", "Specific mdb_uids to compare (comma-separated)")
	compareCmd.Flags().StringVar(&compareFormat, "format", "summary", "Output format: summary, diff, json, raw")
	compareCmd.Flags().StringVar(&compareOutput, "output", "", "Write output to file instead of stdout")

	RootCmd.AddCommand(compareCmd)
}

func compareIndicesHandler(cmd *cobra.Command, args []string) {
	log.Info("Starting index comparison")
	log.Infof("Type: %s | Language: %s | Format: %s", compareType, compareLanguage, compareFormat)

	// Initialize common resources WITH ES6 (needed for comparison)
	common.InitWithOptions(nil, nil, true)
	defer common.Shutdown()

	// Get ES6 client
	log.Info("Connecting to ES6...")
	es6Client, err := common.ESC.GetClient()
	if err != nil {
		log.Fatalf("Failed to get ES6 client: %v", err)
	}
	log.Info("✓ ES6 connection successful")

	// Initialize ES9 client
	log.Info("Connecting to ES9...")
	es9URL := viper.GetString("elasticsearch9.url")
	if es9URL == "" {
		log.Fatal("elasticsearch9.url not configured")
	}
	es9Manager := es9common.MakeES9Manager(es9URL)
	defer es9Manager.Stop()

	es9Client, err := es9Manager.GetClient()
	if err != nil {
		log.Fatalf("Failed to get ES9 client: %v", err)
	}
	log.Info("✓ ES9 connection successful")

	ctx := context.Background()

	// Create comparator based on type
	var comparator compare.ResultTypeComparator
	switch compareType {
	case "content_units":
		comparator = compare.NewContentUnitsComparator(es6Client, es9Client)
	default:
		log.Fatalf("Unsupported result type: %s (only 'content_units' supported currently)", compareType)
	}

	// Validate language
	validLanguage := false
	for _, lang := range consts.ALL_KNOWN_LANGS {
		if lang == compareLanguage {
			validLanguage = true
			break
		}
	}
	if !validLanguage {
		log.Fatalf("Invalid language: %s", compareLanguage)
	}

	// Get document UIDs to compare
	var uids []string
	if compareIDs != "" {
		// User-specified UIDs
		uids = strings.Split(compareIDs, ",")
		for i := range uids {
			uids[i] = strings.TrimSpace(uids[i])
		}
		log.Infof("Comparing %d specific documents", len(uids))
	} else {
		// Sample random documents
		log.Infof("Sampling %d random documents from ES6...", compareSampleSize)
		uids, err = comparator.Sample(ctx, compareLanguage, compareSampleSize)
		if err != nil {
			log.Fatalf("Failed to sample documents: %v", err)
		}
		log.Infof("Found %d documents to compare", len(uids))
	}

	if len(uids) == 0 {
		log.Fatal("No documents found to compare")
	}

	// Create comparison summary
	summary := compare.NewComparisonSummary(compareType, compareLanguage)

	// Compare each document
	log.Info("Comparing documents...")
	startTime := time.Now()

	for i, uid := range uids {
		if i > 0 && i%10 == 0 {
			log.Infof("Progress: %d/%d documents compared", i, len(uids))
		}

		// Fetch from ES6
		es6Doc, err := comparator.FetchES6Document(ctx, compareLanguage, uid)
		if err != nil {
			log.Warnf("Failed to fetch %s from ES6: %v", uid, err)
			continue
		}

		// Fetch from ES9
		es9Doc, err := comparator.FetchES9Document(ctx, compareLanguage, uid)
		if err != nil {
			log.Warnf("Failed to fetch %s from ES9: %v", uid, err)
			continue
		}

		// Compare
		result := comparator.Compare(es6Doc, es9Doc)
		result.Language = compareLanguage
		result.ResultType = compareType
		summary.AddResult(result)
	}

	elapsed := time.Since(startTime)
	log.Infof("✓ Compared %d documents in %v", summary.TotalCompared, elapsed)

	// Format output
	var output string
	switch compareFormat {
	case "summary":
		output = summary.FormatSummary()
	case "diff":
		output = summary.FormatDiff()
	case "json":
		output = summary.FormatJSON()
	case "raw":
		output = summary.FormatRaw()
	default:
		log.Fatalf("Invalid format: %s (must be: summary, diff, json, raw)", compareFormat)
	}

	// Write output
	if compareOutput != "" {
		log.Infof("Writing output to: %s", compareOutput)
		if err := os.WriteFile(compareOutput, []byte(output), 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		log.Info("✓ Output written successfully")
	} else {
		fmt.Println(output)
	}

	// Exit with error code if critical mismatches found
	if summary.CriticalErrors > 0 {
		log.Errorf("Found %d documents with critical field mismatches", summary.CriticalErrors)
		os.Exit(1)
	}

	log.Info("✓ Comparison completed successfully")
}
