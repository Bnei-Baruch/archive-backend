package types

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"jaytaylor.com/html2text"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

// BlogPostsIndexer implements the Indexer interface for blog posts
type BlogPostsIndexer struct {
	manager       *common.ES9Manager
	db            *sql.DB
	indexNameBase string
}

// NewBlogPostsIndexer creates a new blog posts indexer
func NewBlogPostsIndexer(mgr *common.ES9Manager, db *sql.DB, indexNameBase string) *BlogPostsIndexer {
	if indexNameBase == "" {
		indexNameBase = "results" // Blog posts share the same index as other result types
	}
	return &BlogPostsIndexer{
		manager:       mgr,
		db:            db,
		indexNameBase: indexNameBase,
	}
}

// GetIndexNameBase returns the base name for indices
func (idx *BlogPostsIndexer) GetIndexNameBase() string {
	return idx.indexNameBase
}

// GetLanguages returns all known languages
func (idx *BlogPostsIndexer) GetLanguages() []string {
	return consts.ALL_KNOWN_LANGS[:]
}

// GetDocumentType returns the document type name for logging
func (idx *BlogPostsIndexer) GetDocumentType() string {
	return "blog post"
}

// blogIdToLanguageMapping maps blog IDs to language codes
func (idx *BlogPostsIndexer) blogIdToLanguageMapping() map[int]string {
	return map[int]string{
		1: consts.LANG_RUSSIAN,
		2: consts.LANG_ENGLISH,
		3: consts.LANG_SPANISH,
		4: consts.LANG_HEBREW,
	}
}

// PrepareDocument converts a blog post to an ES9 Result document
func (idx *BlogPostsIndexer) PrepareDocument(ctx context.Context, item interface{}, lang string, indexData *es.IndexData, indexDate *utils.Date, progress *indexing.ProgressTracker) (doc *es.Result, skip bool) {
	post, ok := item.(*mdbmodels.BlogPost)
	if !ok {
		log.Errorf("Invalid item type for blog posts indexer: %T", item)
		return nil, true
	}

	// Map blog ID to language
	langMapping := idx.blogIdToLanguageMapping()
	postLang := langMapping[int(post.BlogID)]

	// Skip if not the requested language
	if lang != postLang {
		return nil, true
	}

	// Convert HTML to text inline (happens during document preparation in pipeline)
	content, err := html2text.FromString(post.Content, html2text.Options{OmitLinks: true})
	if err != nil {
		log.Errorf("Failed to convert HTML to text for blog post %d: %v", post.ID, err)
		return nil, true
	}

	// Blog Id + WPID is used instead of ID for building correct URL in frontend
	idStr := fmt.Sprintf("%v-%v", post.BlogID, post.WPID)

	// Create document
	doc = &es.Result{
		ResultType:    consts.ES_RESULT_TYPE_BLOG_POSTS,
		IndexDate:     indexDate,
		MDB_UID:       idStr,
		TypedUids:     []string{es.KeyValue(consts.ES_UID_TYPE_BLOG_POST, idStr)},
		FilterValues:  []string{es.KeyValue("content_type", consts.CT_BLOG_POST), es.KeyValue(consts.FILTER_MEDIA_LANGUAGE, postLang)},
		Title:         html.UnescapeString(post.Title),
		TitleSuggest:  es.SuggestField{Input: es.Suffixes(post.Title), Weight: 1},
		EffectiveDate: &utils.Date{Time: post.PostedAt},
		Content:       html.UnescapeString(content),
	}

	return doc, false
}

// LoadRelationships loads any relationships needed for blog posts (none needed)
func (idx *BlogPostsIndexer) LoadRelationships(ctx context.Context, items []interface{}) error {
	// Blog posts don't have relationships to load
	return nil
}

