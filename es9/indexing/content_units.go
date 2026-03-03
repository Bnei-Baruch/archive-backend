package indexing

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/integration"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	// Producer-Consumer Configuration
	numProducers               = 20    // Parallel transcript loading workers (increased for I/O-bound work)
	numConsumers               = 3     // Parallel bulk request workers (reduced to avoid overwhelming ES)
	queueCapacity              = 10000 // Bounded queue size (backpressure limit)
	relationshipChunkSize      = 300   // Load relationships from DB in chunks
	consumerBatchSizeMaxMB     = 90.0  // Maximum bulk request size in MB
	progressMonitorIntervalSec = 10    // Print aggregated progress every N seconds
)

// ContentUnitsIndexer handles ES9 indexing for content units
type ContentUnitsIndexer struct {
	manager       *common.ES9Manager
	db            *sql.DB
	indexNameBase string // e.g., "results"
	assetsService integration.AssetsService
}

// ProgressTracker tracks indexing progress with atomic counters for thread-safe aggregation
type ProgressTracker struct {
	// Unit tracking (high-level progress)
	totalUnits     int64
	unitsProcessed int64

	// Task tracking (low-level progress)
	tasksCreated   int64
	tasksQueued    int64
	tasksCompleted int64

	// Transcript tracking
	transcriptsFound   int64
	transcriptsFailed  int64
	transcriptsEmpty   int64
	transcriptsSkipped int64

	// Bulk request tracking
	bulkRequestsSent int64
	bulkRequestBytes int64
	documentsIndexed int64
	documentsFailed  int64

	// Per-language stats (protected by mutex)
	mu          sync.Mutex
	perLanguage map[string]int64
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(totalUnits int) *ProgressTracker {
	return &ProgressTracker{
		totalUnits:  int64(totalUnits),
		perLanguage: make(map[string]int64),
	}
}

// RecordUnitProcessed increments units processed counter
func (pt *ProgressTracker) RecordUnitProcessed() {
	atomic.AddInt64(&pt.unitsProcessed, 1)
}

// RecordTaskCreated increments tasks created counter
func (pt *ProgressTracker) RecordTaskCreated() {
	atomic.AddInt64(&pt.tasksCreated, 1)
}

// RecordTaskQueued increments tasks queued counter
func (pt *ProgressTracker) RecordTaskQueued() {
	atomic.AddInt64(&pt.tasksQueued, 1)
}

// RecordTaskCompleted increments tasks completed counter
func (pt *ProgressTracker) RecordTaskCompleted() {
	atomic.AddInt64(&pt.tasksCompleted, 1)
}

// RecordTranscript records transcript load result
func (pt *ProgressTracker) RecordTranscript(found, success bool, err error) {
	if !found {
		atomic.AddInt64(&pt.transcriptsSkipped, 1)
		return
	}
	atomic.AddInt64(&pt.transcriptsFound, 1)
	if !success {
		if err != nil {
			atomic.AddInt64(&pt.transcriptsFailed, 1)
		} else {
			atomic.AddInt64(&pt.transcriptsEmpty, 1)
		}
	}
}

// RecordBulkRequest records a bulk request sent to ES
func (pt *ProgressTracker) RecordBulkRequest(sizeBytes int, docsIndexed, docsFailed int) {
	atomic.AddInt64(&pt.bulkRequestsSent, 1)
	atomic.AddInt64(&pt.bulkRequestBytes, int64(sizeBytes))
	atomic.AddInt64(&pt.documentsIndexed, int64(docsIndexed))
	atomic.AddInt64(&pt.documentsFailed, int64(docsFailed))
}

// RecordLanguage increments counter for a specific language
func (pt *ProgressTracker) RecordLanguage(lang string, count int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.perLanguage[lang] += int64(count)
}

// GetStats returns current statistics snapshot
func (pt *ProgressTracker) GetStats() map[string]interface{} {
	pt.mu.Lock()
	langStats := make(map[string]int64, len(pt.perLanguage))
	for k, v := range pt.perLanguage {
		langStats[k] = v
	}
	pt.mu.Unlock()

	avgSizeBytes := int64(0)
	if atomic.LoadInt64(&pt.bulkRequestsSent) > 0 {
		avgSizeBytes = atomic.LoadInt64(&pt.bulkRequestBytes) / atomic.LoadInt64(&pt.bulkRequestsSent)
	}

	unitsProcessed := atomic.LoadInt64(&pt.unitsProcessed)
	totalUnits := atomic.LoadInt64(&pt.totalUnits)
	unitsPercent := float64(0)
	if totalUnits > 0 {
		unitsPercent = float64(unitsProcessed) / float64(totalUnits) * 100
	}

	return map[string]interface{}{
		"total_units":         totalUnits,
		"units_processed":     unitsProcessed,
		"units_percent":       unitsPercent,
		"tasks_created":       atomic.LoadInt64(&pt.tasksCreated),
		"tasks_queued":        atomic.LoadInt64(&pt.tasksQueued),
		"tasks_completed":     atomic.LoadInt64(&pt.tasksCompleted),
		"transcripts_found":   atomic.LoadInt64(&pt.transcriptsFound),
		"transcripts_failed":  atomic.LoadInt64(&pt.transcriptsFailed),
		"transcripts_empty":   atomic.LoadInt64(&pt.transcriptsEmpty),
		"transcripts_skipped": atomic.LoadInt64(&pt.transcriptsSkipped),
		"bulk_requests":       atomic.LoadInt64(&pt.bulkRequestsSent),
		"bulk_avg_size_mb":    float64(avgSizeBytes) / 1024 / 1024,
		"bulk_total_size_gb":  float64(atomic.LoadInt64(&pt.bulkRequestBytes)) / 1024 / 1024 / 1024,
		"docs_indexed":        atomic.LoadInt64(&pt.documentsIndexed),
		"docs_failed":         atomic.LoadInt64(&pt.documentsFailed),
		"per_language":        langStats,
	}
}

// PrintProgress prints aggregated progress to log
func (pt *ProgressTracker) PrintProgress(queueSize, queueCap int) {
	stats := pt.GetStats()
	queuePercent := float64(queueSize) / float64(queueCap) * 100

	log.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Infof("UNITS: %d / %d (%.1f%%) processed",
		stats["units_processed"], stats["total_units"], stats["units_percent"])
	log.Infof("TASKS: Created=%d | Queued=%d | Completed=%d | Queue=%d (%.0f%% full)",
		stats["tasks_created"], stats["tasks_queued"], stats["tasks_completed"], queueSize, queuePercent)
	log.Infof("TRANSCRIPTS: Found=%d | Failed=%d | Empty=%d | Skipped=%d",
		stats["transcripts_found"], stats["transcripts_failed"],
		stats["transcripts_empty"], stats["transcripts_skipped"])
	log.Infof("BULK REQUESTS: Sent=%d | Avg=%.2f MB | Total=%.2f GB",
		stats["bulk_requests"], stats["bulk_avg_size_mb"], stats["bulk_total_size_gb"])
	log.Infof("DOCUMENTS: Indexed=%d | Failed=%d",
		stats["docs_indexed"], stats["docs_failed"])

	// Print per-language stats (top 5)
	langStats := stats["per_language"].(map[string]int64)
	if len(langStats) > 0 {
		type langCount struct {
			lang  string
			count int64
		}
		langs := make([]langCount, 0, len(langStats))
		for lang, count := range langStats {
			langs = append(langs, langCount{lang, count})
		}
		// Sort by count descending
		for i := 0; i < len(langs)-1; i++ {
			for j := i + 1; j < len(langs); j++ {
				if langs[j].count > langs[i].count {
					langs[i], langs[j] = langs[j], langs[i]
				}
			}
		}
		// Print top 5
		topN := 5
		if len(langs) < topN {
			topN = len(langs)
		}
		langStrs := make([]string, topN)
		for i := 0; i < topN; i++ {
			langStrs[i] = fmt.Sprintf("%s=%d", langs[i].lang, langs[i].count)
		}
		log.Infof("TOP LANGUAGES: %s", strings.Join(langStrs, " | "))
	}
	log.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// NewContentUnitsIndexer creates a new content units indexer for ES9
func NewContentUnitsIndexer(manager *common.ES9Manager, db *sql.DB, indexNameBase string, assetsService integration.AssetsService) *ContentUnitsIndexer {
	return &ContentUnitsIndexer{
		manager:       manager,
		db:            db,
		indexNameBase: indexNameBase,
		assetsService: assetsService,
	}
}

// IndexAll indexes all content units from MDB to ES9 (all language indices)
// Uses batching to handle large SQL IN clauses (200K+ UIDs)
// Parameters:
//   - reset: if true, delete all existing content units before reindexing (full reset)
//     if false, skip already-indexed units (incremental mode)
func (idx *ContentUnitsIndexer) IndexAll(ctx context.Context, reset bool) error {
	startTime := time.Now()
	mode := "INCREMENTAL"
	if reset {
		mode = "RESET"
	}
	log.Infof("Starting ES9 indexing (%s mode)", mode)

	// Ensure all language indices exist (create if needed)
	if err := idx.ensureIndicesExist(ctx); err != nil {
		return errors.Wrap(err, "ensure indices exist")
	}

	// Handle reset mode: delete all content units from indices
	if reset {
		if err := idx.deleteExistingUnits(ctx); err != nil {
			return errors.Wrap(err, "delete existing units")
		}
	}

	// Get all content units from MDB with default filtering
	contentUnits, err := idx.fetchContentUnits(ctx, defaultContentUnitScope())
	if err != nil {
		return errors.Wrap(err, "fetch content units")
	}

	log.Infof("Found %d content units in MDB", len(contentUnits))

	if len(contentUnits) == 0 {
		log.Info("No content units to index")
		return nil
	}

	// Filter out already-indexed units in incremental mode
	if !reset {
		contentUnits, err = idx.filterExistingUnits(ctx, contentUnits)
		if err != nil {
			return errors.Wrap(err, "filter existing units")
		}

		if len(contentUnits) == 0 {
			log.Info("✓ All content units already indexed")
			return nil
		}

		log.Infof("Filtered: %d new units to index", len(contentUnits))
	}

	// Load sources and tags ONCE for all content units
	log.Info("Loading sources/tags for all units...")
	indexDataStart := time.Now()
	allIndexData, err := idx.loadIndexData(ctx, contentUnits)
	if err != nil {
		return errors.Wrap(err, "load index data for all units")
	}
	log.Infof("✓ Loaded sources/tags in %v", time.Since(indexDataStart))

	// Index in batches
	if err := idx.indexInBatches(ctx, contentUnits, allIndexData, reset); err != nil {
		return err
	}

	log.Infof("✓ Indexing completed in %v", time.Since(startTime))
	return nil
}

// defaultContentUnitScope returns the default SQL scope for content units
// Matches ES6 logic: only published, public units, excluding certain types
func defaultContentUnitScope() []qm.QueryMod {
	// Exclude certain content types
	excludedTypeIDs := []int64{
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LELO_MIKUD].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_PUBLICATION].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONG].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOK].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BLOG_POST].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_RESEARCH_MATERIAL].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KTAIM_NIVCHARIM].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_ARTICLE].ID,
	}

	log.Debugf("Building SQL scope with excluded type IDs: %v", excludedTypeIDs)

	// Manually build NOT IN clause string (more reliable than SQLBoiler's WhereNotIn)
	excludedIDsStr := int64SliceToString(excludedTypeIDs)
	notInClause := fmt.Sprintf("type_id NOT IN (%s)", excludedIDsStr)

	scope := []qm.QueryMod{
		qm.Where("secure = 0"),
		qm.Where("published IS TRUE"),
		qm.Where(notInClause),
		// NOTE: Removed eager loading here to avoid massive IN clauses (200K+ IDs)
		// Relationships will be loaded per batch (5000 units) in loadRelationshipsForBatch()
	}

	log.Debug("SQL scope built WITHOUT eager loading (will load per batch)")
	return scope
}

