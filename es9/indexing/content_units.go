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

// ContentUnitsIndexer handles ES9 indexing for content units
type ContentUnitsIndexer struct {
	manager       *common.ES9Manager
	db            *sql.DB
	indexNameBase string // e.g., "results"
	assetsService integration.AssetsService
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
func (idx *ContentUnitsIndexer) IndexAll(ctx context.Context) error {
	log.Info("Starting content units indexing to ES9")

	// Ensure all language indices exist (create if needed)
	if err := idx.ensureIndicesExist(ctx); err != nil {
		return errors.Wrap(err, "ensure indices exist")
	}

	// Get all content units from MDB with default filtering
	// Loading 200K units into memory is fine (~2GB max)
	contentUnits, err := idx.fetchContentUnits(ctx, defaultContentUnitScope())
	if err != nil {
		return errors.Wrap(err, "fetch content units")
	}

	log.Infof("Found %d content units to index", len(contentUnits))

	if len(contentUnits) == 0 {
		log.Info("No content units to index")
		return nil
	}

	// Index in batches to avoid SQL IN clause length limits
	// Problem: SQL query "cu.uid IN ('uid1', 'uid2', ..., 'uid200000')" is too long
	// Solution: Process in batches of 5000 UIDs
	return idx.indexInBatches(ctx, contentUnits)
}

// defaultContentUnitScope returns the default SQL scope for content units
// Matches ES6 logic: only published, public units, excluding certain types
func defaultContentUnitScope() []qm.QueryMod {
	// Exclude certain content types
	excludedTypes := []interface{}{
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LELO_MIKUD].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_PUBLICATION].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONG].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOK].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BLOG_POST].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_RESEARCH_MATERIAL].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KTAIM_NIVCHARIM].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_ARTICLE].ID,
	}

	return []qm.QueryMod{
		qm.Where("secure = 0"),
		qm.Where("published IS TRUE"),
		qm.WhereNotIn("type_id IN ?", excludedTypes...),
		qm.Load("ContentUnitI18ns"),
		qm.Load("CollectionsContentUnits.Collection"),
		qm.Load("CollectionsContentUnits.Collection.CollectionI18ns"),
		qm.Load("ContentUnitsPersons.Person"),
	}
}

// fetchContentUnits loads content units from MDB
func (idx *ContentUnitsIndexer) fetchContentUnits(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.ContentUnit, error) {
	return mdbmodels.ContentUnits(scope...).All(idx.db)
}

// loadIndexData fetches supplementary data for indexing (sources, tags, etc.)
func (idx *ContentUnitsIndexer) loadIndexData(ctx context.Context, contentUnits []*mdbmodels.ContentUnit) (*es.IndexData, error) {
	log.Info("Loading index data (sources, tags, transcripts, media languages)")

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

	log.Infof("Loaded index data: %d source relations, %d tag relations, %d media language sets, %d transcript sets",
		len(indexData.Sources),
		len(indexData.Tags),
		len(indexData.MediaLanguages),
		len(indexData.Transcripts))

	return indexData, nil
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

// indexInBatches processes content units in batches to avoid SQL IN clause length limits
func (idx *ContentUnitsIndexer) indexInBatches(ctx context.Context, contentUnits []*mdbmodels.ContentUnit) error {
	const batchSize = 5000 // Process 5000 units at a time to keep SQL IN clauses manageable
	total := len(contentUnits)

	log.Infof("Indexing %d content units in batches of %d", total, batchSize)

	// Initialize statistics tracker
	stats := NewIndexingStats(total, batchSize)

	for offset := 0; offset < total; offset += batchSize {
		end := offset + batchSize
		if end > total {
			end = total
		}

		batch := contentUnits[offset:end]

		// Load supplementary data (sources, tags, media languages, transcripts) for this batch only
		// This keeps the SQL IN clause to max 5000 UIDs instead of 200K
		indexData, err := idx.loadIndexData(ctx, batch)
		if err != nil {
			return errors.Wrapf(err, "load index data for batch %d/%d", stats.BatchesProcessed+1, stats.TotalBatches)
		}

		// Index this batch to all languages
		if err := idx.indexToAllLanguages(ctx, batch, indexData, stats); err != nil {
			return errors.Wrapf(err, "index batch %d/%d to all languages", stats.BatchesProcessed+1, stats.TotalBatches)
		}

		// Record batch completion
		stats.RecordBatch(len(batch))

		// Print progress
		stats.PrintBatchProgress()
	}

	// Print final summary
	stats.PrintFinalSummary()

	return nil
}

// indexToAllLanguages indexes content units to all language indices in parallel
func (idx *ContentUnitsIndexer) indexToAllLanguages(ctx context.Context, contentUnits []*mdbmodels.ContentUnit, indexData *es.IndexData, stats *IndexingStats) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(consts.ALL_KNOWN_LANGS))

	for _, lang := range consts.ALL_KNOWN_LANGS {
		wg.Add(1)
		go func(language string) {
			defer wg.Done()

			if err := idx.indexToLanguage(ctx, language, contentUnits, indexData, stats); err != nil {
				log.Errorf("Failed to index language %s: %v", language, err)
				stats.RecordIndexingError(language, err)
				errChan <- fmt.Errorf("language %s: %w", language, err)
			}
		}(lang)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("indexing failed for %d languages: %v", len(errs), errs)
	}

	return nil
}

