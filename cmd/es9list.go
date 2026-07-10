package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/consts"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
)

var es9listCmd = &cobra.Command{
	Use:   "es9-list-indices",
	Short: "List all ES9 indices with statistics and aliases",
	Long: `List all Elasticsearch 9 indices showing:
- Index names grouped by base name
- Document counts per index and per result type
- Index sizes
- Aliases and their targets
- Summary statistics by language and result type`,
	Run: func(cmd *cobra.Command, args []string) {
		pattern := cmd.Flag("pattern").Value.String()

		log.Infof("Listing ES9 indices (pattern: %s)", pattern)

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

		// List indices
		if err := listIndices(ctx, manager, pattern); err != nil {
			log.Fatalf("Failed to list indices: %v", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(es9listCmd)

	es9listCmd.Flags().StringP(
		"pattern",
		"p",
		"*",
		"Index name pattern (supports wildcards, e.g., 'results_*', 'test_*')",
	)
}

type IndexInfo struct {
	Name       string
	DocCount   int64
	Size       string
	SizeBytes  int64
	ResultType map[string]int64 // result_type -> count
	Aliases    []string         // Alias names pointing to this index
	IsAlias    bool             // True if this name is an alias (not a real index)
	AliasFor   []string         // If IsAlias=true, the indices this alias points to
}

type IndexStats struct {
	BaseGroups     map[string][]IndexInfo // base name -> indices
	TotalIndices   int
	TotalDocs      int64
	TotalSizeBytes int64
	ByLanguage     map[string]*LanguageStats
	ByResultType   map[string]int64
}

type LanguageStats struct {
	DocCount  int64
	SizeBytes int64
}

func listIndices(ctx context.Context, manager *es9common.ES9Manager, pattern string) error {
	client, err := manager.GetClient()
	if err != nil {
		return fmt.Errorf("get ES9 client: %w", err)
	}

	// Get all indices matching pattern
	catResp, err := client.Cat.Indices(
		client.Cat.Indices.WithIndex(pattern),
		client.Cat.Indices.WithFormat("json"),
		client.Cat.Indices.WithH("index", "docs.count", "store.size", "pri.store.size"),
		client.Cat.Indices.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("cat indices: %w", err)
	}
	defer catResp.Body.Close()

	if catResp.IsError() {
		return fmt.Errorf("cat indices error: %s", catResp.String())
	}

	var indices []map[string]interface{}
	if err := json.NewDecoder(catResp.Body).Decode(&indices); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(indices) == 0 {
		fmt.Printf("\nNo indices found matching pattern: %s\n", pattern)
		return nil
	}

	// Get alias information for all indices
	aliasMap, err := getAllAliases(ctx, client, pattern)
	if err != nil {
		log.Warnf("Failed to get aliases: %v", err)
		aliasMap = make(map[string][]string) // Continue without aliases
	}

	// Collect detailed stats for each index
	stats := &IndexStats{
		BaseGroups:   make(map[string][]IndexInfo),
		ByLanguage:   make(map[string]*LanguageStats),
		ByResultType: make(map[string]int64),
	}

	for _, idx := range indices {
		indexName := idx["index"].(string)
		docCount := parseInt64(idx["docs.count"])
		storeSize, _ := idx["store.size"].(string)

		// Parse size in bytes
		sizeBytes := parseSizeToBytes(storeSize)

		// Get result type breakdown
		resultTypeCounts, err := getResultTypeBreakdown(ctx, manager, indexName)
		if err != nil {
			log.Warnf("Failed to get result type breakdown for %s: %v", indexName, err)
			resultTypeCounts = make(map[string]int64)
		}

		// Get aliases for this index
		aliases := aliasMap[indexName]

		info := IndexInfo{
			Name:       indexName,
			DocCount:   docCount,
			Size:       storeSize,
			SizeBytes:  sizeBytes,
			ResultType: resultTypeCounts,
			Aliases:    aliases,
		}

		// Extract base name and language
		baseName, lang := parseIndexName(indexName)

		stats.BaseGroups[baseName] = append(stats.BaseGroups[baseName], info)
		stats.TotalIndices++
		stats.TotalDocs += docCount
		stats.TotalSizeBytes += sizeBytes

		// Aggregate by language
		if stats.ByLanguage[lang] == nil {
			stats.ByLanguage[lang] = &LanguageStats{}
		}
		stats.ByLanguage[lang].DocCount += docCount
		stats.ByLanguage[lang].SizeBytes += sizeBytes

		// Aggregate by result type
		for resultType, count := range resultTypeCounts {
			stats.ByResultType[resultType] += count
		}
	}

	// Print results
	printIndexStats(stats)

	return nil
}

func getAllAliases(ctx context.Context, client *elasticsearch.Client, pattern string) (map[string][]string, error) {
	// Get aliases for all indices matching pattern
	// Returns map[indexName][]aliasName
	aliasResp, err := client.Indices.GetAlias(
		client.Indices.GetAlias.WithIndex(pattern),
		client.Indices.GetAlias.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("get aliases: %w", err)
	}
	defer aliasResp.Body.Close()

	if aliasResp.IsError() {
		// No aliases is not an error, just return empty map
		if aliasResp.StatusCode == 404 {
			return make(map[string][]string), nil
		}
		return nil, fmt.Errorf("get aliases error: %s", aliasResp.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(aliasResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode aliases response: %w", err)
	}

	// Parse response: { "index_name": { "aliases": { "alias1": {}, "alias2": {} } } }
	aliasMap := make(map[string][]string)
	for indexName, indexData := range result {
		if indexInfo, ok := indexData.(map[string]interface{}); ok {
			if aliases, ok := indexInfo["aliases"].(map[string]interface{}); ok {
				for aliasName := range aliases {
					aliasMap[indexName] = append(aliasMap[indexName], aliasName)
				}
				// Sort aliases for consistent display
				sort.Strings(aliasMap[indexName])
			}
		}
	}

	return aliasMap, nil
}

func getResultTypeBreakdown(ctx context.Context, manager *es9common.ES9Manager, indexName string) (map[string]int64, error) {
	client, err := manager.GetClient()
	if err != nil {
		return nil, err
	}

	// Aggregation query to count by result_type
	query := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"by_type": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "result_type",
					"size":  100,
				},
			},
		},
	}

	resp, err := client.Search(
		client.Search.WithIndex(indexName),
		client.Search.WithBody(strings.NewReader(toJSON(query))),
		client.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("search error: %s", resp.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	counts := make(map[string]int64)

	if aggs, ok := result["aggregations"].(map[string]interface{}); ok {
		if byType, ok := aggs["by_type"].(map[string]interface{}); ok {
			if buckets, ok := byType["buckets"].([]interface{}); ok {
				for _, bucket := range buckets {
					b := bucket.(map[string]interface{})
					key := b["key"].(string)
					count := int64(b["doc_count"].(float64))
					counts[key] = count
				}
			}
		}
	}

	return counts, nil
}

func printIndexStats(stats *IndexStats) {
	fmt.Printf("\nES9 INDICES\n")
	fmt.Printf("==============================================================================\n\n")

	// Print by base name and language
	baseNames := make([]string, 0, len(stats.BaseGroups))
	for baseName := range stats.BaseGroups {
		baseNames = append(baseNames, baseName)
	}
	sort.Strings(baseNames)

	for _, baseName := range baseNames {
		indices := stats.BaseGroups[baseName]

		// Sort by document count (descending)
		sort.Slice(indices, func(i, j int) bool {
			return indices[i].DocCount > indices[j].DocCount
		})

		// Show total language count in header
		totalLanguages := len(indices)
		fmt.Printf("Base: %s (%d languages)\n", baseName, totalLanguages)
		fmt.Printf("------------------------------------------------------------------------------\n")
		fmt.Printf("%-25s %10s %10s  %-30s %s\n", "Index", "Docs", "Size", "Result Types", "Aliases")
		fmt.Printf("------------------------------------------------------------------------------\n")

		// Show only top 10 languages
		displayCount := totalLanguages
		if displayCount > 10 {
			displayCount = 10
		}

		for i := 0; i < displayCount; i++ {
			idx := indices[i]
			// Build result type summary
			resultTypeSummary := ""
			if len(idx.ResultType) > 0 {
				// Sort result types
				types := make([]string, 0, len(idx.ResultType))
				for t := range idx.ResultType {
					types = append(types, t)
				}
				sort.Strings(types)

				parts := make([]string, 0, len(types))
				for _, t := range types {
					count := idx.ResultType[t]
					// Abbreviate result type names
					abbrev := abbreviateType(t)
					parts = append(parts, fmt.Sprintf("%s:%s", abbrev, formatCompact(count)))
				}
				resultTypeSummary = strings.Join(parts, " ")
			}

			// Build alias summary
			aliasSummary := ""
			if len(idx.Aliases) > 0 {
				if len(idx.Aliases) == 1 {
					aliasSummary = fmt.Sprintf("→ %s", idx.Aliases[0])
				} else {
					aliasSummary = fmt.Sprintf("→ %s +%d", idx.Aliases[0], len(idx.Aliases)-1)
				}
			}

			fmt.Printf("%-25s %10s %10s  %-30s %s\n",
				idx.Name,
				formatCompact(idx.DocCount),
				idx.Size,
				resultTypeSummary,
				aliasSummary)
		}

		// Show message if there are more languages
		if totalLanguages > 10 {
			fmt.Printf("... and %d more languages\n", totalLanguages-10)
		}

		fmt.Println()
	}

	// Print summary
	fmt.Printf("SUMMARY\n")
	fmt.Printf("==============================================================================\n")
	fmt.Printf("Indices: %d  |  Documents: %s  |  Size: %s\n\n",
		stats.TotalIndices,
		formatCompact(stats.TotalDocs),
		formatBytes(stats.TotalSizeBytes))
}

func abbreviateType(resultType string) string {
	abbrevs := map[string]string{
		"content-units": "cu",
		"collections":   "col",
		"sources":       "src",
		"tags":          "tag",
		"blog-posts":    "blog",
		"tweets":        "tw",
	}
	if abbrev, ok := abbrevs[resultType]; ok {
		return abbrev
	}
	// Take first 3-4 chars if not found
	if len(resultType) > 4 {
		return resultType[:4]
	}
	return resultType
}

func formatCompact(n int64) string {
	// Compact number formatting (e.g., 1.2K, 45K, 1.2M)
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 10000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else if n < 1000000 {
		return fmt.Sprintf("%dK", n/1000)
	} else if n < 10000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else {
		return fmt.Sprintf("%dM", n/1000000)
	}
}

func parseIndexName(indexName string) (baseName, lang string) {
	// Split by last underscore to get base and language
	// e.g., "results_en" -> base="results", lang="en"
	// e.g., "test_results_he" -> base="test_results", lang="he"

	parts := strings.Split(indexName, "_")
	if len(parts) < 2 {
		return indexName, ""
	}

	// Check if last part is a known language
	lastPart := parts[len(parts)-1]
	for _, knownLang := range consts.ALL_KNOWN_LANGS {
		if lastPart == knownLang {
			lang = lastPart
			baseName = strings.Join(parts[:len(parts)-1], "_")
			return
		}
	}

	// Not a known language pattern
	return indexName, ""
}

func parseInt64(val interface{}) int64 {
	switch v := val.(type) {
	case string:
		var i int64
		fmt.Sscanf(v, "%d", &i)
		return i
	case float64:
		return int64(v)
	case int64:
		return v
	default:
		return 0
	}
}

func parseSizeToBytes(size string) int64 {
	// Parse sizes like "496.3mb", "1.3gb", "5.2kb"
	size = strings.ToLower(strings.TrimSpace(size))

	var value float64
	var unit string
	fmt.Sscanf(size, "%f%s", &value, &unit)

	multiplier := int64(1)
	switch unit {
	case "kb":
		multiplier = 1024
	case "mb":
		multiplier = 1024 * 1024
	case "gb":
		multiplier = 1024 * 1024 * 1024
	case "tb":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "b", "":
		multiplier = 1
	}

	return int64(value * float64(multiplier))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatNumber(n int64) string {
	// Add thousand separators
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, digit := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(digit)
	}
	return result.String()
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
