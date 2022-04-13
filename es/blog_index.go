package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"gopkg.in/olivere/elastic.v6"
	"jaytaylor.com/html2text"

	"github.com/Bnei-Baruch/archive-backend/consts"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
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
	_, indexErrors := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(index.resultType))
	if err := indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "BlogIndex"); err != nil {
		return err
	}
	return indexErrors.Join(index.addToIndexSql(defaultBlogPostsSql()), "").CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "BlogIndex")
}

func (index *BlogIndex) RemoveFromIndex(scope Scope) (map[string][]string, error) {
	log.Debugf("BlogIndex.RemovedFromIndex - Scope: %+v.", scope)
	removed, indexErrors := index.removeFromIndex(scope)
	return removed, indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "BlogIndex")
}

func (index *BlogIndex) AddToIndex(scope Scope, removedUIDs []string) error {
	log.Debugf("BlogIndex.AddToIndex - Scope: %+v, removedUIDs: %+v.", scope, removedUIDs)
	return index.addToIndex(scope, removedUIDs).CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "BlogIndex")
}

func (index *BlogIndex) addToIndex(scope Scope, removedPosts []string) *IndexErrors {
	sqlScope := defaultBlogPostsSql()
	ids := removedPosts
	if scope.BlogPostWPID != "" {
		ids = append(ids, scope.BlogPostWPID)
	}
	if len(ids) == 0 {
		return MakeIndexErrors()
	}
	quoted := make([]string, len(ids))
	for i, id := range ids {
		s := strings.Split(id, "-")
		blogId := s[0]
		wpId := s[1]
		quoted[i] = fmt.Sprintf("(p.blog_id = %s and p.wp_id = %s)", blogId, wpId)
	}
	sqlScope = fmt.Sprintf("%s AND (%s)", sqlScope, strings.Join(quoted, " or "))
	return index.addToIndexSql(sqlScope).Wrap("blog posts index addToIndex addToIndexSql")
}

func (index *BlogIndex) removeFromIndex(scope Scope) (map[string][]string, *IndexErrors) {
	if scope.BlogPostWPID != "" {
		elasticScope := index.FilterByResultTypeQuery(index.resultType).
			Filter(elastic.NewTermsQuery("mdb_uid", scope.BlogPostWPID))
		return index.RemoveFromIndexQuery(elasticScope)
	}

	// Nothing to remove.
	return make(map[string][]string), MakeIndexErrors()
}

func (index *BlogIndex) bulkIndexPosts(bulk OffsetLimitJob, sqlScope string) *IndexErrors {
	var posts []*mdbmodels.BlogPost
	if err := mdbmodels.NewQuery(
		qm.From("blog_posts as p"),
		qm.Where(sqlScope),
		qm.OrderBy("id"), // Required for same order results in each query
		qm.Offset(bulk.Offset),
		qm.Limit(bulk.Limit)).Bind(nil, index.db, &posts); err != nil {
		return MakeIndexErrors().SetError(err).Wrap(fmt.Sprintf("Fetch blog posts from mdb. Offset: %d", bulk.Offset))
	}
	log.Infof("Adding %d blog posts (offset %d total %d).", len(posts), bulk.Offset, bulk.Total)
	indexErrors := MakeIndexErrors()
	for _, post := range posts {
		indexErrors.Join(index.indexPost(post), "BlogIndex, bulkIndexPosts")
	}
	indexErrors.PrintIndexCounts(fmt.Sprintf("BlogIndex %d - %d", bulk.Offset, bulk.Offset+bulk.Limit))
	return indexErrors
}

func (index *BlogIndex) addToIndexSql(sqlScope string) *IndexErrors {
	var count int
	if err := mdbmodels.NewQuery(
		qm.Select("count(id)"),
		qm.From("blog_posts as p"),
		qm.Where(sqlScope)).QueryRow(index.db).Scan(&count); err != nil {
		return MakeIndexErrors().SetError(err)
	}
	log.Infof("Blog Posts Index - Adding %d posts. Scope: %s.", count, sqlScope)

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
				errs <- index.bulkIndexPosts(task, sqlScope)
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

func (index *BlogIndex) indexPost(mdbPost *mdbmodels.BlogPost) *IndexErrors {
	langMapping := index.blogIdToLanguageMapping()
	postLang := langMapping[int(mdbPost.BlogID)]

	// Blog Id + WPID is taken instead of ID for the building of correct URL in frontend.
	// The API BlogPostHandler expects for Blog Name + WPID and not for ID.
	idStr := fmt.Sprintf("%v-%v", mdbPost.BlogID, mdbPost.WPID)

	indexErrors := MakeIndexErrors().ShouldIndex(postLang)
	content, err := html2text.FromString(mdbPost.Content, html2text.Options{OmitLinks: true})
	indexErrors.DocumentError(postLang, err, fmt.Sprintf("BlogIndex, indexPost, FromString, blog_id: %d", mdbPost.BlogID))
	if err != nil {
		return indexErrors
	}

	post := Result{
		ResultType:    index.resultType,
		IndexDate:     &utils.Date{Time: time.Now()},
		MDB_UID:       idStr,
		TypedUids:     []string{KeyValue(consts.ES_UID_TYPE_BLOG_POST, idStr)},
		FilterValues:  []string{KeyValue("content_type", consts.SCT_BLOG_POST), KeyValue(consts.FILTER_LANGUAGE, postLang)},
		Title:         html.UnescapeString(mdbPost.Title),
		TitleSuggest:  SuggestField{Suffixes(mdbPost.Title), float64(1)},
		EffectiveDate: &utils.Date{Time: mdbPost.PostedAt},
		Content:       html.UnescapeString(content),
	}

	indexName := index.IndexName(postLang)
	vBytes, err := json.Marshal(post)
	indexErrors.DocumentError(postLang, err, fmt.Sprintf("BlogIndex, indexPost, Marshal, blog_id: %d", mdbPost.BlogID))
	if err != nil {
		return indexErrors
	}
	log.Debugf("Blog Posts Index - Add blog post %s to index %s", string(vBytes), indexName)
	resp, err := index.esc.Index().
		Index(indexName).
		Type("result").
		BodyJson(post).
		Do(context.TODO())
	indexErrors.DocumentError(postLang, err, fmt.Sprintf("BlogIndex, indexPost, Index blog post %s %s", indexName, idStr))
	if err != nil {
		return indexErrors
	}
	errNotCreated := (error)(nil)
	if resp.Result != "created" {
		errNotCreated = errors.Errorf("Not created: blog post %s %s", indexName, idStr)
	} else {
		indexErrors.Indexed(postLang)
	}
	indexErrors.DocumentError(postLang, errNotCreated, "BlogIndex")

	atomic.AddUint64(&index.Progress, 1)
	progress := atomic.LoadUint64(&index.Progress)
	if progress%1000 == 0 {
		log.Debugf("Progress blog posts %d", progress)
	}

	return indexErrors
}
