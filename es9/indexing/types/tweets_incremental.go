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

// UpdateScope incrementally reindexes the tweet doc affected by an MDB change.
// Mirrors es6 TweeterIndex.Update: tweets respond only to a TweetTID scope.
func (idx *TweetsIndexer) UpdateScope(ctx context.Context, scope es.Scope) error {
	if scope.TweetTID == "" {
		return nil
	}
	removed, err := idx.removeFromScope(ctx, scope)
	if err != nil {
		return err
	}
	tids := dedupStrings(append([]string{scope.TweetTID}, removed...))
	tweets, err := idx.FetchTweets(ctx, append(DefaultTweetsScope(), inScope("twitter_id", tids)))
	if err != nil {
		return errors.Wrap(err, "fetch tweets by scope")
	}
	return idx.indexTweets(ctx, tweets)
}

func (idx *TweetsIndexer) removeFromScope(ctx context.Context, scope es.Scope) ([]string, error) {
	typedUids := []string{es.KeyValue(consts.ES_UID_TYPE_TWEET, scope.TweetTID)}
	var removed []string
	for _, lang := range idx.GetLanguages() {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)
		uids, err := idx.manager.DeleteByTypedUids(ctx, indexName, consts.ES_RESULT_TYPE_TWEETS, typedUids)
		if err != nil {
			log.Warnf("Tweet incremental - failed delete from %s: %v", indexName, err)
			continue
		}
		removed = append(removed, uids...)
	}
	return removed, nil
}

func (idx *TweetsIndexer) indexTweets(ctx context.Context, tweets []*mdbmodels.TwitterTweet) error {
	if len(tweets) == 0 {
		return nil
	}
	items := make([]interface{}, len(tweets))
	for i, t := range tweets {
		items[i] = t
	}
	pipeline := indexing.NewPipeline(idx.manager, nil)
	return pipeline.RunPipeline(ctx, idx, items, &es.IndexData{})
}
