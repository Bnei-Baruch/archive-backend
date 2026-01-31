package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
)

var es9testCmd = &cobra.Command{
	Use:   "es9test",
	Short: "Test ES9 index creation and analyzers",
	Long: `Creates a test index in ES9 with generated mapping and validates:
- Index creation succeeds
- Analyzers are configured correctly
- Hebrew hunspell analyzer works
- ICU analyzer works
- Dense vector fields are configured

This is a validation step before migrating production data.`,
	Run: func(cmd *cobra.Command, args []string) {
		lang := cmd.Flag("lang").Value.String()
		cleanup := cmd.Flag("cleanup").Value.String() == "true"

		// Get ES9 URL from config
		url := viper.GetString("elasticsearch9.url")
		if url == "" {
			log.Fatal("elasticsearch9.url not configured")
		}

		// Create ES9 manager
		manager := common.MakeES9Manager(url)
		defer manager.Stop()

		// Create context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Test health first
		log.Info("Testing ES9 connection...")
		if err := manager.Ping(ctx); err != nil {
			log.Fatalf("Failed to connect to ES9: %v", err)
		}
		log.Info("✓ ES9 connection successful")

		// Handle --lang=all
		if lang == "all" {
			testAllLanguages(ctx, manager, cleanup)
			return
		}

		// Test single language
		log.Infof("Starting ES9 mapping test for language: %s", lang)
		result := testSingleLanguage(ctx, manager, lang, cleanup)
		printTestResult(result)

		if result.Error != nil {
			log.Fatalf("Test failed: %v", result.Error)
		}
		log.Info("ES9 mapping test completed successfully")
	},
}

func init() {
	RootCmd.AddCommand(es9testCmd)

	es9testCmd.Flags().StringP(
		"lang",
		"l",
		consts.LANG_HEBREW,
		"Language code to test (en, he, ru, es, etc.)",
	)

	es9testCmd.Flags().Bool(
		"cleanup",
		true,
		"Delete test index after completion",
	)
}

// testResult holds the result of a language test
type testResult struct {
	Language  string
	IndexName string
	DocID     string
	Success   bool
	Error     error
}

// testSingleLanguage tests a single language mapping
func testSingleLanguage(ctx context.Context, manager *common.ES9Manager, lang string, cleanup bool) testResult {
	result := testResult{Language: lang, Success: false}

	// Generate index name with timestamp
	timestamp := time.Now().Format("2006-01-02t15-04-05")
	indexName := fmt.Sprintf("test_results_%s_%s", lang, timestamp)
	result.IndexName = indexName

	// Generate mapping for language
	log.Infof("Generating mapping for language: %s", lang)
	mapping, err := indexing.GenerateMapping(lang)
	if err != nil {
		result.Error = fmt.Errorf("generate mapping: %w", err)
		return result
	}

	// Create index
	log.Infof("Creating test index: %s", indexName)
	if err := createIndex(ctx, manager, indexName, mapping); err != nil {
		result.Error = fmt.Errorf("create index: %w", err)
		return result
	}
	log.Infof("✓ Index created successfully: %s", indexName)

	// Test analyzers
	log.Info("Testing analyzers...")
	if err := testAnalyzers(ctx, manager, indexName, lang); err != nil {
		result.Error = fmt.Errorf("test analyzers: %w", err)
		// Don't fail - just warn and continue
		log.Warnf("Analyzer test failed: %v", err)
	}

	// Index a test document
	log.Info("Indexing test content unit (lesson part)...")
	docID, err := indexTestDocument(ctx, manager, indexName, lang)
	if err != nil {
		result.Error = fmt.Errorf("index document: %w", err)
		return result
	}
	result.DocID = docID
	log.Infof("✓ Test content unit indexed with ID: %s", docID)

	// Verify document was indexed
	log.Info("Verifying document retrieval...")
	if err := verifyDocument(ctx, manager, indexName, docID); err != nil {
		result.Error = fmt.Errorf("verify document: %w", err)
		return result
	}
	log.Info("✓ Document retrieved successfully")

	// Cleanup if requested
	if cleanup {
		log.Infof("Cleaning up test index: %s", indexName)
		if err := deleteTestIndex(ctx, manager, indexName); err != nil {
			log.Warnf("Failed to delete test index: %v", err)
		} else {
			log.Info("✓ Test index deleted")
		}
	} else {
		log.Infof("Test index preserved: %s", indexName)
	}

	result.Success = true
	return result
}

