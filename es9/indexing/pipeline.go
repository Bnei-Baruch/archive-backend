package indexing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/elastic/go-elasticsearch/v9/esapi"

	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// PipelineConfig holds configuration for the indexing pipeline
type PipelineConfig struct {
	NumProducers           int     // Parallel workers for document preparation (I/O-bound)
	NumConsumers           int     // Parallel workers for bulk requests to ES (reduced to avoid overwhelming ES)
	QueueCapacity          int     // Bounded queue size for backpressure
	RelationshipChunkSize  int     // Load relationships from DB in chunks of this size
	ConsumerBatchSizeMaxMB float64 // Maximum bulk request size in MB
	ProgressIntervalSec    int     // Print progress stats every N seconds
}

// DefaultPipelineConfig returns the default pipeline configuration
// Tuned for content units indexing with transcript loading (I/O-bound)
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		NumProducers:           20,   // High for I/O-bound transcript loading
		NumConsumers:           3,    // Low to avoid overwhelming ES
		QueueCapacity:          10000, // Large buffer for backpressure
		RelationshipChunkSize:  300,   // DB query batch size
		ConsumerBatchSizeMaxMB: 90.0,  // Max bulk request size
		ProgressIntervalSec:    10,    // Progress update frequency
	}
}

// IndexTask represents a fully prepared document ready to be indexed
// Generic across all indexer types - always contains an es.Result document
type IndexTask struct {
	IndexName string     // Target index (e.g., "results_he", "collections_en")
	Doc       *es.Result // Fully prepared document ready for indexing
	Lang      string     // Language for stats tracking
}

// Pipeline orchestrates the producer-consumer pattern for indexing
type Pipeline struct {
	config  *PipelineConfig
	manager *common.ES9Manager
}

// NewPipeline creates a new indexing pipeline with the given configuration
func NewPipeline(manager *common.ES9Manager, config *PipelineConfig) *Pipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}
	return &Pipeline{
		config:  config,
		manager: manager,
	}
}

// RunPipeline executes the full indexing pipeline for the given items
// Generic function that works with any Indexer implementation
//
// Flow:
//  1. Create bounded task queue
//  2. Start progress monitor
//  3. Start producer workers (prepare documents)
//  4. Start consumer workers (bulk index to ES)
//  5. Wait for completion
//  6. Return errors if any
func (p *Pipeline) RunPipeline(
	ctx context.Context,
	indexer Indexer,
	items []interface{},
	indexData *es.IndexData,
) error {
	// Create bounded queue with configured capacity
	taskQueue := make(chan IndexTask, p.config.QueueCapacity)

	languages := indexer.GetLanguages()
	maxTasks := len(items) * len(languages)
	log.Infof("Starting pipeline for %s: %d items (up to %d tasks)",
		indexer.GetDocumentType(), len(items), maxTasks)

	// Create progress tracker
	progress := NewProgressTracker(len(items))

	// Start progress monitor (prints aggregated stats periodically)
	monitorDone := make(chan bool)
	go p.monitorProgress(ctx, taskQueue, progress, monitorDone)
	defer func() { monitorDone <- true }()

	// Start producers (parallel document preparation)
	p.startProducers(ctx, taskQueue, indexer, items, indexData, progress)

	// Start consumers (parallel bulk indexing)
	var consumerWg sync.WaitGroup
	errChan := make(chan error, p.config.NumConsumers)

	for workerID := 0; workerID < p.config.NumConsumers; workerID++ {
		consumerWg.Add(1)
		go func(id int) {
			defer consumerWg.Done()
			if err := p.consumeIndexTasks(ctx, id, taskQueue, progress); err != nil {
				errChan <- fmt.Errorf("consumer %d: %w", id, err)
			}
		}(workerID)
	}

	consumerWg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("indexing failed for %d consumers: %v", len(errs), errs)
	}

	// Print final summary
	log.Info("━━━━━━━━━━━━━━━━━━ FINAL SUMMARY ━━━━━━━━━━━━━━━━━━")
	progress.PrintProgress(0, p.config.QueueCapacity)
	log.Info("✓ Indexing completed successfully")
	return nil
}

// startProducers starts multiple producer goroutines to prepare documents
func (p *Pipeline) startProducers(
	ctx context.Context,
	taskQueue chan<- IndexTask,
	indexer Indexer,
	items []interface{},
	indexData *es.IndexData,
	progress *ProgressTracker,
) {
	// Split items across producers
	chunkSize := (len(items) + p.config.NumProducers - 1) / p.config.NumProducers

	var producerWg sync.WaitGroup

	for i := 0; i < p.config.NumProducers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(items) {
			end = len(items)
		}

		chunk := items[start:end]

		producerWg.Add(1)
		go func(producerID int, itemChunk []interface{}) {
			defer producerWg.Done()
			p.produceIndexTasks(ctx, taskQueue, indexer, itemChunk, indexData, progress, producerID)
		}(i, chunk)
	}

	// Close queue when all producers are done
	go func() {
		producerWg.Wait()
		close(taskQueue)
	}()
}

