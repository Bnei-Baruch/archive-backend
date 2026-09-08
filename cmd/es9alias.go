package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/consts"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
)

var es9switchAliasCmd = &cobra.Command{
	Use:   "es9-switch-alias",
	Short: "Switch ES9 alias to point to a different index (atomic)",
	Long: `Switch an Elasticsearch 9 alias to point to a different index atomically.
If the alias doesn't exist, it will be created.
If it exists, it will be atomically updated to point to the new index (removing from old indices).

By default, switches the alias for ALL known languages.

Examples:
  # Switch for all languages (default)
  es9-switch-alias -a prod_results -i results_v2

  # Switch for specific language
  es9-switch-alias -l en -a prod_results -i results_v2

  # Switch for multiple specific languages
  es9-switch-alias -l en,ru,he -a prod_results -i results_v2`,
	Run: func(cmd *cobra.Command, args []string) {
		aliasName := cmd.Flag("alias").Value.String()
		indexName := cmd.Flag("index").Value.String()
		languages, _ := cmd.Flags().GetStringSlice("language")

		if aliasName == "" {
			log.Fatal("--alias flag is required")
		}
		if indexName == "" {
			log.Fatal("--index flag is required")
		}

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
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// If no languages specified, use all known languages
		if len(languages) == 0 || (len(languages) == 1 && languages[0] == "") {
			languages = consts.ALL_KNOWN_LANGS[:]
			log.Infof("Switching alias for all languages: %v", languages)
		} else {
			log.Infof("Switching alias for languages: %v", languages)
		}

		// Switch alias for each language
		successCount := 0
		errorCount := 0
		for _, lang := range languages {
			fullAliasName := aliasName + "_" + lang
			fullIndexName := indexName + "_" + lang

			log.Infof("\n[%s] Switching '%s' to '%s'", lang, fullAliasName, fullIndexName)

			if err := switchAlias(ctx, manager, fullAliasName, fullIndexName); err != nil {
				log.Errorf("[%s] Failed: %v", lang, err)
				errorCount++
				continue
			}

			log.Infof("[%s] ✓ Success", lang)
			successCount++
		}

		// Summary
		log.Infof("\n=== Summary ===")
		log.Infof("Successful: %d/%d", successCount, len(languages))
		if errorCount > 0 {
			log.Errorf("Failed: %d/%d", errorCount, len(languages))
			log.Fatal("Some alias switches failed")
		}
		log.Info("✓ All alias switches completed successfully")
	},
}

func init() {
	RootCmd.AddCommand(es9switchAliasCmd)

	es9switchAliasCmd.Flags().StringSliceP(
		"language",
		"l",
		[]string{},
		"Language codes (e.g., en, ru, he). Multiple values supported. Default: all languages",
	)

	es9switchAliasCmd.Flags().StringP(
		"alias",
		"a",
		"prod_results",
		"Alias base name (default: prod_results)",
	)

	es9switchAliasCmd.Flags().StringP(
		"index",
		"i",
		"",
		"Target index base name to point the alias to",
	)

	es9switchAliasCmd.MarkFlagRequired("index")
}

func switchAlias(ctx context.Context, manager *es9common.ES9Manager, aliasName, indexName string) error {
	client, err := manager.GetClient()
	if err != nil {
		return fmt.Errorf("get ES9 client: %w", err)
	}

	// 1. Check if target index exists
	exists, err := aliasIndexExists(ctx, client, indexName)
	if err != nil {
		return fmt.Errorf("check if target index exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("target index '%s' does not exist", indexName)
	}

	fmt.Printf("Target index '%s' exists\n", indexName)

	// 2. Check if alias name is actually a real index (not an alias)
	isIndex, err := isRealIndex(ctx, client, aliasName)
	if err != nil {
		return fmt.Errorf("check if name is real index: %w", err)
	}
	if isIndex {
		return fmt.Errorf("'%s' is a real index, not an alias - cannot switch", aliasName)
	}

	// 3. Get current alias info (which indices it points to)
	currentIndices, err := getAliasIndices(ctx, client, aliasName)
	if err != nil {
		return fmt.Errorf("get current alias info: %w", err)
	}

	// 4. Build alias actions
	var actions []map[string]interface{}

	// Remove alias from all current indices
	for _, idx := range currentIndices {
		actions = append(actions, map[string]interface{}{
			"remove": map[string]interface{}{
				"index": idx,
				"alias": aliasName,
			},
		})
	}

	// Add alias to new index
	actions = append(actions, map[string]interface{}{
		"add": map[string]interface{}{
			"index": indexName,
			"alias": aliasName,
		},
	})

	// 5. Execute atomic alias update
	body := map[string]interface{}{
		"actions": actions,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal alias actions: %w", err)
	}

	resp, err := client.Indices.UpdateAliases(
		strings.NewReader(string(bodyJSON)),
		client.Indices.UpdateAliases.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("update aliases: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("update aliases error: %s", resp.String())
	}

	if len(currentIndices) == 0 {
		fmt.Printf("\nCreated new alias '%s' -> '%s'\n", aliasName, indexName)
	} else {
		fmt.Printf("\nSwitched alias '%s': %v -> '%s'\n", aliasName, currentIndices, indexName)
	}

	return nil
}

// aliasIndexExists checks if an index exists
func aliasIndexExists(ctx context.Context, client *elasticsearch.Client, indexName string) (bool, error) {
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

// isRealIndex checks if the name is a real index (not an alias)
func isRealIndex(ctx context.Context, client *elasticsearch.Client, name string) (bool, error) {
	// Check if it exists as an index
	resp, err := client.Indices.Exists(
		[]string{name},
		client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Doesn't exist as index or alias
		return false, nil
	}

	// It exists, now check if it's an alias or a real index
	// Try to get it as an alias
	aliasResp, err := client.Indices.GetAlias(
		client.Indices.GetAlias.WithName(name),
		client.Indices.GetAlias.WithContext(ctx),
	)
	if err != nil {
		return false, err
	}
	defer aliasResp.Body.Close()

	// If the alias endpoint returns 404, then 'name' is a real index
	if aliasResp.StatusCode == 404 {
		return true, nil
	}

	// If it returns 200, then 'name' is an alias
	return false, nil
}

// getAliasIndices returns all indices that the alias currently points to
func getAliasIndices(ctx context.Context, client *elasticsearch.Client, aliasName string) ([]string, error) {
	resp, err := client.Indices.GetAlias(
		client.Indices.GetAlias.WithName(aliasName),
		client.Indices.GetAlias.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 404 means alias doesn't exist yet
	if resp.StatusCode == 404 {
		return []string{}, nil
	}

	if resp.IsError() {
		return nil, fmt.Errorf("get alias error: %s", resp.String())
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Extract index names
	indices := make([]string, 0, len(result))
	for indexName := range result {
		indices = append(indices, indexName)
	}

	return indices, nil
}
