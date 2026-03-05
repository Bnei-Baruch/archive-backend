package types

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	"github.com/Bnei-Baruch/archive-backend/integration"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// SourcesIndexer implements the Indexer interface for sources
type SourcesIndexer struct {
	manager        *common.ES9Manager
	db             *sql.DB
	indexNameBase  string
	assetsService  integration.AssetsService
	// Hierarchical data loaded once for all sources
	pathsMap       map[string][]string              // uid => parent UIDs path
	idsMap         map[string][]int64               // uid => parent IDs path
	authorsByLang  map[string]map[string][]string   // uid => lang => author names
	hierarchyMu    sync.RWMutex
	skippedSources map[string]bool                  // Track sources without author hierarchy
	skippedMu      sync.Mutex
}

// NewSourcesIndexer creates a new sources indexer
func NewSourcesIndexer(mgr *common.ES9Manager, db *sql.DB, indexNameBase string, unzipUrl string) *SourcesIndexer {
	if indexNameBase == "" {
		indexNameBase = "results" // Sources share the same index as other result types
	}
	return &SourcesIndexer{
		manager:        mgr,
		db:             db,
		indexNameBase:  indexNameBase,
		assetsService:  integration.NewAssetsService(unzipUrl),
		skippedSources: make(map[string]bool),
	}
}

// GetIndexNameBase returns the base name for indices
func (idx *SourcesIndexer) GetIndexNameBase() string {
	return idx.indexNameBase
}

// GetLanguages returns all known languages
func (idx *SourcesIndexer) GetLanguages() []string {
	return consts.ALL_KNOWN_LANGS[:]
}

// GetDocumentType returns the document type name for logging
func (idx *SourcesIndexer) GetDocumentType() string {
	return "source"
}

