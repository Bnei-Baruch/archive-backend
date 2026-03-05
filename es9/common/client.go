package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/spf13/viper"
)

// ES9Manager manages Elasticsearch 9.x client connections
type ES9Manager struct {
	client *elasticsearch.Client
	url    string
}

// MakeES9Manager creates a new ES9Manager with the given URL
func MakeES9Manager(url string) *ES9Manager {
	manager := &ES9Manager{
		url: url,
	}
	// Initialize client on creation
	_, err := manager.GetClient()
	if err != nil {
		log.Warnf("Failed to initialize ES9 client: %v", err)
	}
	return manager
}

// GetClient returns the Elasticsearch 9 client, creating it if necessary
func (m *ES9Manager) GetClient() (*elasticsearch.Client, error) {
	var err error
	if m.client == nil {
		log.Info("Setting up new connection to Elasticsearch 9")

		cfg := elasticsearch.Config{
			Addresses: []string{m.url},
			// Retry configuration
			RetryOnStatus: []int{502, 503, 504, 429},
			MaxRetries:    3,

			// For debugging, can be enabled in development
			// Logger: &estransport.ColorLogger{
			// 	Output:             os.Stdout,
			// 	EnableRequestBody:  true,
			// 	EnableResponseBody: true,
			// },
		}

		m.client, err = elasticsearch.NewClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("error creating ES9 client: %w", err)
		}

		// Verify connection
		info, err := m.client.Info()
		if err != nil {
			return nil, fmt.Errorf("error connecting to ES9: %w", err)
		}
		defer info.Body.Close()

		// Parse version info
		var r map[string]interface{}
		if err := json.NewDecoder(info.Body).Decode(&r); err != nil {
			log.Warnf("Error parsing ES9 info response: %v", err)
		} else {
			log.Infof("Elasticsearch 9 connected: %s", r["version"].(map[string]interface{})["number"])
		}
	}
	return m.client, err
}

// HealthCheck performs a health check on the Elasticsearch cluster
func (m *ES9Manager) HealthCheck(ctx context.Context) (*HealthInfo, error) {
	client, err := m.GetClient()
	if err != nil {
		return nil, err
	}

	// Get cluster info
	info, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster info: %w", err)
	}
	defer info.Body.Close()

	var infoResp map[string]interface{}
	if err := json.NewDecoder(info.Body).Decode(&infoResp); err != nil {
		return nil, fmt.Errorf("error parsing info response: %w", err)
	}

	// Get cluster health
	health, err := client.Cluster.Health()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster health: %w", err)
	}
	defer health.Body.Close()

	var healthResp map[string]interface{}
	if err := json.NewDecoder(health.Body).Decode(&healthResp); err != nil {
		return nil, fmt.Errorf("error parsing health response: %w", err)
	}

	// Extract version info
	version := ""
	if v, ok := infoResp["version"].(map[string]interface{}); ok {
		if num, ok := v["number"].(string); ok {
			version = num
		}
	}

	// Extract cluster name
	clusterName := ""
	if name, ok := infoResp["cluster_name"].(string); ok {
		clusterName = name
	}

	return &HealthInfo{
		ClusterName:  clusterName,
		Status:       fmt.Sprintf("%v", healthResp["status"]),
		Version:      version,
		NumberOfNodes: int(healthResp["number_of_nodes"].(float64)),
		ActiveShards: int(healthResp["active_shards"].(float64)),
		Timestamp:    time.Now(),
	}, nil
}

// Ping checks if the Elasticsearch cluster is reachable
func (m *ES9Manager) Ping(ctx context.Context) error {
	client, err := m.GetClient()
	if err != nil {
		return err
	}

	res, err := client.Ping()
	if err != nil {
		return fmt.Errorf("error pinging ES9: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES9 ping returned error: %s", res.Status())
	}

	return nil
}