// indexToLanguage indexes content units to a specific language index
func (idx *ContentUnitsIndexer) indexToLanguage(ctx context.Context, lang string, contentUnits []*mdbmodels.ContentUnit, indexData *es.IndexData, stats *IndexingStats) error {
	// Build index name: results_en, results_he, etc.
	indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

	// Prepare documents for this language
	docs, skipped, err := idx.prepareDocuments(lang, contentUnits, indexData, stats)
	if err != nil {
		return errors.Wrap(err, "prepare documents")
	}

	// Record skipped documents
	for i := 0; i < skipped; i++ {
		stats.RecordSkipped(lang)
	}

	if len(docs) == 0 {
		return nil
	}

	// Bulk index documents
	if err := idx.bulkIndex(ctx, indexName, docs); err != nil {
		return errors.Wrap(err, "bulk index")
	}

	// Record successfully indexed documents
	stats.RecordIndexed(lang, len(docs))

	return nil
}

// prepareDocuments creates Result documents for a specific language
// Returns: docs, skippedCount, error
func (idx *ContentUnitsIndexer) prepareDocuments(lang string, contentUnits []*mdbmodels.ContentUnit, indexData *es.IndexData, stats *IndexingStats) ([]*es.Result, int, error) {
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
		doc.TypedUids = idx.buildTypedUids(cu, indexData)

		// Build filter values
		doc.FilterValues = idx.buildFilterValues(cu, indexData)

		// Extract content from transcript (if available)
		doc.Content = idx.extractContent(cu, indexData, lang, stats)

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
func (idx *ContentUnitsIndexer) buildTypedUids(cu *mdbmodels.ContentUnit, indexData *es.IndexData) []string {
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

	// Add transcript file UIDs if available
	// Transcripts is map[cuUID]map[language][]fileUIDs
	if transcriptLangs, ok := indexData.Transcripts[cu.UID]; ok {
		for _, fileUIDs := range transcriptLangs {
			for _, fileUID := range fileUIDs {
				uids = append(uids, es.KeyValue(consts.ES_UID_TYPE_FILE, fileUID))
			}
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
func (idx *ContentUnitsIndexer) extractContent(cu *mdbmodels.ContentUnit, indexData *es.IndexData, lang string, stats *IndexingStats) string {
	// Get transcript file UIDs for this content unit
	transcripts, ok := indexData.Transcripts[cu.UID]
	if !ok || len(transcripts) == 0 {
		stats.RecordTranscript(false, false, nil)
		return ""
	}

	// Get transcripts for this specific language
	langTranscripts, ok := transcripts[lang]
	if !ok || len(langTranscripts) == 0 {
		stats.RecordTranscript(false, false, nil)
		return ""
	}

	// Use the first transcript file UID
	fileUID := langTranscripts[0]

	// Convert DOCX to text using assets service
	content, err := idx.assetsService.Doc2Text(fileUID)
	if err != nil {
		log.Warnf("Content Units Index - Error converting transcript to text: %s (file: %s): %v", cu.UID, fileUID, err)
		stats.RecordTranscript(true, false, err)
		return ""
	}

	if content == "" {
		log.Warnf("Content Units Index - Transcript empty: %s (file: %s)", cu.UID, fileUID)
		stats.RecordTranscript(true, false, fmt.Errorf("empty transcript for %s (file: %s)", cu.UID, fileUID))
		return ""
	}

	stats.RecordTranscript(true, true, nil)
	return content
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

// bulkIndex performs bulk indexing to ES9
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
		return fmt.Errorf("bulk request failed: %s", res.String())
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

	log.Debugf("Successfully bulk indexed %d documents to %s", len(docs), indexName)
	return nil
}