// int64SliceToString converts []int64 to comma-separated string "1,2,3"
func int64SliceToString(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}

	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(strs, ",")
}

// fetchContentUnits loads content units from MDB
func (idx *ContentUnitsIndexer) fetchContentUnits(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.ContentUnit, error) {
	units, err := mdbmodels.ContentUnits(scope...).All(idx.db)
	if err != nil {
		return nil, err
	}

	log.Debugf("✓ Fetched %d content units", len(units))
	return units, nil
}

// loadIndexData fetches supplementary data for indexing (sources, tags, etc.)
func (idx *ContentUnitsIndexer) loadIndexData(ctx context.Context, contentUnits []*mdbmodels.ContentUnit) (*es.IndexData, error) {
	// Build SQL scope from content unit UIDs
	cuUIDs := make([]string, len(contentUnits))
	for i, cu := range contentUnits {
		cuUIDs[i] = cu.UID
	}

	// Create SQL IN clause for content unit UIDs
	sqlScope := fmt.Sprintf("cu.uid IN ('%s')", joinWithQuotes(cuUIDs, "','"))

	// Create IndexData using ES6 helper (loads all data in one call)
	indexData, err := es.MakeIndexData(idx.db, sqlScope)
	if err != nil {
		return nil, errors.Wrap(err, "make index data")
	}

	log.Debugf("Index data: %d sources, %d tags, %d media_langs, %d transcripts",
		len(indexData.Sources),
		len(indexData.Tags),
		len(indexData.MediaLanguages),
		len(indexData.Transcripts))

	return indexData, nil
}

