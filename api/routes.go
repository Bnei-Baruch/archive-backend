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
	}

	router.GET("/feeds/rus_zohar", FeedRusZohar)
	router.GET("/feeds/rus_zohar.rss", FeedRusZohar)
	router.GET("/feeds/rus_for_laitman_ru", FeedRusForLaitmanRu)
	router.GET("/feeds/rus_for_laitman_ru.rss", FeedRusForLaitmanRu)
	router.GET("/feeds/wsxml.xml", FeedWSXML)
	router.GET("/rss.php", FeedRssPhp)
	router.GET("/feeds/rss_video.php", FeedRssVideo)
	router.GET("/feeds/podcast/:DLANG/:DF", FeedPodcast)
	router.GET("/feeds/podcast.rss/:DLANG/:DF", FeedPodcast)

	router.GET("/feeds/morning_lesson", FeedMorningLesson)

	cms := router.Group("/cms")
	cms.GET("/persons/:id", CMSPerson)
	cms.GET("/banners/:id", CMSBanner)
	cms.GET("/sources/:id", CMSSource)
	cms.GET("/topics", CMSTopics)
	cms.GET("/images/*path", CMSImage)

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
	handler := http.HandlerFunc(h)
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}
