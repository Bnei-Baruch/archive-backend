package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
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
	log.Infof("TweeterIndex.Reindex All.")
	if _, err := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_TWEETS)); err != nil {
		return err
	}
	return index.addToIndexSql("1=1") // SQL to always match any tweet
}

func (index *TweeterIndex) Update(scope Scope) error {
	log.Infof("TweeterIndex.Update - Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *TweeterIndex) addToIndex(scope Scope, removedIDs []string) error {
	ids := removedIDs
	if scope.TweetTID != "" {
		ids = append(ids, scope.TweetTID)
	}
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%s", id)
	}
	sqlScope := fmt.Sprintf("p.wp_id IN (%s)", strings.Join(quoted, ","))
	if err := index.addToIndexSql(sqlScope); err != nil {
		return errors.Wrap(err, "tweets index addToIndex addToIndexSql")
	}
	return nil
}

func (index *TweeterIndex) removeFromIndex(scope Scope) ([]string, error) {
	if scope.TweetTID != "" {
		elasticScope := index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_TWEETS).
			Filter(elastic.NewTermsQuery("mdb_uid", scope.TweetTID))
		return index.RemoveFromIndexQuery(elasticScope)
	}

	// Nothing to remove.
	return []string{}, nil
}

func (index *TweeterIndex) bulkIndexTweets(offset int, limit int, sqlScope string) error {
	var tweets []*mdbmodels.TwitterTweet
	err := mdbmodels.NewQuery(index.db,
		qm.From("twitter_tweets as t"),
		qm.Where(sqlScope),
		qm.Offset(offset),
		qm.Limit(limit)).Bind(&tweets)
	if err != nil {
		log.Errorf("bulkIndexTweets error at offset %d. error: %v", offset, err)
		return errors.Wrap(err, "Fetch tweetsfrom mdb.")
	}
	log.Infof("Adding %d tweets (offset %d).", len(tweets), offset)
	for _, tweet := range tweets {
		if err := index.indexTweet(tweet); err != nil {
			log.Errorf("indexTweet error at tweet id %d. error: %v", tweet.ID, err)
			return err
		}
	}
	log.Info("Indexing tweets - finished.")
	return nil
}

func (index *TweeterIndex) addToIndexSql(sqlScope string) error {
	var count int64
	if err := mdbmodels.NewQuery(index.db,
		qm.Select("count(id)"),
		qm.From("twitter_tweets as t"),
		qm.Where(sqlScope)).QueryRow().Scan(&count); err != nil {
		return err
	}
	log.Infof("Tweeter Index - Adding %d tweets. Scope: %s.", count, sqlScope)

	limit := 20
	tasks := make(chan OffsetLimitJob, (count/int64(limit) + int64(limit)))
	errors := make(chan error, 300)
	doneAdding := make(chan bool, 1)

	tasksCount := 0
	go func() {
		offset := 0
		for offset < int(count) {
			tasks <- OffsetLimitJob{offset, limit}
			tasksCount++
			offset += limit
		}
		close(tasks)
		doneAdding <- true
	}()

	for w := 1; w <= 10; w++ {
		go func(tasks <-chan OffsetLimitJob, errs chan<- error) {
			for task := range tasks {
				errors <- index.bulkIndexTweets(task.Offset, task.Limit, sqlScope)
			}
		}(tasks, errors)
	}
	<-doneAdding
	for a := 1; a <= tasksCount; a++ {
		e := <-errors
		if e != nil {
			log.Errorf("tasksCount loop error: %v", e)
			return e
		}
	}

	return nil
}

func (index *TweeterIndex) indexTweet(mdbTweet *mdbmodels.TwitterTweet) error {

	langMapping := index.userIdToLanguageMapping()

	title := ""
	if mdbTweet.Raw.Valid {
		var raw interface{}
		err := json.Unmarshal(mdbTweet.Raw.JSON, &raw)
		if err != nil {
			return errors.Wrapf(err, "Cannot unmarshal raw from tweet id %d", mdbTweet.ID)
		}
		r := raw.(map[string]interface{})
		if val, ok := r["text"]; ok {
			title = val.(string)
		}
	}

	tweet := Result{
		ResultType:    consts.ES_RESULT_TYPE_TWEETS,
		MDB_UID:       mdbTweet.TwitterID, // TwitterID is taken instead of ID
		TypedUids:     []string{keyValue("tweet", mdbTweet.TwitterID)},
		FilterValues:  []string{keyValue("content_type", consts.CT_TWEET)},
		Title:         title,
		TitleSuggest:  Suffixes(title),
		EffectiveDate: &utils.Date{Time: mdbTweet.TweetAt},
		Content:       mdbTweet.FullText,
	}

	indexName := index.indexName(langMapping[int(mdbTweet.UserID)])
	vBytes, err := json.Marshal(tweet)
	if err != nil {
		return err
	}
	log.Infof("Tweets Index - Add tweet %s to index %s", string(vBytes), indexName)
	resp, err := index.esc.Index().
		Index(indexName).
		Type("result").
		BodyJson(tweet).
		Do(context.TODO())
	if err != nil {
		return errors.Wrapf(err, "Index tweet %s %s", indexName, mdbTweet.ID)
	}
	if resp.Result != "created" {
		return errors.Errorf("Not created: tweet %s %s", indexName, mdbTweet.ID)
	}

	atomic.AddUint64(&index.Progress, 1)
	progress := atomic.LoadUint64(&index.Progress)
	if progress%10 == 0 {
		log.Infof("Progress tweet %d", progress)
	}

	return nil
}
