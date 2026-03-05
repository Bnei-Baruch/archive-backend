package indexing

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
)

// TypeProgress tracks progress for a specific content type
type TypeProgress struct {
	totalItems     int64
	itemsProcessed int64
}

// ProgressTracker tracks indexing progress with atomic counters for thread-safe aggregation
// Generic tracker used by all indexer types (content units, collections, sources, etc.)
type ProgressTracker struct {
	// Item tracking (high-level progress)
	totalItems     int64
	itemsProcessed int64

	// Task tracking (low-level progress)
	tasksCreated   int64
	tasksQueued    int64
	tasksCompleted int64

	// Content tracking (files, transcripts, attachments, etc.)
	contentFound   int64
	contentFailed  int64
	contentEmpty   int64
	contentSkipped int64

	// Bulk request tracking
	bulkRequestsSent int64
	bulkRequestBytes int64
	documentsIndexed int64
	documentsFailed  int64

	// Per-language stats (protected by mutex)
	mu          sync.Mutex
	perLanguage map[string]int64
	perType     map[string]*TypeProgress
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(totalItems int) *ProgressTracker {
	return &ProgressTracker{
		totalItems:  int64(totalItems),
		perLanguage: make(map[string]int64),
		perType:     make(map[string]*TypeProgress),
	}
}

// SetTypeTotal sets the total items for a specific type
func (pt *ProgressTracker) SetTypeTotal(typeName string, total int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.perType[typeName] == nil {
		pt.perType[typeName] = &TypeProgress{}
	}
	atomic.StoreInt64(&pt.perType[typeName].totalItems, int64(total))
}

// RecordTypeItemProcessed increments the processed count for a specific type
func (pt *ProgressTracker) RecordTypeItemProcessed(typeName string) {
	pt.mu.Lock()
	if pt.perType[typeName] == nil {
		pt.perType[typeName] = &TypeProgress{}
	}
	tp := pt.perType[typeName]
	pt.mu.Unlock()

	atomic.AddInt64(&tp.itemsProcessed, 1)
}

// RecordItemProcessed increments items processed counter
func (pt *ProgressTracker) RecordItemProcessed() {
	atomic.AddInt64(&pt.itemsProcessed, 1)
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

// RecordContent records content load result (transcripts, files, attachments, etc.)
func (pt *ProgressTracker) RecordContent(found, success bool, err error) {
	if !found {
		atomic.AddInt64(&pt.contentSkipped, 1)
		return
	}
	atomic.AddInt64(&pt.contentFound, 1)
	if !success {
		if err != nil {
			atomic.AddInt64(&pt.contentFailed, 1)
		} else {
			atomic.AddInt64(&pt.contentEmpty, 1)
		}
	}
}

// RecordTranscript is an alias for RecordContent for backward compatibility with content units
func (pt *ProgressTracker) RecordTranscript(found, success bool, err error) {
	pt.RecordContent(found, success, err)
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

	itemsProcessed := atomic.LoadInt64(&pt.itemsProcessed)
	totalItems := atomic.LoadInt64(&pt.totalItems)
	itemsPercent := float64(0)
	if totalItems > 0 {
		itemsPercent = float64(itemsProcessed) / float64(totalItems) * 100
	}

	return map[string]interface{}{
		"total_items":       totalItems,
		"items_processed":   itemsProcessed,
		"items_percent":     itemsPercent,
		"tasks_created":     atomic.LoadInt64(&pt.tasksCreated),
		"tasks_queued":      atomic.LoadInt64(&pt.tasksQueued),
		"tasks_completed":   atomic.LoadInt64(&pt.tasksCompleted),
		"content_found":     atomic.LoadInt64(&pt.contentFound),
		"content_failed":    atomic.LoadInt64(&pt.contentFailed),
		"content_empty":     atomic.LoadInt64(&pt.contentEmpty),
		"content_skipped":   atomic.LoadInt64(&pt.contentSkipped),
		"bulk_requests":     atomic.LoadInt64(&pt.bulkRequestsSent),
		"bulk_avg_size_mb":  float64(avgSizeBytes) / 1024 / 1024,
		"bulk_total_size_gb": float64(atomic.LoadInt64(&pt.bulkRequestBytes)) / 1024 / 1024 / 1024,
		"docs_indexed":      atomic.LoadInt64(&pt.documentsIndexed),
		"docs_failed":       atomic.LoadInt64(&pt.documentsFailed),
		"per_language":      langStats,
	}
}

// PrintProgress prints aggregated progress to log
func (pt *ProgressTracker) PrintProgress(queueSize, queueCap int) {
	stats := pt.GetStats()
	queuePercent := float64(queueSize) / float64(queueCap) * 100

	log.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Print per-type progress in one row
	pt.mu.Lock()
	typeNames := make([]string, 0, len(pt.perType))
	for typeName := range pt.perType {
		typeNames = append(typeNames, typeName)
	}
	pt.mu.Unlock()

	if len(typeNames) > 0 {
		// Build progress line: Overall + per-type
		progressParts := []string{}

		// Overall
		totalProcessed := stats["items_processed"].(int64)
		totalItems := stats["total_items"].(int64)
		overallPercent := stats["items_percent"].(float64)
		progressParts = append(progressParts,
			fmt.Sprintf("Overall: %s/%s (%.0f%%)",
				formatCompact(totalProcessed),
				formatCompact(totalItems),
				overallPercent))

		// Per type
		for _, typeName := range typeNames {
			pt.mu.Lock()
			tp := pt.perType[typeName]
			processed := atomic.LoadInt64(&tp.itemsProcessed)
			total := atomic.LoadInt64(&tp.totalItems)
			pt.mu.Unlock()

			percent := float64(0)
			if total > 0 {
				percent = float64(processed) / float64(total) * 100
			}

			abbrev := abbreviateType(typeName)
			progressParts = append(progressParts,
				fmt.Sprintf("%s: %s/%s (%.0f%%)",
					abbrev,
					formatCompact(processed),
					formatCompact(total),
					percent))
		}

		log.Infof("PROGRESS: %s", strings.Join(progressParts, " | "))
	} else {
		// Fallback to simple progress
		log.Infof("ITEMS: %d / %d (%.1f%%) processed",
			stats["items_processed"], stats["total_items"], stats["items_percent"])
	}

	log.Infof("TASKS: Created=%d | Queued=%d | Completed=%d | Queue=%d (%.0f%% full)",
		stats["tasks_created"], stats["tasks_queued"], stats["tasks_completed"], queueSize, queuePercent)
	log.Infof("CONTENT: Found=%d | Failed=%d | Empty=%d | Skipped=%d",
		stats["content_found"], stats["content_failed"],
		stats["content_empty"], stats["content_skipped"])
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

// abbreviateType returns abbreviated type name for compact display
func abbreviateType(typeName string) string {
	abbrevs := map[string]string{
		"content-units": "cu",
		"collections":   "col",
		"sources":       "src",
		"tags":          "tag",
		"blog-posts":    "blog",
		"tweets":        "tw",
	}
	if abbrev, ok := abbrevs[typeName]; ok {
		return abbrev
	}
	return typeName
}

// formatCompact formats numbers compactly (K, M suffix)
func formatCompact(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 10000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else if n < 1000000 {
		return fmt.Sprintf("%dK", n/1000)
	} else if n < 10000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	return fmt.Sprintf("%dM", n/1000000)
}