// loadRelationshipsForBatch loads ContentUnitI18ns, Collections, and Persons for a batch
// This replaces eager loading to avoid massive IN clauses (200K+ IDs)
func (idx *ContentUnitsIndexer) loadRelationshipsForBatch(ctx context.Context, contentUnits []*mdbmodels.ContentUnit) error {
	if len(contentUnits) == 0 {
		return nil
	}

	log.Debugf("Loading relationships for batch of %d content units", len(contentUnits))

	// Extract content unit IDs for IN clause
	cuIDs := make([]interface{}, len(contentUnits))
	for i, cu := range contentUnits {
		cuIDs[i] = cu.ID
	}

	// Load ContentUnitI18ns
	i18ns, err := mdbmodels.ContentUnitI18ns(
		qm.WhereIn("content_unit_id IN ?", cuIDs...),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load content_unit_i18ns")
	}

	// Load CollectionsContentUnits with nested Collection and CollectionI18ns
	ccus, err := mdbmodels.CollectionsContentUnits(
		qm.WhereIn("content_unit_id IN ?", cuIDs...),
		qm.Load("Collection"),
		qm.Load("Collection.CollectionI18ns"),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load collections_content_units")
	}

	// Load ContentUnitsPersons with nested Person
	cups, err := mdbmodels.ContentUnitsPersons(
		qm.WhereIn("content_unit_id IN ?", cuIDs...),
		qm.Load("Person"),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load content_units_persons")
	}

	// Map relationships back to content units
	// Create lookup maps by content_unit_id
	i18nMap := make(map[int64][]*mdbmodels.ContentUnitI18n)
	for _, i18n := range i18ns {
		i18nMap[i18n.ContentUnitID] = append(i18nMap[i18n.ContentUnitID], i18n)
	}

	ccuMap := make(map[int64][]*mdbmodels.CollectionsContentUnit)
	for _, ccu := range ccus {
		ccuMap[ccu.ContentUnitID] = append(ccuMap[ccu.ContentUnitID], ccu)
	}

	cupMap := make(map[int64][]*mdbmodels.ContentUnitsPerson)
	for _, cup := range cups {
		cupMap[cup.ContentUnitID] = append(cupMap[cup.ContentUnitID], cup)
	}

	// Assign relationships to each content unit
	for _, cu := range contentUnits {
		// Initialize R if nil
		if cu.R == nil {
			cu.R = cu.R.NewStruct()
		}

		// Assign i18ns
		if i18nList, ok := i18nMap[cu.ID]; ok {
			cu.R.ContentUnitI18ns = i18nList
		}

		// Assign collections
		if ccuList, ok := ccuMap[cu.ID]; ok {
			cu.R.CollectionsContentUnits = ccuList
		}

		// Assign persons
		if cupList, ok := cupMap[cu.ID]; ok {
			cu.R.ContentUnitsPersons = cupList
		}
	}

	log.Debugf("✓ Loaded relationships: %d i18ns, %d collections, %d persons",
		len(i18ns), len(ccus), len(cups))

	return nil
}

// joinWithQuotes joins strings with a separator, used for SQL IN clauses
func joinWithQuotes(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// deleteExistingUnits deletes all content units from all language indices
func (idx *ContentUnitsIndexer) deleteExistingUnits(ctx context.Context) error {
	log.Info("Deleting existing content units from all language indices")

	languages := consts.ALL_KNOWN_LANGS[:]
	totalDeleted := 0

	for _, lang := range languages {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

		deleted, err := idx.manager.DeleteByResultType(ctx, indexName, consts.ES_RESULT_TYPE_UNITS)
		if err != nil {
			log.Warnf("Failed to delete from %s: %v", indexName, err)
			continue
		}

		if deleted > 0 {
			log.Infof("  %s: deleted %d documents", lang, deleted)
			totalDeleted += deleted
		}
	}

	log.Infof("✓ Deleted %d total documents across all languages", totalDeleted)
	return nil
}

// filterExistingUnits removes content units that are already indexed in ES9
// Returns only units that need to be indexed (new units)
func (idx *ContentUnitsIndexer) filterExistingUnits(ctx context.Context, contentUnits []*mdbmodels.ContentUnit) ([]*mdbmodels.ContentUnit, error) {
	log.Info("Loading existing content unit IDs from ES9 indices")

	// Collect existing UIDs from all language indices
	// A unit is considered "existing" if it exists in ANY language index
	existingUIDs := make(map[string]bool)

	languages := consts.ALL_KNOWN_LANGS[:]
	for _, lang := range languages {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

		uids, err := idx.manager.GetExistingUIDs(ctx, indexName, consts.ES_RESULT_TYPE_UNITS)
		if err != nil {
			log.Warnf("Failed to get existing UIDs from %s: %v", indexName, err)
			continue
		}

		// Merge UIDs from this language
		for uid := range uids {
			existingUIDs[uid] = true
		}
	}

	log.Infof("Found %d existing content units across all languages", len(existingUIDs))

	// Filter out existing units
	newUnits := make([]*mdbmodels.ContentUnit, 0)
	for _, cu := range contentUnits {
		if !existingUIDs[cu.UID] {
			newUnits = append(newUnits, cu)
		}
	}

	skipped := len(contentUnits) - len(newUnits)
	log.Infof("Filtered: %d existing (skipped), %d new (will index)", skipped, len(newUnits))

	return newUnits, nil
}

// ensureIndicesExist creates all language indices if they don't exist
func (idx *ContentUnitsIndexer) ensureIndicesExist(ctx context.Context) error {
	log.Info("Ensuring ES9 indices exist for all languages")

	languages := consts.ALL_KNOWN_LANGS[:]
	var wg sync.WaitGroup
	errChan := make(chan error, len(languages))

	for _, lang := range languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

			// Check if index exists
			exists, err := idx.manager.IndexExists(ctx, indexName)
			if err != nil {
				errChan <- errors.Wrapf(err, "check index existence: %s", indexName)
				return
			}

			if exists {
				log.Infof("Index already exists: %s", indexName)
				return
			}

			// Load mapping for this language
			mapping, err := idx.loadMappingFile(lang)
			if err != nil {
				errChan <- errors.Wrapf(err, "load mapping for %s", lang)
				return
			}

			// Create index with mapping
			if err := idx.manager.CreateIndex(ctx, indexName, mapping); err != nil {
				errChan <- errors.Wrapf(err, "create index: %s", indexName)
				return
			}

			log.Infof("✓ Created index: %s", indexName)
		}(lang)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to create %d indices: %v", len(errs), errs[0])
	}

	log.Info("✓ All indices ready")
	return nil
}

