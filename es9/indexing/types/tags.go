package types

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// TagHierarchyNode stores tag hierarchy information
type TagHierarchyNode struct {
	UID       string
	ParentUID string
	I18ns     map[string]string // language -> label
}

// TagsIndexer implements the Indexer interface for tags
type TagsIndexer struct {
	manager       *common.ES9Manager
	db            *sql.DB
	indexNameBase string
	hierarchy     map[string]*TagHierarchyNode // UID -> node (loaded once, reused for all items)
	hierarchyMu   sync.RWMutex
}

// NewTagsIndexer creates a new tags indexer
func NewTagsIndexer(mgr *common.ES9Manager, db *sql.DB, indexNameBase string) *TagsIndexer {
	if indexNameBase == "" {
		indexNameBase = "results" // Tags share the same index as other result types
	}
	return &TagsIndexer{
		manager:       mgr,
		db:            db,
		indexNameBase: indexNameBase,
	}
}

// GetIndexNameBase returns the base name for indices
func (idx *TagsIndexer) GetIndexNameBase() string {
	return idx.indexNameBase
}

// GetLanguages returns all known languages
func (idx *TagsIndexer) GetLanguages() []string {
	return consts.ALL_KNOWN_LANGS[:]
}

// GetDocumentType returns the document type name for logging
func (idx *TagsIndexer) GetDocumentType() string {
	return "tag"
}

// PrepareDocument converts a tag to an ES9 Result document
// For tags, this is called once per tag, and we generate documents for all languages where i18n exists
func (idx *TagsIndexer) PrepareDocument(ctx context.Context, item interface{}, lang string, indexData *es.IndexData, indexDate *utils.Date, progress *indexing.ProgressTracker) (doc *es.Result, skip bool) {
	tag, ok := item.(*mdbmodels.Tag)
	if !ok {
		log.Errorf("Invalid item type for tags indexer: %T", item)
		return nil, true
	}

	// Skip root tags (no parent)
	if !tag.ParentID.Valid {
		return nil, true
	}

	// Check if we have relationships loaded
	if tag.R == nil || tag.R.TagI18ns == nil {
		log.Errorf("Tag %s missing i18n relationships", tag.UID)
		return nil, true
	}

	// Find i18n for requested language
	var i18n *mdbmodels.TagI18n
	for _, ti18n := range tag.R.TagI18ns {
		if ti18n.Language == lang {
			i18n = ti18n
			break
		}
	}

	// Skip if no i18n for this language or label is empty
	if i18n == nil || !i18n.Label.Valid || strings.TrimSpace(i18n.Label.String) == "" {
		return nil, true
	}

	// Build path names and parent UIDs by traversing up the cached hierarchy
	pathNames := []string{i18n.Label.String}
	parentUids := []string{tag.UID}

	// Walk up the hierarchy using cached data
	idx.hierarchyMu.RLock()
	currentUID := tag.UID
	for {
		node, exists := idx.hierarchy[currentUID]
		if !exists || node.ParentUID == "" {
			break // Reached root or missing node
		}

		// Get parent node
		parentNode, exists := idx.hierarchy[node.ParentUID]
		if !exists {
			log.Warnf("Parent node %s not found in hierarchy for tag %s", node.ParentUID, tag.UID)
			break
		}

		// Get parent label for this language
		parentLabel, hasLabel := parentNode.I18ns[lang]
		if !hasLabel || parentLabel == "" {
			// No label for this language in parent, stop here
			break
		}

		// Prepend parent to paths
		pathNames = append([]string{parentLabel}, pathNames...)
		parentUids = append([]string{parentNode.UID}, parentUids...)

		// Move to parent
		currentUID = node.ParentUID
	}
	idx.hierarchyMu.RUnlock()

	// Need at least 2 levels (tag + parent) for a valid path
	if len(parentUids) < 2 {
		return nil, true
	}

	// Create document
	doc = &es.Result{
		ResultType:   consts.ES_RESULT_TYPE_TAGS,
		IndexDate:    indexDate,
		MDB_UID:      tag.UID,
		FilterValues: es.KeyValues(consts.ES_UID_TYPE_TAG, parentUids),
		TypedUids:    []string{es.KeyValue(consts.ES_UID_TYPE_TAG, tag.UID)},
		Title:        strings.Join(pathNames, " - "),
		TitleSuggest: es.SuggestField{Input: es.Suffixes(strings.Join(pathNames, " ")), Weight: 1},
	}

	return doc, false
}