// PrepareDocument converts a source to an ES9 Result document
func (idx *SourcesIndexer) PrepareDocument(ctx context.Context, item interface{}, lang string, indexData *es.IndexData, indexDate *utils.Date, progress *indexing.ProgressTracker) (doc *es.Result, skip bool) {
	source, ok := item.(*mdbmodels.Source)
	if !ok {
		log.Errorf("Invalid item type for sources indexer: %T", item)
		return nil, true
	}

	// Check if we have relationships loaded
	if source.R == nil || source.R.SourceI18ns == nil {
		log.Errorf("Source %s missing i18n relationships", source.UID)
		return nil, true
	}

	// Find i18n for requested language
	var i18n *mdbmodels.SourceI18n
	for _, si18n := range source.R.SourceI18ns {
		if si18n.Language == lang {
			i18n = si18n
			break
		}
	}

	// Skip if no i18n for this language or name is empty
	if i18n == nil || !i18n.Name.Valid || i18n.Name.String == "" {
		return nil, true
	}

	// Get hierarchical paths for this source
	idx.hierarchyMu.RLock()
	parents, ok := idx.pathsMap[source.UID]
	idx.hierarchyMu.RUnlock()

	if !ok {
		// Source has no author hierarchy - collect for summary
		idx.skippedMu.Lock()
		idx.skippedSources[source.UID] = true
		idx.skippedMu.Unlock()
		return nil, true
	}

	parentIds, ok := idx.idsMap[source.UID]
	if !ok {
		log.Warnf("Source %s not found in IDs map", source.UID)
		return nil, true
	}

	authors, ok := idx.authorsByLang[source.UID]
	if !ok {
		log.Warnf("Source %s not found in authors map", source.UID)
		return nil, true
	}

	// Build path names by fetching parent i18ns
	pathNames := []string{}
	for _, parentID := range parentIds {
		ni18n, err := mdbmodels.FindSourceI18n(idx.db, parentID, i18n.Language)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			log.Warnf("Error finding parent i18n for source %s, parent ID %d: %v", source.UID, parentID, err)
			continue
		}
		if ni18n.Name.Valid && ni18n.Name.String != "" {
			pathNames = append(pathNames, ni18n.Name.String)
		}
	}

	// Get author names for this language
	langAuthors := authors[i18n.Language]
	allPathElements := append(langAuthors, pathNames...)

	if len(allPathElements) == 0 {
		log.Warnf("No path elements for source %s in language %s", source.UID, i18n.Language)
		return nil, true
	}

	leaf := allPathElements[len(allPathElements)-1]

	// Create basic document
	doc = &es.Result{
		ResultType:   consts.ES_RESULT_TYPE_SOURCES,
		IndexDate:    indexDate,
		MDB_UID:      source.UID,
		FilterValues: es.KeyValues(consts.ES_UID_TYPE_SOURCE, parents),
		TypedUids:    []string{es.KeyValue(consts.ES_UID_TYPE_SOURCE, source.UID)},
		Title:        leaf,
		FullTitle:    strings.Join(allPathElements, " > "),
	}

	// Add description
	if i18n.Description.Valid && i18n.Description.String != "" && i18n.Description.String != " " {
		if _, ok := consts.SRC_TYPES_FOR_TITLE_DESCRIPTION_CONCAT[source.TypeID]; ok {
			// Combine title and description for better search support
			doc.Description = fmt.Sprintf("%s %s", leaf, i18n.Description.String)
		} else {
			doc.Description = i18n.Description.String
		}
	}

	// Build title suggest suffixes
	suffixes := es.Suffixes(strings.Join(allPathElements, " "))
	if len(allPathElements) > 2 {
		suffixes = append(suffixes, es.ConcateFirstToLast(allPathElements))
	}

	// Add "מאמר" prefix for certain sources
	if _, ok := consts.ES_SRC_ADD_MAAMAR_TO_SUGGEST[source.UID]; ok {
		suffixes = append(suffixes, fmt.Sprintf("מאמר %s", leaf))
	}

	// Add chapter number/letter for Shamati articles and similar
	if source.ParentID.Valid && source.Position.Valid && source.Position.Int > 0 {
		addPosition := false
		var positionIndexType consts.PositionIndexType
		for _, parent := range parents {
			if val, ok := consts.ES_SRC_PARENTS_FOR_CHAPTER_POSITION_INDEX[parent]; ok {
				addPosition = true
				positionIndexType = val
				break
			}
		}

		if addPosition {
			position := strconv.Itoa(source.Position.Int)
			if i18n.Language == consts.LANG_HEBREW && positionIndexType == consts.LETTER_IF_HEBREW {
				position = utils.NumberInHebrew(source.Position.Int) // Convert to Hebrew letter
			}

			// Example: "קלג. אורות דשבת" or "133. The Lights of Shabbat"
			leafWithChapter := fmt.Sprintf("%s. %s", position, leaf)
			pathWithChapter := append(allPathElements[:len(allPathElements)-1], leafWithChapter)
			suffixesWithChapter := es.Suffixes(strings.Join(pathWithChapter, " "))
			suffixes = es.Unique(append(suffixes, suffixesWithChapter...))
		}
	}

	// Set title suggest weight
	if weight, ok := consts.ES_SUGGEST_SOURCES_WEIGHT[source.UID]; ok {
		doc.TitleSuggest = es.SuggestField{Input: suffixes, Weight: weight}
	} else {
		doc.TitleSuggest = es.SuggestField{Input: suffixes, Weight: float64(consts.ES_SOURCES_SUGGEST_DEFAULT_WEIGHT)}
	}

	// Try to fetch document content
	if content, err := idx.fetchDocContent(source.UID, i18n.Language); err == nil && content != "" {
		doc.Content = content
	}

	return doc, false
}

// fetchDocContent tries to fetch document content from source files
func (idx *SourcesIndexer) fetchDocContent(sourceUID string, lang string) (string, error) {
	// Try to get docx path from sources folder
	docxPath, err := idx.getDocxPath(sourceUID, lang)
	if err == nil && docxPath != "" {
		// TODO: Extract text from docx file
		// For now, just return empty to avoid blocking
		return "", nil
	}

	// Try to fetch from content unit file
	return idx.fetchDocxFromContentUnit(sourceUID, lang)
}

