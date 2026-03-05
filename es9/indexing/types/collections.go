package types

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// CollectionsIndexer handles ES9 indexing for collections
type CollectionsIndexer struct {
	manager       *common.ES9Manager
	db            *sql.DB
	indexNameBase string // e.g., "collections"
}

// NewCollectionsIndexer creates a new collections indexer for ES9
func NewCollectionsIndexer(manager *common.ES9Manager, db *sql.DB, indexNameBase string) *CollectionsIndexer {
	return &CollectionsIndexer{
		manager:       manager,
		db:            db,
		indexNameBase: indexNameBase,
	}
}

// Indexer interface implementation

// PrepareDocument implements the Indexer interface
func (idx *CollectionsIndexer) PrepareDocument(
	ctx context.Context,
	item interface{},
	lang string,
	indexData *es.IndexData,
	indexDate *utils.Date,
	progress *indexing.ProgressTracker,
) (doc *es.Result, skip bool) {
	collection, ok := item.(*mdbmodels.Collection)
	if !ok {
		log.Errorf("PrepareDocument: item is not *mdbmodels.Collection: %T", item)
		return nil, true
	}
	return idx.prepareDocumentForLanguage(collection, lang, indexDate, progress)
}

// LoadRelationships implements the Indexer interface
func (idx *CollectionsIndexer) LoadRelationships(ctx context.Context, items []interface{}) error {
	if len(items) == 0 {
		return nil
	}

	// Convert []interface{} to []*mdbmodels.Collection
	collections := make([]*mdbmodels.Collection, len(items))
	for i, item := range items {
		c, ok := item.(*mdbmodels.Collection)
		if !ok {
			return fmt.Errorf("LoadRelationships: item %d is not *mdbmodels.Collection: %T", i, item)
		}
		collections[i] = c
	}

	return idx.loadRelationshipsForBatch(ctx, collections)
}

// GetIndexNameBase implements the Indexer interface
func (idx *CollectionsIndexer) GetIndexNameBase() string {
	return idx.indexNameBase
}

// GetLanguages implements the Indexer interface
func (idx *CollectionsIndexer) GetLanguages() []string {
	return consts.ALL_KNOWN_LANGS[:]
}

// GetDocumentType implements the Indexer interface
func (idx *CollectionsIndexer) GetDocumentType() string {
	return "collection"
}

// IndexAll indexes all collections from MDB to ES9 (all language indices)
func (idx *CollectionsIndexer) IndexAll(ctx context.Context, reset bool) error {
	startTime := time.Now()
	mode := "INCREMENTAL"
	if reset {
		mode = "RESET"
	}
	log.Infof("Starting ES9 collections indexing (%s mode)", mode)

	// Ensure all language indices exist
	if err := idx.ensureIndicesExist(ctx); err != nil {
		return errors.Wrap(err, "ensure indices exist")
	}

	// Handle reset mode: delete all collections from indices
	if reset {
		if err := idx.deleteExistingCollections(ctx); err != nil {
			return errors.Wrap(err, "delete existing collections")
		}
	}

	// Get all collections from MDB
	collections, err := idx.FetchCollections(ctx, DefaultCollectionsScope())
	if err != nil {
		return errors.Wrap(err, "fetch collections")
	}

	log.Infof("Found %d collections in MDB", len(collections))

	if len(collections) == 0 {
		log.Info("No collections to index")
		return nil
	}

	// Filter out already-indexed collections in incremental mode
	if !reset {
		collections, err = idx.FilterExistingCollections(ctx, collections)
		if err != nil {
			return errors.Wrap(err, "filter existing collections")
		}

		if len(collections) == 0 {
			log.Info("✓ All collections already indexed")
			return nil
		}

		log.Infof("Filtered: %d new collections to index", len(collections))
	}

	// Load index data (we don't need sources/tags for collections, they're in properties)
	indexData := &es.IndexData{}

	// Convert []*mdbmodels.Collection to []interface{}
	items := make([]interface{}, len(collections))
	for i, c := range collections {
		items[i] = c
	}

	// Use generic pipeline for indexing
	pipeline := indexing.NewPipeline(idx.manager, nil) // nil = use default config
	if err := pipeline.RunPipeline(ctx, idx, items, indexData); err != nil {
		return errors.Wrap(err, "run indexing pipeline")
	}

	log.Infof("✓ Collections indexing completed in %v", time.Since(startTime))
	return nil
}

