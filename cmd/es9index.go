package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing"
	"github.com/Bnei-Baruch/archive-backend/es9/indexing/types"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/integration"
)

var es9indexCmd = &cobra.Command{
	Use:   "es9index",
	Short: "Index content to Elasticsearch 9",
	Long: `Index content from MDB (PostgreSQL) to Elasticsearch 9.

This command performs ETL (Extract, Transform, Load) of content units, collections,
sources, tags, and other entities from the MDB to ES9 indices.

Supports:
- Content units (lesson parts, lectures, etc.)
- Collections (programs, congresses, etc.)
- Sources (Kabbalah sources hierarchy)
- Tags (content categorization)
- Blog posts (articles and news)
- Tweets (Twitter/social media content)`,
	Run: func(cmd *cobra.Command, args []string) {
		contentTypes, _ := cmd.Flags().GetStringSlice("type")
		indexName := cmd.Flag("index").Value.String()
		reset, _ := cmd.Flags().GetBool("reset")

		mode := "incremental"
		if reset {
			mode = "full reset"
		}

		// Handle "all" or empty list
		allTypes := []string{"content-units", "collections", "sources", "tags", "blog-posts", "tweets"}
		if len(contentTypes) == 0 || (len(contentTypes) == 1 && contentTypes[0] == "all") {
			contentTypes = allTypes
			log.Infof("Starting ES9 indexing for all content types (mode: %s)", mode)
		} else {
			log.Infof("Starting ES9 indexing for types: %v (mode: %s)", contentTypes, mode)
		}

		// Initialize common resources WITHOUT ES6 (not needed for ES9 indexing)
		common.InitWithOptions(nil, nil, false)
		defer common.Shutdown()

		// Get ES9 URL from config
		es9URL := viper.GetString("elasticsearch9.url")
		if es9URL == "" {
			log.Fatal("elasticsearch9.url not configured")
		}

		// Create ES9 manager
		manager := es9common.MakeES9Manager(es9URL)
		defer manager.Stop()

		// Test ES9 connection
		ctx := context.Background()
		if err := manager.Ping(ctx); err != nil {
			log.Fatalf("Failed to connect to ES9: %v", err)
		}
		log.Info("✓ ES9 connection successful")

		// Setup graceful shutdown on Ctrl+C
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()

		// Handle SIGINT (Ctrl+C) and SIGTERM gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Run indexing in goroutine so we can handle signals
		errChan := make(chan error, 1)
		startTime := time.Now()

		go func() {
			errChan <- runUnifiedIndexing(ctx, manager, indexName, contentTypes, reset)
		}()

		// Wait for completion or signal
		select {
		case err := <-errChan:
			// Indexing completed
			if err != nil {
				log.Fatalf("Failed to index: %v", err)
			}
			duration := time.Since(startTime)
			log.Infof("\n=== Summary ===")
			log.Infof("Total time: %s", duration.Round(time.Second))
			log.Info("✓ All indexing completed successfully")

		case sig := <-sigChan:
			// Received interrupt signal
			log.Warnf("\n⚠ Received signal: %v", sig)
			log.Warn("⚠ Gracefully shutting down (finishing current index creation)...")
			log.Warn("⚠ Press Ctrl+C again to force quit (may leave indices in bad state!)")

			// Cancel context to stop new work
			cancel()

			// Setup force-quit handler
			forceQuitChan := make(chan os.Signal, 1)
			signal.Notify(forceQuitChan, os.Interrupt, syscall.SIGTERM)

			// Wait for graceful completion or force quit
			select {
			case err := <-errChan:
				duration := time.Since(startTime)
				if err != nil && err != context.Canceled {
					log.Errorf("⚠ Indexing interrupted with error: %v", err)
					log.Errorf("⚠ Some indices may be partially created")
					log.Errorf("⚠ Run with --reset to clean up, or delete bad indices manually")
					os.Exit(1)
				}
				log.Infof("✓ Graceful shutdown completed in %s", duration.Round(time.Second))
				log.Info("✓ All in-progress index creations finished cleanly")

			case <-forceQuitChan:
				log.Error("⚠⚠⚠ FORCE QUIT! Indices may be left in inconsistent state!")
				log.Error("⚠⚠⚠ You may need to delete partially created indices manually:")
				log.Error("⚠⚠⚠   curl -X DELETE 'localhost:9200/{index_name}'")
				os.Exit(2)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(es9indexCmd)

	es9indexCmd.Flags().StringSliceP(
		"type",
		"t",
		[]string{"all"},
		"Content types to index (content-units, collections, sources, tags, blog-posts, tweets, all). Multiple values supported. Default: all",
	)

	es9indexCmd.Flags().StringP(
		"index",
		"i",
		"results",
		"Base index name (e.g., 'results' creates results_en, results_he, etc.)",
	)

	es9indexCmd.Flags().BoolP(
		"reset",
		"r",
		false,
		"Reset mode: delete all documents of this type before reindexing (default: incremental mode - skip existing)",
	)
}

// runUnifiedIndexing runs the unified indexing pipeline for all specified types
func runUnifiedIndexing(
	ctx context.Context,
	manager *es9common.ES9Manager,
	indexNameBase string,
	contentTypes []string,
	reset bool,
) error {
	// Phase 1: Delete existing documents if reset=true
	if reset {
		log.Infof("Reset mode: deleting existing documents for types: %v", contentTypes)
		if err := deleteTypesInParallel(ctx, manager, indexNameBase, contentTypes); err != nil {
			return fmt.Errorf("delete existing documents: %w", err)
		}
		log.Info("✓ Deleted existing documents")
	}

	// Phase 2: Create indexers for all types
	log.Infof("Creating indexers for %d types", len(contentTypes))

	unzipURL := viper.GetString("elasticsearch.unzip-url")
	if unzipURL == "" {
		log.Warn("elasticsearch.unzip-url not configured - document extraction will be skipped")
	}

	indexers := make(map[string]indexing.Indexer)
	typeCounts := make(map[string]int)

	for _, contentType := range contentTypes {
		switch contentType {
		case "content-units":
			assetsService := integration.NewAssetsService(unzipURL)
			indexers[contentType] = types.NewContentUnitsIndexer(manager, common.DB, indexNameBase, assetsService)
		case "collections":
			indexers[contentType] = types.NewCollectionsIndexer(manager, common.DB, indexNameBase)
		case "sources":
			indexers[contentType] = types.NewSourcesIndexer(manager, common.DB, indexNameBase, unzipURL)
		case "tags":
			indexers[contentType] = types.NewTagsIndexer(manager, common.DB, indexNameBase)
		case "blog-posts":
			indexers[contentType] = types.NewBlogPostsIndexer(manager, common.DB, indexNameBase)
		case "tweets":
			indexers[contentType] = types.NewTweetsIndexer(manager, common.DB, indexNameBase)
		default:
			return fmt.Errorf("unknown content type: %s", contentType)
		}
	}

	// Phase 2.5: Ensure ALL indices exist with correct mapping BEFORE indexing
	// CRITICAL: Must happen before Phase 3 to prevent ES auto-creating with dynamic mapping
	log.Info("Ensuring all language indices exist with correct mapping...")

	// Use content-units indexer to create all results indices (all types share same indices)
	cuIndexer, ok := indexers["content-units"].(*types.ContentUnitsIndexer)
	if !ok && len(indexers) > 0 {
		// If content-units not in the list, use any indexer that can create indices
		// All indexers share the same index base, so any will work
		for _, idx := range indexers {
			if cuIdx, ok := idx.(*types.ContentUnitsIndexer); ok {
				cuIndexer = cuIdx
				break
			}
		}
	}

	if cuIndexer == nil {
		// Create a temporary indexer just to ensure indices exist
		assetsService := integration.NewAssetsService(unzipURL)
		cuIndexer = types.NewContentUnitsIndexer(manager, common.DB, indexNameBase, assetsService)
	}

	// This will create all 39 language indices with proper mappings
	// and validate existing indices have correct mapping
	if err := cuIndexer.EnsureIndicesExist(ctx); err != nil {
		return fmt.Errorf("ensure indices exist: %w", err)
	}
	log.Info("✓ All indices ready with correct mapping")

	// Phase 3: Fetch items AND load index data in parallel
	log.Info("Fetching items and loading index data (parallel)...")

	var fetchWg sync.WaitGroup
	allItems := make(map[string][]interface{})
	totalItems := 0
	var indexData *es.IndexData
	var indexDataErr error

	// Launch IndexData loading in parallel
	fetchWg.Add(1)
	go func() {
		defer fetchWg.Done()
		log.Info("  Loading index data (sources, tags, transcripts, media languages)...")
		indexData, indexDataErr = es.MakeIndexData(common.DB, "TRUE")
		if indexDataErr != nil {
			log.Errorf("  ✗ Failed to load index data: %v", indexDataErr)
		} else {
			log.Info("  ✓ Index data loaded")
		}
	}()

	// Launch item fetchers in parallel
	type fetchResult struct {
		contentType string
		items       []interface{}
		err         error
	}

	resultChan := make(chan fetchResult, len(indexers))

	for contentType, indexer := range indexers {
		fetchWg.Add(1)
		go func(typeName string, idx indexing.Indexer) {
			defer fetchWg.Done()
			log.Infof("  → Fetching %s...", typeName)
			items, err := fetchItemsForType(ctx, typeName, idx, reset)

			resultChan <- fetchResult{
				contentType: typeName,
				items:       items,
				err:         err,
			}
		}(contentType, indexer)
	}

	// Close result channel when all fetchers are done
	go func() {
		fetchWg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		if result.err != nil {
			return fmt.Errorf("fetch %s: %w", result.contentType, result.err)
		}
		allItems[result.contentType] = result.items
		typeCounts[result.contentType] = len(result.items)
		totalItems += len(result.items)
		log.Infof("  ✓ %s: %d items", result.contentType, len(result.items))
	}

	// Wait for index data loading to complete
	fetchWg.Wait()

	// Check for index data loading error
	if indexDataErr != nil {
		return fmt.Errorf("load index data: %w", indexDataErr)
	}

	if totalItems == 0 {
		log.Info("No items to index")
		return nil
	}

	log.Infof("Total items to index: %d\n", totalItems)

	// Phase 4: Create unified pipeline and feed items
	itemQueue := make(chan indexing.ItemTask, 10000) // Large buffer
	progress := indexing.NewProgressTracker(totalItems)

	// Set per-type totals
	for contentType, count := range typeCounts {
		progress.SetTypeTotal(contentType, count)
	}

	// Start pipeline in background
	pipeline := indexing.NewPipeline(manager, nil)

	// Set transcript stats getter if content-units indexer is available
	if cuIndexer, ok := indexers["content-units"].(*types.ContentUnitsIndexer); ok {
		pipeline.SetTranscriptStatsGetter(cuIndexer.GetTranscriptFailureStats)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- pipeline.RunUnifiedPipeline(ctx, itemQueue, indexData, progress)
	}()

	// Feed items from all types concurrently
	var feedWg sync.WaitGroup
	for contentType, items := range allItems {
		if len(items) == 0 {
			continue
		}

		feedWg.Add(1)
		go func(typeName string, typeItems []interface{}, typeIndexer indexing.Indexer) {
			defer feedWg.Done()

			log.Infof("Feeding %s items to pipeline...", typeName)
			for _, item := range typeItems {
				select {
				case itemQueue <- indexing.ItemTask{
					Item:    item,
					Indexer: typeIndexer,
					Type:    typeName,
				}:
				case <-ctx.Done():
					log.Warnf("%s feeder: context cancelled", typeName)
					return
				}
			}
			log.Infof("✓ %s: all items fed to pipeline", typeName)
		}(contentType, items, indexers[contentType])
	}

	// Wait for all feeders to complete, then close the queue
	go func() {
		feedWg.Wait()
		close(itemQueue)
		log.Info("✓ All items fed to pipeline")
	}()

	// Wait for pipeline to complete
	if err := <-errChan; err != nil {
		return fmt.Errorf("unified pipeline: %w", err)
	}

	// Log transcript failure statistics for content units
	if cuIndexer, ok := indexers["content-units"].(*types.ContentUnitsIndexer); ok {
		cuIndexer.LogTranscriptFailureStats()
	}

	return nil
}

// deleteTypesInParallel deletes documents for all specified types in parallel
func deleteTypesInParallel(
	ctx context.Context,
	manager *es9common.ES9Manager,
	indexNameBase string,
	contentTypes []string,
) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(contentTypes)*len(consts.ALL_KNOWN_LANGS))

	resultTypes := make(map[string]string)
	resultTypes["content-units"] = consts.ES_RESULT_TYPE_UNITS
	resultTypes["collections"] = consts.ES_RESULT_TYPE_COLLECTIONS
	resultTypes["sources"] = consts.ES_RESULT_TYPE_SOURCES
	resultTypes["tags"] = consts.ES_RESULT_TYPE_TAGS
	resultTypes["blog-posts"] = consts.ES_RESULT_TYPE_BLOG_POSTS
	resultTypes["tweets"] = consts.ES_RESULT_TYPE_TWEETS

	client, err := manager.GetClient()
	if err != nil {
		return fmt.Errorf("get ES9 client: %w", err)
	}

	// Deletion statistics tracking
	type deleteStats struct {
		typeName      string
		langStats     map[string]int // lang -> deleted count
		totalDeleted  int
		skippedLangs  int
	}

	statsChan := make(chan *deleteStats, len(contentTypes))

	for _, contentType := range contentTypes {
		resultType, ok := resultTypes[contentType]
		if !ok {
			log.Warnf("Unknown content type for deletion: %s", contentType)
			continue
		}

		wg.Add(1)
		go func(typeName, resType string) {
			defer wg.Done()

			languages := consts.ALL_KNOWN_LANGS[:]
			stats := &deleteStats{
				typeName:  typeName,
				langStats: make(map[string]int),
			}

			for _, lang := range languages {
				indexName := fmt.Sprintf("%s_%s", indexNameBase, lang)

				// Check if index exists first
				resp, err := client.Indices.Exists(
					[]string{indexName},
					client.Indices.Exists.WithContext(ctx),
				)
				if err != nil {
					errChan <- fmt.Errorf("check if %s exists: %w", indexName, err)
					return
				}
				resp.Body.Close()

				if resp.StatusCode == 404 {
					// Index doesn't exist, skip
					stats.skippedLangs++
					continue
				}

				// Index exists, delete documents
				deletedCount, err := manager.DeleteByResultType(ctx, indexName, resType)
				if err != nil {
					errChan <- fmt.Errorf("delete %s from %s: %w", typeName, indexName, err)
					return
				}

				if deletedCount > 0 {
					stats.langStats[lang] = deletedCount
					stats.totalDeleted += deletedCount
				}
			}

			statsChan <- stats
		}(contentType, resultType)
	}

	wg.Wait()
	close(errChan)
	close(statsChan)

	// Check for errors
	if len(errChan) > 0 {
		return <-errChan
	}

	// Display deletion statistics
	log.Info("\n=== Deletion Summary ===")
	grandTotal := 0
	for stats := range statsChan {
		if stats.totalDeleted == 0 && stats.skippedLangs > 0 {
			log.Infof("  ⊘ %s: skipped (%d indices don't exist yet)", stats.typeName, stats.skippedLangs)
			continue
		}

		if stats.totalDeleted > 0 {
			log.Infof("  ✓ %s: deleted %d documents", stats.typeName, stats.totalDeleted)

			// Show per-language breakdown (only languages with deletions)
			for _, lang := range consts.ALL_KNOWN_LANGS {
				if count, ok := stats.langStats[lang]; ok && count > 0 {
					log.Infof("      %s: %d docs", lang, count)
				}
			}

			grandTotal += stats.totalDeleted
		}

		if stats.skippedLangs > 0 {
			log.Infof("      (skipped %d languages - indices don't exist)", stats.skippedLangs)
		}
	}

	if grandTotal > 0 {
		log.Infof("\nTotal deleted: %d documents across all types", grandTotal)
	}

	return nil
}

// fetchItemsForType fetches and filters items for a specific content type
func fetchItemsForType(
	ctx context.Context,
	contentType string,
	indexer indexing.Indexer,
	reset bool,
) ([]interface{}, error) {
	switch contentType {
	case "content-units":
		return fetchContentUnits(ctx, indexer, reset)
	case "collections":
		return fetchCollections(ctx, indexer, reset)
	case "sources":
		return fetchSources(ctx, indexer, reset)
	case "tags":
		return fetchTags(ctx, indexer, reset)
	case "blog-posts":
		return fetchBlogPosts(ctx, indexer, reset)
	case "tweets":
		return fetchTweets(ctx, indexer, reset)
	default:
		return nil, fmt.Errorf("unknown content type: %s", contentType)
	}
}

// Fetch functions for each type
func fetchContentUnits(ctx context.Context, indexer indexing.Indexer, reset bool) ([]interface{}, error) {
	cuIndexer := indexer.(*types.ContentUnitsIndexer)
	scope := types.DefaultContentUnitScope()
	cus, err := cuIndexer.FetchContentUnits(ctx, scope)
	if err != nil {
		return nil, err
	}

	if !reset {
		cus, err = cuIndexer.FilterExistingUnits(ctx, cus)
		if err != nil {
			return nil, err
		}
	}

	items := make([]interface{}, len(cus))
	for i, cu := range cus {
		items[i] = cu
	}
	return items, nil
}

func fetchCollections(ctx context.Context, indexer indexing.Indexer, reset bool) ([]interface{}, error) {
	colIndexer := indexer.(*types.CollectionsIndexer)
	scope := types.DefaultCollectionsScope()
	cols, err := colIndexer.FetchCollections(ctx, scope)
	if err != nil {
		return nil, err
	}

	if !reset {
		cols, err = colIndexer.FilterExistingCollections(ctx, cols)
		if err != nil {
			return nil, err
		}
	}

	items := make([]interface{}, len(cols))
	for i, col := range cols {
		items[i] = col
	}
	return items, nil
}

func fetchSources(ctx context.Context, indexer indexing.Indexer, reset bool) ([]interface{}, error) {
	srcIndexer := indexer.(*types.SourcesIndexer)
	scope := types.DefaultSourcesScope()
	sources, err := srcIndexer.FetchSources(ctx, scope)
	if err != nil {
		return nil, err
	}

	if !reset {
		sources, err = srcIndexer.FilterExistingSources(ctx, sources)
		if err != nil {
			return nil, err
		}
	}

	items := make([]interface{}, len(sources))
	for i, src := range sources {
		items[i] = src
	}
	return items, nil
}

func fetchTags(ctx context.Context, indexer indexing.Indexer, reset bool) ([]interface{}, error) {
	tagIndexer := indexer.(*types.TagsIndexer)
	scope := types.DefaultTagsScope()
	tags, err := tagIndexer.FetchTags(ctx, scope)
	if err != nil {
		return nil, err
	}

	if !reset {
		tags, err = tagIndexer.FilterExistingTags(ctx, tags)
		if err != nil {
			return nil, err
		}
	}

	items := make([]interface{}, len(tags))
	for i, tag := range tags {
		items[i] = tag
	}
	return items, nil
}

func fetchBlogPosts(ctx context.Context, indexer indexing.Indexer, reset bool) ([]interface{}, error) {
	blogIndexer := indexer.(*types.BlogPostsIndexer)
	scope := types.DefaultBlogPostsScope()
	posts, err := blogIndexer.FetchBlogPosts(ctx, scope)
	if err != nil {
		return nil, err
	}

	if !reset {
		posts, err = blogIndexer.FilterExistingBlogPosts(ctx, posts)
		if err != nil {
			return nil, err
		}
	}

	items := make([]interface{}, len(posts))
	for i, post := range posts {
		items[i] = post
	}
	return items, nil
}

func fetchTweets(ctx context.Context, indexer indexing.Indexer, reset bool) ([]interface{}, error) {
	tweetIndexer := indexer.(*types.TweetsIndexer)
	scope := types.DefaultTweetsScope()
	tweets, err := tweetIndexer.FetchTweets(ctx, scope)
	if err != nil {
		return nil, err
	}

	if !reset {
		tweets, err = tweetIndexer.FilterExistingTweets(ctx, tweets)
		if err != nil {
			return nil, err
		}
	}

	items := make([]interface{}, len(tweets))
	for i, tweet := range tweets {
		items[i] = tweet
	}
	return items, nil
}

// indexContentUnits indexes all content units to ES9
func indexContentUnits(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing content units to ES9")

	// Get assets service URL from config
	unzipURL := viper.GetString("elasticsearch.unzip-url")
	if unzipURL == "" {
		log.Warn("elasticsearch.unzip-url not configured - transcript extraction will be skipped")
	}

	// Create assets service for DOCX to text conversion
	assetsService := integration.NewAssetsService(unzipURL)

	// Create indexer with assets service
	indexer := types.NewContentUnitsIndexer(manager, common.DB, indexNameBase, assetsService)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all content units: %w", err)
	}

	log.Info("✓ Content units indexed successfully")
	return nil
}

