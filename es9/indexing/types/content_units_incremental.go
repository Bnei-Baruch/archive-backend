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

// UpdateScope incrementally reindexes the content-unit docs affected by an MDB change.
// Mirrors es6 ContentUnitsIndex.Update: remove the affected docs, then re-add from MDB.
// Order matters: remove (refresh=true) before add, so re-indexing creates no duplicates.
func (idx *ContentUnitsIndexer) UpdateScope(ctx context.Context, scope es.Scope) error {
	removed, err := idx.removeFromScope(ctx, scope)
	if err != nil {
		return err
	}
	return idx.addFromScope(ctx, scope, removed)
}

// removeFromScope deletes, across all language indices, the content-unit docs whose
// typed_uids reference any entity in the scope, returning the removed unit uids.
func (idx *ContentUnitsIndexer) removeFromScope(ctx context.Context, scope es.Scope) ([]string, error) {
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
		uids, err := idx.manager.DeleteByTypedUids(ctx, indexName, consts.ES_RESULT_TYPE_UNITS, typedUids)
		if err != nil {
			log.Warnf("ContentUnit incremental - failed delete from %s: %v", indexName, err)
			continue
		}
		removed = append(removed, uids...)
	}
	return removed, nil
}

// addFromScope re-fetches the content units affected by the scope (plus any removed uids),
// applying the default publish/secure filter, and indexes them (mirrors es6 addToIndex).
func (idx *ContentUnitsIndexer) addFromScope(ctx context.Context, scope es.Scope, removedUIDs []string) error {
	uids, err := idx.scopeToUnitUIDs(scope)
	if err != nil {
		return err
	}
	uids = dedupStrings(append(uids, removedUIDs...))
	if len(uids) == 0 {
		return nil
	}
	fetchScope := append(DefaultContentUnitScope(), uidInScope(uids))
	units, err := idx.FetchContentUnits(ctx, fetchScope)
	if err != nil {
		return errors.Wrap(err, "fetch units by scope")
	}
	return idx.indexUnits(ctx, units)
}

// indexUnits loads supplementary data and runs the shared pipeline for a set of units.
func (idx *ContentUnitsIndexer) indexUnits(ctx context.Context, units []*mdbmodels.ContentUnit) error {
	if len(units) == 0 {
		return nil
	}
	indexData, err := idx.loadIndexData(ctx, units)
	if err != nil {
		return errors.Wrap(err, "load index data")
	}
	items := make([]interface{}, len(units))
	for i, cu := range units {
		items[i] = cu
	}
	pipeline := indexing.NewPipeline(idx.manager, nil)
	return pipeline.RunPipeline(ctx, idx, items, indexData)
}

// scopeToUnitUIDs expands a scope into the affected content-unit UIDs (add side).
func (idx *ContentUnitsIndexer) scopeToUnitUIDs(scope es.Scope) ([]string, error) {
	var uids []string
	if scope.ContentUnitUID != "" {
		uids = append(uids, scope.ContentUnitUID)
	}
	if scope.FileUID != "" {
		more, err := es.ContentUnitsScopeByFile(idx.db, scope.FileUID)
		if err != nil {
			return nil, err
		}
		uids = append(uids, more...)
	}
	if scope.CollectionUID != "" {
		more, err := es.ContentUnitsScopeByCollection(idx.db, scope.CollectionUID)
		if err != nil {
			return nil, err
		}
		uids = append(uids, more...)
	}
	if scope.SourceUID != "" {
		more, err := es.ContentUnitsScopeBySource(idx.db, scope.SourceUID)
		if err != nil {
			return nil, err
		}
		uids = append(uids, more...)
	}
	return uids, nil
}

// scopeToTypedUids expands a scope into the typed_uids to match for removal (remove side).
func (idx *ContentUnitsIndexer) scopeToTypedUids(scope es.Scope) ([]string, error) {
	var tu []string
	if scope.ContentUnitUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_CONTENT_UNIT, scope.ContentUnitUID))
	}
	if scope.FileUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_FILE, scope.FileUID))
		more, err := es.ContentUnitsScopeByFile(idx.db, scope.FileUID)
		if err != nil {
			return nil, err
		}
		tu = append(tu, es.KeyValues(consts.ES_UID_TYPE_CONTENT_UNIT, more)...)
	}
	if scope.CollectionUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_COLLECTION, scope.CollectionUID))
		more, err := es.ContentUnitsScopeByCollection(idx.db, scope.CollectionUID)
		if err != nil {
			return nil, err
		}
		tu = append(tu, es.KeyValues(consts.ES_UID_TYPE_CONTENT_UNIT, more)...)
	}
	if scope.TagUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_TAG, scope.TagUID))
	}
	if scope.SourceUID != "" {
		tu = append(tu, es.KeyValue(consts.ES_UID_TYPE_SOURCE, scope.SourceUID))
		more, err := es.ContentUnitsScopeBySource(idx.db, scope.SourceUID)
		if err != nil {
			return nil, err
		}
		tu = append(tu, es.KeyValues(consts.ES_UID_TYPE_CONTENT_UNIT, more)...)
	}
	return tu, nil
}