// loadTagsHierarchy loads the entire tags hierarchy with i18n in one SQL query
func (idx *TagsIndexer) loadTagsHierarchy(ctx context.Context) error {
	idx.hierarchyMu.Lock()
	defer idx.hierarchyMu.Unlock()

	// Already loaded
	if idx.hierarchy != nil {
		return nil
	}

	log.Debug("Loading complete tags hierarchy with i18n...")

	// Recursive SQL to get all tags with their parent UIDs and i18n labels
	rows, err := queries.Raw(`
		WITH RECURSIVE rec_tags AS (
			SELECT
				t.id,
				t.uid,
				t.parent_id,
				NULL::VARCHAR as parent_uid
			FROM tags t
			WHERE parent_id IS NULL
			UNION
			SELECT
				t.id,
				t.uid,
				t.parent_id,
				rt.uid as parent_uid
			FROM tags t
			INNER JOIN rec_tags rt ON t.parent_id = rt.id
		)
		SELECT
			rt.uid,
			rt.parent_uid,
			ti.language,
			ti.label
		FROM rec_tags rt
		LEFT JOIN tag_i18n ti ON ti.tag_id = rt.id
		WHERE ti.label IS NOT NULL AND ti.label != ''
		ORDER BY rt.uid, ti.language
	`).Query(idx.db)
	if err != nil {
		return errors.Wrap(err, "load tags hierarchy")
	}
	defer rows.Close()

	hierarchy := make(map[string]*TagHierarchyNode)

	for rows.Next() {
		var uid, language, label string
		var parentUID sql.NullString
		if err := rows.Scan(&uid, &parentUID, &language, &label); err != nil {
			return errors.Wrap(err, "scan hierarchy row")
		}

		// Get or create node
		node, exists := hierarchy[uid]
		if !exists {
			node = &TagHierarchyNode{
				UID:   uid,
				I18ns: make(map[string]string),
			}
			hierarchy[uid] = node
		}

		// Set parent UID
		if parentUID.Valid {
			node.ParentUID = parentUID.String
		}

		// Add i18n
		node.I18ns[language] = label
	}

	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "iterate hierarchy rows")
	}

	idx.hierarchy = hierarchy
	log.Debugf("✓ Loaded tags hierarchy: %d tags", len(hierarchy))

	return nil
}

// LoadRelationships loads TagI18ns for a batch of tags
func (idx *TagsIndexer) LoadRelationships(ctx context.Context, items []interface{}) error {
	if len(items) == 0 {
		return nil
	}

	// Load complete hierarchy once (cached for subsequent calls)
	if err := idx.loadTagsHierarchy(ctx); err != nil {
		return errors.Wrap(err, "load tags hierarchy")
	}

	log.Debugf("Loading relationships for batch of %d tags", len(items))

	// Extract tag IDs
	tagIDs := make([]interface{}, len(items))
	for i, item := range items {
		tag, ok := item.(*mdbmodels.Tag)
		if !ok {
			return errors.Errorf("invalid item type: %T", item)
		}
		tagIDs[i] = tag.ID
	}

	// Load TagI18ns
	i18ns, err := mdbmodels.TagI18ns(
		qm.WhereIn("tag_id IN ?", tagIDs...),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load tag_i18ns")
	}

	// Map relationships back to tags
	i18nMap := make(map[int64][]*mdbmodels.TagI18n)
	for _, i18n := range i18ns {
		i18nMap[i18n.TagID] = append(i18nMap[i18n.TagID], i18n)
	}

	// Assign relationships to each tag
	for _, item := range items {
		tag, ok := item.(*mdbmodels.Tag)
		if !ok {
			continue
		}

		if tag.R == nil {
			tag.R = tag.R.NewStruct()
		}

		if i18nList, ok := i18nMap[tag.ID]; ok {
			tag.R.TagI18ns = i18nList
		}
	}

	log.Debugf("✓ Loaded relationships: %d i18ns", len(i18ns))

	return nil
}

