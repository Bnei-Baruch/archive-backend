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

// UpdateScope incrementally reindexes the tag doc affected by an MDB change.
// Mirrors es6 TagsIndex.Update: tags respond only to a TagUID scope.
func (idx *TagsIndexer) UpdateScope(ctx context.Context, scope es.Scope) error {
	if scope.TagUID == "" {
		return nil
	}
	removed, err := idx.removeFromScope(ctx, scope)
	if err != nil {
		return err
	}
	uids := dedupStrings(append([]string{scope.TagUID}, removed...))
	tags, err := idx.FetchTags(ctx, append(DefaultTagsScope(), uidInScope(uids)))
	if err != nil {
		return errors.Wrap(err, "fetch tags by scope")
	}
	return idx.indexTags(ctx, tags)
}

func (idx *TagsIndexer) removeFromScope(ctx context.Context, scope es.Scope) ([]string, error) {
	typedUids := []string{es.KeyValue(consts.ES_UID_TYPE_TAG, scope.TagUID)}
	var removed []string
	for _, lang := range idx.GetLanguages() {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)
		uids, err := idx.manager.DeleteByTypedUids(ctx, indexName, consts.ES_RESULT_TYPE_TAGS, typedUids)
		if err != nil {
			log.Warnf("Tag incremental - failed delete from %s: %v", indexName, err)
			continue
		}
		removed = append(removed, uids...)
	}
	return removed, nil
}

func (idx *TagsIndexer) indexTags(ctx context.Context, tags []*mdbmodels.Tag) error {
	if len(tags) == 0 {
		return nil
	}
	items := make([]interface{}, len(tags))
	for i, t := range tags {
		items[i] = t
	}
	pipeline := indexing.NewPipeline(idx.manager, nil)
	return pipeline.RunPipeline(ctx, idx, items, &es.IndexData{})
}
