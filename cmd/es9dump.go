package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/consts"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
)

var dumpCmd = &cobra.Command{
	Use:   "es9_dump",
	Short: "Dump all ES9 documents to a JSONL file",
	Long: `Dump all documents from all ES9 indices (all languages, all types) to a JSONL file.
Each line is a JSON object with ES9 metadata (es9_id, es9_index, es9_index_lang) plus all document fields.

Examples:
  # Dump to default file (dump.jsonl)
  ./archive-backend es9_dump

  # Dump to custom file
  ./archive-backend es9_dump -o /data/es9_dump.jsonl

  # Use custom index base name
  ./archive-backend es9_dump --es9-index staging_results -o staging_dump.jsonl
`,
	Run: dumpHandler,
}

var (
	dumpOutput   string
	dumpES9Index string
)

func init() {
	dumpCmd.Flags().StringVarP(&dumpOutput, "output", "o", "dump.jsonl", "Output file path (JSONL format)")
	dumpCmd.Flags().StringVar(&dumpES9Index, "es9-index", "prod_results", "ES9 index base name (default: 'prod_results')")
	RootCmd.AddCommand(dumpCmd)
}

func dumpHandler(cmd *cobra.Command, args []string) {
	log.Info("Starting ES9 dump")

	es9URL := viper.GetString("elasticsearch9.url")
	if es9URL == "" {
		log.Fatal("elasticsearch9.url not configured")
	}

	es9Manager := es9common.MakeES9Manager(es9URL)
	defer es9Manager.Stop()

	client, err := es9Manager.GetClient()
	if err != nil {
		log.Fatalf("Failed to get ES9 client: %v", err)
	}
	log.Info("✓ ES9 connection successful")

	f, err := os.Create(dumpOutput)
	if err != nil {
		log.Fatalf("Failed to create output file %s: %v", dumpOutput, err)
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, 1<<20) // 1MB write buffer

	ctx := context.Background()
	totalDocs := 0

	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := fmt.Sprintf("%s_%s", dumpES9Index, lang)
		count, err := dumpIndex(ctx, client, indexName, lang, writer)
		if err != nil {
			log.Warnf("Error dumping index %s: %v", indexName, err)
			continue
		}
		if count > 0 {
			log.Infof("✓ %s: %d documents", indexName, count)
			totalDocs += count
		}
	}

	if err := writer.Flush(); err != nil {
		log.Fatalf("Failed to flush output: %v", err)
	}

	log.Infof("Done. Total documents: %d → %s", totalDocs, dumpOutput)
}

func dumpIndex(ctx context.Context, client *elasticsearch.Client, indexName, lang string, writer *bufio.Writer) (int, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 1000,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return 0, fmt.Errorf("encode query: %w", err)
	}

	res, err := client.Search(
		client.Search.WithContext(ctx),
		client.Search.WithIndex(indexName),
		client.Search.WithBody(&buf),
		client.Search.WithScroll(5*time.Minute),
	)
	if err != nil {
		return 0, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return 0, nil // index doesn't exist for this language
		}
		return 0, fmt.Errorf("search failed: %s", res.String())
	}

	var searchResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	scrollID, ok := searchResult["_scroll_id"].(string)
	if !ok {
		return 0, fmt.Errorf("no scroll_id in response")
	}
	defer clearScroll(ctx, client, scrollID)

	count := 0

	hits := extractHits(searchResult)
	n, err := writeHits(hits, indexName, lang, writer)
	if err != nil {
		return 0, err
	}
	count += n

	for {
		scrollQuery := map[string]interface{}{
			"scroll":    "5m",
			"scroll_id": scrollID,
		}

		var scrollBuf bytes.Buffer
		if err := json.NewEncoder(&scrollBuf).Encode(scrollQuery); err != nil {
			return count, fmt.Errorf("encode scroll query: %w", err)
		}

		res, err := client.Scroll(
			client.Scroll.WithContext(ctx),
			client.Scroll.WithBody(&scrollBuf),
		)
		if err != nil {
			return count, fmt.Errorf("scroll: %w", err)
		}

		if res.IsError() {
			res.Body.Close()
			return count, fmt.Errorf("scroll failed: %s", res.String())
		}

		var scrollResult map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&scrollResult); err != nil {
			res.Body.Close()
			return count, fmt.Errorf("decode scroll response: %w", err)
		}
		res.Body.Close()

		if newScrollID, ok := scrollResult["_scroll_id"].(string); ok {
			scrollID = newScrollID
		}

		hits := extractHits(scrollResult)
		if len(hits) == 0 {
			break
		}

		n, err := writeHits(hits, indexName, lang, writer)
		if err != nil {
			return count, err
		}
		count += n
	}

	return count, nil
}

func extractHits(result map[string]interface{}) []interface{} {
	hitsWrapper, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil
	}
	hits, _ := hitsWrapper["hits"].([]interface{})
	return hits
}

func writeHits(hits []interface{}, indexName, lang string, writer *bufio.Writer) (int, error) {
	count := 0
	for _, hit := range hits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, _ := hitMap["_source"].(map[string]interface{})
		if source == nil {
			source = make(map[string]interface{})
		}

		source["es9_id"] = hitMap["_id"]
		source["es9_index"] = indexName
		source["es9_index_lang"] = lang

		line, err := json.Marshal(source)
		if err != nil {
			return count, fmt.Errorf("marshal document: %w", err)
		}

		if _, err := writer.Write(line); err != nil {
			return count, fmt.Errorf("write document: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			return count, fmt.Errorf("write newline: %w", err)
		}

		count++
	}
	return count, nil
}

func clearScroll(ctx context.Context, client *elasticsearch.Client, scrollID string) {
	clearQuery := map[string]interface{}{"scroll_id": []string{scrollID}}
	var clearBuf bytes.Buffer
	json.NewEncoder(&clearBuf).Encode(clearQuery) //nolint:errcheck
	client.ClearScroll(                           //nolint:errcheck
		client.ClearScroll.WithContext(ctx),
		client.ClearScroll.WithBody(&clearBuf),
	)
}
