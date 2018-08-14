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
	"jaytaylor.com/html2text"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeBlogIndex(namespace string, indexDate string, db *sql.DB, esc *elastic.Client) *BlogIndex {
	bi := new(BlogIndex)
	bi.resultType = consts.ES_RESULT_TYPE_BLOG_POSTS
	bi.baseName = consts.ES_RESULTS_INDEX
	bi.namespace = namespace
	bi.indexDate = indexDate
	bi.db = db
	bi.esc = esc
	return bi
}

type BlogIndex struct {
	BaseIndex
	Progress uint64
}

func defaultBlogPostsSql() string {
	return "p.filtered = false"
}

func (index *BlogIndex) blogIdToLanguageMapping() map[int]string {
	return map[int]string{
		1: "ru",
		2: "en",
		3: "es",
		4: "he",
	}
}

func (index *BlogIndex) ReindexAll() error {
	log.Infof("BlogIndex.Reindex All.")
	if _, err := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_BLOG_POSTS)); err != nil {
		return err
	}
	return index.addToIndexSql(defaultBlogPostsSql())
}

func (index *BlogIndex) Update(scope Scope) error {
	log.Infof("BlogIndex.Update - Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *BlogIndex) addToIndex(scope Scope, removedUIDs []string) error {
	sqlScope := defaultBlogPostsSql()
	uids := removedUIDs
	if scope.BlogPostID != "" {
		uids = append(uids, scope.BlogPostID)
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	sqlScope = fmt.Sprintf("%s AND p.id IN (%s)", sqlScope, strings.Join(quoted, ","))
	if err := index.addToIndexSql(sqlScope); err != nil {
		return errors.Wrap(err, "blog posts index addToIndex addToIndexSql")
	}
	return nil
}

func (index *BlogIndex) removeFromIndex(scope Scope) ([]string, error) {
	if scope.BlogPostID != "" {
		elasticScope := index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_BLOG_POSTS).
			Filter(elastic.NewTermsQuery("mdb_uid", scope.BlogPostID))
		return index.RemoveFromIndexQuery(elasticScope)
	}

	// Nothing to remove.
	return []string{}, nil
}

func (index *BlogIndex) bulkIndexPosts(offset int, limit int, sqlScope string) error {
	var posts []*mdbmodels.BlogPost
	err := mdbmodels.NewQuery(index.db,
		qm.From("blog_posts as p"),
		qm.Where(sqlScope),
		qm.Offset(offset),
		qm.Limit(limit)).Bind(&posts)
	if err != nil {
		return errors.Wrap(err, "Fetch blog posts from mdb.")
	}
	log.Infof("Adding %d blog posts (offset %d).", len(posts), offset)
	for _, post := range posts {
		if err := index.indexPost(post); err != nil {
			return err
		}
	}
	return nil
}

func (index *BlogIndex) addToIndexSql(sqlScope string) error {
	var count int64
	if err := mdbmodels.NewQuery(index.db,
		qm.Select("count(id)"),
		qm.From("blog_posts as p"),
		qm.Where(sqlScope)).QueryRow().Scan(&count); err != nil {
		return err
	}
	log.Infof("Blog Posts Index - Adding %d posts. Scope: %s.", count, sqlScope)

	tasks := make(chan OffsetLimitJob, 300)
	errors := make(chan error, 300)
	doneAdding := make(chan bool)

	tasksCount := 0
	go func() {
		offset := 0
		limit := 20
		for offset < int(count) {
			tasks <- OffsetLimitJob{offset, limit}
			tasksCount += 1
			offset += limit
		}
		close(tasks)
		doneAdding <- true
	}()

	for w := 1; w <= 10; w++ {
		go func(tasks <-chan OffsetLimitJob, errors chan<- error) {
			for task := range tasks {
				errors <- index.bulkIndexPosts(task.Offset, task.Limit, sqlScope)
			}
		}(tasks, errors)
	}

	<-doneAdding
	for a := 1; a <= tasksCount; a++ {
		e := <-errors
		if e != nil {
			return e
		}
	}

	return nil
}

func (index *BlogIndex) indexPost(mdbPost *mdbmodels.BlogPost) error {

	langMapping := index.blogIdToLanguageMapping()
	idStr := fmt.Sprintf("%v", mdbPost.BlogID)

	content, err := html2text.FromString(mdbPost.Content, html2text.Options{OmitLinks: true})

	post := Result{
		ResultType:    consts.ES_RESULT_TYPE_BLOG_POSTS,
		MDB_UID:       idStr,
		TypedUids:     []string{keyValue("blog_post", idStr)},
		FilterValues:  []string{keyValue("content_type", consts.CT_BLOG_POST)},
		Title:         mdbPost.Title,
		TitleSuggest:  Suffixes(mdbPost.Title),
		EffectiveDate: &utils.Date{Time: mdbPost.PostedAt},
		Content:       content,
	}

	indexName := index.indexName(langMapping[int(mdbPost.BlogID)])
	vBytes, err := json.Marshal(post)
	if err != nil {
		return err
	}
	log.Infof("Blog Posts Index - Add blog post %s to index %s", string(vBytes), indexName)
	resp, err := index.esc.Index().
		Index(indexName).
		Type("result").
		BodyJson(post).
		Do(context.TODO())
	if err != nil {
		return errors.Wrapf(err, "Index blog post %s %s", indexName, mdbPost.ID)
	}
	if resp.Result != "created" {
		return errors.Errorf("Not created: blog post %s %s", indexName, mdbPost.ID)
	}

	atomic.AddUint64(&index.Progress, 1)
	progress := atomic.LoadUint64(&index.Progress)
	if progress%10 == 0 {
		log.Infof("Progress blog posts %d", progress)
	}

	return nil
}
