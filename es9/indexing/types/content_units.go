package types

import (
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
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
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

// Indexer interface implementation

// PrepareDocument implements the Indexer interface
// Wraps prepareDocumentForLanguage with interface-compatible signature
func (idx *ContentUnitsIndexer) PrepareDocument(
	ctx context.Context,
	item interface{},
	lang string,
	indexData *es.IndexData,
	indexDate *utils.Date,
	progress *indexing.ProgressTracker,
) (doc *es.Result, skip bool) {
	cu, ok := item.(*mdbmodels.ContentUnit)
	if !ok {
		log.Errorf("PrepareDocument: item is not *mdbmodels.ContentUnit: %T", item)
		return nil, true
	}
	return idx.prepareDocumentForLanguage(cu, lang, indexData, indexDate, progress)
}

// LoadRelationships implements the Indexer interface
// Wraps loadRelationshipsForBatch with interface-compatible signature
func (idx *ContentUnitsIndexer) LoadRelationships(ctx context.Context, items []interface{}) error {
	if len(items) == 0 {
		return nil
	}

	// Convert []interface{} to []*mdbmodels.ContentUnit
	contentUnits := make([]*mdbmodels.ContentUnit, len(items))
	for i, item := range items {
		cu, ok := item.(*mdbmodels.ContentUnit)
		if !ok {
			return fmt.Errorf("LoadRelationships: item %d is not *mdbmodels.ContentUnit: %T", i, item)
		}
		contentUnits[i] = cu
	}

	return idx.loadRelationshipsForBatch(ctx, contentUnits)
}

// GetIndexNameBase implements the Indexer interface
func (idx *ContentUnitsIndexer) GetIndexNameBase() string {
	return idx.indexNameBase
}

// GetLanguages implements the Indexer interface
func (idx *ContentUnitsIndexer) GetLanguages() []string {
	return consts.ALL_KNOWN_LANGS[:]
}

// GetDocumentType implements the Indexer interface
func (idx *ContentUnitsIndexer) GetDocumentType() string {
	return "content unit"
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

	// Convert []*mdbmodels.ContentUnit to []interface{} for generic pipeline
	items := make([]interface{}, len(contentUnits))
	for i, cu := range contentUnits {
		items[i] = cu
	}

	// Use generic pipeline for indexing
	pipeline := indexing.NewPipeline(idx.manager, nil) // nil = use default config
	if err := pipeline.RunPipeline(ctx, idx, items, allIndexData); err != nil {
		return errors.Wrap(err, "run indexing pipeline")
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
	// Always use "results" for mapping directory, regardless of actual index name
	mappingPath := fmt.Sprintf("es9/data/mappings/results/results-%s.json", lang)

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

// prepareDocumentForLanguage creates a fully prepared document for a specific language
func (idx *ContentUnitsIndexer) prepareDocumentForLanguage(
	cu *mdbmodels.ContentUnit,
	lang string,
	indexData *es.IndexData,
	indexDate *utils.Date,
	progress *indexing.ProgressTracker,
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
func (idx *ContentUnitsIndexer) extractContent(cu *mdbmodels.ContentUnit, indexData *es.IndexData, lang string, progress *indexing.ProgressTracker) string {
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
func (idx *ContentUnitsIndexer) extractWithFallback(cu *mdbmodels.ContentUnit, files []es.TranscriptFile, lang string, progress *indexing.ProgressTracker) string {
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
func (idx *ContentUnitsIndexer) extractAndConcatenate(cu *mdbmodels.ContentUnit, files []es.TranscriptFile, lang string, progress *indexing.ProgressTracker) string {
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
