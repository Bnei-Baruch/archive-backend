package types

import (
	"context"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
)

// UpdateScope incrementally reindexes the collection docs affected by an MDB change.
// Mirrors es6 CollectionsIndex.Update: remove affected docs, then re-add from MDB.
func (idx *CollectionsIndexer) UpdateScope(ctx context.Context, scope es.Scope) error {
	removed, err := idx.removeFromScope(ctx, scope)
	if err != nil {
		return err
	}
	return idx.addFromScope(ctx, scope, removed)
}

// removeFromScope deletes, across all language indices, the collection docs whose
// typed_uids reference any entity in the scope, returning the removed collection uids.
func (idx *CollectionsIndexer) removeFromScope(ctx context.Context, scope es.Scope) ([]string, error) {
	typedUids, err := idx.scopeToTypedUids(scope)
	if err != nil {
		return nil, err
	}
	if len(typedUids) == 0 {
		return nil, nil
	}
	var removed []string
	for _, lang := range idx.GetLanguages() {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)
		uids, err := idx.manager.DeleteByTypedUids(ctx, indexName, consts.ES_RESULT_TYPE_COLLECTIONS, typedUids)
		if err != nil {
			log.Warnf("Collection incremental - failed delete from %s: %v", indexName, err)
			continue
		}
		removed = append(removed, uids...)
	}
	return removed, nil
}

// addFromScope re-fetches the collections affected by the scope (plus any removed uids),
// applying the default filter, and indexes them (mirrors es6 addToIndex).
func (idx *CollectionsIndexer) addFromScope(ctx context.Context, scope es.Scope, removedUIDs []string) error {
	uids, err := idx.scopeToCollectionUIDs(scope)
	if err != nil {
		return err
	}
	uids = dedupStrings(append(uids, removedUIDs...))
	if len(uids) == 0 {
		return nil
	}
	fetchScope := append(DefaultCollectionsScope(), uidInScope(uids))
	collections, err := idx.FetchCollections(ctx, fetchScope)
	if err != nil {
		return errors.Wrap(err, "fetch collections by scope")
	}
	return idx.indexCollections(ctx, collections)
}

// indexCollections runs the shared pipeline for a set of collections.
func (idx *CollectionsIndexer) indexCollections(ctx context.Context, collections []*mdbmodels.Collection) error {
	if len(collections) == 0 {
		return nil
	}
	items := make([]interface{}, len(collections))
	for i, c := range collections {
		items[i] = c
	}
	pipeline := indexing.NewPipeline(idx.manager, nil)
	return pipeline.RunPipeline(ctx, idx, items, &es.IndexData{})
}

// scopeToCollectionUIDs expands a scope into the affected collection UIDs (add side).
func (idx *CollectionsIndexer) scopeToCollectionUIDs(scope es.Scope) ([]string, error) {
	var uids []string
	if scope.CollectionUID != "" {
		uids = append(uids, scope.CollectionUID)
	}
	if scope.ContentUnitUID != "" {
		more, err := es.CollectionsScopeByContentUnit(idx.db, scope.ContentUnitUID)
		if err != nil {
			return nil, err
		}
		uids = append(uids, more...)
	}
	return uids, nil
}

// scopeToTypedUids expands a scope into the typed_uids to match for removal (remove side).
func (idx *CollectionsIndexer) scopeToTypedUids(scope es.Scope) ([]string, error) {
	var tu []string
	if scope.CollectionUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_COLLECTION, scope.CollectionUID))
	}
	if scope.FileUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_FILE, scope.FileUID))
	}
	if scope.ContentUnitUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_CONTENT_UNIT, scope.ContentUnitUID))
		more, err := es.CollectionsScopeByContentUnit(idx.db, scope.ContentUnitUID)
		if err != nil {
			return nil, err
		}
		tu = append(tu, es.KeyValues(consts.ES_UID_TYPE_COLLECTION, more)...)
	}
	if scope.TagUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_TAG, scope.TagUID))
	}
	if scope.SourceUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_SOURCE, scope.SourceUID))
	}
	return tu, nil
}
