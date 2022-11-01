package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeTweeterIndex(namespace string, indexDate string, db *sql.DB, esc *elastic.Client) *TweeterIndex {
	ti := new(TweeterIndex)
	ti.resultType = consts.ES_RESULT_TYPE_TWEETS
	ti.baseName = consts.ES_RESULTS_INDEX
	ti.namespace = namespace
	ti.indexDate = indexDate
	ti.db = db
	ti.esc = esc
	return ti
}

type TweeterIndex struct {
	BaseIndex
	Progress uint64
}

func (index *TweeterIndex) userIdToLanguageMapping() map[int]string {
	return map[int]string{
		1: consts.LANG_RUSSIAN,
		2: consts.LANG_HEBREW,
		3: consts.LANG_ENGLISH,
		4: consts.LANG_SPANISH,
	}
}

func (index *TweeterIndex) ReindexAll() error {
	log.Info("TweeterIndex.Reindex All.")
	_, indexErrors := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(index.resultType))
	if err := indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TweeterIndex"); err != nil {
		return err
	}
	// SQL to always match any tweet
	return indexErrors.Join(index.addToIndexSql("1=1"), "").CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TweeterIndex")
}

func (index *TweeterIndex) RemoveFromIndex(scope Scope) (map[string][]string, error) {
	log.Debugf("TweeterIndex.RemoveFromIndex - Scope: %+v.", scope)
	removed, indexErrors := index.removeFromIndex(scope)
	return removed, indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TweeterIndex")
}

func (index *TweeterIndex) AddToIndex(scope Scope, removedUIDs []string) error {
	log.Debugf("TweeterIndex.AddToIndex - Scope: %+v, removedUIDs: %+v.", scope, removedUIDs)
	return index.addToIndex(scope, removedUIDs).CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TweeterIndex")
}

func (index *TweeterIndex) addToIndex(scope Scope, removedIDs []string) *IndexErrors {
	ids := removedIDs
	if scope.TweetTID != "" {
		ids = append(ids, scope.TweetTID)
	}
	if len(ids) == 0 {
		return MakeIndexErrors()
	}
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("'%s'", id)
	}
	sqlScope := fmt.Sprintf("t.twitter_id IN (%s)", strings.Join(quoted, ","))
	return index.addToIndexSql(sqlScope)
}

func (index *TweeterIndex) removeFromIndex(scope Scope) (map[string][]string, *IndexErrors) {
	if scope.TweetTID != "" {
		elasticScope := index.FilterByResultTypeQuery(index.resultType).
			Filter(elastic.NewTermsQuery("mdb_uid", scope.TweetTID))
		return index.RemoveFromIndexQuery(elasticScope)
	}

	// Nothing to remove.
	return make(map[string][]string), MakeIndexErrors()
}

func (index *TweeterIndex) bulkIndexTweets(bulk OffsetLimitJob, sqlScope string) *IndexErrors {
	var tweets []*mdbmodels.TwitterTweet
	if err := mdbmodels.NewQuery(
		qm.From("twitter_tweets as t"),
		qm.Where(sqlScope),
		qm.OrderBy("id"), // Required for same order results in each query
		qm.Offset(bulk.Offset),
		qm.Limit(bulk.Limit)).Bind(nil, index.db, &tweets); err != nil {
		return MakeIndexErrors().SetError(err).Wrap(fmt.Sprintf("bulkIndexTweets error at offset %d. error: %v", bulk.Offset, err))
	}
	log.Infof("Adding %d tweets (offset %d, total %d).", len(tweets), bulk.Offset, bulk.Total)
	indexErrors := MakeIndexErrors()
	for _, tweet := range tweets {
		indexErrors.Join(index.indexTweet(tweet), "")
	}
	indexErrors.PrintIndexCounts(fmt.Sprintf("TweeterIndex %d - %d", bulk.Offset, bulk.Offset+bulk.Limit))
	return indexErrors
}