// Stop closes the Elasticsearch client
func (m *ES9Manager) Stop() {
	// The go-elasticsearch v9 client doesn't require explicit cleanup
	// But we can set it to nil for consistency
	m.client = nil
	log.Info("ES9 client stopped")
}

// GetURL returns the configured Elasticsearch URL
func (m *ES9Manager) GetURL() string {
	return m.url
}

// IndexExists checks if an index exists
func (m *ES9Manager) IndexExists(ctx context.Context, indexName string) (bool, error) {
	client, err := m.GetClient()
	if err != nil {
		return false, err
	}

	res, err := client.Indices.Exists([]string{indexName})
	if err != nil {
		return false, fmt.Errorf("error checking index existence: %w", err)
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

// CreateIndex creates an index with the given mapping
func (m *ES9Manager) CreateIndex(ctx context.Context, indexName string, mapping map[string]interface{}) error {
	client, err := m.GetClient()
	if err != nil {
		return err
	}

	// Check if index already exists
	exists, err := m.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("error checking if index exists: %w", err)
	}
	if exists {
		// Validate existing index has correct mapping
		log.Debugf("Index already exists, validating mapping: %s", indexName)
		if err := m.ValidateIndexMapping(ctx, indexName, mapping); err != nil {
			log.Errorf("⚠ MAPPING MISMATCH: Index %s has incorrect mapping", indexName)
			log.Errorf("  This usually happens when index creation was interrupted (Ctrl+C)")
			log.Errorf("  Validation error: %v", err)
			log.Errorf("")
			log.Errorf("  To fix, delete the index and re-run:")

			// Extract language from index name (e.g., "results_20260305_0_en" -> "en")
			parts := strings.Split(indexName, "_")
			if len(parts) > 0 {
				lang := parts[len(parts)-1]
				log.Errorf("    ./archive-backend es9-delete-indices -l %s --force", lang)
			} else {
				log.Errorf("    ./archive-backend es9-delete-indices --index \"%s\" --force", indexName)
			}
			log.Errorf("    ./archive-backend es9index --reset")
			log.Errorf("")
			return fmt.Errorf("index %s has incorrect mapping (interrupted creation?): %w", indexName, err)
		}
		log.Infof("Index already exists with correct mapping: %s", indexName)
		return nil
	}

	// Marshal mapping to JSON
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("error marshaling mapping: %w", err)
	}

	// Create index with mapping
	res, err := client.Indices.Create(
		indexName,
		client.Indices.Create.WithBody(bytes.NewReader(mappingJSON)),
	)
	if err != nil {
		return fmt.Errorf("error creating index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating index: %s", res.String())
	}

	log.Infof("✓ Index created successfully: %s", indexName)

	// Verify the mapping was applied correctly (catch partial creations from Ctrl+C)
	if err := m.ValidateIndexMapping(ctx, indexName, mapping); err != nil {
		log.Errorf("⚠ CRITICAL: Index created but mapping validation failed!")
		log.Errorf("  Index: %s", indexName)
		log.Errorf("  Validation error: %v", err)
		log.Errorf("  This indicates a serious ES issue or interrupted creation")
		log.Errorf("")
		log.Errorf("  To fix: Delete and recreate:")

		// Extract language from index name
		parts := strings.Split(indexName, "_")
		if len(parts) > 0 {
			lang := parts[len(parts)-1]
			log.Errorf("    ./archive-backend es9-delete-indices -l %s --force", lang)
		} else {
			log.Errorf("    ./archive-backend es9-delete-indices --index \"%s\" --force", indexName)
		}
		log.Errorf("    # Then re-run the indexing")
		log.Errorf("")
		return fmt.Errorf("index created but mapping incorrect: %w", err)
	}

	log.Debugf("✓ Mapping validated for: %s", indexName)
	return nil
}

