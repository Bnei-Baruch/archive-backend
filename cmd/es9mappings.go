package cmd

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
)

var es9mappingsCmd = &cobra.Command{
	Use:   "es9mappings",
	Short: "Generate ES9 mapping files for all languages",
	Long: `Generates Elasticsearch 9 mapping JSON files for all supported languages.

The mappings are generated from Go code and written to data/es9/mappings/results/ directory.
One file per language: results-en.json, results-he.json, etc.

This replaces the Python make.py script from ES6.`,
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := cmd.Flag("output").Value.String()

		log.Infof("Generating ES9 mappings to: %s", outputDir)

		if err := indexing.GenerateAllMappings(outputDir); err != nil {
			log.Fatalf("Failed to generate mappings: %v", err)
		}

		fmt.Println("\n✓ Successfully generated all language mappings")
		log.Info("ES9 mapping generation completed successfully")
	},
}

func init() {
	RootCmd.AddCommand(es9mappingsCmd)

	// Output directory flag
	es9mappingsCmd.Flags().StringP(
		"output",
		"o",
		"es9/data/mappings/results",
		"Output directory for generated mapping files",
	)
}
