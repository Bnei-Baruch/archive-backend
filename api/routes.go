package api

import (
	"net/http"
	"net/http/pprof"

	"github.com/spf13/viper"
	"gopkg.in/gin-gonic/gin.v1"
)

func SetupRoutes(router *gin.Engine) {
	router.GET("/health_check", HealthCheckHandler)

	router.GET("/collections", CollectionsHandler)
	router.POST("/collections", CollectionsHandler)
	router.GET("/collections/:uid", CollectionHandler)
	mobileApi := router.Group("/mobile")
	{
		mobileApi.GET("/lessons", LessonOverviewHandler)
		mobileApi.GET("/programs", MobileProgramsPageHandler)
		mobileApi.GET("/search", MobileSearchHandler)
	}

	router.GET("/content_units", ContentUnitsHandler)
	router.GET("/content_units/:uid", ContentUnitHandler)
	router.GET("/lessons", LessonsHandler)
	router.POST("/lessons", LessonsHandler)
	router.GET("/events", EventsHandler)
	router.GET("/sources", SourcesHierarchyHandler)
	router.GET("/tags", TagsHierarchyHandler)
	router.GET("/tags/dashboard", TagDashboardHandler)
	router.GET("/publishers", PublishersHandler)
	router.GET("/recently_updated", RecentlyUpdatedHandler)
	router.GET("/search", SearchHandler)
	router.GET("/stats/search_class", SearchStatsHandler)
	router.GET("/autocomplete", AutocompleteHandler)
	router.GET("/home", HomePageHandler)
	router.GET("/latestLesson", LatestLessonHandler)
	router.GET("/sqdata", SemiQuasiDataHandler)
	router.GET("/stats/cu_class", StatsCUClassHandler)
	router.GET("/stats/label_class", StatsLabelClassHandler)
	router.GET("/stats/c_class", StatsCClassHandler)
	router.GET("/tweets", TweetsHandler)
	router.GET("/posts", BlogPostsHandler)
	router.GET("/posts/:blog/:id", BlogPostHandler)
	router.GET("/simple", SimpleModeHandler)
	router.GET("/labels", LabelHandler)

	if onlineEval := viper.GetBool("test.enable-online-eval"); onlineEval {
		router.StaticFile("/eval.html", "./search/eval.html")
		router.POST("/eval/query", EvalQueryHandler)
		router.POST("/eval/set", EvalSetHandler)
		router.POST("/eval/sxs", EvalSxSHandler)
	}

	router.GET("/rss.php", FeedRssPhp)
	feeds := router.Group("/feeds")
	{
		headAndGet(feeds, "/rus_zohar", FeedRusZohar)
		headAndGet(feeds, "/rus_zohar.rss", FeedRusZohar)
		headAndGet(feeds, "/rus_for_laitman_ru", FeedRusForLaitmanRu)
		headAndGet(feeds, "/rus_for_laitman_ru.rss", FeedRusForLaitmanRu)
		headAndGet(feeds, "/wsxml.xml", FeedWSXML)
		headAndGet(feeds, "/rss_video.php", FeedRssVideo)
		headAndGet(feeds, "/podcast/:DLANG/:DF", FeedPodcast)
		headAndGet(feeds, "/podcast.rss/:DLANG/:DF", FeedPodcast)
		headAndGet(feeds, "/morning_lesson", FeedMorningLesson)

		collections := feeds.Group("/collections/:DLANG")
		{
			headAndGet(collections, "/:COLLECTION", FeedCollections)
			headAndGet(collections, "/:COLLECTION/df/:DF", FeedCollections)
			headAndGet(collections, "/:COLLECTION/tag/:TAG", FeedCollections)
			headAndGet(collections, "/:COLLECTION/df/:DF/tag/:TAG", FeedCollections)
			headAndGet(collections, "/:COLLECTION/tag/:TAG/df/:DF", FeedCollections)
		}

		ct := feeds.Group("/content_type/:DLANG/:CT")
		{
			headAndGet(ct, "/", FeedByContentType)
			headAndGet(ct, "/df/:DF", FeedByContentType)
			headAndGet(ct, "/df/:DF/tag/:TAG", FeedByContentType)
			headAndGet(ct, "/tag/:TAG", FeedByContentType)
			headAndGet(ct, "/tag/:TAG/df/:DF", FeedByContentType)
		}
	}

	cms := router.Group("/cms")
	{
		cms.GET("/persons/:id", CMSPerson)
		cms.GET("/about", CMSAbout)
		cms.GET("/banners/:id", CMSBanner)
		cms.GET("/banners-list", CMSBanners)
		cms.GET("/sources/:id", CMSSource)
		cms.GET("/sourceIndex/:id", CMSSourceIndex)
		cms.GET("/topics", CMSTopics)
		cms.GET("/images/*path", CMSImage)
	}

	//router.GET("/_recover", func(c *gin.Context) {
	//	panic("test recover")
	//})

	if pass := viper.GetString("server.http-pprof-pass"); pass != "" {
		pRouter := router.Group("debug/pprof", gin.BasicAuth(gin.Accounts{"debug": pass}))
		pRouter.GET("/", pprofHandler(pprof.Index))
		pRouter.GET("/cmdline", pprofHandler(pprof.Cmdline))
		pRouter.GET("/profile", pprofHandler(pprof.Profile))
		pRouter.POST("/symbol", pprofHandler(pprof.Symbol))
		pRouter.GET("/symbol", pprofHandler(pprof.Symbol))
		pRouter.GET("/trace", pprofHandler(pprof.Trace))
		pRouter.GET("/block", pprofHandler(pprof.Handler("block").ServeHTTP))
		pRouter.GET("/goroutine", pprofHandler(pprof.Handler("goroutine").ServeHTTP))
		pRouter.GET("/heap", pprofHandler(pprof.Handler("heap").ServeHTTP))
		pRouter.GET("/mutex", pprofHandler(pprof.Handler("mutex").ServeHTTP))
		pRouter.GET("/threadcreate", pprofHandler(pprof.Handler("threadcreate").ServeHTTP))
	}
}

func pprofHandler(h http.HandlerFunc) gin.HandlerFunc {
	handler := h
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func headAndGet(group *gin.RouterGroup, relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return group.GET(relativePath, handlers...).
		HEAD(relativePath, handlers...)
}
