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

// UpdateScope incrementally reindexes the source doc affected by an MDB change.
// Mirrors es6 SourcesIndex.Update: sources respond only to a SourceUID scope.
func (idx *SourcesIndexer) UpdateScope(ctx context.Context, scope es.Scope) error {
	if scope.SourceUID == "" {
		return nil
	}
	removed, err := idx.removeFromScope(ctx, scope)
	if err != nil {
		return err
	}
	return idx.addFromScope(ctx, scope, removed)
}

// removeFromScope deletes the source's doc across all language indices, returning removed uids.
func (idx *SourcesIndexer) removeFromScope(ctx context.Context, scope es.Scope) ([]string, error) {
	typedUids := []string{es.KeyValue(consts.ES_UID_TYPE_SOURCE, scope.SourceUID)}
	var removed []string
	for _, lang := range idx.GetLanguages() {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)
		uids, err := idx.manager.DeleteByTypedUids(ctx, indexName, consts.ES_RESULT_TYPE_SOURCES, typedUids)
		if err != nil {
			log.Warnf("Source incremental - failed delete from %s: %v", indexName, err)
			continue
		}
		removed = append(removed, uids...)
	}
	return removed, nil
}

// addFromScope re-fetches and indexes the affected sources (mirrors es6 addToIndex).
func (idx *SourcesIndexer) addFromScope(ctx context.Context, scope es.Scope, removedUIDs []string) error {
	uids := dedupStrings(append([]string{scope.SourceUID}, removedUIDs...))
	fetchScope := append(DefaultSourcesScope(), uidInScope(uids))
	sources, err := idx.FetchSources(ctx, fetchScope)
	if err != nil {
		return errors.Wrap(err, "fetch sources by scope")
	}
	return idx.indexSources(ctx, sources)
}

// indexSources loads the source hierarchy and runs the shared pipeline.
func (idx *SourcesIndexer) indexSources(ctx context.Context, sources []*mdbmodels.Source) error {
	if len(sources) == 0 {
		return nil
	}
	if err := idx.loadSourcesHierarchy(ctx); err != nil {
		return errors.Wrap(err, "load sources hierarchy")
	}
	items := make([]interface{}, len(sources))
	for i, s := range sources {
		items[i] = s
	}
	pipeline := indexing.NewPipeline(idx.manager, nil)
	return pipeline.RunPipeline(ctx, idx, items, &es.IndexData{})
}