func (index *TweeterIndex) addToIndexSql(sqlScope string) *IndexErrors {
	var count int
	if err := mdbmodels.NewQuery(
		qm.Select("count(id)"),
		qm.From("twitter_tweets as t"),
		qm.Where(sqlScope)).QueryRow(index.db).Scan(&count); err != nil {
		return MakeIndexErrors().SetError(err).Wrap(fmt.Sprintf("Failed TwitterIndex addToIndexSql: %s", sqlScope))
	}
	log.Debugf("Tweeter Index - Adding %d tweets. Scope: %s.", count, sqlScope)

	limit := utils.MaxInt(10, utils.MinInt(1000, count/10))
	tasks := make(chan OffsetLimitJob, count/limit+limit)
	errChan := make(chan *IndexErrors, 300)
	doneAdding := make(chan bool, 1)

	tasksCount := 0
	go func() {
		offset := 0
		for offset < count {
			tasks <- OffsetLimitJob{offset, limit, count}
			tasksCount++
			offset += limit
		}
		close(tasks)
		doneAdding <- true
	}()

	for w := 1; w <= 10; w++ {
		go func(tasks <-chan OffsetLimitJob, errs chan<- *IndexErrors) {
			for task := range tasks {
				errs <- index.bulkIndexTweets(task, sqlScope)
			}
		}(tasks, errChan)
	}
	<-doneAdding
	indexErrors := MakeIndexErrors()
	for a := 1; a <= tasksCount; a++ {
		indexErrors.Join(<-errChan, "")
	}
	return indexErrors
}

func (index *TweeterIndex) indexTweet(mdbTweet *mdbmodels.TwitterTweet) *IndexErrors {
	langMapping := index.userIdToLanguageMapping()
	tweetLang := langMapping[int(mdbTweet.UserID)]

	indexErrors := MakeIndexErrors().ShouldIndex(tweetLang)

	tweet := Result{
		ResultType:    index.resultType,
		IndexDate:     &utils.Date{Time: time.Now()},
		MDB_UID:       mdbTweet.TwitterID, // TwitterID is taken instead of UID
		TypedUids:     []string{KeyValue(consts.ES_UID_TYPE_TWEET, mdbTweet.TwitterID)},
		FilterValues:  []string{KeyValue("content_type", consts.SCT_TWEET), KeyValue(consts.FILTER_MEDIA_LANGUAGE, tweetLang)},
		Title:         "",
		EffectiveDate: &utils.Date{Time: mdbTweet.TweetAt},
		Content:       mdbTweet.FullText,
		TitleSuggest:  SuggestField{[]string{}, float64(0)},
	}

	indexName := index.IndexName(tweetLang)
	vBytes, err := json.Marshal(tweet)
	indexErrors.DocumentError(tweetLang, err, fmt.Sprintf("Failed marshling tweet: %d", mdbTweet.ID))
	if err != nil {
		return indexErrors
	}
	log.Debugf("Tweets Index - Add tweet %s to index %s", string(vBytes), indexName)
	resp, err := index.esc.Index().
		Index(indexName).
		Type("result").
		BodyJson(tweet).
		Do(context.TODO())
	indexErrors.DocumentError(tweetLang, err, fmt.Sprintf("Index tweet %s %d", indexName, mdbTweet.ID))
	if err != nil {
		return indexErrors
	}
	errNotCreated := (error)(nil)
	if resp.Result != "created" {
		errNotCreated = errors.New(fmt.Sprintf("Not created: tweet %s %d", indexName, mdbTweet.ID))
	} else {
		indexErrors.Indexed(tweetLang)
	}
	indexErrors.DocumentError(tweetLang, errNotCreated, "TweeterIndex")

	atomic.AddUint64(&index.Progress, 1)
	progress := atomic.LoadUint64(&index.Progress)
	if progress%10 == 0 {
		log.Debugf("Progress tweet %d", progress)
	}

	return indexErrors
}
