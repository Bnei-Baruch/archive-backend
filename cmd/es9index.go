package cmd

import (
	"context"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/integration"
)

var es9indexCmd = &cobra.Command{
	Use:   "es9index",
	Short: "Index content to Elasticsearch 9",
	Long: `Index content from MDB (PostgreSQL) to Elasticsearch 9.

This command performs ETL (Extract, Transform, Load) of content units, collections,
sources, tags, and other entities from the MDB to ES9 indices.

Currently supports:
- Content units (lesson parts, lectures, etc.)

Future support:
- Collections
- Sources
- Tags
- Blog posts
- Tweets`,
	Run: func(cmd *cobra.Command, args []string) {
		contentType := cmd.Flag("type").Value.String()
		indexName := cmd.Flag("index").Value.String()
		reset, _ := cmd.Flags().GetBool("reset")

		mode := "incremental"
		if reset {
			mode = "full reset"
		}
		log.Infof("Starting ES9 indexing for content type: %s (mode: %s)", contentType, mode)

		// Initialize common resources WITHOUT ES6 (not needed for ES9 indexing)
		common.InitWithOptions(nil, nil, false)
		defer common.Shutdown()

		// Get ES9 URL from config
		es9URL := viper.GetString("elasticsearch9.url")
		if es9URL == "" {
			log.Fatal("elasticsearch9.url not configured")
		}

		// Create ES9 manager
		manager := es9common.MakeES9Manager(es9URL)
		defer manager.Stop()

		// Test ES9 connection
		ctx := context.Background()
		if err := manager.Ping(ctx); err != nil {
			log.Fatalf("Failed to connect to ES9: %v", err)
		}
		log.Info("✓ ES9 connection successful")

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Index based on content type
		startTime := time.Now()
		switch contentType {
		case "content-units":
			if err := indexContentUnits(ctx, manager, indexName, reset); err != nil {
				log.Fatalf("Failed to index content units: %v", err)
			}

		default:
			log.Fatalf("Unknown content type: %s (supported: content-units)", contentType)
		}

		duration := time.Since(startTime)
		log.Infof("✓ Indexing completed successfully in %s", duration.Round(time.Second))
	},
}

func init() {
	RootCmd.AddCommand(es9indexCmd)

	es9indexCmd.Flags().StringP(
		"type",
		"t",
		"content-units",
		"Content type to index (content-units, collections, sources, tags)",
	)

	es9indexCmd.Flags().StringP(
		"index",
		"i",
		"results",
		"Base index name (e.g., 'results' creates results_en, results_he, etc.)",
	)

	es9indexCmd.Flags().BoolP(
		"reset",
		"r",
		false,
		"Reset mode: delete all documents of this type before reindexing (default: incremental mode - skip existing)",
	)
}

// indexContentUnits indexes all content units to ES9
func indexContentUnits(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing content units to ES9")

	// Get assets service URL from config
	unzipURL := viper.GetString("elasticsearch.unzip-url")
	if unzipURL == "" {
		log.Warn("elasticsearch.unzip-url not configured - transcript extraction will be skipped")
	}

	// Create assets service for DOCX to text conversion
	assetsService := integration.NewAssetsService(unzipURL)

	// Create indexer with assets service
	indexer := indexing.NewContentUnitsIndexer(manager, common.DB, indexNameBase, assetsService)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all content units: %w", err)
	}

	log.Info("✓ Content units indexed successfully")
	return nil
}
