package api

import (
	"net/http"
	"net/http/pprof"

	"github.com/spf13/viper"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

func SetupRoutes(router *gin.Engine) {
	router.GET("/health_check", HealthCheckHandler)

	router.GET("/collections", CollectionsHandler)
	router.POST("/collections", CollectionsHandler)
	router.GET("/collections/:uid", CollectionHandler)
	router.GET("/content_units", ContentUnitsHandler)
	router.GET("/content_units/:uid", ContentUnitHandler)
	router.GET("/lessons", LessonsHandler)
	router.POST("/lessons", LessonsHandler)
	router.GET("/sources", SourcesHierarchyHandler)
	router.GET("/tags", TagsHierarchyHandler)
	router.GET("/tags/:uid/dashboard", TagDashboardHandler)
	router.GET("/publishers", PublishersHandler)
	router.GET("/recently_updated", RecentlyUpdatedHandler)
	router.GET("/search", SearchHandler)
	router.GET("/click", ClickHandler)
	router.GET("/autocomplete", AutocompleteHandler)
	router.GET("/home", HomePageHandler)
	router.GET("/latestLesson", LatestLessonHandler)
	router.GET("/sqdata", SemiQuasiDataHandler)
	router.GET("/stats/cu_class", StatsCUClassHandler)
	router.GET("/tweets", TweetsHandler)
	router.GET("/posts", BlogPostsHandler)
	router.GET("/posts/:blog/:id", BlogPostHandler)
	router.GET("/simple", SimpleModeHandler)

	if onlineEval := viper.GetBool("test.enable-online-eval"); onlineEval {
		router.StaticFile("/eval.html", "./search/eval.html")
		router.POST("/eval/query", EvalQueryHandler)
		router.POST("/eval/set", EvalSetHandler)
		router.POST("/eval/sxs", EvalSxSHandler)
	}

	router.GET("/rss.php", FeedRssPhp)
	feeds := router.Group("/feeds")
	{
		feeds.GET("/rus_zohar", FeedRusZohar)
		feeds.GET("/rus_zohar.rss", FeedRusZohar)
		feeds.GET("/rus_for_laitman_ru", FeedRusForLaitmanRu)
		feeds.GET("/rus_for_laitman_ru.rss", FeedRusForLaitmanRu)
		feeds.GET("/wsxml.xml", FeedWSXML)
		feeds.GET("/rss_video.php", FeedRssVideo)
		feeds.GET("/podcast/:DLANG/:DF", FeedPodcast)
		feeds.GET("/podcast.rss/:DLANG/:DF", FeedPodcast)
		feeds.GET("/morning_lesson", FeedMorningLesson)

		collections := feeds.Group("/collections/:DLANG")
		{
			collections.GET("/:COLLECTION", FeedCollections)
			collections.HEAD("/:COLLECTION", FeedCollections)
			collections.GET("/:COLLECTION/df/:DF", FeedCollections)
			collections.HEAD("/:COLLECTION/df/:DF", FeedCollections)
			collections.GET("/:COLLECTION/tag/:TAG", FeedCollections)
			collections.HEAD("/:COLLECTION/tag/:TAG", FeedCollections)
			collections.GET("/:COLLECTION/df/:DF/tag/:TAG", FeedCollections)
			collections.HEAD("/:COLLECTION/df/:DF/tag/:TAG", FeedCollections)
			collections.GET("/:COLLECTION/tag/:TAG/df/:DF", FeedCollections)
			collections.HEAD("/:COLLECTION/tag/:TAG/df/:DF", FeedCollections)
		}

		ct := feeds.Group("/content_type/:DLANG/:CT")
		{
			ct.GET("/", FeedByContentType)
			ct.HEAD("/", FeedByContentType)
			ct.GET("/df/:DF", FeedByContentType)
			ct.HEAD("/df/:DF", FeedByContentType)
			ct.GET("/df/:DF/tag/:TAG", FeedByContentType)
			ct.HEAD("/df/:DF/tag/:TAG", FeedByContentType)
			ct.GET("/tag/:TAG", FeedByContentType)
			ct.HEAD("/tag/:TAG", FeedByContentType)
			ct.GET("/tag/:TAG/df/:DF", FeedByContentType)
			ct.HEAD("/tag/:TAG/df/:DF", FeedByContentType)
		}
	}

	cms := router.Group("/cms")
	{
		cms.GET("/persons/:id", CMSPerson)
		cms.GET("/banners/:id", CMSBanner)
		cms.GET("/sources/:id", CMSSource)
		cms.GET("/sourceIndex/:id", CMSSourceIndex)
		cms.GET("/topics", CMSTopics)
		cms.GET("/images/*path", CMSImage)
	}

	my := router.Group("/my", utils.AuthenticationMiddleware())
	{
		my.GET("/playlists", MyPlaylistListHandler)
		my.POST("/playlists", MyPlaylistListHandler)
		my.PATCH("/playlists/:id", MyPlaylistHandler)
		my.DELETE("/playlists/:id", MyPlaylistHandler)
		my.POST("/playlists/:id/units", MyPlaylistHandler)
		my.DELETE("/playlists/:id/units", MyPlaylistItemHandler)
		my.GET("/likes", MyLikesHandler)
		my.POST("/likes", MyLikesHandler)
		my.DELETE("/likes", MyLikesHandler)
		my.GET("/subscriptions", MySubscriptionHandler)
		my.POST("/subscriptions", MySubscriptionHandler)
		my.DELETE("/subscriptions", MySubscriptionHandler)
		my.GET("/subscriptions", MyHistoryHandler)
		my.DELETE("/subscriptions", MyHistoryHandler)
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
