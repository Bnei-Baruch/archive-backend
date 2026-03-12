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
	ConsumerBatchMaxDocs   int     // Maximum documents per batch (flush when reached)
	ProgressIntervalSec    int     // Print progress stats every N seconds
}

// DefaultPipelineConfig returns the default pipeline configuration
// Tuned for content units indexing with transcript loading (I/O-bound)
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		NumProducers:           20,    // High for I/O-bound transcript loading
		NumConsumers:           3,     // Low to avoid overwhelming ES
		QueueCapacity:          10000, // Large buffer for backpressure
		RelationshipChunkSize:  300,   // DB query batch size
		ConsumerBatchSizeMaxMB: 90.0,  // Max bulk request size
		ConsumerBatchMaxDocs:   1000,  // Max docs per batch (flush when reached)
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

// ItemTask represents a raw item to be processed by its indexer
// Used in the unified pipeline to feed items from multiple types
type ItemTask struct {
	Item     interface{} // Raw item from MDB
	Indexer  Indexer     // Indexer that knows how to process this item
	Type     string      // Type name for progress tracking ("content-units", "collections", etc.)
}

// Pipeline orchestrates the producer-consumer pattern for indexing
type Pipeline struct {
	config                *PipelineConfig
	manager               *common.ES9Manager
	transcriptStatsGetter func() *TranscriptFailureSnapshot // Optional: returns transcript failure stats
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

// SetTranscriptStatsGetter sets a function to retrieve transcript failure statistics
func (p *Pipeline) SetTranscriptStatsGetter(getter func() *TranscriptFailureSnapshot) {
	p.transcriptStatsGetter = getter
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
	// Monitor will exit when context is cancelled
	go p.monitorProgress(ctx, taskQueue, progress)

	// Start producers (parallel document preparation)
	p.startProducers(ctx, taskQueue, indexer, items, indexData, progress)

	// Start consumers (parallel bulk indexing)
	var consumerWg sync.WaitGroup
	errCollector := NewErrorCollector()

	for workerID := 0; workerID < p.config.NumConsumers; workerID++ {
		consumerWg.Add(1)
		go func(id int) {
			defer consumerWg.Done()
			if err := p.consumeIndexTasks(ctx, id, taskQueue, progress); err != nil {
				errCollector.Add(fmt.Errorf("consumer %d: %w", id, err))
			}
		}(workerID)
	}

	// Wait for consumers to drain the queue
	// During normal operation, give consumers unlimited time to process all tasks
	// Only apply timeout if context is cancelled (shutdown signal)
	select {
	case <-waitWithContext(&consumerWg):
		// Consumers finished normally
		log.Info("All consumers finished")
	case <-ctx.Done():
		// Context cancelled (shutdown), wait up to 60s for consumers to finish
		log.Warn("Context cancelled, waiting up to 60s for consumers to finish...")
		select {
		case <-waitWithContext(&consumerWg):
			log.Info("Consumers finished after context cancellation")
		case <-time.After(60 * time.Second):
			log.Warn("Consumers did not finish within 60s - forcing shutdown")
		}
	}

	// Check for errors
	if err := errCollector.Error(); err != nil {
		return err
	}

	// Print final summary
	log.Info("━━━━━━━━━━━━━━━━━━ FINAL SUMMARY ━━━━━━━━━━━━━━━━━━")
	// Get transcript failure stats if available
	var transcriptStats *TranscriptFailureSnapshot
	if p.transcriptStatsGetter != nil {
		transcriptStats = p.transcriptStatsGetter()
	}
	progress.PrintProgress(0, p.config.QueueCapacity, transcriptStats)
	log.Info("✓ Indexing completed successfully")
	return nil
}

// RunUnifiedPipeline executes the indexing pipeline for multiple types concurrently
// Items from all types are fed into a shared channel and processed together
//
// Flow:
//  1. Create bounded task queues (itemQueue -> indexTaskQueue)
//  2. Start progress monitor
//  3. Start producer workers (prepare documents from mixed item types)
//  4. Start consumer workers (bulk index to ES)
//  5. Wait for completion
//  6. Return errors if any
func (p *Pipeline) RunUnifiedPipeline(
	ctx context.Context,
	itemQueue <-chan ItemTask,
	indexData *es.IndexData,
	progress *ProgressTracker,
) error {
	// Create bounded queue for prepared documents
	indexTaskQueue := make(chan IndexTask, p.config.QueueCapacity)

	log.Info("Starting unified pipeline for all types")

	// Start progress monitor (exits when context is cancelled)
	go p.monitorProgress(ctx, indexTaskQueue, progress)

	// Start producers (parallel document preparation from mixed types)
	p.startUnifiedProducers(ctx, indexTaskQueue, itemQueue, indexData, progress)

	// Start consumers (parallel bulk indexing)
	var consumerWg sync.WaitGroup
	errCollector := NewErrorCollector()

	for workerID := 0; workerID < p.config.NumConsumers; workerID++ {
		consumerWg.Add(1)
		go func(id int) {
			defer consumerWg.Done()
			if err := p.consumeIndexTasks(ctx, id, indexTaskQueue, progress); err != nil {
				errCollector.Add(fmt.Errorf("consumer %d: %w", id, err))
			}
		}(workerID)
	}

	// Wait for consumers to drain the queue
	// During normal operation, give consumers unlimited time to process all tasks
	// Only apply timeout if context is cancelled (shutdown signal)
	select {
	case <-waitWithContext(&consumerWg):
		// Consumers finished normally
		log.Info("All consumers finished")
	case <-ctx.Done():
		// Context cancelled (shutdown), wait up to 60s for consumers to finish
		log.Warn("Context cancelled, waiting up to 60s for consumers to finish...")
		select {
		case <-waitWithContext(&consumerWg):
			log.Info("Consumers finished after context cancellation")
		case <-time.After(60 * time.Second):
			log.Warn("Consumers did not finish within 60s - forcing shutdown")
		}
	}

	// Check for errors
	if err := errCollector.Error(); err != nil {
		return err
	}

	// Print final summary
	log.Info("━━━━━━━━━━━━━━━━━━ FINAL SUMMARY ━━━━━━━━━━━━━━━━━━")
	// Get transcript failure stats if available
	var transcriptStats *TranscriptFailureSnapshot
	if p.transcriptStatsGetter != nil {
		transcriptStats = p.transcriptStatsGetter()
	}
	progress.PrintProgress(0, p.config.QueueCapacity, transcriptStats)
	log.Info("✓ All types indexed successfully")
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
	// If context is cancelled, force close after timeout to prevent deadlock
	go func() {
		select {
		case <-waitWithContext(&producerWg):
			// All producers finished normally
			close(taskQueue)
			log.Info("All producers finished, closing taskQueue")
		case <-ctx.Done():
			// Context cancelled, wait up to 30 seconds for producers to finish
			select {
			case <-waitWithContext(&producerWg):
				close(taskQueue)
				log.Info("Producers finished after context cancellation, closing taskQueue")
			case <-time.After(30 * time.Second):
				// Force close after timeout to prevent deadlock
				close(taskQueue)
				log.Warn("Force closing taskQueue after 30s timeout - some producers may be stuck")
			}
		}
	}()
}

// startUnifiedProducers starts producer workers that consume from itemQueue
// and produce IndexTasks for all types
func (p *Pipeline) startUnifiedProducers(
	ctx context.Context,
	indexTaskQueue chan<- IndexTask,
	itemQueue <-chan ItemTask,
	indexData *es.IndexData,
	progress *ProgressTracker,
) {
	var producerWg sync.WaitGroup

	for i := 0; i < p.config.NumProducers; i++ {
		producerWg.Add(1)
		go func(producerID int) {
			defer producerWg.Done()
			p.produceFromUnifiedQueue(ctx, indexTaskQueue, itemQueue, indexData, progress, producerID)
		}(i)
	}

	// Close indexTaskQueue when all producers are done
	// If context is cancelled, force close after timeout to prevent deadlock
	go func() {
		select {
		case <-waitWithContext(&producerWg):
			// All producers finished normally
			close(indexTaskQueue)
			log.Info("All producers finished, closing indexTaskQueue")
		case <-ctx.Done():
			// Context cancelled, wait up to 30 seconds for producers to finish
			select {
			case <-waitWithContext(&producerWg):
				close(indexTaskQueue)
				log.Info("Producers finished after context cancellation, closing indexTaskQueue")
			case <-time.After(30 * time.Second):
				// Force close after timeout to prevent deadlock
				close(indexTaskQueue)
				log.Warn("Force closing indexTaskQueue after 30s timeout - some producers may be stuck")
			}
		}
	}()
}

// produceFromUnifiedQueue consumes ItemTasks and produces IndexTasks
func (p *Pipeline) produceFromUnifiedQueue(
	ctx context.Context,
	indexTaskQueue chan<- IndexTask,
	itemQueue <-chan ItemTask,
	indexData *es.IndexData,
	progress *ProgressTracker,
	producerID int,
) {
	indexDate := &utils.Date{Time: time.Now()}

	// Batch items by indexer for relationship loading efficiency
	const batchSize = 100
	itemBatch := make(map[Indexer][]ItemTask)

	flushBatch := func() {
		for indexer, items := range itemBatch {
			if len(items) == 0 {
				continue
			}

			// Extract raw items for LoadRelationships
			rawItems := make([]interface{}, len(items))
			for i, it := range items {
				rawItems[i] = it.Item
			}

			// Load relationships for this batch
			if err := indexer.LoadRelationships(ctx, rawItems); err != nil {
				log.Errorf("Producer %d: failed to load relationships: %v", producerID, err)
				continue
			}

			// Process each item
			for _, itemTask := range items {
				languages := itemTask.Indexer.GetLanguages()
				indexNameBase := itemTask.Indexer.GetIndexNameBase()

				// Expand to all languages
				for _, lang := range languages {
					doc, skip := itemTask.Indexer.PrepareDocument(
						ctx, itemTask.Item, lang, indexData, indexDate, progress)
					if skip || doc == nil {
						continue
					}

					indexName := fmt.Sprintf("%s_%s", indexNameBase, lang)

					progress.RecordTaskCreated()
					progress.RecordLanguage(lang, 1)

					// Push to index task queue
					select {
					case indexTaskQueue <- IndexTask{
						IndexName: indexName,
						Doc:       doc,
						Lang:      lang,
					}:
						progress.RecordTaskQueued()
					case <-ctx.Done():
						log.Warnf("Producer %d: context cancelled", producerID)
						return
					}
				}

				// Record item completion for type tracking
				progress.RecordItemProcessed()
				progress.RecordTypeItemProcessed(itemTask.Type)
			}
		}

		// Clear batch
		for indexer := range itemBatch {
			itemBatch[indexer] = itemBatch[indexer][:0]
		}
	}

	// Consume items from queue
	for {
		select {
		case itemTask, ok := <-itemQueue:
			if !ok {
				// Queue closed, flush remaining batch
				flushBatch()
				return
			}

			// Add to batch
			if itemBatch[itemTask.Indexer] == nil {
				itemBatch[itemTask.Indexer] = make([]ItemTask, 0, batchSize)
			}
			itemBatch[itemTask.Indexer] = append(itemBatch[itemTask.Indexer], itemTask)

			// Flush if batch is full
			if len(itemBatch[itemTask.Indexer]) >= batchSize {
				flushBatch()
			}

		case <-ctx.Done():
			log.Warnf("Producer %d: context cancelled", producerID)
			return
		}
	}
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

			// Check if we should flush the batch:
			// 1. Size exceeds limit OR
			// 2. Document count exceeds limit
			shouldFlush := len(batch) > 0 && (
				currentSize+taskSizeMB > p.config.ConsumerBatchSizeMaxMB ||
				len(batch) >= p.config.ConsumerBatchMaxDocs)

			if shouldFlush {
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
// Exits when context is cancelled
func (p *Pipeline) monitorProgress(
	ctx context.Context,
	taskQueue <-chan IndexTask,
	progress *ProgressTracker,
) {
	ticker := time.NewTicker(time.Duration(p.config.ProgressIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			queueSize := len(taskQueue)
			// Get transcript failure stats if available
			var transcriptStats *TranscriptFailureSnapshot
			if p.transcriptStatsGetter != nil {
				transcriptStats = p.transcriptStatsGetter()
			}
			progress.PrintProgress(queueSize, p.config.QueueCapacity, transcriptStats)
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