// indexCollections indexes all collections to ES9
func indexCollections(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing collections to ES9")

	// Create indexer
	indexer := types.NewCollectionsIndexer(manager, common.DB, indexNameBase)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all collections: %w", err)
	}

	log.Info("✓ Collections indexed successfully")
	return nil
}

// indexSources indexes all sources to ES9
func indexSources(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing sources to ES9")

	// Get assets service URL from config
	unzipURL := viper.GetString("elasticsearch.unzip-url")
	if unzipURL == "" {
		log.Warn("elasticsearch.unzip-url not configured - document content extraction will be skipped")
	}

	// Create indexer
	indexer := types.NewSourcesIndexer(manager, common.DB, indexNameBase, unzipURL)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all sources: %w", err)
	}

	log.Info("✓ Sources indexed successfully")
	return nil
}

// indexTags indexes all tags to ES9
func indexTags(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing tags to ES9")

	// Create indexer
	indexer := types.NewTagsIndexer(manager, common.DB, indexNameBase)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all tags: %w", err)
	}

	log.Info("✓ Tags indexed successfully")
	return nil
}

// indexBlogPosts indexes all blog posts to ES9
func indexBlogPosts(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing blog posts to ES9")

	// Create indexer
	indexer := types.NewBlogPostsIndexer(manager, common.DB, indexNameBase)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all blog posts: %w", err)
	}

	log.Info("✓ Blog posts indexed successfully")
	return nil
}

// indexTweets indexes all tweets to ES9
func indexTweets(ctx context.Context, manager *es9common.ES9Manager, indexNameBase string, reset bool) error {
	log.Info("Indexing tweets to ES9")

	// Create indexer
	indexer := types.NewTweetsIndexer(manager, common.DB, indexNameBase)

	if err := indexer.IndexAll(ctx, reset); err != nil {
		return fmt.Errorf("index all tweets: %w", err)
	}

	log.Info("✓ Tweets indexed successfully")
	return nil
}