// IndexAll indexes all tags
func (idx *TagsIndexer) IndexAll(ctx context.Context, reset bool) error {
	startTime := time.Now()
	log.Infof("Starting tags indexing (reset=%v)", reset)

	// Delete existing tags if reset
	if reset {
		if err := idx.deleteExistingTags(ctx); err != nil {
			return errors.Wrap(err, "delete existing tags")
		}
	}

	// Fetch all tags
	scope := DefaultTagsScope()
	tags, err := idx.FetchTags(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "fetch tags")
	}

	log.Infof("Fetched %d tags from MDB", len(tags))

	if len(tags) == 0 {
		log.Info("No tags to index")
		return nil
	}

	// Filter existing tags if not reset
	if !reset {
		tags, err = idx.FilterExistingTags(ctx, tags)
		if err != nil {
			return errors.Wrap(err, "filter existing tags")
		}

		if len(tags) == 0 {
			log.Info("✓ All tags already indexed")
			return nil
		}

		log.Infof("Filtered: %d new tags to index", len(tags))
	}

	// Load index data (not needed for tags)
	indexData := &es.IndexData{}

	// Convert []*mdbmodels.Tag to []interface{}
	items := make([]interface{}, len(tags))
	for i, t := range tags {
		items[i] = t
	}

	// Use generic pipeline for indexing
	pipeline := indexing.NewPipeline(idx.manager, nil) // nil = use default config
	if err := pipeline.RunPipeline(ctx, idx, items, indexData); err != nil {
		return errors.Wrap(err, "run indexing pipeline")
	}

	log.Infof("✓ Tags indexing completed in %v", time.Since(startTime))
	return nil
}

// DefaultTagsScope returns the default SQL scope for tags (all tags)
func DefaultTagsScope() []qm.QueryMod {
	return []qm.QueryMod{
		qm.Where("TRUE"),
		qm.OrderBy("id"),
	}
}

// FetchTags loads tags from MDB with the given scope
func (idx *TagsIndexer) FetchTags(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.Tag, error) {
	tags, err := mdbmodels.Tags(scope...).All(idx.db)
	if err != nil {
		return nil, errors.Wrap(err, "query tags")
	}
	return tags, nil
}

// deleteExistingTags deletes all tag documents from all language indices
func (idx *TagsIndexer) deleteExistingTags(ctx context.Context) error {
	languages := consts.ALL_KNOWN_LANGS[:]
	log.Infof("Deleting existing tags from %d language indices", len(languages))

	var wg sync.WaitGroup
	errChan := make(chan error, len(languages))

	for _, lang := range languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

			// Delete by result type
			_, err := idx.manager.DeleteByResultType(ctx, indexName, consts.ES_RESULT_TYPE_TAGS)
			if err != nil {
				errChan <- errors.Wrapf(err, "delete tags from %s", indexName)
				return
			}

			log.Debugf("✓ Deleted tags from: %s", indexName)
		}(lang)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return <-errChan
	}

	log.Info("✓ Deleted existing tags from all language indices")
	return nil
}

// FilterExistingTags filters out tags that are already indexed
func (idx *TagsIndexer) FilterExistingTags(ctx context.Context, tags []*mdbmodels.Tag) ([]*mdbmodels.Tag, error) {
	if len(tags) == 0 {
		return tags, nil
	}

	// Build UID map
	uidMap := make(map[string]*mdbmodels.Tag)
	for _, tag := range tags {
		uidMap[tag.UID] = tag
	}

	// Check all language indices (tags can appear in multiple languages)
	existingUIDs := make(map[string]bool)
	languages := consts.ALL_KNOWN_LANGS[:]

	for _, lang := range languages {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

		// Check which UIDs already exist
		existing, err := idx.manager.GetExistingUIDs(ctx, indexName, consts.ES_RESULT_TYPE_TAGS)
		if err != nil {
			return nil, errors.Wrapf(err, "check existing tags in %s", indexName)
		}

		// Merge into existingUIDs
		for uid := range existing {
			existingUIDs[uid] = true
		}
	}

	// Filter out existing tags
	var newTags []*mdbmodels.Tag
	for _, tag := range tags {
		if !existingUIDs[tag.UID] {
			newTags = append(newTags, tag)
		}
	}

	return newTags, nil
}