// getDocxPath gets the path to a docx file for a source
func (idx *SourcesIndexer) getDocxPath(uid string, lang string) (string, error) {
	uidPath := path.Join(es.SourcesFolder(), uid)
	jsonPath := path.Join(uidPath, "index.json")
	jsonCnt, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("unable to read from file %s: %w", jsonPath, err)
	}

	var m map[string]map[string]string
	err = json.Unmarshal(jsonCnt, &m)
	if err != nil {
		return "", err
	}

	if val, ok := m[lang]; ok {
		docxPath := path.Join(uidPath, val["docx"])
		if _, err := os.Stat(docxPath); err == nil {
			return docxPath, nil
		}
	}

	return "", errors.New("docx not found in index.json")
}

// fetchDocxFromContentUnit fetches docx content from a content unit associated with the source
func (idx *SourcesIndexer) fetchDocxFromContentUnit(sourceUID string, lang string) (string, error) {
	queryMask := `select f.uid from files f
		join content_units cu ON cu.id = f.content_unit_id
		where cu.published IS TRUE and cu.secure = %d and f.secure = %d and f.published IS TRUE and f.removed_at IS NULL
		and f.name like '%%.doc%%'
		and cu.uid = '%s' AND language = '%s'`
	query := fmt.Sprintf(queryMask, consts.SEC_PUBLIC, consts.SEC_PUBLIC, sourceUID, lang)

	var fileUID string
	err := queries.Raw(query).QueryRow(idx.db).Scan(&fileUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	return idx.assetsService.Doc2Text(fileUID)
}

// LoadRelationships loads hierarchical paths, authors, and i18ns for sources
func (idx *SourcesIndexer) LoadRelationships(ctx context.Context, items []interface{}) error {
	if len(items) == 0 {
		return nil
	}

	log.Debugf("Loading relationships for batch of %d sources", len(items))

	// Extract source IDs
	sourceIDs := make([]interface{}, len(items))
	for i, item := range items {
		source, ok := item.(*mdbmodels.Source)
		if !ok {
			return errors.Errorf("invalid item type: %T", item)
		}
		sourceIDs[i] = source.ID
	}

	// Load SourceI18ns
	i18ns, err := mdbmodels.SourceI18ns(
		qm.WhereIn("source_id IN ?", sourceIDs...),
	).All(idx.db)
	if err != nil {
		return errors.Wrap(err, "load source_i18ns")
	}

	// Map relationships back to sources
	i18nMap := make(map[int64][]*mdbmodels.SourceI18n)
	for _, i18n := range i18ns {
		i18nMap[i18n.SourceID] = append(i18nMap[i18n.SourceID], i18n)
	}

	// Assign relationships to each source
	// Note: Authors are loaded globally in loadSourcesHierarchy, not here
	for _, item := range items {
		source, ok := item.(*mdbmodels.Source)
		if !ok {
			continue
		}

		if source.R == nil {
			source.R = source.R.NewStruct()
		}

		if i18nList, ok := i18nMap[source.ID]; ok {
			source.R.SourceI18ns = i18nList
		}
	}

	log.Debugf("✓ Loaded relationships: %d i18ns", len(i18ns))

	return nil
}

// IndexAll indexes all sources
func (idx *SourcesIndexer) IndexAll(ctx context.Context, reset bool) error {
	startTime := time.Now()
	log.Infof("Starting sources indexing (reset=%v)", reset)

	// Delete existing sources if reset
	if reset {
		if err := idx.deleteExistingSources(ctx); err != nil {
			return errors.Wrap(err, "delete existing sources")
		}
	}

	// Load hierarchical paths and authors for ALL sources first
	// This is needed because source documents reference their parents
	log.Info("Loading hierarchical paths and authors for all sources...")
	if err := idx.loadSourcesHierarchy(ctx); err != nil {
		return errors.Wrap(err, "load sources hierarchy")
	}
	log.Infof("✓ Loaded hierarchy for %d sources", len(idx.pathsMap))

	// Fetch all sources
	scope := DefaultSourcesScope()
	sources, err := idx.FetchSources(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "fetch sources")
	}

	log.Infof("Fetched %d sources from MDB", len(sources))

	if len(sources) == 0 {
		log.Info("No sources to index")
		return nil
	}

	// Filter existing sources if not reset
	if !reset {
		sources, err = idx.FilterExistingSources(ctx, sources)
		if err != nil {
			return errors.Wrap(err, "filter existing sources")
		}

		if len(sources) == 0 {
			log.Info("✓ All sources already indexed")
			return nil
		}

		log.Infof("Filtered: %d new sources to index", len(sources))
	}

	// Load index data (not needed for sources)
	indexData := &es.IndexData{}

	// Convert []*mdbmodels.Source to []interface{}
	items := make([]interface{}, len(sources))
	for i, s := range sources {
		items[i] = s
	}

	// Use generic pipeline for indexing
	pipeline := indexing.NewPipeline(idx.manager, nil) // nil = use default config
	if err := pipeline.RunPipeline(ctx, idx, items, indexData); err != nil {
		return errors.Wrap(err, "run indexing pipeline")
	}

	// Print summary of skipped sources with titles
	idx.skippedMu.Lock()
	skippedCount := len(idx.skippedSources)
	if skippedCount > 0 {
		skippedUIDs := make([]string, 0, skippedCount)
		for uid := range idx.skippedSources {
			skippedUIDs = append(skippedUIDs, uid)
		}

		log.Warnf("⚠ Skipped %d sources without author hierarchy:", skippedCount)

		// Fetch English titles for skipped sources
		rows, err := queries.Raw(`
			SELECT s.uid, si.name
			FROM sources s
			LEFT JOIN source_i18n si ON si.source_id = s.id AND si.language = 'en'
			WHERE s.uid = ANY($1)
		`, pq.Array(skippedUIDs)).Query(idx.db)

		if err != nil {
			log.Warnf("  Failed to fetch titles: %v", err)
			log.Warnf("  UIDs: %v", skippedUIDs)
		} else {
			defer rows.Close()

			titlesMap := make(map[string]string)
			for rows.Next() {
				var uid string
				var name sql.NullString
				if err := rows.Scan(&uid, &name); err != nil {
					continue
				}
				if name.Valid && name.String != "" {
					titlesMap[uid] = name.String
				} else {
					titlesMap[uid] = "(no title)"
				}
			}

			for _, uid := range skippedUIDs {
				if title, ok := titlesMap[uid]; ok {
					log.Warnf("  - %s: %s", uid, title)
				} else {
					log.Warnf("  - %s: (title not found)", uid)
				}
			}
		}
	}
	idx.skippedMu.Unlock()

	log.Infof("✓ Sources indexing completed in %v", time.Since(startTime))
	return nil
}

