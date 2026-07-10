package types

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// TweetsIndexer implements the Indexer interface for tweets
type TweetsIndexer struct {
	manager       *common.ES9Manager
	db            *sql.DB
	indexNameBase string
}

// NewTweetsIndexer creates a new tweets indexer
func NewTweetsIndexer(mgr *common.ES9Manager, db *sql.DB, indexNameBase string) *TweetsIndexer {
	if indexNameBase == "" {
		indexNameBase = "results" // Tweets share the same index as other result types
	}
	return &TweetsIndexer{
		manager:       mgr,
		db:            db,
		indexNameBase: indexNameBase,
	}
}

// GetIndexNameBase returns the base name for indices
func (idx *TweetsIndexer) GetIndexNameBase() string {
	return idx.indexNameBase
}

// GetLanguages returns all known languages
func (idx *TweetsIndexer) GetLanguages() []string {
	return consts.ALL_KNOWN_LANGS[:]
}

// GetDocumentType returns the document type name for logging
func (idx *TweetsIndexer) GetDocumentType() string {
	return "tweet"
}

// GetResultType returns the result type for tweets
func (idx *TweetsIndexer) GetResultType() string {
	return consts.ES_RESULT_TYPE_TWEETS
}

// GetCriticalFields returns fields that must match exactly between ES6 and ES9
func (idx *TweetsIndexer) GetCriticalFields() []string {
	return []string{"mdb_uid", "result_type", "content", "effective_date", "filter_values", "typed_uids"}
}

// userIdToLanguageMapping maps Twitter user IDs to language codes
func (idx *TweetsIndexer) userIdToLanguageMapping() map[int]string {
	return map[int]string{
		1: consts.LANG_RUSSIAN,
		2: consts.LANG_HEBREW,
		3: consts.LANG_ENGLISH,
		4: consts.LANG_SPANISH,
	}
}

// PrepareDocument converts a tweet to an ES9 Result document
func (idx *TweetsIndexer) PrepareDocument(ctx context.Context, item interface{}, lang string, indexData *es.IndexData, indexDate *utils.Date, progress *indexing.ProgressTracker) (doc *es.Result, skip bool) {
	tweet, ok := item.(*mdbmodels.TwitterTweet)
	if !ok {
		log.Errorf("Invalid item type for tweets indexer: %T", item)
		return nil, true
	}

	// Map user ID to language
	langMapping := idx.userIdToLanguageMapping()
	tweetLang := langMapping[int(tweet.UserID)]

	// Skip if not the requested language
	if lang != tweetLang {
		return nil, true
	}

	// Create document
	doc = &es.Result{
		ResultType:    consts.ES_RESULT_TYPE_TWEETS,
		IndexDate:     indexDate,
		MDB_UID:       tweet.TwitterID, // TwitterID is used instead of UID
		TypedUids:     []string{es.KeyValue(consts.ES_UID_TYPE_TWEET, tweet.TwitterID)},
		FilterValues:  []string{es.KeyValue("content_type", consts.SCT_TWEET), es.KeyValue(consts.FILTER_MEDIA_LANGUAGE, tweetLang)},
		Title:         "",
		EffectiveDate: &utils.Date{Time: tweet.TweetAt},
		Content:       tweet.FullText,
		TitleSuggest:  es.SuggestField{Input: []string{}, Weight: 0},
	}

	return doc, false
}

// LoadRelationships loads any relationships needed for tweets (none needed for tweets)
func (idx *TweetsIndexer) LoadRelationships(ctx context.Context, items []interface{}) error {
	// Tweets don't have relationships to load
	return nil
}