// defaultCollectionsScope returns the default SQL scope for collections
// Excludes certain collection types per ES6 logic
func DefaultCollectionsScope() []qm.QueryMod {
	excludedTypeIDs := []int64{
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_DAILY_LESSON].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SPECIAL_LESSON].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONGS].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOKS].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_HOLIDAY].ID, // we use grammar for holiday collections
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_UNKNOWN].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SOURCE].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LIKUTIM].ID,
	}

	// Manually build NOT IN clause string (more reliable than SQLBoiler's WhereNotIn)
	excludedIDsStr := utils.JoinInt64(excludedTypeIDs, ",")
	notInClause := fmt.Sprintf("type_id NOT IN (%s)", excludedIDsStr)

	return []qm.QueryMod{
		qm.Where("secure = 0 AND published IS TRUE"),
		qm.Where(notInClause),
		qm.OrderBy("id"),
	}
}

// fetchCollections loads collections from MDB with the given scope
func (idx *CollectionsIndexer) FetchCollections(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.Collection, error) {
	collections, err := mdbmodels.Collections(scope...).All(idx.db)
	if err != nil {
		return nil, errors.Wrap(err, "query collections")
	}
	return collections, nil
}

// loadRelationshipsForBatch loads relationships for a batch of collections
func (idx *CollectionsIndexer) loadRelationshipsForBatch(ctx context.Context, collections []*mdbmodels.Collection) error {
	if len(collections) == 0 {
		return nil
	}

	log.Debugf("Loading relationships for batch of %d collections", len(collections))

	// Extract collection IDs
	collectionIDs := make([]interface{}, len(collections))
	for i, c := range collections {
		collectionIDs[i] = c.ID
	}

	// Load CollectionI18ns
	i18ns, err := mdbmodels.CollectionI18ns(
		qm.WhereIn("collection_id IN ?", collectionIDs...),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load collection_i18ns")
	}

	// Load CollectionsContentUnits with nested ContentUnit
	ccus, err := mdbmodels.CollectionsContentUnits(
		qm.WhereIn("collection_id IN ?", collectionIDs...),
		qm.Load("ContentUnit"),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load collections_content_units")
	}

	// Map relationships back to collections
	i18nMap := make(map[int64][]*mdbmodels.CollectionI18n)
	for _, i18n := range i18ns {
		i18nMap[i18n.CollectionID] = append(i18nMap[i18n.CollectionID], i18n)
	}

	ccuMap := make(map[int64][]*mdbmodels.CollectionsContentUnit)
	for _, ccu := range ccus {
		ccuMap[ccu.CollectionID] = append(ccuMap[ccu.CollectionID], ccu)
	}

	// Assign relationships to each collection
	for _, c := range collections {
		if c.R == nil {
			c.R = c.R.NewStruct()
		}

		if i18nList, ok := i18nMap[c.ID]; ok {
			c.R.CollectionI18ns = i18nList
		}

		if ccuList, ok := ccuMap[c.ID]; ok {
			c.R.CollectionsContentUnits = ccuList
		}
	}

	log.Debugf("✓ Loaded relationships: %d i18ns, %d collections_content_units",
		len(i18ns), len(ccus))

	return nil
}

