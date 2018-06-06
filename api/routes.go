package api

import (
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

	router.GET("/feeds/rus_zohar.rss", FeedRusZohar)
	router.GET("/feeds/rus_for_laitman_ru.rss", FeedRusForLaitmanRu)
	router.GET("/feeds/morning_lesson.rss", FeedMorningLesson)
	router.GET("/feeds/rss.php", FeedRssPhp)
	router.GET("/feeds/podcast.rss", FeedPodcast)
	router.GET("/feeds/wsxml", FeedWSXML)

	router.GET("/_recover", func(c *gin.Context) {
		panic("test recover")
	})
}
