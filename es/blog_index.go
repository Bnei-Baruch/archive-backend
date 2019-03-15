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
		1: consts.LANG_RUSSIAN,
		2: consts.LANG_ENGLISH,
		3: consts.LANG_SPANISH,
		4: consts.LANG_HEBREW,
	}
}

func (index *BlogIndex) ReindexAll() error {
	log.Info("BlogIndex.Reindex All.")
	if _, err := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_BLOG_POSTS)); err != nil {
		return err
	}
	return index.addToIndexSql(defaultBlogPostsSql())
}

func (index *BlogIndex) Update(scope Scope) error {
	log.Debugf("BlogIndex.Update - Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	// We want to run addToIndex anyway and return joint error.
	return utils.JoinErrors(err, index.addToIndex(scope, removed))
}

func (index *BlogIndex) addToIndex(scope Scope, removedPosts []string) error {
	sqlScope := defaultBlogPostsSql()
	ids := removedPosts
	if scope.BlogPostWPID != "" {
		ids = append(ids, scope.BlogPostWPID)
	}
	if len(ids) == 0 {
		return nil
	}
	quoted := make([]string, len(ids))
	for i, id := range ids {
		s := strings.Split(id, "-")
		blogId := s[0]
		wpId := s[1]
		quoted[i] = fmt.Sprintf("(p.blog_id = %s and p.wp_id = %s)", blogId, wpId)
	}
	sqlScope = fmt.Sprintf("%s AND (%s)", sqlScope, strings.Join(quoted, " or "))
	if err := index.addToIndexSql(sqlScope); err != nil {
		return errors.Wrap(err, "blog posts index addToIndex addToIndexSql")
	}
	return nil
}

func (index *BlogIndex) removeFromIndex(scope Scope) ([]string, error) {
	if scope.BlogPostWPID != "" {
		elasticScope := index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_BLOG_POSTS).
			Filter(elastic.NewTermsQuery("mdb_uid", scope.BlogPostWPID))
		return index.RemoveFromIndexQuery(elasticScope)
	}

	// Nothing to remove.
	return []string{}, nil
}

func (index *BlogIndex) bulkIndexPosts(bulk OffsetLimitJob, sqlScope string) error {
	var posts []*mdbmodels.BlogPost
	err := mdbmodels.NewQuery(index.db,
		qm.From("blog_posts as p"),
		qm.Where(sqlScope),
		qm.OrderBy("id"), // Required for same order results in each query
		qm.Offset(bulk.Offset),
		qm.Limit(bulk.Limit)).Bind(&posts)
	if err != nil {
		log.Errorf("indexPost error at offset %d. error: %v", bulk.Offset, err)
		return errors.Wrap(err, "Fetch blog posts from mdb.")
	}
	log.Infof("Adding %d blog posts (offset %d total %d).", len(posts), bulk.Offset, bulk.Total)
	for _, post := range posts {
		err = utils.JoinErrors(err, index.indexPost(post))
	}
	if err != nil {
		log.Errorf("indexPost error at post bulk. error: %v", err)
		return err
	}
	return nil
}

func (index *BlogIndex) addToIndexSql(sqlScope string) error {
	var count int
	if err := mdbmodels.NewQuery(index.db,
		qm.Select("count(id)"),
		qm.From("blog_posts as p"),
		qm.Where(sqlScope)).QueryRow().Scan(&count); err != nil {
		return err
	}
	log.Infof("Blog Posts Index - Adding %d posts. Scope: %s.", count, sqlScope)

	limit := 1000
	tasks := make(chan OffsetLimitJob, (count/limit + limit))
	errors := make(chan error, 300)
	doneAdding := make(chan bool, 1)

	tasksCount := 0
	go func() {
		offset := 0
		for offset < int(count) {
			tasks <- OffsetLimitJob{offset, limit, count}
			tasksCount++
			offset += limit
		}
		close(tasks)
		doneAdding <- true
	}()

	for w := 1; w <= 10; w++ {
		go func(tasks <-chan OffsetLimitJob, errs chan<- error) {
			for task := range tasks {
				errors <- index.bulkIndexPosts(task, sqlScope)
			}
		}(tasks, errors)
	}
	<-doneAdding
	err := (error)(nil)
	for a := 1; a <= tasksCount; a++ {
		err = utils.JoinErrors(err, <-errors)
	}
	if err != nil {
		log.Errorf("tasksCount loop error: %v", err)
		return err
	}

	return nil
}

func (index *BlogIndex) indexPost(mdbPost *mdbmodels.BlogPost) error {
	langMapping := index.blogIdToLanguageMapping()
	postLang := langMapping[int(mdbPost.BlogID)]

	// Blog Id + WPID is taken instead of ID for the building of correct URL in frontend.
	// The API BlogPostHandler expects for Blog Name + WPID and not for ID.
	idStr := fmt.Sprintf("%v-%v", mdbPost.BlogID, mdbPost.WPID)

	content, err := html2text.FromString(mdbPost.Content, html2text.Options{OmitLinks: true})
	if err != nil {
		return errors.Wrapf(err, " blog_id: %d", mdbPost.BlogID)
	}

	post := Result{
		ResultType:    consts.ES_RESULT_TYPE_BLOG_POSTS,
		MDB_UID:       idStr,
		TypedUids:     []string{keyValue("blog_post", idStr)},
		FilterValues:  []string{keyValue("content_type", consts.SCT_BLOG_POST), keyValue(consts.FILTER_LANGUAGE, postLang)},
		Title:         mdbPost.Title,
		TitleSuggest:  Suffixes(mdbPost.Title),
		EffectiveDate: &utils.Date{Time: mdbPost.PostedAt},
		Content:       content,
	}

	indexName := index.indexName(postLang)
	vBytes, err := json.Marshal(post)
	if err != nil {
		return errors.Wrapf(err, " blog_id: %d", mdbPost.BlogID)
	}
	log.Debugf("Blog Posts Index - Add blog post %s to index %s", string(vBytes), indexName)
	resp, err := index.esc.Index().
		Index(indexName).
		Type("result").
		BodyJson(post).
		Do(context.TODO())
	if err != nil {
		return errors.Wrapf(err, "Index blog post %s %s", indexName, idStr)
	}
	if resp.Result != "created" {
		return errors.Errorf("Not created: blog post %s %s", indexName, idStr)
	}

	atomic.AddUint64(&index.Progress, 1)
	progress := atomic.LoadUint64(&index.Progress)
	if progress%1000 == 0 {
		log.Debugf("Progress blog posts %d", progress)
	}

	return nil
}