// deleteExistingCollections deletes all collection documents from all language indices
func (idx *CollectionsIndexer) deleteExistingCollections(ctx context.Context) error {
	languages := consts.ALL_KNOWN_LANGS[:]
	log.Infof("Deleting existing collections from %d language indices", len(languages))

	var wg sync.WaitGroup
	errChan := make(chan error, len(languages))

	for _, lang := range languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

			// Delete by result type
			_, err := idx.manager.DeleteByResultType(ctx, indexName, consts.ES_RESULT_TYPE_COLLECTIONS)
			if err != nil {
				errChan <- errors.Wrapf(err, "delete collections from %s", indexName)
				return
			}

			log.Debugf("✓ Deleted collections from: %s", indexName)
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
		return fmt.Errorf("failed to delete from %d indices: %v", len(errs), errs[0])
	}

	log.Info("✓ Deleted existing collections from all indices")
	return nil
}

// filterExistingCollections filters out collections that are already indexed
func (idx *CollectionsIndexer) FilterExistingCollections(ctx context.Context, collections []*mdbmodels.Collection) ([]*mdbmodels.Collection, error) {
	if len(collections) == 0 {
		return collections, nil
	}

	// Use first language index to check existence
	indexName := fmt.Sprintf("%s_en", idx.indexNameBase)

	// Extract UIDs
	uids := make([]string, len(collections))
	for i, c := range collections {
		uids[i] = c.UID
	}

	// Check which UIDs already exist
	existingUIDs, err := idx.manager.GetExistingUIDs(ctx, indexName, consts.ES_RESULT_TYPE_COLLECTIONS)
	if err != nil {
		return nil, errors.Wrap(err, "check existing collections")
	}

	// Filter out existing
	newCollections := make([]*mdbmodels.Collection, 0)
	for _, c := range collections {
		if !existingUIDs[c.UID] {
			newCollections = append(newCollections, c)
		}
	}

	skipped := len(collections) - len(newCollections)
	log.Infof("Filtered: %d existing (skipped), %d new (will index)", skipped, len(newCollections))

	return newCollections, nil
}