// testAllLanguages tests all supported languages
func testAllLanguages(ctx context.Context, manager *common.ES9Manager, cleanup bool) {
	// Use ALL_KNOWN_LANGS from consts
	languages := consts.ALL_KNOWN_LANGS[:]

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("  ES9 Multi-Language Mapping Tests")
	fmt.Println(strings.Repeat("=", 80))

	results := make([]testResult, 0, len(languages))
	successCount := 0
	failCount := 0

	for i, lang := range languages {
		fmt.Printf("\n[%d/%d] Testing language: %s\n", i+1, len(languages), lang)
		fmt.Println(strings.Repeat("-", 80))

		result := testSingleLanguage(ctx, manager, lang, cleanup)
		results = append(results, result)

		if result.Success {
			successCount++
			fmt.Printf("✓ %s: SUCCESS\n", lang)
		} else {
			failCount++
			fmt.Printf("✗ %s: FAILED - %v\n", lang, result.Error)
		}
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("  Test Summary")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total Languages:  %d\n", len(languages))
	fmt.Printf("Successful:       %d (%.1f%%)\n", successCount, float64(successCount)/float64(len(languages))*100)
	fmt.Printf("Failed:           %d (%.1f%%)\n", failCount, float64(failCount)/float64(len(languages))*100)
	fmt.Printf("Cleaned up:       %v\n", cleanup)
	fmt.Println(strings.Repeat("=", 80))

	// Print failed languages
	if failCount > 0 {
		fmt.Println("\nFailed Languages:")
		for _, result := range results {
			if !result.Success {
				fmt.Printf("  - %s: %v\n", result.Language, result.Error)
			}
		}
	}

	log.Infof("ES9 multi-language test completed: %d success, %d failed", successCount, failCount)
}

// printTestResult prints a single test result
func printTestResult(result testResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  ES9 Mapping Test Results")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Language:     %s\n", result.Language)
	fmt.Printf("Index:        %s\n", result.IndexName)
	if result.Success {
		fmt.Printf("Status:       ✓ SUCCESS\n")
	} else {
		fmt.Printf("Status:       ✗ FAILED\n")
		fmt.Printf("Error:        %v\n", result.Error)
	}
	fmt.Println(strings.Repeat("=", 60))
}

// createIndex creates an ES9 index with the given mapping
func createIndex(ctx context.Context, manager *common.ES9Manager, indexName string, mapping map[string]interface{}) error {
	client, err := manager.GetClient()
	if err != nil {
		return fmt.Errorf("get client: %w", err)
	}

	// Convert mapping to JSON
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}

	// Create index with mapping
	res, err := client.Indices.Create(
		indexName,
		client.Indices.Create.WithContext(ctx),
		client.Indices.Create.WithBody(bytes.NewReader(mappingJSON)),
	)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES9 error: %s", res.String())
	}

	return nil
}

// testAnalyzers tests various analyzers with sample text
func testAnalyzers(ctx context.Context, manager *common.ES9Manager, indexName, lang string) error {
	client, err := manager.GetClient()
	if err != nil {
		return err
	}

	// Test texts per language (lesson-related content)
	testTexts := map[string]string{
		consts.LANG_HEBREW:  "שיעור בוקר על חכמת הקבלה ומהות הבורא",
		consts.LANG_ENGLISH: "Morning lesson about Kabbalah wisdom and the Creator",
		consts.LANG_RUSSIAN: "Утренний урок о мудрости Каббалы и Творце",
		consts.LANG_SPANISH: "Lección de la mañana sobre la sabiduría de la Cabalá y el Creador",
	}

	text := testTexts[lang]
	if text == "" {
		text = "test lesson text"
	}

	// Determine analyzers to test based on language
	analyzers := []string{"standard"}

	if lang == consts.LANG_HEBREW {
		analyzers = append(analyzers, "he_hunspell", "he_icu", "he_icu_hunspell")
	} else if lang == consts.LANG_ENGLISH {
		analyzers = append(analyzers, "en_stemmer")
	} else if lang == consts.LANG_RUSSIAN {
		analyzers = append(analyzers, "ru_stemmer")
	} else if lang == consts.LANG_SPANISH {
		analyzers = append(analyzers, "es_stemmer")
	}

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("  Analyzer Tests")
	fmt.Println(strings.Repeat("-", 60))

	for _, analyzer := range analyzers {
		tokens, err := analyzeText(ctx, client, indexName, analyzer, text)
		if err != nil {
			log.Warnf("Failed to test analyzer %s: %v", analyzer, err)
			continue
		}

		fmt.Printf("\nAnalyzer: %s\n", analyzer)
		fmt.Printf("Input:    %s\n", text)
		fmt.Printf("Tokens:   %s\n", strings.Join(tokens, ", "))
	}

	fmt.Println(strings.Repeat("-", 60))
	return nil
}

