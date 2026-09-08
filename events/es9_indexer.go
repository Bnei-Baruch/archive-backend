package events

import (
	"context"
	"database/sql"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/es"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing/types"
	"github.com/Bnei-Baruch/archive-backend/integration"
)

// EventIndexer is the incremental-indexing surface the NATS event handlers use.
// Both *es.Indexer (ES6) and *ES9Indexer satisfy it; selected by elasticsearch.use-es9.
type EventIndexer interface {
	CollectionUpdate(uid string) error
	ContentUnitUpdate(uid string) error
	FileUpdate(uid string) error
	SourceUpdate(uid string) error
	TagUpdate(uid string) error
	PersonUpdate(uid string) error
	PublisherUpdate(uid string) error
	BlogPostUpdate(id string) error
	TweetUpdate(tid string) error
}

// scopedIndexer is one ES9 result-type indexer able to reindex the docs affected by a scope.
// Each type indexer no-ops for scopes that don't affect its result type.
type scopedIndexer interface {
	UpdateScope(ctx context.Context, scope es.Scope) error
}

// ES9Indexer routes MDB-change events to the ES9 incremental indexers. Every event builds
// an es.Scope and fans it out to all registered type indexers (es6 Update model). Fan-out
// grows coverage by appending indexers here; unhandled result types stay stale until added.
type ES9Indexer struct {
	indexers     []scopedIndexer
	manager      *es9common.ES9Manager
	indexPattern string // e.g. "results_*"
}

func MakeES9Indexer(db *sql.DB, manager *es9common.ES9Manager, indexNameBase, unzipURL string) *ES9Indexer {
	assets := integration.NewAssetsService(unzipURL)
	return &ES9Indexer{
		manager:      manager,
		indexPattern: indexNameBase + "_*",
		indexers: []scopedIndexer{
			types.NewContentUnitsIndexer(manager, db, indexNameBase, assets),
			types.NewCollectionsIndexer(manager, db, indexNameBase),
			types.NewSourcesIndexer(manager, db, indexNameBase, unzipURL),
			types.NewTagsIndexer(manager, db, indexNameBase),
			types.NewBlogPostsIndexer(manager, db, indexNameBase),
			types.NewTweetsIndexer(manager, db, indexNameBase),
		},
	}
}

func (e *ES9Indexer) update(scope es.Scope) error {
	ctx := context.Background()
	var result error
	for _, idx := range e.indexers {
		if err := idx.UpdateScope(ctx, scope); err != nil {
			log.Errorf("ES9 incremental UpdateScope %+v: %v", scope, err)
			result = err
		}
	}
	// Make the remove+re-add visible together promptly (closes the ~1s vanish window).
	if err := e.manager.Refresh(ctx, e.indexPattern); err != nil {
		log.Warnf("ES9 incremental: refresh %s failed: %v", e.indexPattern, err)
	}
	return result
}

func (e *ES9Indexer) ContentUnitUpdate(uid string) error { return e.update(es.Scope{ContentUnitUID: uid}) }
func (e *ES9Indexer) CollectionUpdate(uid string) error  { return e.update(es.Scope{CollectionUID: uid}) }
func (e *ES9Indexer) FileUpdate(uid string) error        { return e.update(es.Scope{FileUID: uid}) }
func (e *ES9Indexer) SourceUpdate(uid string) error      { return e.update(es.Scope{SourceUID: uid}) }
func (e *ES9Indexer) TagUpdate(uid string) error         { return e.update(es.Scope{TagUID: uid}) }
func (e *ES9Indexer) PersonUpdate(uid string) error      { return e.update(es.Scope{PersonUID: uid}) }
func (e *ES9Indexer) PublisherUpdate(uid string) error   { return e.update(es.Scope{PublisherUID: uid}) }
func (e *ES9Indexer) BlogPostUpdate(id string) error     { return e.update(es.Scope{BlogPostWPID: id}) }
func (e *ES9Indexer) TweetUpdate(tid string) error       { return e.update(es.Scope{TweetTID: tid}) }