// ensureIndicesExist creates all language indices if they don't exist
func (idx *CollectionsIndexer) ensureIndicesExist(ctx context.Context) error {
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
func (idx *CollectionsIndexer) loadMappingFile(lang string) (map[string]interface{}, error) {
	// Mapping file path: es9/data/mappings/results/results-{lang}.json
	// Always use "results" for mapping directory, regardless of actual index name
	// Collections share the same index structure as content units
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

// prepareDocumentForLanguage creates a document for a specific language
func (idx *CollectionsIndexer) prepareDocumentForLanguage(
	c *mdbmodels.Collection,
	lang string,
	indexDate *utils.Date,
	progress *indexing.ProgressTracker,
) (*es.Result, bool) {
	// Find i18n for this language
	var i18n *mdbmodels.CollectionI18n
	if c.R != nil {
		for _, ci := range c.R.CollectionI18ns {
			if ci.Language == lang {
				i18n = ci
				break
			}
		}
	}

	// Skip if no translation for this language
	if i18n == nil || !i18n.Name.Valid || i18n.Name.String == "" {
		return nil, true // skip
	}

	// Calculate effective date from content units
	effectiveDate := idx.calculateEffectiveDate(c)

	// Get source and tags from properties
	src, tags := idx.extractSourceAndTags(c)

	// Build typed UIDs
	typedUIDs := []string{es.KeyValue(consts.ES_UID_TYPE_COLLECTION, c.UID)}

	// Add content unit UIDs
	if c.R != nil {
		for _, ccu := range c.R.CollectionsContentUnits {
			if ccu.R != nil && ccu.R.ContentUnit != nil {
				typedUIDs = append(typedUIDs, es.KeyValue(consts.ES_UID_TYPE_CONTENT_UNIT, ccu.R.ContentUnit.UID))
			}
		}
	}

	// Add program collection mapping if exists
	if programCollectionUID, ok := consts.ARTICLE_COLLECTION_TO_PROGRAM_COLLECTION[c.UID]; ok {
		typedUIDs = append(typedUIDs, es.KeyValue(consts.ES_UID_TYPE_COLLECTION, programCollectionUID))
	}

	// Add source
	if src != "" {
		typedUIDs = append(typedUIDs, es.KeyValue(consts.ES_UID_TYPE_SOURCE, src))
	}

	// Add tags
	for _, tag := range tags {
		typedUIDs = append(typedUIDs, es.KeyValue(consts.ES_UID_TYPE_TAG, tag))
	}

	// Build filter values
	filterValues := []string{
		es.KeyValue("collections_content_type", mdb.CONTENT_TYPE_REGISTRY.ByID[c.TypeID].Name),
	}

	// Add content types from content units
	contentTypes := idx.extractContentTypes(c)
	for _, ct := range contentTypes {
		filterValues = append(filterValues, es.KeyValue("content_type", ct))
	}

	// Create Result document
	doc := &es.Result{
		ResultType:   consts.ES_RESULT_TYPE_COLLECTIONS,
		IndexDate:    indexDate,
		MDB_UID:      c.UID,
		TypedUids:    typedUIDs,
		FilterValues: filterValues,
		Title:        i18n.Name.String,
		TitleSuggest: es.SuggestField{
			Input:  es.Suffixes(i18n.Name.String),
			Weight: float64(consts.ES_COLLECTIONS_SUGGEST_DEFAULT_WEIGHT),
		},
	}

	// Set effective date
	if effectiveDate != nil {
		doc.EffectiveDate = effectiveDate
	}

	// Set description
	if i18n.Description.Valid && i18n.Description.String != "" {
		doc.Description = i18n.Description.String
	}

	return doc, false // not skipped
}

// calculateEffectiveDate returns the latest film_date from any content unit
func (idx *CollectionsIndexer) calculateEffectiveDate(c *mdbmodels.Collection) *utils.Date {
	var effectiveDate *utils.Date

	if c.R == nil {
		return nil
	}

	for _, ccu := range c.R.CollectionsContentUnits {
		if ccu.R == nil || ccu.R.ContentUnit == nil {
			continue
		}

		cu := ccu.R.ContentUnit
		if !cu.Properties.Valid {
			continue
		}

		var props map[string]interface{}
		if err := json.Unmarshal(cu.Properties.JSON, &props); err != nil {
			continue
		}

		if filmDate, ok := props["film_date"]; ok {
			dateStr := strings.Split(filmDate.(string), "T")[0] // remove time part
			val, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}

			if effectiveDate == nil || effectiveDate.Time.Before(val) {
				effectiveDate = &utils.Date{Time: val}
			}
		}
	}

	return effectiveDate
}

// extractSourceAndTags extracts source and tags from collection properties
func (idx *CollectionsIndexer) extractSourceAndTags(c *mdbmodels.Collection) (string, []string) {
	src := ""
	tags := []string{}

	if !c.Properties.Valid {
		return src, tags
	}

	var props map[string]interface{}
	if err := json.Unmarshal(c.Properties.JSON, &props); err != nil {
		return src, tags
	}

	// Extract source
	if s, ok := props[consts.ES_UID_TYPE_SOURCE]; ok {
		src = s.(string)
	}

	// Extract tags
	if t, ok := props[consts.ES_UID_TYPE_TAGS]; ok {
		itags := t.([]interface{})
		for _, tagName := range itags {
			tags = append(tags, tagName.(string))
		}
	}

	return src, tags
}

// extractContentTypes extracts unique content types from content units
func (idx *CollectionsIndexer) extractContentTypes(c *mdbmodels.Collection) []string {
	if c.R == nil {
		return []string{}
	}

	typeMap := make(map[string]bool)
	for _, ccu := range c.R.CollectionsContentUnits {
		if ccu.R != nil && ccu.R.ContentUnit != nil {
			cu := ccu.R.ContentUnit
			typeName := mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name
			typeMap[typeName] = true
		}
	}

	types := make([]string, 0, len(typeMap))
	for t := range typeMap {
		types = append(types, t)
	}

	return types
}