// loadMappingFile loads a mapping file from disk for a specific language
func (idx *ContentUnitsIndexer) loadMappingFile(lang string) (map[string]interface{}, error) {
	// Mapping file path: es9/data/mappings/results/results-{lang}.json
	mappingPath := fmt.Sprintf("es9/data/mappings/%s/%s-%s.json", idx.indexNameBase, idx.indexNameBase, lang)

	// Read mapping file
	data, err := os.ReadFile(mappingPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read mapping file: %s", mappingPath)
	}

	// Parse JSON
	var mapping map[string]interface{}
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, errors.Wrapf(err, "parse mapping JSON: %s", mappingPath)
	}

	return mapping, nil
}

// indexInBatches processes all content units using producer-consumer pattern
// allIndexData contains sources and tags loaded once for all units (efficient)
// Producers load relationships (i18ns, collections, persons) in chunks of 100 for DB efficiency
func (idx *ContentUnitsIndexer) indexInBatches(ctx context.Context, contentUnits []*mdbmodels.ContentUnit, allIndexData *es.IndexData, resetMode bool) error {
	total := len(contentUnits)
	log.Infof("Starting indexing for %d content units", total)

	// Index all units using producer-consumer pattern
	// Producers will load relationships in chunks of 100
	if err := idx.indexToAllLanguages(ctx, contentUnits, allIndexData); err != nil {
		return errors.Wrap(err, "index to all languages")
	}

	return nil
}

// IndexTask represents a fully prepared document ready to be indexed
type IndexTask struct {
	IndexName string     // Target index (e.g., "results_he")
	Doc       *es.Result // Fully prepared document with transcript
	Lang      string     // Language for stats tracking
}

