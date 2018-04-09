package api

import (
	"gopkg.in/gin-gonic/gin.v1"
)

func SetupRoutes(router *gin.Engine) {
	router.GET("/collections", CollectionsHandler)
	router.POST("/collections", CollectionsHandler)
	router.GET("/collections/:uid", CollectionHandler)
	router.GET("/content_units", ContentUnitsHandler)
	router.GET("/content_units/:uid", ContentUnitHandler)
	router.GET("/lessons", LessonsHandler)
	router.POST("/lessons", LessonsHandler)
	router.GET("/sources", SourcesHierarchyHandler)
	router.GET("/tags", TagsHierarchyHandler)
	router.GET("/publishers", PublishersHandler)
	router.GET("/recently_updated", RecentlyUpdatedHandler)
	router.GET("/search", SearchHandler)
	router.GET("/click", ClickHandler)
	router.GET("/autocomplete", AutocompleteHandler)
	router.GET("/home", HomePageHandler)
	router.GET("/latestLesson", LatestLessonHandler)
	router.GET("/sqdata", SemiQuasiDataHandler)

	router.GET("/_recover", func(c *gin.Context) {
		panic("test recover")
	})
}