// IndexAll indexes all tweets
func (idx *TweetsIndexer) IndexAll(ctx context.Context, reset bool) error {
	startTime := time.Now()
	log.Infof("Starting tweets indexing (reset=%v)", reset)

	// Delete existing tweets if reset
	if reset {
		if err := idx.deleteExistingTweets(ctx); err != nil {
			return errors.Wrap(err, "delete existing tweets")
		}
	}

	// Fetch all tweets
	scope := DefaultTweetsScope()
	tweets, err := idx.FetchTweets(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "fetch tweets")
	}

	log.Infof("Fetched %d tweets from MDB", len(tweets))

	if len(tweets) == 0 {
		log.Info("No tweets to index")
		return nil
	}

	// Filter existing tweets if not reset
	if !reset {
		tweets, err = idx.FilterExistingTweets(ctx, tweets)
		if err != nil {
			return errors.Wrap(err, "filter existing tweets")
		}

		if len(tweets) == 0 {
			log.Info("✓ All tweets already indexed")
			return nil
		}

		log.Infof("Filtered: %d new tweets to index", len(tweets))
	}

	// Load index data (not needed for tweets)
	indexData := &es.IndexData{}

	// Convert []*mdbmodels.TwitterTweet to []interface{}
	items := make([]interface{}, len(tweets))
	for i, t := range tweets {
		items[i] = t
	}

	// Use generic pipeline for indexing
	pipeline := indexing.NewPipeline(idx.manager, nil) // nil = use default config
	if err := pipeline.RunPipeline(ctx, idx, items, indexData); err != nil {
		return errors.Wrap(err, "run indexing pipeline")
	}

	log.Infof("✓ Tweets indexing completed in %v", time.Since(startTime))
	return nil
}

// defaultTweetsScope returns the default SQL scope for tweets (all tweets)
func DefaultTweetsScope() []qm.QueryMod {
	return []qm.QueryMod{
		qm.Where("1=1"), // No filtering - index all tweets
		qm.OrderBy("id"),
	}
}

// fetchTweets loads tweets from MDB with the given scope
func (idx *TweetsIndexer) FetchTweets(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.TwitterTweet, error) {
	tweets, err := mdbmodels.TwitterTweets(scope...).All(idx.db)
	if err != nil {
		return nil, errors.Wrap(err, "query twitter_tweets")
	}
	return tweets, nil
}

// deleteExistingTweets deletes all tweet documents from all language indices
func (idx *TweetsIndexer) deleteExistingTweets(ctx context.Context) error {
	languages := consts.ALL_KNOWN_LANGS[:]
	log.Infof("Deleting existing tweets from %d language indices", len(languages))

	var wg sync.WaitGroup
	errChan := make(chan error, len(languages))

	for _, lang := range languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

			// Delete by result type
			_, err := idx.manager.DeleteByResultType(ctx, indexName, consts.ES_RESULT_TYPE_TWEETS)
			if err != nil {
				errChan <- errors.Wrapf(err, "delete tweets from %s", indexName)
				return
			}

			log.Debugf("✓ Deleted tweets from: %s", indexName)
		}(lang)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return <-errChan
	}

	log.Info("✓ Deleted existing tweets from all language indices")
	return nil
}

// filterExistingTweets filters out tweets that are already indexed
func (idx *TweetsIndexer) FilterExistingTweets(ctx context.Context, tweets []*mdbmodels.TwitterTweet) ([]*mdbmodels.TwitterTweet, error) {
	if len(tweets) == 0 {
		return tweets, nil
	}

	// Group tweets by language
	tweetsByLang := make(map[string]map[string]*mdbmodels.TwitterTweet)
	langMapping := idx.userIdToLanguageMapping()

	for _, tweet := range tweets {
		lang := langMapping[int(tweet.UserID)]
		if tweetsByLang[lang] == nil {
			tweetsByLang[lang] = make(map[string]*mdbmodels.TwitterTweet)
		}
		tweetsByLang[lang][tweet.TwitterID] = tweet
	}

	// Check each language index
	var newTweets []*mdbmodels.TwitterTweet
	for lang, tweetsMap := range tweetsByLang {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

		// Check which UIDs already exist
		existingUIDs, err := idx.manager.GetExistingUIDs(ctx, indexName, consts.ES_RESULT_TYPE_TWEETS)
		if err != nil {
			return nil, errors.Wrapf(err, "check existing tweets in %s", indexName)
		}

		// Add tweets that don't exist yet
		for uid, tweet := range tweetsMap {
			if !existingUIDs[uid] {
				newTweets = append(newTweets, tweet)
			}
		}
	}

	return newTweets, nil
}