// indexToAllLanguages indexes content units using producer-consumer pattern with bounded queue
func (idx *ContentUnitsIndexer) indexToAllLanguages(ctx context.Context, contentUnits []*mdbmodels.ContentUnit, indexData *es.IndexData) error {
	// Create bounded queue with configured capacity
	taskQueue := make(chan IndexTask, queueCapacity)

	maxTasks := len(contentUnits) * len(consts.ALL_KNOWN_LANGS)
	log.Infof("Starting producer-consumer indexing: %d units (up to %d tasks)",
		len(contentUnits), maxTasks)

	// Create progress tracker with total units count
	progress := NewProgressTracker(len(contentUnits))

	// Start progress monitor (prints aggregated stats every 10 seconds)
	monitorDone := make(chan bool)
	go idx.monitorProgress(ctx, taskQueue, progress, monitorDone)
	defer func() { monitorDone <- true }()

	// Start producers (5 parallel for transcript loading)
	// Producers load relationships in chunks of 100 for DB efficiency
	idx.startProducers(ctx, taskQueue, contentUnits, indexData, progress)

	// Start consumers (configured parallel workers)
	var consumerWg sync.WaitGroup
	errChan := make(chan error, numConsumers)

	for workerID := 0; workerID < numConsumers; workerID++ {
		consumerWg.Add(1)
		go func(id int) {
			defer consumerWg.Done()
			if err := idx.consumeIndexTasks(ctx, id, taskQueue, progress); err != nil {
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
	progress.PrintProgress(0, queueCapacity)
	log.Info("✓ Indexing completed successfully")
	return nil
}

// startProducers starts multiple producer goroutines to load and prepare documents
func (idx *ContentUnitsIndexer) startProducers(
	ctx context.Context,
	taskQueue chan<- IndexTask,
	contentUnits []*mdbmodels.ContentUnit,
	indexData *es.IndexData,
	progress *ProgressTracker,
) {
	// Split content units across producers (by count, not size - acceptable for transcript loading)
	chunkSize := (len(contentUnits) + numProducers - 1) / numProducers

	var producerWg sync.WaitGroup

	for i := 0; i < numProducers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(contentUnits) {
			end = len(contentUnits)
		}

		chunk := contentUnits[start:end]

		producerWg.Add(1)
		go func(id int, units []*mdbmodels.ContentUnit) {
			defer producerWg.Done()
			idx.produceIndexTasks(ctx, taskQueue, units, indexData, progress, id)
		}(i, chunk)
	}

	// Close queue when all producers done
	go func() {
		producerWg.Wait()
		close(taskQueue)
	}()
}

// produceIndexTasks prepares documents and pushes them to the queue
// Loads relationships (i18ns, collections, persons) in chunks for DB efficiency
func (idx *ContentUnitsIndexer) produceIndexTasks(
	ctx context.Context,
	taskQueue chan<- IndexTask,
	contentUnits []*mdbmodels.ContentUnit,
	indexData *es.IndexData,
	progress *ProgressTracker,
	producerID int,
) {
	indexDate := utils.Date{Time: time.Now()}

	// Process units in chunks
	for offset := 0; offset < len(contentUnits); offset += relationshipChunkSize {
		end := offset + relationshipChunkSize
		if end > len(contentUnits) {
			end = len(contentUnits)
		}

		chunk := contentUnits[offset:end]

		// Load relationships for this chunk from DB
		if err := idx.loadRelationshipsForBatch(ctx, chunk); err != nil {
			log.Errorf("Producer %d: failed to load relationships for chunk %d-%d: %v",
				producerID, offset, end, err)
			continue
		}

		// Process each unit in the chunk
		for _, cu := range chunk {
			// Expand to all languages inline
			for _, lang := range consts.ALL_KNOWN_LANGS {
				// Prepare FULL document (including transcript loading!)
				doc, skip := idx.prepareDocumentForLanguage(cu, lang, indexData, &indexDate, progress)
				if skip {
					continue
				}
				if doc == nil {
					continue // No i18n for this language
				}

				indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

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
					return
				}
			}

			// Unit fully processed (expanded to all languages)
			progress.RecordUnitProcessed()
		}
	}
}

// monitorProgress logs aggregated progress periodically
func (idx *ContentUnitsIndexer) monitorProgress(
	ctx context.Context,
	taskQueue chan IndexTask,
	progress *ProgressTracker,
	done <-chan bool,
) {
	ticker := time.NewTicker(time.Duration(progressMonitorIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			queueSize := len(taskQueue)
			queueCap := cap(taskQueue)
			progress.PrintProgress(queueSize, queueCap)

			// Diagnostic warnings
			fillPercent := float64(queueSize) / float64(queueCap) * 100
			if fillPercent > 90 {
				log.Warn("⚠ Queue >90% full - consumers may be bottleneck")
			} else if queueSize > 0 && fillPercent < 10 {
				log.Warn("⚠ Queue <10% full - producers may be bottleneck")
			}

		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// consumeIndexTasks pulls tasks from queue, batches by size, and sends to ES
func (idx *ContentUnitsIndexer) consumeIndexTasks(
	ctx context.Context,
	consumerID int,
	taskQueue <-chan IndexTask,
	progress *ProgressTracker,
) error {
	const maxSizeBytes = int(consumerBatchSizeMaxMB * 1024 * 1024)

	currentBatch := []IndexTask{}
	currentSize := 0

	for task := range taskQueue { // Task already has full document!
		// Estimate size
		docSize := idx.estimateDocumentSize(task.Doc, task.IndexName)

		// Check if batch is full
		if currentSize+docSize > maxSizeBytes && len(currentBatch) > 0 {
			// Send batch and record completion AFTER sending
			if err := idx.sendBulkBatch(ctx, currentBatch, progress); err != nil {
				return err
			}
			// Record tasks as completed AFTER successful send
			for range currentBatch {
				progress.RecordTaskCompleted()
			}
			currentBatch = []IndexTask{}
			currentSize = 0
		}

		// Add to batch
		currentBatch = append(currentBatch, task)
		currentSize += docSize
	}

	// Send final batch
	if len(currentBatch) > 0 {
		if err := idx.sendBulkBatch(ctx, currentBatch, progress); err != nil {
			return err
		}
		// Record final batch as completed
		for range currentBatch {
			progress.RecordTaskCompleted()
		}
	}

	return nil
}

// prepareDocumentForLanguage creates a fully prepared document for a specific language
func (idx *ContentUnitsIndexer) prepareDocumentForLanguage(
	cu *mdbmodels.ContentUnit,
	lang string,
	indexData *es.IndexData,
	indexDate *utils.Date,
	progress *ProgressTracker,
) (*es.Result, bool) {
	// Find i18n for this language
	var i18n *mdbmodels.ContentUnitI18n
	if cu.R != nil {
		for _, cui := range cu.R.ContentUnitI18ns {
			if cui.Language == lang {
				i18n = cui
				break
			}
		}
	}

	// Skip if no translation for this language
	if i18n == nil {
		return nil, true // skip
	}

	// Skip if no name
	if !i18n.Name.Valid || i18n.Name.String == "" {
		return nil, true // skip
	}

	// Create Result document
	doc := &es.Result{
		ResultType: consts.ES_RESULT_TYPE_UNITS,
		MDB_UID:    cu.UID,
		Title:      html.UnescapeString(i18n.Name.String),
		IndexDate:  indexDate,
	}

	// Description
	if i18n.Description.Valid && i18n.Description.String != "" {
		doc.Description = html.UnescapeString(i18n.Description.String)
	}

	// Full title (hierarchical path from collections)
	doc.FullTitle = idx.buildFullTitle(cu, lang)

	// Effective date from properties
	if cu.Properties.Valid {
		propertiesJSON, _ := cu.Properties.MarshalJSON()
		doc.EffectiveDate = idx.extractEffectiveDate(string(propertiesJSON))
	}

	// Build typed UIDs
	doc.TypedUids = idx.buildTypedUids(cu, lang, indexData)

	// Build filter values
	doc.FilterValues = idx.buildFilterValues(cu, indexData)

	// Extract content from transcript (if available)
	doc.Content = idx.extractContent(cu, indexData, lang, progress)

	// Build full content for LLM/vector search (combines all fields)
	doc.FullContent = idx.buildFullContent(doc)

	// Build title suggest for autocomplete (if no sources)
	doc.TitleSuggest = idx.buildTitleSuggest(cu, doc.Title, indexData)

	return doc, false // not skipped
}

// estimateDocumentSize estimates the size of a document in bytes
func (idx *ContentUnitsIndexer) estimateDocumentSize(doc *es.Result, indexName string) int {
	// Rough estimate based on JSON serialization
	size := 0

	// Metadata
	size += len(indexName) + len(doc.MDB_UID) + 100

	// Text fields
	size += len(doc.Title)
	size += len(doc.Description)
	size += len(doc.Content) // This is the big one (transcript)
	size += len(doc.FullContent)
	size += len(doc.FullTitle)

	// Arrays
	size += len(doc.TypedUids) * 20
	size += len(doc.FilterValues) * 20
	size += len(doc.TitleSuggest.Input) * 20

	// Overhead for JSON structure
	size += 200

	return size
}

// indexToLanguage indexes content units to a specific language index (LEGACY - NOT USED)
func (idx *ContentUnitsIndexer) indexToLanguage(ctx context.Context, lang string, contentUnits []*mdbmodels.ContentUnit, indexData *es.IndexData, progress *ProgressTracker) error {
	// Build index name: results_en, results_he, etc.
	indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

	// Prepare documents for this language
	docs, skipped, err := idx.prepareDocuments(lang, contentUnits, indexData, progress)
	if err != nil {
		return errors.Wrap(err, "prepare documents")
	}

	// Record skipped documents (legacy tracking)
	_ = skipped

	if len(docs) == 0 {
		return nil
	}

	// Bulk index documents
	if err := idx.bulkIndex(ctx, indexName, docs); err != nil {
		return errors.Wrap(err, "bulk index")
	}

	// Record successfully indexed documents
	progress.RecordLanguage(lang, len(docs))

	return nil
}

// prepareDocuments creates Result documents for a specific language (LEGACY - NOT USED)
// Returns: docs, skippedCount, error
func (idx *ContentUnitsIndexer) prepareDocuments(lang string, contentUnits []*mdbmodels.ContentUnit, indexData *es.IndexData, progress *ProgressTracker) ([]*es.Result, int, error) {
	docs := make([]*es.Result, 0, len(contentUnits))
	indexDate := utils.Date{Time: time.Now()}
	skipped := 0

	for _, cu := range contentUnits {
		// Find i18n for this language
		var i18n *mdbmodels.ContentUnitI18n
		for _, cui := range cu.R.ContentUnitI18ns {
			if cui.Language == lang {
				i18n = cui
				break
			}
		}

		// Skip if no translation for this language
		if i18n == nil {
			skipped++
			continue
		}

		// Skip if no name
		if !i18n.Name.Valid || i18n.Name.String == "" {
			skipped++
			continue
		}

		// Create Result document
		doc := &es.Result{
			ResultType: consts.ES_RESULT_TYPE_UNITS,
			MDB_UID:    cu.UID,
			Title:      html.UnescapeString(i18n.Name.String),
			IndexDate:  &indexDate,
		}

		// Description
		if i18n.Description.Valid && i18n.Description.String != "" {
			doc.Description = html.UnescapeString(i18n.Description.String)
		}

		// Full title (hierarchical path from collections)
		doc.FullTitle = idx.buildFullTitle(cu, lang)

		// Effective date from properties
		if cu.Properties.Valid {
			propertiesJSON, _ := cu.Properties.MarshalJSON()
			doc.EffectiveDate = idx.extractEffectiveDate(string(propertiesJSON))
		}

		// Build typed UIDs
		doc.TypedUids = idx.buildTypedUids(cu, lang, indexData)

		// Build filter values
		doc.FilterValues = idx.buildFilterValues(cu, indexData)

		// Extract content from transcript (if available)
		doc.Content = idx.extractContent(cu, indexData, lang, progress)

		// Build full content for LLM/vector search (combines all fields)
		doc.FullContent = idx.buildFullContent(doc)

		// Build title suggest for autocomplete (if no sources)
		doc.TitleSuggest = idx.buildTitleSuggest(cu, doc.Title, indexData)

		docs = append(docs, doc)
	}

	return docs, skipped, nil
}

// buildFullTitle constructs the full hierarchical title from collections
func (idx *ContentUnitsIndexer) buildFullTitle(cu *mdbmodels.ContentUnit, lang string) string {
	if cu.R == nil || len(cu.R.CollectionsContentUnits) == 0 {
		return ""
	}

	// Get the first collection (usually the primary parent)
	ccu := cu.R.CollectionsContentUnits[0]
	if ccu.R == nil || ccu.R.Collection == nil {
		return ""
	}

	collection := ccu.R.Collection

	// Find collection i18n for this language
	for _, ci18n := range collection.R.CollectionI18ns {
		if ci18n.Language == lang && ci18n.Name.Valid {
			collectionName := html.UnescapeString(ci18n.Name.String)

			// Find content unit i18n for this language
			for _, cui18n := range cu.R.ContentUnitI18ns {
				if cui18n.Language == lang && cui18n.Name.Valid {
					cuName := html.UnescapeString(cui18n.Name.String)
					// Format: "Collection Name > Content Unit Name"
					return fmt.Sprintf("%s > %s", collectionName, cuName)
				}
			}
		}
	}

	return ""
}

// extractEffectiveDate parses effective_date from properties JSON
func (idx *ContentUnitsIndexer) extractEffectiveDate(propertiesJSON string) *utils.Date {
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &props); err != nil {
		return nil
	}

	if filmDate, ok := props["film_date"].(string); ok {
		if t, err := time.Parse("2006-01-02", filmDate); err == nil {
			return &utils.Date{Time: t}
		}
	}

	return nil
}

// buildTypedUids creates the typed_uids array for relation tracking
func (idx *ContentUnitsIndexer) buildTypedUids(cu *mdbmodels.ContentUnit, lang string, indexData *es.IndexData) []string {
	uids := []string{es.KeyValue(consts.ES_UID_TYPE_CONTENT_UNIT, cu.UID)}

	// Add collections
	if cu.R != nil {
		for _, ccu := range cu.R.CollectionsContentUnits {
			if ccu.R != nil && ccu.R.Collection != nil {
				uids = append(uids, es.KeyValue(consts.ES_UID_TYPE_COLLECTION, ccu.R.Collection.UID))
			}
		}
	}

	// Add sources (from index data)
	if sources, ok := indexData.Sources[cu.UID]; ok {
		uids = append(uids, es.KeyValues(consts.ES_UID_TYPE_SOURCE, sources)...)
	}

	// Add tags (from index data)
	if tags, ok := indexData.Tags[cu.UID]; ok {
		uids = append(uids, es.KeyValues(consts.ES_UID_TYPE_TAG, tags)...)
	}

	// Add transcript file UID for the current language if available
	// Note: Transcripts structure is map[cuUID]map[language][]TranscriptFile
	// We use the first file which is already prioritized by insert_type and created_at
	if transcriptLangs, ok := indexData.Transcripts[cu.UID]; ok {
		if langTranscripts, ok := transcriptLangs[lang]; ok && len(langTranscripts) > 0 {
			file := langTranscripts[0]
			uids = append(uids, es.KeyValue(consts.ES_UID_TYPE_FILE, file.UID))
		}
	}

	return uids
}

// buildFilterValues creates the filter_values array for faceted search
func (idx *ContentUnitsIndexer) buildFilterValues(cu *mdbmodels.ContentUnit, indexData *es.IndexData) []string {
	filters := make([]string, 0)

	// Content type
	contentType := mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name
	filters = append(filters, es.KeyValue("content_type", contentType))

	// Collection content types (parent collections)
	if cu.R != nil {
		for _, ccu := range cu.R.CollectionsContentUnits {
			if ccu.R != nil && ccu.R.Collection != nil {
				collectionType := mdb.CONTENT_TYPE_REGISTRY.ByID[ccu.R.Collection.TypeID].Name
				filters = append(filters, es.KeyValue("collections_content_type", collectionType))
			}
		}
	}

	// Persons
	if cu.R != nil {
		for _, cup := range cu.R.ContentUnitsPersons {
			if cup.R != nil && cup.R.Person != nil {
				filters = append(filters, es.KeyValue("person", cup.R.Person.UID))
			}
		}
	}

	// Sources (from index data)
	if sources, ok := indexData.Sources[cu.UID]; ok {
		for _, sourceUID := range sources {
			filters = append(filters, es.KeyValue("source", sourceUID))
		}
	}

	// Tags (from index data)
	if tags, ok := indexData.Tags[cu.UID]; ok {
		for _, tagUID := range tags {
			filters = append(filters, es.KeyValue("tag", tagUID))
		}
	}

	// Media languages (from index data)
	if mediaLangs, ok := indexData.MediaLanguages[cu.UID]; ok {
		for _, mlang := range mediaLangs {
			filters = append(filters, es.KeyValue("media_language", mlang))
		}
	}

	// Original language from properties
	if cu.Properties.Valid {
		propertiesJSON, _ := cu.Properties.MarshalJSON()
		var props map[string]interface{}
		if err := json.Unmarshal(propertiesJSON, &props); err == nil {
			if origLang, ok := props["original_language"].(string); ok {
				filters = append(filters, es.KeyValue("original_language", origLang))
			}
		}
	}

	return filters
}

// extractContent extracts text content from transcripts
// Handles Pattern 1 (duplicate filenames) and Pattern 3 (sequential parts)
func (idx *ContentUnitsIndexer) extractContent(cu *mdbmodels.ContentUnit, indexData *es.IndexData, lang string, progress *ProgressTracker) string {
	// Get transcript files for this content unit
	transcripts, ok := indexData.Transcripts[cu.UID]
	if !ok || len(transcripts) == 0 {
		progress.RecordTranscript(false, false, nil)
		return ""
	}

	// Get transcripts for this specific language
	files, ok := transcripts[lang]
	if !ok || len(files) == 0 {
		progress.RecordTranscript(false, false, nil)
		return ""
	}

	// Pattern 1: Duplicate filenames (same name, different content)
	// Try each file in order (latest first) until one succeeds
	if idx.hasDuplicateFilenames(files) {
		return idx.extractWithFallback(cu, files, lang, progress)
	}

	// Pattern 3: Sequential parts (_1_c, _2_c, _01, _02, etc.)
	// Concatenate files in the correct order
	if idx.hasSequentialParts(files) {
		return idx.extractAndConcatenate(cu, files, lang, progress)
	}

	// Default: Use first file (already prioritized by insert_type and created_at)
	file := files[0]
	content, err := idx.assetsService.Doc2Text(file.UID)
	if err != nil {
		log.Warnf("⚠ Transcript failed: %s | %s | %s (%s) | %v", cu.UID, lang, file.UID, file.Name, err)
		progress.RecordTranscript(true, false, err)
		return ""
	}

	if content == "" {
		progress.RecordTranscript(true, false, fmt.Errorf("empty transcript"))
		return ""
	}

	progress.RecordTranscript(true, true, nil)
	return content
}

// hasDuplicateFilenames checks if multiple files have the same filename
func (idx *ContentUnitsIndexer) hasDuplicateFilenames(files []es.TranscriptFile) bool {
	if len(files) <= 1 {
		return false
	}
	nameMap := make(map[string]int)
	for _, f := range files {
		nameMap[f.Name]++
		if nameMap[f.Name] > 1 {
			return true
		}
	}
	return false
}

// hasSequentialParts checks if files have sequential naming patterns
// Patterns: _1_c, _2_c or _01, _02 or _001_, _002_ or _part1, _part2
func (idx *ContentUnitsIndexer) hasSequentialParts(files []es.TranscriptFile) bool {
	if len(files) <= 1 {
		return false
	}
	// Check for common sequential patterns in filenames
	for _, f := range files {
		// Pattern: _N_c (where N is a digit)
		if strings.Contains(f.Name, "_1_c") || strings.Contains(f.Name, "_2_c") {
			return true
		}
		// Pattern: _NN. (two or more digits before extension)
		if strings.Contains(f.Name, "_01") || strings.Contains(f.Name, "_02") {
			return true
		}
		// Pattern: _NNN_ (three digits with underscores)
		if strings.Contains(f.Name, "_001_") || strings.Contains(f.Name, "_002_") {
			return true
		}
		// Pattern: _partN or -partN
		if strings.Contains(f.Name, "_part") || strings.Contains(f.Name, "-part") {
			return true
		}
	}
	return false
}

// extractWithFallback tries each file in order until one succeeds (Pattern 1)
func (idx *ContentUnitsIndexer) extractWithFallback(cu *mdbmodels.ContentUnit, files []es.TranscriptFile, lang string, progress *ProgressTracker) string {
	var lastErr error
	for i, file := range files {
		content, err := idx.assetsService.Doc2Text(file.UID)
		if err == nil && content != "" {
			if i > 0 {
				log.Infof("✓ Fallback success: %s | %s | tried %d files, used %s (%s)", cu.UID, lang, i+1, file.UID, file.Name)
			}
			progress.RecordTranscript(true, true, nil)
			return content
		}
		if err != nil {
			lastErr = err
		}
	}
	log.Warnf("⚠ All %d duplicate files failed: %s | %s | %v", len(files), cu.UID, lang, lastErr)
	progress.RecordTranscript(true, false, lastErr)
	return ""
}

// extractAndConcatenate concatenates sequential files in order (Pattern 3)
func (idx *ContentUnitsIndexer) extractAndConcatenate(cu *mdbmodels.ContentUnit, files []es.TranscriptFile, lang string, progress *ProgressTracker) string {
	var parts []string
	var failedFiles []string

	for _, file := range files {
		content, err := idx.assetsService.Doc2Text(file.UID)
		if err != nil {
			log.Warnf("⚠ Sequential part failed: %s | %s | %s (%s) | %v", cu.UID, lang, file.UID, file.Name, err)
			failedFiles = append(failedFiles, file.Name)
			continue
		}
		if content != "" {
			parts = append(parts, content)
		}
	}

	if len(parts) == 0 {
		progress.RecordTranscript(true, false, fmt.Errorf("all %d sequential parts failed", len(files)))
		return ""
	}

	if len(failedFiles) > 0 {
		log.Warnf("⚠ Partial concatenation: %s | %s | succeeded: %d/%d | failed: %v", cu.UID, lang, len(parts), len(files), failedFiles)
	} else {
		log.Infof("✓ Concatenated %d parts: %s | %s", len(parts), cu.UID, lang)
	}

	progress.RecordTranscript(true, true, nil)
	return strings.Join(parts, "\n\n")
}

// buildFullContent creates a combined content field for LLM/RAG search
// This field contains all metadata and content in a structured format
func (idx *ContentUnitsIndexer) buildFullContent(doc *es.Result) string {
	var parts []string

	// Title
	if doc.Title != "" {
		parts = append(parts, fmt.Sprintf("Title: %s", doc.Title))
	}

	// Full hierarchical path
	if doc.FullTitle != "" {
		parts = append(parts, fmt.Sprintf("Full Path: %s", doc.FullTitle))
	}

	// Effective date
	if doc.EffectiveDate != nil {
		parts = append(parts, fmt.Sprintf("Date: %s", doc.EffectiveDate.Format("2006-01-02")))
	}

	// Content type
	if doc.ResultType != "" {
		parts = append(parts, fmt.Sprintf("Type: %s", doc.ResultType))
	}

	// Description
	if doc.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", doc.Description))
	}

	// Content separator
	if doc.Content != "" {
		parts = append(parts, "")
		parts = append(parts, doc.Content)
	}

	return strings.Join(parts, "\n")
}

// buildTitleSuggest creates the autocomplete suggest field
func (idx *ContentUnitsIndexer) buildTitleSuggest(cu *mdbmodels.ContentUnit, title string, indexData *es.IndexData) es.SuggestField {
	// Don't create suggestions for units with sources (ES6 logic)
	if sources, ok := indexData.Sources[cu.UID]; ok && len(sources) > 0 {
		return es.SuggestField{Input: []string{}, Weight: 0}
	}

	// Create word suffixes for autocomplete (reuse ES6 logic)
	suffixes := es.Suffixes(title)
	return es.SuggestField{
		Input:  suffixes,
		Weight: 1.0,
	}
}

// bulkIndexBySizeWithWorkerID batches tasks by request size (< 100MB) and sends bulk requests (LEGACY - NOT USED)
func (idx *ContentUnitsIndexer) bulkIndexBySizeWithWorkerID(ctx context.Context, tasks []IndexTask, workerID int, progress *ProgressTracker) error {
	if len(tasks) == 0 {
		return nil
	}

	const maxSizeMB = 90.0 // Keep under 100MB (90MB for safety margin)
	const maxSizeBytes = int(maxSizeMB * 1024 * 1024)

	currentBatch := make([]IndexTask, 0)
	currentSize := 0
	batchNum := 0

	for i, task := range tasks {
		// Estimate size for this document
		metaJSON, err := json.Marshal(map[string]interface{}{
			"index": map[string]interface{}{
				"_index": task.IndexName,
				"_id":    task.Doc.MDB_UID,
			},
		})
		if err != nil {
			return errors.Wrap(err, "marshal meta")
		}

		docJSON, err := json.Marshal(task.Doc)
		if err != nil {
			return errors.Wrap(err, "marshal document")
		}

		taskSize := len(metaJSON) + len(docJSON) + 2 // +2 for newlines

		// Check if adding this task would exceed the size limit
		if currentSize+taskSize > maxSizeBytes && len(currentBatch) > 0 {
			// Send current batch
			batchNum++
			if err := idx.sendBulkBatch(ctx, currentBatch, progress); err != nil {
				return err
			}

			// Start new batch
			currentBatch = make([]IndexTask, 0)
			currentSize = 0
		}

		// Add task to current batch
		currentBatch = append(currentBatch, task)
		currentSize += taskSize

		// Also send if this is the last task
		if i == len(tasks)-1 && len(currentBatch) > 0 {
			batchNum++
			if err := idx.sendBulkBatch(ctx, currentBatch, progress); err != nil {
				return err
			}
		}
	}

	return nil
}

// sendBulkBatch sends a single bulk request with mixed indices
func (idx *ContentUnitsIndexer) sendBulkBatch(ctx context.Context, tasks []IndexTask, progress *ProgressTracker) error {
	client, err := idx.manager.GetClient()
	if err != nil {
		return errors.Wrap(err, "get ES9 client")
	}

	// Build bulk request body (NDJSON format)
	var buf bytes.Buffer
	for _, task := range tasks {
		// Index action metadata
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": task.IndexName,
				"_id":    task.Doc.MDB_UID,
			},
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return errors.Wrap(err, "marshal meta")
		}

		// Document data
		docJSON, err := json.Marshal(task.Doc)
		if err != nil {
			return errors.Wrap(err, "marshal document")
		}

		buf.Write(metaJSON)
		buf.WriteByte('\n')
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	sizeBytes := buf.Len()

	// Execute bulk request
	res, err := client.Bulk(
		bytes.NewReader(buf.Bytes()),
		client.Bulk.WithContext(ctx),
		client.Bulk.WithRefresh("false"),
	)
	if err != nil {
		return errors.Wrap(err, "bulk request")
	}
	defer res.Body.Close()

	if res.IsError() {
		sizeMB := float64(sizeBytes) / 1024 / 1024
		return fmt.Errorf("bulk request failed (size: %.2f MB): %s", sizeMB, res.String())
	}

	// Parse response to check for errors
	var bulkResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&bulkResp); err != nil {
		return errors.Wrap(err, "decode bulk response")
	}

	docsIndexed := len(tasks)
	docsFailed := 0

	if errors, ok := bulkResp["errors"].(bool); ok && errors {
		if items, ok := bulkResp["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if indexResp, ok := itemMap["index"].(map[string]interface{}); ok {
						if status, ok := indexResp["status"].(float64); ok && status >= 400 {
							docsFailed++
							if docsFailed <= 3 {
								log.Warnf("⚠ Index error: %v", indexResp["error"])
							}
						}
					}
				}
			}
			if docsFailed > 0 {
				log.Warnf("⚠ Bulk request had %d errors out of %d documents", docsFailed, len(tasks))
			}
		}
	}

	// Record bulk request statistics
	progress.RecordBulkRequest(sizeBytes, docsIndexed-docsFailed, docsFailed)

	return nil
}