// DeleteIndex deletes an index
func (m *ES9Manager) DeleteIndex(ctx context.Context, indexName string) error {
	client, err := m.GetClient()
	if err != nil {
		return err
	}

	res, err := client.Indices.Delete([]string{indexName})
	if err != nil {
		return fmt.Errorf("error deleting index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("error deleting index: %s", res.String())
	}

	log.Infof("✓ Index deleted: %s", indexName)
	return nil
}

// ValidateIndexMapping validates that an index has the expected mapping
// Returns error if critical fields have wrong types (e.g., result_type as text instead of keyword)
func (m *ES9Manager) ValidateIndexMapping(ctx context.Context, indexName string, expectedMapping map[string]interface{}) error {
	client, err := m.GetClient()
	if err != nil {
		return err
	}

	// Get actual mapping from ES
	res, err := client.Indices.GetMapping(
		client.Indices.GetMapping.WithIndex(indexName),
		client.Indices.GetMapping.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("error getting mapping: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error getting mapping: %s", res.String())
	}

	// Parse response
	var mappingResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&mappingResp); err != nil {
		return fmt.Errorf("error decoding mapping response: %w", err)
	}

	// Extract actual properties
	indexMapping, ok := mappingResp[indexName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("index not found in mapping response")
	}

	mappings, ok := indexMapping["mappings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("mappings not found in response")
	}

	actualProperties, ok := mappings["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("properties not found in mappings")
	}

	// Extract expected properties
	expectedProperties, ok := expectedMapping["mappings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid expected mapping structure")
	}
	expectedProps, ok := expectedProperties["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected properties not found")
	}

	// Validate critical fields
	criticalFields := []string{"result_type", "mdb_uid", "typed_uids", "filter_values"}

	for _, fieldName := range criticalFields {
		expectedField, hasExpected := expectedProps[fieldName]
		actualField, hasActual := actualProperties[fieldName]

		if !hasExpected {
			continue // Skip if not in expected mapping
		}

		if !hasActual {
			return fmt.Errorf("field '%s' missing in actual mapping", fieldName)
		}

		// Check field type
		expectedFieldMap, ok1 := expectedField.(map[string]interface{})
		actualFieldMap, ok2 := actualField.(map[string]interface{})
		if !ok1 || !ok2 {
			continue
		}

		expectedType, ok1 := expectedFieldMap["type"].(string)
		actualType, ok2 := actualFieldMap["type"].(string)
		if !ok1 || !ok2 {
			continue
		}

		if expectedType != actualType {
			return fmt.Errorf("field '%s' type mismatch: expected '%s', got '%s'",
				fieldName, expectedType, actualType)
		}
	}

	return nil
}

// DeleteByResultType deletes all documents with a specific result_type from an index
func (m *ES9Manager) DeleteByResultType(ctx context.Context, indexName string, resultType string) (int, error) {
	client, err := m.GetClient()
	if err != nil {
		return 0, err
	}

	// Build delete-by-query request
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"result_type": resultType,
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return 0, fmt.Errorf("error encoding query: %w", err)
	}

	// Execute delete by query
	res, err := client.DeleteByQuery(
		[]string{indexName},
		&buf,
		client.DeleteByQuery.WithContext(ctx),
		client.DeleteByQuery.WithRefresh(true), // Refresh immediately to make visible
	)
	if err != nil {
		return 0, fmt.Errorf("error executing delete by query: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("delete by query failed: %s", res.String())
	}

	// Parse response to get deleted count
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("error parsing delete response: %w", err)
	}

	deleted := 0
	if deletedVal, ok := result["deleted"].(float64); ok {
		deleted = int(deletedVal)
	}

	return deleted, nil
}