// loadSourcesHierarchy loads the hierarchical paths and authors for all sources using recursive SQL
func (idx *SourcesIndexer) loadSourcesHierarchy(ctx context.Context) error {
	rows, err := queries.Raw(`
		WITH recursive rec_sources AS (
			SELECT s.id, s.uid,
				   array[s.uid] "path",
				   array[s.id]::bigint[] "idspath",
				   authors,
				   aun.language,
				   aun.author_names
			FROM sources s
			JOIN (
				SELECT source_id, array_agg(a.id), array_agg(a.code) AS authors
				FROM authors_sources aas
				JOIN authors a ON a.id = aas.author_id
				GROUP BY source_id
			) au ON au.source_id = s.id
			JOIN (
				SELECT source_id, an.language,
					   array[an.name] AS author_names
				FROM authors_sources aas
				JOIN authors a ON a.id = aas.author_id
				LEFT JOIN author_i18n an ON an.author_id = a.id
			) aun ON aun.source_id = s.id
			UNION
			SELECT s.id, s.uid,
				   (rs.path || s.uid)::character(8)[],
				   (rs.idspath || s.id)::bigint[],
				   rs.authors,
				   rs.language,
				   rs.author_names
			FROM sources s
			INNER JOIN rec_sources rs ON s.parent_id = rs.id
		)
		SELECT uid, array_cat(path, authors) "path", idspath, language, author_names
		FROM rec_sources
		WHERE 1=1
	`).Query(idx.db)

	if err != nil {
		return errors.Wrap(err, "query sources hierarchy")
	}
	defer rows.Close()

	idx.pathsMap = make(map[string][]string)
	idx.idsMap = make(map[string][]int64)
	idx.authorsByLang = make(map[string]map[string][]string)

	for rows.Next() {
		var uid string
		var lang string
		var authors pq.StringArray
		var codeValues pq.StringArray
		var idValues pq.Int64Array
		err := rows.Scan(&uid, &codeValues, &idValues, &lang, &authors)
		if err != nil {
			return errors.Wrap(err, "scan row")
		}

		if idx.authorsByLang[uid] == nil {
			idx.authorsByLang[uid] = make(map[string][]string)
		}

		stringAuthors := make([]string, 0, len(authors))
		for _, a := range authors {
			stringAuthors = append(stringAuthors, a)
		}

		stringCodeValues := make([]string, 0, len(codeValues))
		for _, cv := range codeValues {
			stringCodeValues = append(stringCodeValues, cv)
		}

		idx.authorsByLang[uid][lang] = stringAuthors
		if _, ok := idx.pathsMap[uid]; !ok {
			idx.pathsMap[uid] = stringCodeValues
			idx.idsMap[uid] = idValues
		}
	}

	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "iterate rows")
	}

	return nil
}