// bulkIndex performs bulk indexing to ES9 (legacy function, kept for compatibility)
func (idx *ContentUnitsIndexer) bulkIndex(ctx context.Context, indexName string, docs []*es.Result) error {
	client, err := idx.manager.GetClient()
	if err != nil {
		return errors.Wrap(err, "get ES9 client")
	}

	// Build bulk request body (NDJSON format)
	var buf bytes.Buffer
	for _, doc := range docs {
		// Index action metadata (no _type in ES9)
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": indexName,
				"_id":    doc.MDB_UID,
			},
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return errors.Wrap(err, "marshal meta")
		}

		// Document data
		docJSON, err := json.Marshal(doc)
		if err != nil {
			return errors.Wrap(err, "marshal document")
		}

		buf.Write(metaJSON)
		buf.WriteByte('\n')
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// Log bulk request size for debugging
	requestSize := buf.Len()
	requestSizeMB := float64(requestSize) / 1024 / 1024
	log.Debugf("Bulk indexing %d docs to %s (%.2f MB)", len(docs), indexName, requestSizeMB)

	// Execute bulk request
	res, err := client.Bulk(
		bytes.NewReader(buf.Bytes()),
		client.Bulk.WithContext(ctx),
		client.Bulk.WithIndex(indexName),
		client.Bulk.WithRefresh("false"), // Don't refresh immediately for performance
	)
	if err != nil {
		return errors.Wrap(err, "bulk request")
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk request failed (size: %.2f MB): %s", requestSizeMB, res.String())
	}

	// Parse response to check for errors
	var bulkResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&bulkResp); err != nil {
		return errors.Wrap(err, "decode bulk response")
	}

	if bulkResp["errors"].(bool) {
		// Log details about errors but don't fail completely
		items := bulkResp["items"].([]interface{})
		errorCount := 0
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			if indexResp, ok := itemMap["index"].(map[string]interface{}); ok {
				if indexResp["status"].(float64) >= 400 {
					errorCount++
					if errorCount <= 5 { // Log first 5 errors
						log.Warnf("Index error: %v", indexResp["error"])
					}
				}
			}
		}
		log.Warnf("Bulk indexing had %d errors out of %d documents", errorCount, len(docs))
	}

	return nil
}
