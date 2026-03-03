package indexing

import (
	"context"

	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// Indexer defines the interface that all indexer types must implement
// Each indexer (content units, collections, sources, tags, books, songs) implements this interface
type Indexer interface {
	// PrepareDocument prepares a single document for indexing in a specific language
	// Returns:
	//   - doc: the prepared document (or nil if no i18n for this language)
	//   - skip: true if this document should be skipped entirely
	PrepareDocument(
		ctx context.Context,
		item interface{},
		lang string,
		indexData *es.IndexData,
		indexDate *utils.Date,
		progress *ProgressTracker,
	) (doc *es.Result, skip bool)

	// LoadRelationships loads relationships from the database for a batch of items
	// This is called in chunks for DB efficiency (e.g., load i18ns, collections, persons)
	LoadRelationships(ctx context.Context, items []interface{}) error

	// GetIndexNameBase returns the base name for indices (e.g., "results", "collections")
	GetIndexNameBase() string

	// GetLanguages returns the list of languages this indexer should create documents for
	// Most indexers use consts.ALL_KNOWN_LANGS, but some might have custom lists
	GetLanguages() []string

	// GetDocumentType returns the document type name for logging (e.g., "content unit", "collection")
	GetDocumentType() string
}