// defaultSourcesScope returns the default SQL scope for sources (all sources)
func DefaultSourcesScope() []qm.QueryMod {
	return []qm.QueryMod{
		qm.Where("1=1"), // No filtering - index all sources
		qm.OrderBy("id"),
	}
}

// fetchSources loads sources from MDB with the given scope
func (idx *SourcesIndexer) FetchSources(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.Source, error) {
	sources, err := mdbmodels.Sources(scope...).All(idx.db)
	if err != nil {
		return nil, errors.Wrap(err, "query sources")
	}
	return sources, nil
}

// deleteExistingSources deletes all source documents from all language indices
func (idx *SourcesIndexer) deleteExistingSources(ctx context.Context) error {
	languages := consts.ALL_KNOWN_LANGS[:]
	log.Infof("Deleting existing sources from %d language indices", len(languages))

	var wg sync.WaitGroup
	errChan := make(chan error, len(languages))

	for _, lang := range languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

			// Delete by result type
			_, err := idx.manager.DeleteByResultType(ctx, indexName, consts.ES_RESULT_TYPE_SOURCES)
			if err != nil {
				errChan <- errors.Wrapf(err, "delete sources from %s", indexName)
				return
			}

			log.Debugf("✓ Deleted sources from: %s", indexName)
		}(lang)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return <-errChan
	}

	log.Info("✓ Deleted existing sources from all language indices")
	return nil
}

// filterExistingSources filters out sources that are already indexed
func (idx *SourcesIndexer) FilterExistingSources(ctx context.Context, sources []*mdbmodels.Source) ([]*mdbmodels.Source, error) {
	if len(sources) == 0 {
		return sources, nil
	}

	// Build UID map
	uidMap := make(map[string]*mdbmodels.Source)
	for _, source := range sources {
		uidMap[source.UID] = source
	}

	// Check all language indices (sources can appear in multiple languages)
	existingUIDs := make(map[string]bool)
	languages := consts.ALL_KNOWN_LANGS[:]

	for _, lang := range languages {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

		// Check which UIDs already exist
		existing, err := idx.manager.GetExistingUIDs(ctx, indexName, consts.ES_RESULT_TYPE_SOURCES)
		if err != nil {
			return nil, errors.Wrapf(err, "check existing sources in %s", indexName)
		}

		// Merge into existingUIDs
		for uid := range existing {
			existingUIDs[uid] = true
		}
	}

	// Filter out existing sources
	var newSources []*mdbmodels.Source
	for _, source := range sources {
		if !existingUIDs[source.UID] {
			newSources = append(newSources, source)
		}
	}

	return newSources, nil
}