// IndexAll indexes all blog posts
func (idx *BlogPostsIndexer) IndexAll(ctx context.Context, reset bool) error {
	startTime := time.Now()
	log.Infof("Starting blog posts indexing (reset=%v)", reset)

	// Delete existing blog posts if reset
	if reset {
		if err := idx.deleteExistingBlogPosts(ctx); err != nil {
			return errors.Wrap(err, "delete existing blog posts")
		}
	}

	// Fetch all blog posts
	scope := DefaultBlogPostsScope()
	posts, err := idx.FetchBlogPosts(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "fetch blog posts")
	}

	log.Infof("Fetched %d blog posts from MDB", len(posts))

	if len(posts) == 0 {
		log.Info("No blog posts to index")
		return nil
	}

	// Filter existing blog posts if not reset
	if !reset {
		posts, err = idx.FilterExistingBlogPosts(ctx, posts)
		if err != nil {
			return errors.Wrap(err, "filter existing blog posts")
		}

		if len(posts) == 0 {
			log.Info("✓ All blog posts already indexed")
			return nil
		}

		log.Infof("Filtered: %d new blog posts to index", len(posts))
	}

	// Load index data (not needed for blog posts)
	indexData := &es.IndexData{}

	// Convert []*mdbmodels.BlogPost to []interface{}
	items := make([]interface{}, len(posts))
	for i, p := range posts {
		items[i] = p
	}

	// Use generic pipeline for indexing
	pipeline := indexing.NewPipeline(idx.manager, nil) // nil = use default config
	if err := pipeline.RunPipeline(ctx, idx, items, indexData); err != nil {
		return errors.Wrap(err, "run indexing pipeline")
	}

	log.Infof("✓ Blog posts indexing completed in %v", time.Since(startTime))
	return nil
}

// defaultBlogPostsScope returns the default SQL scope for blog posts
func DefaultBlogPostsScope() []qm.QueryMod {
	return []qm.QueryMod{
		qm.Where("filtered = false"),
		qm.OrderBy("id"),
	}
}

// fetchBlogPosts loads blog posts from MDB with the given scope
func (idx *BlogPostsIndexer) FetchBlogPosts(ctx context.Context, scope []qm.QueryMod) ([]*mdbmodels.BlogPost, error) {
	posts, err := mdbmodels.BlogPosts(scope...).All(idx.db)
	if err != nil {
		return nil, errors.Wrap(err, "query blog_posts")
	}
	return posts, nil
}

// deleteExistingBlogPosts deletes all blog post documents from all language indices
func (idx *BlogPostsIndexer) deleteExistingBlogPosts(ctx context.Context) error {
	languages := consts.ALL_KNOWN_LANGS[:]
	log.Infof("Deleting existing blog posts from %d language indices", len(languages))

	var wg sync.WaitGroup
	errChan := make(chan error, len(languages))

	for _, lang := range languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

			// Delete by result type
			_, err := idx.manager.DeleteByResultType(ctx, indexName, consts.ES_RESULT_TYPE_BLOG_POSTS)
			if err != nil {
				errChan <- errors.Wrapf(err, "delete blog posts from %s", indexName)
				return
			}

			log.Debugf("✓ Deleted blog posts from: %s", indexName)
		}(lang)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return <-errChan
	}

	log.Info("✓ Deleted existing blog posts from all language indices")
	return nil
}

// filterExistingBlogPosts filters out blog posts that are already indexed
func (idx *BlogPostsIndexer) FilterExistingBlogPosts(ctx context.Context, posts []*mdbmodels.BlogPost) ([]*mdbmodels.BlogPost, error) {
	if len(posts) == 0 {
		return posts, nil
	}

	// Group blog posts by language
	postsByLang := make(map[string]map[string]*mdbmodels.BlogPost)
	langMapping := idx.blogIdToLanguageMapping()

	for _, post := range posts {
		lang := langMapping[int(post.BlogID)]
		if postsByLang[lang] == nil {
			postsByLang[lang] = make(map[string]*mdbmodels.BlogPost)
		}
		// Blog ID + WPID is used as the UID
		uid := fmt.Sprintf("%v-%v", post.BlogID, post.WPID)
		postsByLang[lang][uid] = post
	}

	// Check each language index
	var newPosts []*mdbmodels.BlogPost
	for lang, postsMap := range postsByLang {
		indexName := fmt.Sprintf("%s_%s", idx.indexNameBase, lang)

		// Check which UIDs already exist
		existingUIDs, err := idx.manager.GetExistingUIDs(ctx, indexName, consts.ES_RESULT_TYPE_BLOG_POSTS)
		if err != nil {
			return nil, errors.Wrapf(err, "check existing blog posts in %s", indexName)
		}

		// Add blog posts that don't exist yet
		for uid, post := range postsMap {
			if !existingUIDs[uid] {
				newPosts = append(newPosts, post)
			}
		}
	}

	return newPosts, nil
}