// produceIndexTasks prepares documents and pushes them to the queue
// Loads relationships in chunks for DB efficiency
func (p *Pipeline) produceIndexTasks(
	ctx context.Context,
	taskQueue chan<- IndexTask,
	indexer Indexer,
	items []interface{},
	indexData *es.IndexData,
	progress *ProgressTracker,
	producerID int,
) {
	indexDate := &utils.Date{Time: time.Now()}
	languages := indexer.GetLanguages()
	indexNameBase := indexer.GetIndexNameBase()

	// Process items in chunks for DB efficiency
	for offset := 0; offset < len(items); offset += p.config.RelationshipChunkSize {
		end := offset + p.config.RelationshipChunkSize
		if end > len(items) {
			end = len(items)
		}

		chunk := items[offset:end]

		// Load relationships for this chunk from DB
		if err := indexer.LoadRelationships(ctx, chunk); err != nil {
			log.Errorf("Producer %d: failed to load relationships for chunk %d-%d: %v",
				producerID, offset, end, err)
			continue
		}

		// Process each item in the chunk
		for _, item := range chunk {
			// Expand to all languages
			for _, lang := range languages {
				// Prepare FULL document (including any content loading)
				doc, skip := indexer.PrepareDocument(ctx, item, lang, indexData, indexDate, progress)
				if skip {
					continue
				}
				if doc == nil {
					continue // No i18n for this language
				}

				indexName := fmt.Sprintf("%s_%s", indexNameBase, lang)

				progress.RecordTaskCreated()
				progress.RecordLanguage(lang, 1)

				// Push to queue (blocks if queue is full - backpressure!)
				select {
				case taskQueue <- IndexTask{
					IndexName: indexName,
					Doc:       doc,
					Lang:      lang,
				}:
					progress.RecordTaskQueued()
				case <-ctx.Done():
					log.Warnf("Producer %d: context cancelled, stopping", producerID)
					return
				}
			}

			progress.RecordItemProcessed()
		}
	}
}

// consumeIndexTasks consumes tasks from the queue and batches them for bulk indexing
func (p *Pipeline) consumeIndexTasks(
	ctx context.Context,
	workerID int,
	taskQueue <-chan IndexTask,
	progress *ProgressTracker,
) error {
	batch := make([]IndexTask, 0, 100) // Initial capacity
	currentSize := 0.0                   // Current batch size in MB

	for {
		select {
		case task, ok := <-taskQueue:
			if !ok {
				// Queue closed, flush remaining batch
				if len(batch) > 0 {
					if err := p.sendBulkBatch(ctx, batch, progress); err != nil {
						return err
					}
				}
				return nil
			}

			// Estimate task size
			taskSize := estimateTaskSize(task)
			taskSizeMB := float64(taskSize) / 1024 / 1024

			// Check if adding this task would exceed max batch size
			if len(batch) > 0 && currentSize+taskSizeMB > p.config.ConsumerBatchSizeMaxMB {
				// Send current batch before adding new task
				if err := p.sendBulkBatch(ctx, batch, progress); err != nil {
					return err
				}
				batch = batch[:0] // Reset batch
				currentSize = 0
			}

			// Add task to batch
			batch = append(batch, task)
			currentSize += taskSizeMB
			progress.RecordTaskCompleted()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// sendBulkBatch sends a batch of tasks to Elasticsearch using bulk API
func (p *Pipeline) sendBulkBatch(
	ctx context.Context,
	tasks []IndexTask,
	progress *ProgressTracker,
) error {
	if len(tasks) == 0 {
		return nil
	}

	// Build bulk request body
	var buf bytes.Buffer
	for _, task := range tasks {
		// Action line
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": task.IndexName,
				"_id":    task.Doc.MDB_UID,
			},
		}
		if err := json.NewEncoder(&buf).Encode(action); err != nil {
			return fmt.Errorf("encode action: %w", err)
		}

		// Document line
		if err := json.NewEncoder(&buf).Encode(task.Doc); err != nil {
			return fmt.Errorf("encode document: %w", err)
		}
	}

	// Send bulk request
	req := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}

	client, err := p.manager.GetClient()
	if err != nil {
		return fmt.Errorf("get ES client: %w", err)
	}

	res, err := req.Do(ctx, client)
	if err != nil {
		return fmt.Errorf("bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk request failed: %s", res.Status())
	}

	// Parse response to get success/failure counts
	var bulkRes struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Status int    `json:"status"`
				Error  interface{} `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&bulkRes); err != nil {
		return fmt.Errorf("decode bulk response: %w", err)
	}

	successCount := 0
	failCount := 0
	for _, item := range bulkRes.Items {
		if item.Index.Status >= 200 && item.Index.Status < 300 {
			successCount++
		} else {
			failCount++
		}
	}

	// Record metrics
	progress.RecordBulkRequest(buf.Len(), successCount, failCount)

	return nil
}

// monitorProgress prints progress stats periodically
func (p *Pipeline) monitorProgress(
	ctx context.Context,
	taskQueue <-chan IndexTask,
	progress *ProgressTracker,
	done <-chan bool,
) {
	ticker := time.NewTicker(time.Duration(p.config.ProgressIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			queueSize := len(taskQueue)
			progress.PrintProgress(queueSize, p.config.QueueCapacity)
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// estimateTaskSize estimates the size of a task in bytes
func estimateTaskSize(task IndexTask) int {
	// Rough estimation: JSON-encoded size
	data, err := json.Marshal(task.Doc)
	if err != nil {
		return 1024 // Default estimate if marshal fails
	}
	return len(data) + len(task.IndexName) + len(task.Lang) + 100 // Add overhead
}
