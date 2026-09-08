package types

import (
	"context"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
)

// UpdateScope incrementally reindexes the blog-post doc affected by an MDB change.
// Mirrors es6 BlogIndex.Update: posts respond only to a BlogPostWPID ("<blogID>-<wpID>") scope.
func (idx *BlogPostsIndexer) UpdateScope(ctx context.Context, scope es.Scope) error {
	if scope.BlogPostWPID == "" {
		return nil
	}
	removed, err := idx.removeFromScope(ctx, scope)
	if err != nil {
		return err
	}
	ids := dedupStrings(append([]string{scope.BlogPostWPID}, removed...))
	where := blogPostsWhere(ids)
	if where == nil {
		return nil
	}
	posts, err := idx.FetchBlogPosts(ctx, append(DefaultBlogPostsScope(), where))
	if err != nil {
		return errors.Wrap(err, "fetch blog posts by scope")
	}
	return idx.indexBlogPosts(ctx, posts)
}

func (idx *BlogPostsIndexer) removeFromScope(ctx context.Context, scope es.Scope) ([]string, error) {
	typedUids := []string{es.KeyValue(consts.ES_UID_TYPE_BLOG_POST, scope.BlogPostWPID)}
	var removed []string
	for _, lang := range idx.GetLanguages() {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)
		uids, err := idx.manager.DeleteByTypedUids(ctx, indexName, consts.ES_RESULT_TYPE_BLOG_POSTS, typedUids)
		if err != nil {
			log.Warnf("BlogPost incremental - failed delete from %s: %v", indexName, err)
			continue
		}
		removed = append(removed, uids...)
	}
	return removed, nil
}

// blogPostsWhere turns "<blogID>-<wpID>" ids into an OR of (blog_id, wp_id) predicates.
func blogPostsWhere(ids []string) qm.QueryMod {
	clauses := make([]string, 0, len(ids))
	for _, id := range ids {
		parts := strings.SplitN(id, "-", 2)
		if len(parts) == 2 {
			clauses = append(clauses, fmt.Sprintf("(blog_id = %s AND wp_id = %s)", parts[0], parts[1]))
		}
	}
	if len(clauses) == 0 {
		return nil
	}
	return qm.Where(strings.Join(clauses, " OR "))
}

func (idx *BlogPostsIndexer) indexBlogPosts(ctx context.Context, posts []*mdbmodels.BlogPost) error {
	if len(posts) == 0 {
		return nil
	}
	items := make([]interface{}, len(posts))
	for i, p := range posts {
		items[i] = p
	}
	pipeline := indexing.NewPipeline(idx.manager, nil)
	return pipeline.RunPipeline(ctx, idx, items, &es.IndexData{})
}
