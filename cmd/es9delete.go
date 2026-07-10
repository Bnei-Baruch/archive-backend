package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
)

var es9deleteCmd = &cobra.Command{
	Use:   "es9-delete-indices",
	Short: "Delete ES9 indices by language or pattern",
	Long: `Delete Elasticsearch 9 indices with flexible filtering:

Examples:
  # Delete single language
  es9-delete-indices -l he

  # Delete multiple languages
  es9-delete-indices -l en,he,ru

  # Delete with pattern matching
  es9-delete-indices --index "test_*"

  # Delete all indices matching base name
  es9-delete-indices --index results --all

  # Dry run to preview deletion
  es9-delete-indices -l en --dry-run

  # Force deletion without confirmation
  es9-delete-indices -l en --force`,
	Run: func(cmd *cobra.Command, args []string) {
		languages, _ := cmd.Flags().GetStringSlice("language")
		indexPattern := cmd.Flag("index").Value.String()
		all, _ := cmd.Flags().GetBool("all")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")

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

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Delete indices
		if err := deleteIndices(ctx, manager, languages, indexPattern, all, dryRun, force); err != nil {
			log.Fatalf("Failed to delete indices: %v", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(es9deleteCmd)

	es9deleteCmd.Flags().StringSliceP(
		"language",
		"l",
		[]string{},
		"Language codes to delete (comma-separated or repeated, e.g., en,he,ru)",
	)

	es9deleteCmd.Flags().StringP(
		"index",
		"i",
		"results",
		"Base index name or pattern (supports wildcards, e.g., 'results', 'test_*')",
	)

	es9deleteCmd.Flags().Bool(
		"all",
		false,
		"Delete all indices matching the pattern (use with caution)",
	)

	es9deleteCmd.Flags().Bool(
		"dry-run",
		false,
		"Show what would be deleted without actually deleting",
	)

	es9deleteCmd.Flags().BoolP(
		"force",
		"f",
		false,
		"Skip confirmation prompt",
	)
}

func deleteIndices(ctx context.Context, manager *es9common.ES9Manager, languages []string, indexPattern string, all bool, dryRun bool, force bool) error {
	client, err := manager.GetClient()
	if err != nil {
		return fmt.Errorf("get ES9 client: %w", err)
	}

	// Build list of index names to delete
	var indicesToDelete []string

	if len(languages) > 0 {
		// Delete specific languages with the given base pattern
		for _, lang := range languages {
			lang = strings.TrimSpace(lang)
			// Handle both simple names and patterns
			if strings.Contains(indexPattern, "*") {
				// Pattern mode: replace * with language pattern
				pattern := strings.Replace(indexPattern, "*", "*", -1)
				indexName := fmt.Sprintf("%s_%s", strings.TrimRight(pattern, "*"), lang)
				indicesToDelete = append(indicesToDelete, indexName)
			} else {
				// Simple mode: base_lang
				indexName := fmt.Sprintf("%s_%s", indexPattern, lang)
				indicesToDelete = append(indicesToDelete, indexName)
			}
		}
	} else if all || strings.Contains(indexPattern, "*") {
		// Pattern matching mode: find all matching indices
		pattern := indexPattern
		if all && !strings.Contains(pattern, "*") {
			pattern = pattern + "_*"
		}

		// Get all matching indices
		catResp, err := client.Cat.Indices(
			client.Cat.Indices.WithIndex(pattern),
			client.Cat.Indices.WithFormat("json"),
			client.Cat.Indices.WithH("index"),
			client.Cat.Indices.WithContext(ctx),
		)
		if err != nil {
			return fmt.Errorf("list indices: %w", err)
		}
		defer catResp.Body.Close()

		if catResp.IsError() {
			return fmt.Errorf("list indices error: %s", catResp.String())
		}

		var indices []map[string]interface{}
		if err := json.NewDecoder(catResp.Body).Decode(&indices); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		for _, idx := range indices {
			indicesToDelete = append(indicesToDelete, idx["index"].(string))
		}
	} else {
		return fmt.Errorf("must specify either --language, --all, or use a wildcard pattern")
	}

	if len(indicesToDelete) == 0 {
		fmt.Println("\nNo indices found matching the criteria")
		return nil
	}

	// Remove duplicates
	indicesToDelete = unique(indicesToDelete)

	// Check which indices actually exist
	existingIndices := []string{}
	for _, indexName := range indicesToDelete {
		exists, err := indexExists(ctx, client, indexName)
		if err != nil {
			log.Warnf("Failed to check if index %s exists: %v", indexName, err)
			continue
		}
		if exists {
			existingIndices = append(existingIndices, indexName)
		}
	}

	if len(existingIndices) == 0 {
		fmt.Println("\nNo matching indices exist")
		return nil
	}

	// Get stats for existing indices and sort by doc count
	type indexWithStats struct {
		name     string
		docCount int64
		size     string
		stats    string
	}
	indicesWithStats := make([]indexWithStats, 0, len(existingIndices))

	for _, indexName := range existingIndices {
		docCount, size, err := getIndexStats(ctx, client, indexName, manager)
		if err != nil {
			log.Warnf("Failed to get stats for %s: %v", indexName, err)
			indicesWithStats = append(indicesWithStats, indexWithStats{
				name:     indexName,
				docCount: 0,
				size:     "",
				stats:    "unknown",
			})
		} else {
			indicesWithStats = append(indicesWithStats, indexWithStats{
				name:     indexName,
				docCount: docCount,
				size:     size,
				stats:    fmt.Sprintf("%s docs, %s", formatNumber(docCount), size),
			})
		}
	}

	// Sort by document count (descending)
	sort.Slice(indicesWithStats, func(i, j int) bool {
		return indicesWithStats[i].docCount > indicesWithStats[j].docCount
	})

	// Rebuild existingIndices and indexStats in sorted order
	existingIndices = make([]string, len(indicesWithStats))
	indexStats := make(map[string]string)
	for i, iws := range indicesWithStats {
		existingIndices[i] = iws.name
		indexStats[iws.name] = iws.stats
	}

	// Print what will be deleted
	fmt.Println()
	if dryRun {
		fmt.Println("DRY RUN - Indices that would be deleted:")
	} else {
		fmt.Println("Indices to be deleted:")
	}
	fmt.Println("==============================================================================")
	fmt.Printf("%-30s  %s\n", "Index", "Stats")
	fmt.Println("------------------------------------------------------------------------------")

	totalDocs := int64(0)
	for _, indexName := range existingIndices {
		stats := indexStats[indexName]
		fmt.Printf("%-30s  %s\n", indexName, stats)

		// Extract doc count for total
		var docCount int64
		fmt.Sscanf(stats, "%d", &docCount)
		totalDocs += docCount
	}

	fmt.Println("------------------------------------------------------------------------------")
	fmt.Printf("Total: %d indices, ~%s documents\n\n", len(existingIndices), formatNumber(totalDocs))

	if dryRun {
		fmt.Println("Dry run mode - no indices were deleted")
		return nil
	}

	// Confirmation prompt
	if !force {
		fmt.Printf("WARNING: This will permanently delete %d indices!\n\n", len(existingIndices))
		fmt.Print("Type 'yes' to confirm deletion: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("\nDeletion cancelled")
			return nil
		}
	}

	// Delete indices
	fmt.Println("\nDeleting indices...")
	successCount := 0
	failedCount := 0

	for _, indexName := range existingIndices {
		fmt.Printf("  Deleting %s... ", indexName)

		resp, err := client.Indices.Delete(
			[]string{indexName},
			client.Indices.Delete.WithContext(ctx),
		)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			failedCount++
			continue
		}
		defer resp.Body.Close()

		if resp.IsError() {
			fmt.Printf("ERROR: %s\n", resp.String())
			failedCount++
			continue
		}

		fmt.Println("OK")
		successCount++
	}

	// Summary
	fmt.Println()
	fmt.Println("SUMMARY")
	fmt.Println("==============================================================================")
	fmt.Printf("Deleted: %d indices\n", successCount)
	if failedCount > 0 {
		fmt.Printf("Failed:  %d indices\n", failedCount)
	}
	fmt.Println()

	if failedCount > 0 {
		return fmt.Errorf("failed to delete %d indices", failedCount)
	}

	return nil
}

func indexExists(ctx context.Context, client *elasticsearch.Client, indexName string) (bool, error) {
	resp, err := client.Indices.Exists(
		[]string{indexName},
		client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

func getIndexStats(ctx context.Context, client *elasticsearch.Client, indexName string, manager *es9common.ES9Manager) (docCount int64, size string, err error) {
	catResp, err := client.Cat.Indices(
		client.Cat.Indices.WithIndex(indexName),
		client.Cat.Indices.WithFormat("json"),
		client.Cat.Indices.WithH("docs.count", "store.size"),
		client.Cat.Indices.WithContext(ctx),
	)
	if err != nil {
		return 0, "", err
	}
	defer catResp.Body.Close()

	if catResp.IsError() {
		return 0, "", fmt.Errorf("cat indices error: %s", catResp.String())
	}

	var indices []map[string]interface{}
	if err := json.NewDecoder(catResp.Body).Decode(&indices); err != nil {
		return 0, "", err
	}

	if len(indices) == 0 {
		return 0, "", fmt.Errorf("index not found")
	}

	docCount = parseInt64(indices[0]["docs.count"])
	size, _ = indices[0]["store.size"].(string)

	return docCount, size, nil
}

func unique(strs []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// matchPattern checks if a string matches a glob pattern
func matchPattern(pattern, str string) bool {
	matched, err := filepath.Match(pattern, str)
	if err != nil {
		return false
	}
	return matched
}