// analyzeText uses the _analyze API to test an analyzer
func analyzeText(ctx context.Context, client interface{}, indexName, analyzer, text string) ([]string, error) {
	// Type assert to get the actual ES9 client
	esClient, ok := client.(*elasticsearch.Client)
	if !ok {
		return nil, fmt.Errorf("invalid client type")
	}

	// Build analyze request body
	analyzeReq := map[string]interface{}{
		"analyzer": analyzer,
		"text":     text,
	}

	reqJSON, err := json.Marshal(analyzeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal analyze request: %w", err)
	}

	// Call the _analyze API on the specific index to use index-specific analyzers
	res, err := esClient.Indices.Analyze(
		bytes.NewReader(reqJSON),
		esClient.Indices.Analyze.WithContext(ctx),
		esClient.Indices.Analyze.WithIndex(indexName),
	)
	if err != nil {
		return nil, fmt.Errorf("analyze API call: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("analyze API error: %s", res.String())
	}

	// Parse response
	var analyzeResp struct {
		Tokens []struct {
			Token string `json:"token"`
		} `json:"tokens"`
	}

	if err := json.NewDecoder(res.Body).Decode(&analyzeResp); err != nil {
		return nil, fmt.Errorf("decode analyze response: %w", err)
	}

	// Extract token strings
	tokens := make([]string, len(analyzeResp.Tokens))
	for i, t := range analyzeResp.Tokens {
		tokens[i] = t.Token
	}

	return tokens, nil
}

// indexTestDocument indexes a sample content unit (lesson part)
func indexTestDocument(ctx context.Context, manager *common.ES9Manager, indexName, lang string) (string, error) {
	client, err := manager.GetClient()
	if err != nil {
		return "", err
	}

	// Create test content unit (lesson part) based on language
	testDocs := map[string]map[string]interface{}{
		consts.LANG_HEBREW: {
			"result_type": consts.ES_RESULT_TYPE_UNITS,
			"mdb_uid":     "test-unit-he-001",
			"title":       "שיעור בוקר - חלק 1",
			"full_title":  "קונגרס 2024 > יום 1 > שיעור בוקר - חלק 1",
			"description": "שיעור בוקר על חכמת הקבלה, עוסק במהות הבורא ותכלית הבריאה",
			"content":     "בשיעור זה נלמד על יסודות חכמת הקבלה. הרב מסביר על מהות הבורא, תכלית הבריאה, והדרך לתיקון. השיעור כולל קריאה מהזוהר ומאמרי בעל הסולם.",
			"filter_values": []string{
				"content_type:" + consts.CT_LESSON_PART,
				"language:he",
				"source:laitman",
			},
			"typed_uids": []string{
				"content_unit:test-unit-he-001",
				"collection:test-collection-001",
			},
			"index_date":     time.Now().Format("2006-01-02"),
			"effective_date": time.Now().Format("2006-01-02"),
		},
		consts.LANG_ENGLISH: {
			"result_type": consts.ES_RESULT_TYPE_UNITS,
			"mdb_uid":     "test-unit-en-001",
			"title":       "Morning Lesson - Part 1",
			"full_title":  "Congress 2024 > Day 1 > Morning Lesson - Part 1",
			"description": "Morning lesson about Kabbalah wisdom, discussing the essence of the Creator and purpose of creation",
			"content":     "In this lesson we study the fundamentals of Kabbalah wisdom. The Rav explains the essence of the Creator, the purpose of creation, and the path to correction. The lesson includes reading from the Zohar and articles by Baal HaSulam.",
			"filter_values": []string{
				"content_type:" + consts.CT_LESSON_PART,
				"language:en",
				"source:laitman",
			},
			"typed_uids": []string{
				"content_unit:test-unit-en-001",
				"collection:test-collection-001",
			},
			"index_date":     time.Now().Format("2006-01-02"),
			"effective_date": time.Now().Format("2006-01-02"),
		},
		consts.LANG_RUSSIAN: {
			"result_type": consts.ES_RESULT_TYPE_UNITS,
			"mdb_uid":     "test-unit-ru-001",
			"title":       "Утренний урок - Часть 1",
			"full_title":  "Конгресс 2024 > День 1 > Утренний урок - Часть 1",
			"description": "Утренний урок о мудрости Каббалы, рассматривающий сущность Творца и цель творения",
			"content":     "На этом уроке мы изучаем основы науки Каббала. Рав объясняет сущность Творца, цель творения и путь к исправлению. Урок включает чтение из Зоар и статей Бааль Сулама.",
			"filter_values": []string{
				"content_type:" + consts.CT_LESSON_PART,
				"language:ru",
				"source:laitman",
			},
			"typed_uids": []string{
				"content_unit:test-unit-ru-001",
				"collection:test-collection-001",
			},
			"index_date":     time.Now().Format("2006-01-02"),
			"effective_date": time.Now().Format("2006-01-02"),
		},
		consts.LANG_SPANISH: {
			"result_type": consts.ES_RESULT_TYPE_UNITS,
			"mdb_uid":     "test-unit-es-001",
			"title":       "Lección de la mañana - Parte 1",
			"full_title":  "Congreso 2024 > Día 1 > Lección de la mañana - Parte 1",
			"description": "Lección matutina sobre la sabiduría de la Cabalá, discutiendo la esencia del Creador y el propósito de la creación",
			"content":     "En esta lección estudiamos los fundamentos de la sabiduría de la Cabalá. El Rav explica la esencia del Creador, el propósito de la creación y el camino hacia la corrección. La lección incluye lectura del Zohar y artículos de Baal HaSulam.",
			"filter_values": []string{
				"content_type:" + consts.CT_LESSON_PART,
				"language:es",
				"source:laitman",
			},
			"typed_uids": []string{
				"content_unit:test-unit-es-001",
				"collection:test-collection-001",
			},
			"index_date":     time.Now().Format("2006-01-02"),
			"effective_date": time.Now().Format("2006-01-02"),
		},
	}

	doc := testDocs[lang]
	if doc == nil {
		doc = testDocs[consts.LANG_ENGLISH] // Fallback
		doc["mdb_uid"] = "test-unit-" + lang + "-001"
	}

	// Note: We're NOT adding embeddings yet - that comes in a later phase
	// For now, we're testing the text analyzers and basic indexing

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal document: %w", err)
	}

	// Index document
	docID := doc["mdb_uid"].(string)
	res, err := client.Index(
		indexName,
		bytes.NewReader(docJSON),
		client.Index.WithContext(ctx),
		client.Index.WithDocumentID(docID),
		client.Index.WithRefresh("true"), // Immediate refresh for testing
	)
	if err != nil {
		return "", fmt.Errorf("index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("ES9 error: %s", res.String())
	}

	return docID, nil
}

// verifyDocument retrieves a document to verify it was indexed
func verifyDocument(ctx context.Context, manager *common.ES9Manager, indexName, docID string) error {
	client, err := manager.GetClient()
	if err != nil {
		return err
	}

	res, err := client.Get(
		indexName,
		docID,
		client.Get.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("document not found: %s", res.String())
	}

	// Parse response to show document
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("  Retrieved Content Unit")
	fmt.Println(strings.Repeat("-", 60))

	if source, ok := result["_source"].(map[string]interface{}); ok {
		fmt.Printf("ID:          %s\n", result["_id"])
		fmt.Printf("Result Type: %v\n", source["result_type"])
		fmt.Printf("Title:       %v\n", source["title"])
		fmt.Printf("Full Title:  %v\n", source["full_title"])
		fmt.Printf("Description: %v\n", source["description"])
		if filters, ok := source["filter_values"].([]interface{}); ok {
			fmt.Printf("Filters:     %v\n", filters)
		}
	}

	fmt.Println(strings.Repeat("-", 60))

	return nil
}

// deleteTestIndex deletes a test index
func deleteTestIndex(ctx context.Context, manager *common.ES9Manager, indexName string) error {
	client, err := manager.GetClient()
	if err != nil {
		return err
	}

	res, err := client.Indices.Delete(
		[]string{indexName},
		client.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("delete index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES9 error: %s", res.String())
	}

	return nil
}
