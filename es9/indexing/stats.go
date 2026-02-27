package indexing

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

// IndexingStats tracks statistics during indexing
type IndexingStats struct {
	StartTime time.Time

	// Per-batch counters
	BatchesProcessed int
	TotalBatches     int

	// Per-unit counters
	UnitsProcessed int
	TotalUnits     int

	// Per-language counters
	DocsIndexed map[string]int // lang -> count
	DocsSkipped map[string]int // lang -> count (no translation)
	DocsErrors  map[string]int // lang -> count

	// Content extraction stats
	TranscriptsFound     int
	TranscriptsExtracted int
	TranscriptsErrors    int

	// Error samples (first 5 of each type)
	IndexingErrors   []error // Bulk indexing errors
	TranscriptErrors []error // Transcript extraction errors
	maxErrorSamples  int

	// Mutex for concurrent updates
	mu sync.Mutex
}

// NewIndexingStats creates a new statistics tracker
func NewIndexingStats(totalUnits int, batchSize int) *IndexingStats {
	return &IndexingStats{
		StartTime:       time.Now(),
		TotalUnits:      totalUnits,
		TotalBatches:    (totalUnits + batchSize - 1) / batchSize,
		DocsIndexed:     make(map[string]int),
		DocsSkipped:     make(map[string]int),
		DocsErrors:      make(map[string]int),
		maxErrorSamples: 5,
	}
}

// RecordBatch records completion of a batch
func (s *IndexingStats) RecordBatch(unitsInBatch int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.BatchesProcessed++
	s.UnitsProcessed += unitsInBatch
}

// RecordIndexed records successful document indexing
func (s *IndexingStats) RecordIndexed(lang string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.DocsIndexed[lang] += count
}

// RecordSkipped records skipped documents (no translation)
func (s *IndexingStats) RecordSkipped(lang string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.DocsSkipped[lang]++
}

// RecordIndexingError records a bulk indexing error (keeps first 5)
func (s *IndexingStats) RecordIndexingError(lang string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.DocsErrors[lang]++

	if len(s.IndexingErrors) < s.maxErrorSamples {
		s.IndexingErrors = append(s.IndexingErrors, fmt.Errorf("[%s] %w", lang, err))
	}
}

// RecordTranscript records transcript extraction
func (s *IndexingStats) RecordTranscript(found bool, extracted bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if found {
		s.TranscriptsFound++
		if extracted {
			s.TranscriptsExtracted++
		} else {
			s.TranscriptsErrors++
			if err != nil && len(s.TranscriptErrors) < s.maxErrorSamples {
				s.TranscriptErrors = append(s.TranscriptErrors, err)
			}
		}
	}
}

// PrintBatchProgress prints progress after a batch
func (s *IndexingStats) PrintBatchProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()

	elapsed := time.Since(s.StartTime)
	unitsPerSec := float64(s.UnitsProcessed) / elapsed.Seconds()

	// Calculate ETA
	remainingUnits := s.TotalUnits - s.UnitsProcessed
	var eta string
	if unitsPerSec > 0 {
		etaSeconds := float64(remainingUnits) / unitsPerSec
		eta = fmt.Sprintf("ETA: %s", time.Duration(etaSeconds*float64(time.Second)).Round(time.Second))
	} else {
		eta = "ETA: calculating..."
	}

	// Per-language summary (show languages with docs)
	type langCount struct {
		lang  string
		count int
	}
	var langCounts []langCount
	for lang, count := range s.DocsIndexed {
		if count > 0 {
			langCounts = append(langCounts, langCount{lang, count})
		}
	}

	// Show all languages with documents
	var langSummaries []string
	for _, lc := range langCounts {
		langSummaries = append(langSummaries, fmt.Sprintf("%s=%d", lc.lang, lc.count))
	}

	totalDocs := s.totalIndexed()
	log.Infof("Progress: batch %d/%d | units %d/%d (%.1f%%) | docs: %d | speed: %.1f units/sec | %s",
		s.BatchesProcessed,
		s.TotalBatches,
		s.UnitsProcessed,
		s.TotalUnits,
		100.0*float64(s.UnitsProcessed)/float64(s.TotalUnits),
		totalDocs,
		unitsPerSec,
		eta)

	if len(langSummaries) > 0 {
		log.Infof("  Languages: %s", strings.Join(langSummaries, ", "))
	}
}

// PrintFinalSummary prints comprehensive final statistics
func (s *IndexingStats) PrintFinalSummary() {
	s.mu.Lock()
	defer s.mu.Unlock()

	elapsed := time.Since(s.StartTime)

	log.Info("================================================================================")
	log.Info("  Indexing Complete - Final Statistics")
	log.Info("================================================================================")
	log.Infof("Total Time:       %s", elapsed.Round(time.Second))
	log.Infof("Units Processed:  %d", s.UnitsProcessed)
	log.Infof("Batches:          %d", s.BatchesProcessed)
	log.Infof("Average Speed:    %.1f units/sec", float64(s.UnitsProcessed)/elapsed.Seconds())
	log.Info("")

	// Per-language breakdown
	log.Info("Documents per Language:")
	totalIndexed := 0
	totalSkipped := 0
	totalErrors := 0

	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexed := s.DocsIndexed[lang]
		skipped := s.DocsSkipped[lang]
		errors := s.DocsErrors[lang]

		if indexed > 0 || skipped > 0 || errors > 0 {
			log.Infof("  %-3s: indexed=%d, skipped=%d, errors=%d",
				lang, indexed, skipped, errors)
			totalIndexed += indexed
			totalSkipped += skipped
			totalErrors += errors
		}
	}

	log.Info("")
	log.Infof("Total Documents:  indexed=%d, skipped=%d, errors=%d",
		totalIndexed, totalSkipped, totalErrors)
	log.Info("")

	// Transcript extraction stats
	if s.TranscriptsFound > 0 {
		log.Info("Transcript Extraction:")
		log.Infof("  Found:      %d", s.TranscriptsFound)
		log.Infof("  Extracted:  %d (%.1f%%)", s.TranscriptsExtracted, 100.0*float64(s.TranscriptsExtracted)/float64(s.TranscriptsFound))
		log.Infof("  Errors:     %d (%.1f%%)", s.TranscriptsErrors, 100.0*float64(s.TranscriptsErrors)/float64(s.TranscriptsFound))

		// Print sample transcript errors
		if len(s.TranscriptErrors) > 0 {
			log.Info("  Sample Errors:")
			for i, err := range s.TranscriptErrors {
				log.Infof("    %d. %v", i+1, err)
			}
		}
		log.Info("")
	}

	// Print sample indexing errors
	if len(s.IndexingErrors) > 0 {
		log.Info("Indexing Errors (sample):")
		for i, err := range s.IndexingErrors {
			log.Infof("  %d. %v", i+1, err)
		}
		log.Info("")
	}

	log.Info("================================================================================")
}

// totalIndexed returns total documents indexed across all languages
func (s *IndexingStats) totalIndexed() int {
	total := 0
	for _, count := range s.DocsIndexed {
		total += count
	}
	return total
}