// GetExistingUIDs retrieves all _id values for documents with a specific result_type
// Returns a map[string]bool for fast lookup
func (m *ES9Manager) GetExistingUIDs(ctx context.Context, indexName string, resultType string) (map[string]bool, error) {
	client, err := m.GetClient()
	if err != nil {
		return nil, err
	}

	existingUIDs := make(map[string]bool)

	// Use scroll API to fetch all document IDs
	// We only need _id field, not the full document (_source: false)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"result_type": resultType,
			},
		},
		"_source": false, // Don't return document source, only IDs
		"size":    10000, // Batch size per scroll
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("error encoding query: %w", err)
	}

	// Initial search request with scroll
	res, err := client.Search(
		client.Search.WithContext(ctx),
		client.Search.WithIndex(indexName),
		client.Search.WithBody(&buf),
		client.Search.WithScroll(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("error executing search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// Index might not exist yet, return empty set
		if res.StatusCode == 404 {
			return existingUIDs, nil
		}
		return nil, fmt.Errorf("search failed: %s", res.String())
	}

	var searchResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("error parsing search response: %w", err)
	}

	// Extract scroll ID
	scrollID, ok := searchResult["_scroll_id"].(string)
	if !ok {
		return nil, fmt.Errorf("no scroll_id in response")
	}

	// Process initial batch
	hits := searchResult["hits"].(map[string]interface{})["hits"].([]interface{})
	for _, hit := range hits {
		hitMap := hit.(map[string]interface{})
		if id, ok := hitMap["_id"].(string); ok {
			existingUIDs[id] = true
		}
	}

	// Continue scrolling until no more results
	for {
		scrollQuery := map[string]interface{}{
			"scroll":    "1m",
			"scroll_id": scrollID,
		}

		var scrollBuf bytes.Buffer
		if err := json.NewEncoder(&scrollBuf).Encode(scrollQuery); err != nil {
			return nil, fmt.Errorf("error encoding scroll query: %w", err)
		}

		res, err := client.Scroll(
			client.Scroll.WithContext(ctx),
			client.Scroll.WithBody(&scrollBuf),
		)
		if err != nil {
			return nil, fmt.Errorf("error executing scroll: %w", err)
		}

		if res.IsError() {
			res.Body.Close()
			return nil, fmt.Errorf("scroll failed: %s", res.String())
		}

		var scrollResult map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&scrollResult); err != nil {
			res.Body.Close()
			return nil, fmt.Errorf("error parsing scroll response: %w", err)
		}
		res.Body.Close()

		// Update scroll ID
		if newScrollID, ok := scrollResult["_scroll_id"].(string); ok {
			scrollID = newScrollID
		}

		// Process hits
		hits := scrollResult["hits"].(map[string]interface{})["hits"].([]interface{})
		if len(hits) == 0 {
			break // No more results
		}

		for _, hit := range hits {
			hitMap := hit.(map[string]interface{})
			if id, ok := hitMap["_id"].(string); ok {
				existingUIDs[id] = true
			}
		}
	}

	// Clean up scroll
	clearScrollQuery := map[string]interface{}{
		"scroll_id": []string{scrollID},
	}
	var clearBuf bytes.Buffer
	json.NewEncoder(&clearBuf).Encode(clearScrollQuery)
	client.ClearScroll(client.ClearScroll.WithBody(&clearBuf))

	return existingUIDs, nil
}

// HealthInfo contains information about Elasticsearch cluster health
type HealthInfo struct {
	ClusterName   string    `json:"cluster_name"`
	Status        string    `json:"status"`
	Version       string    `json:"version"`
	NumberOfNodes int       `json:"number_of_nodes"`
	ActiveShards  int       `json:"active_shards"`
	Timestamp     time.Time `json:"timestamp"`
}

// String returns a formatted string representation of HealthInfo
func (h *HealthInfo) String() string {
	return fmt.Sprintf(
		"Cluster: %s | Status: %s | Version: %s | Nodes: %d | Active Shards: %d",
		h.ClusterName, h.Status, h.Version, h.NumberOfNodes, h.ActiveShards,
	)
}

// GetES9URLFromConfig gets the ES9 URL from viper config
func GetES9URLFromConfig() string {
	return viper.GetString("elasticsearch9.url")
}
